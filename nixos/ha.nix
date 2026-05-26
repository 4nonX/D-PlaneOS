{ config, lib, pkgs, ... }:

let
  cfg = config.services.dplaneos.ha;
in {
  options.services.dplaneos.ha = {
    enable = lib.mkEnableOption "DPlaneOS High Availability (Patroni + HAProxy)";

    role = lib.mkOption {
      type = lib.types.enum [ "primary" "secondary" ];
      default = "primary";
      description = "Initial HA role of this node.";
    };

    localAddress = lib.mkOption {
      type = lib.types.str;
      description = "IP address of this node.";
    };

    peerAddress = lib.mkOption {
      type = lib.types.str;
      description = "IP address of the peer (the other database node).";
    };

    fencing = {
      enable = lib.mkEnableOption "IPMI/Redfish STONITH fencing for automatic failover";
      bmcIP = lib.mkOption {
        type = lib.types.nullOr lib.types.str;
        default = null;
        description = "IP address of the peer node's BMC.";
      };
      bmcUser = lib.mkOption {
        type = lib.types.nullOr lib.types.str;
        default = null;
        description = "Username for the peer node's BMC.";
      };
      bmcPasswordFile = lib.mkOption {
        type = lib.types.nullOr lib.types.str;
        default = null;
        description = "Path to a file containing the peer node's BMC password.";
      };
    };

    witnessAddress = lib.mkOption {
      type = lib.types.str;
      description = "IP address of the etcd witness node.";
    };

    etcdEndpoints = lib.mkOption {
      type = lib.types.listOf lib.types.str;
      default = [ "http://127.0.0.1:2379" ];
      description = "List of etcd endpoints for the Patroni DCS.";
    };

    virtualIP = lib.mkOption {
      type = lib.types.nullOr lib.types.str;
      default = null;
      description = "The floating Virtual IP (VIP) managed by Keepalived.";
    };

    interface = lib.mkOption {
      type = lib.types.nullOr lib.types.str;
      default = null;
      description = "The network interface to bind the Keepalived VIP to.";
    };

    vrrpPassword = lib.mkOption {
      type = lib.types.nullOr lib.types.str;
      default = null;
      description = "Password for Keepalived VRRP authentication between peers.";
    };

    sbd = {
      pool = lib.mkOption {
        type    = lib.types.str;
        default = "";
        description = ''
          ZFS pool holding the SBD lease dataset. Leave empty to disable
          ZFS-property SBD fencing (single-node safe; zero overhead).
          When non-empty, a one-shot service dplaneos-sbd-init creates
          pool/dataset at first boot if it does not already exist.
        '';
      };
      dataset = lib.mkOption {
        type    = lib.types.str;
        default = "sbd-lease";
        description = "Dataset name within ha.sbd.pool used for lease fencing (e.g. sbd-lease).";
      };
    };
  };

  config = lib.mkIf cfg.enable {
    # ─── ZFS hostId ───────────────────────────────────────────────────────
    # module.nix sets boot.supportedFilesystems = ["zfs"], which triggers the
    # NixOS assertion that networking.hostId must be set. HA nodes are already
    # uniquely identified by their local IP, so derive a stable 8-char hex id
    # from it. Operators can override with: networking.hostId = lib.mkForce "...";
    networking.hostId = lib.mkDefault (
      builtins.substring 0 8 (builtins.hashString "md5" cfg.localAddress)
    );

    # ─── HA Firewall ─────────────────────────────────────────────────────
    # The base module.nix opens only 80 and 443. HA requires additional ports
    # between cluster members. Without these, etcd cannot form a cluster,
    # Patroni cannot check peer health via HAProxy, and streaming replication
    # cannot connect.
    networking.firewall.allowedTCPPorts = [
      2379  # etcd client API
      2380  # etcd peer (raft) communication
      5432  # PostgreSQL - streaming replication between Patroni nodes
      8008  # Patroni REST API - HAProxy health checks against both nodes
    ];

    # ─── Etcd ─────────────────────────────────────────────────────────────
    # Standard 3-node etcd cluster for reliable DCS and Patroni leader election
    services.etcd = {
      enable = true;
      name = "etcd-${cfg.localAddress}";
      listenClientUrls = [ "http://0.0.0.0:2379" ];
      listenPeerUrls = [ "http://0.0.0.0:2380" ];
      advertiseClientUrls = [ "http://${cfg.localAddress}:2379" ];
      initialAdvertisePeerUrls = [ "http://${cfg.localAddress}:2380" ];
      initialCluster = [
        "etcd-${cfg.localAddress}=http://${cfg.localAddress}:2380"
        "etcd-${cfg.peerAddress}=http://${cfg.peerAddress}:2380"
        "etcd-witness=http://${cfg.witnessAddress}:2380"
      ];
      initialClusterState = "new";
    };

    # ─── Patroni config auto-generation ──────────────────────────────────
    # Generates /etc/dplaneos/patroni.yaml on first boot if not present.
    # /etc/dplaneos is bind-mounted from /persist via impermanence.nix so the
    # file survives OTA slot swaps. Random replication and superuser passwords
    # are generated once and stay stable for the lifetime of the cluster.
    systemd.services.dplaneos-patroni-init = {
      description = "DPlaneOS Patroni Config Init";
      after       = [ "local-fs.target" ];
      before      = [ "patroni.service" ];
      wantedBy    = [ "multi-user.target" ];
      serviceConfig = {
        Type            = "oneshot";
        RemainAfterExit = true;
        ExecStart       = pkgs.writeShellScript "patroni-init" ''
          set -eu
          CONFIG="/etc/dplaneos/patroni.yaml"
          if [ -f "$CONFIG" ]; then
            echo "patroni-init: $CONFIG already exists, nothing to do"
            exit 0
          fi
          echo "patroni-init: first boot - generating $CONFIG"
          REPL_PASS=$(tr -dc 'A-Za-z0-9' < /dev/urandom | head -c 32)
          SUPER_PASS=$(tr -dc 'A-Za-z0-9' < /dev/urandom | head -c 32)
          mkdir -p /etc/dplaneos
          cat > "$CONFIG" <<EOF
scope: dplaneos
name: dplaneos-${cfg.localAddress}

restapi:
  listen: 0.0.0.0:8008
  connect_address: ${cfg.localAddress}:8008

etcd3:
  hosts: ${cfg.localAddress}:2379,${cfg.peerAddress}:2379,${cfg.witnessAddress}:2379

bootstrap:
  dcs:
    ttl: 30
    loop_wait: 10
    retry_timeout: 10
    maximum_lag_on_failover: 1048576
  initdb:
    - encoding: UTF8
    - data-checksums
  pg_hba:
    - host replication replicator ${cfg.peerAddress}/32 md5
    - host replication replicator 127.0.0.1/32 md5
    - host all all 127.0.0.1/32 trust

postgresql:
  listen: 0.0.0.0:5432
  connect_address: ${cfg.localAddress}:5432
  data_dir: /var/lib/dplaneos/pgsql
  bin_dir: ${pkgs.postgresql_15}/bin
  parameters:
    max_connections: 200
    shared_buffers: 256MB
    wal_level: replica
    max_wal_senders: 5
    max_replication_slots: 5
  authentication:
    replication:
      username: replicator
      password: $REPL_PASS
    superuser:
      username: postgres
      password: $SUPER_PASS

tags:
  nofailover: false
  noloadbalance: false
  clonefrom: false
  nosync: false
EOF
          chmod 0600 "$CONFIG"
          chown postgres:postgres "$CONFIG"
          echo "patroni-init: config written to $CONFIG"
        '';
      };
    };

    # ─── Patroni ──────────────────────────────────────────────────────────
    systemd.services.patroni = {
      description = "Patroni High Availability PostgreSQL";
      after = [ "network.target" "etcd.service" "dplaneos-patroni-init.service" ];
      requires = [ "etcd.service" "dplaneos-patroni-init.service" ];
      wantedBy = [ "multi-user.target" ];
      environment = {
        PATH = lib.mkForce "${pkgs.patroni}/bin:${pkgs.postgresql_15}/bin:${pkgs.coreutils}/bin";
      };
      serviceConfig = {
        Type       = "simple";
        User       = "postgres";
        Group      = "postgres";
        ExecStart  = "${pkgs.patroni}/bin/patroni /etc/dplaneos/patroni.yaml";
        Restart    = "always";
        RestartSec = "5s";
      };
    };

    # Create the postgres user required by Patroni
    users.users.postgres = {
      isSystemUser = true;
      group = "postgres";
      extraGroups = [ "dplaneos" ];
    };
    users.groups.postgres = {};

    systemd.tmpfiles.rules = [
      "d /var/lib/dplaneos/pgsql 0700 postgres postgres -"
    ];

    # ─── HAProxy ──────────────────────────────────────────────────────────
    # Transparently routes traffic to the PostgreSQL primary.
    # Connects to Patroni's REST API (:8008) to discover the primary mode.
    services.haproxy = {
      enable = true;
      config = ''
        global
            maxconn 1000
            log /dev/log local0
            user haproxy
            group haproxy

        defaults
            log global
            mode tcp
            retries 3
            timeout client 30m
            timeout connect 4s
            timeout server 30m
            timeout check 5s

        listen postgresql
            bind 127.0.0.1:5000
            option httpchk GET /primary
            http-check expect status 200
            default-server inter 3s fall 3 rise 2 on-marked-down shutdown-sessions
            server postgresLocal  ${cfg.localAddress}:5432 maxconn 500 check port 8008
            server postgresPeer   ${cfg.peerAddress}:5432  maxconn 500 check port 8008
      '';
    };

    # ─── Keepalived ───────────────────────────────────────────────────────
    # VRRP handles floating the VIP to the healthy primary node.
    # Health checks ping the DPlaneOS daemon directly.
    services.keepalived = lib.mkIf (cfg.virtualIP != null && cfg.interface != null) {
      enable = true;
      vrrpScripts.check_dplaneos = {
        script = "${pkgs.curl}/bin/curl -sf http://127.0.0.1:9000/health";
        interval = 2;
        fall = 3;
        rise = 2;
      };
      vrrpInstances.dplaneos_vip = {
        interface = cfg.interface;
        state = if cfg.role == "primary" then "MASTER" else "BACKUP";
        priority = if cfg.role == "primary" then 100 else 90;
        virtualRouterId = 51;
        virtualIps = [ { addr = cfg.virtualIP; } ];
        trackScripts = [ "check_dplaneos" ];
        extraConfig = lib.optionalString (cfg.vrrpPassword != null) ''
          authentication {
            auth_type PASS
            auth_pass ${cfg.vrrpPassword}
          }
        '';
      };
    };

    # ─── Systemd Overrides for Patroni HA Split-Brain Protection ─────────
    # We must restrict the native NixOS ZFS import units to only execute if
    # this node is actually the Patroni Primary. If the node boots into Standby,
    # the datasets remain physically unmounted to avert split-brain data corruption.
    systemd.services.zfs-import-cache = lib.mkIf cfg.fencing.enable {
      after = [ "patroni.service" ];
      requires = [ "patroni.service" ];
      preStart = ''
        echo "HA GUARD: Waiting for Patroni leadership determination..."
        deadline=120
        elapsed=0
        while [ $elapsed -lt $deadline ]; do
          status=$(${pkgs.curl}/bin/curl -s -o /dev/null -w "%{http_code}" --max-time 3 http://localhost:8008/primary || echo "000")
          if [ "$status" = "200" ]; then
            echo "HA GUARD: Node is Primary. Proceeding to mount ZFS volumes over Native Cache."
            exit 0
          elif [ "$status" = "503" ]; then
            echo "HA GUARD: Node is Standby. Aborting native ZFS mounts."
            exit 1
          fi
          sleep 2
          elapsed=$((elapsed + 2))
        done
        echo "HA GUARD: Timed out after $deadline s waiting for Patroni. Aborting ZFS mount (fail-safe)."
        exit 1
      '';
    };

    systemd.services.zfs-import-scan = lib.mkIf cfg.fencing.enable {
      after = [ "patroni.service" ];
      requires = [ "patroni.service" ];
      preStart = ''
        echo "HA GUARD: Waiting for Patroni leadership determination..."
        deadline=120
        elapsed=0
        while [ $elapsed -lt $deadline ]; do
          status=$(${pkgs.curl}/bin/curl -s -o /dev/null -w "%{http_code}" --max-time 3 http://localhost:8008/primary || echo "000")
          if [ "$status" = "200" ]; then
            echo "HA GUARD: Node is Primary. Proceeding to mount ZFS volumes over Native Scan."
            exit 0
          elif [ "$status" = "503" ]; then
            echo "HA GUARD: Node is Standby. Aborting native ZFS mounts."
            exit 1
          fi
          sleep 2
          elapsed=$((elapsed + 2))
        done
        echo "HA GUARD: Timed out after $deadline s waiting for Patroni. Aborting ZFS mount (fail-safe)."
        exit 1
      '';
    };

    # ─── SBD lease dataset init ──────────────────────────────────────────
    # Creates the ZFS dataset used for ZFS-property SBD fencing if it does
    # not already exist. Only runs when ha.sbd.pool is non-empty.
    # The daemon manages the actual lease property writes at runtime.
    systemd.services.dplaneos-sbd-init = lib.mkIf (cfg.sbd.pool != "") {
      description = "DPlaneOS SBD Lease Dataset Init";
      after       = [ "zfs.target" "dplaneos-zfs-gate.service" ];
      before      = [ "dplaned.service" ];
      wantedBy    = [ "multi-user.target" ];
      serviceConfig = {
        Type            = "oneshot";
        RemainAfterExit = true;
        ExecStart       = pkgs.writeShellScript "dplaneos-sbd-init" ''
          set -eu
          DATASET="${cfg.sbd.pool}/${cfg.sbd.dataset}"
          if zfs list -H "$DATASET" > /dev/null 2>&1; then
            echo "dplaneos-sbd-init: dataset $DATASET already exists"
          else
            echo "dplaneos-sbd-init: creating $DATASET"
            zfs create -o mountpoint=none "$DATASET"
          fi
        '';
      };
    };
  };
}
