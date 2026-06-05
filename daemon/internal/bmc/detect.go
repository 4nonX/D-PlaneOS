package bmc

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// probeRedfish attempts to connect to the Redfish service root using a
// capture-mode client (TOFU first contact). Returns Info on success.
func probeRedfish(ctx context.Context, creds Credentials) (Info, error) {
	fpCh := make(chan string, 1)
	client := httpClientCapture(fpCh, 8)

	url := fmt.Sprintf("https://%s/redfish/v1/", creds.Host)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Info{}, err
	}
	req.SetBasicAuth(creds.Username, creds.Password)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return Info{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return Info{}, fmt.Errorf("auth failed")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Info{}, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var root struct {
		RedfishVersion string `json:"RedfishVersion"`
		Vendor         string `json:"Vendor"`
		Product        string `json:"Product"`
		Oem            struct {
			Hpe struct {
				Manager []struct {
					ManagerType     string `json:"ManagerType"`
					FirmwareVersion string `json:"FirmwareVersion"`
				} `json:"Manager"`
			} `json:"Hpe"`
		} `json:"Oem"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&root); err != nil {
		return Info{Protocol: ProtocolRedfish, Hostname: creds.Host, ReachableAt: time.Now()}, nil
	}

	info := Info{Protocol: ProtocolRedfish, Hostname: creds.Host, ReachableAt: time.Now()}
	product := strings.ToLower(root.Product + root.Vendor)
	switch {
	case strings.Contains(product, "ilo") || len(root.Oem.Hpe.Manager) > 0:
		info.Vendor = "HPE"
		if len(root.Oem.Hpe.Manager) > 0 {
			info.Model = root.Oem.Hpe.Manager[0].ManagerType
			info.FirmwareVersion = root.Oem.Hpe.Manager[0].FirmwareVersion
		}
	case strings.Contains(product, "idrac") || strings.Contains(product, "dell"):
		info.Vendor = "Dell"
		info.Model = "iDRAC"
	case strings.Contains(product, "lenovo") || strings.Contains(product, "xcc"):
		info.Vendor = "Lenovo"
		info.Model = "XCC"
	default:
		info.Vendor = "Generic"
		info.Model = "Redfish " + root.RedfishVersion
	}
	return info, nil
}

// probeILO4 attempts to connect to the iLO 4 legacy REST API.
// iLO 4 uses /rest/v1/ instead of /redfish/v1/ and a "Type" field
// starting with "ServiceRoot." to identify itself.
func probeILO4(ctx context.Context, creds Credentials) (Info, error) {
	// iLO 4 uses a self-signed cert; use capture client since it's legacy HW
	fpCh := make(chan string, 1)
	client := httpClientCapture(fpCh, 8)

	url := fmt.Sprintf("https://%s/rest/v1/", creds.Host)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Info{}, err
	}
	req.SetBasicAuth(creds.Username, creds.Password)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return Info{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Info{}, fmt.Errorf("not iLO 4")
	}

	var root struct {
		Type string `json:"Type"`
		Oem  struct {
			Hp struct {
				Manager []struct {
					ManagerFirmwareVersion string `json:"ManagerFirmwareVersion"`
				} `json:"Manager"`
			} `json:"Hp"`
		} `json:"Oem"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&root); err != nil || !strings.HasPrefix(root.Type, "ServiceRoot.") {
		return Info{}, fmt.Errorf("not iLO 4")
	}

	info := Info{
		Protocol:    ProtocolILO4,
		Vendor:      "HPE",
		Model:       "iLO 4",
		Hostname:    creds.Host,
		ReachableAt: time.Now(),
	}
	if len(root.Oem.Hp.Manager) > 0 {
		info.FirmwareVersion = root.Oem.Hp.Manager[0].ManagerFirmwareVersion
	}
	return info, nil
}

// probeIPMI runs a quick ipmitool mc info to verify IPMI LAN reachability.
func probeIPMI(host string) error {
	// Use a short timeout; no credentials needed to detect IPMI availability
	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
		},
	}
	_ = client // probeIPMI uses ipmitool, not HTTP
	return ipmiRun(8, host, "", "", []string{"mc", "info"})
}
