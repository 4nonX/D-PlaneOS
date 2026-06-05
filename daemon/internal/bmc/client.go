// client.go - unified dispatch layer for BMC operations.
// Callers use these functions; Redfish and IPMI are implementation details.

package bmc

import (
	"context"
	"fmt"
)

// Probe detects the BMC type using the supplied credentials and stored TLS pin.
// Detection order: Redfish (pinned cert) → Redfish (first probe / no pin) →
// iLO 4 legacy REST → IPMI.
func Probe(ctx context.Context, creds Credentials, pin PinnedCert) (Info, error) {
	if creds.Host == "" {
		return Info{Protocol: ProtocolNone}, nil
	}

	// 1. Redfish with pinned cert
	if pin.Fingerprint != "" {
		rc, err := newRedfishClient(creds, pin)
		if err == nil {
			var root struct{ RedfishVersion string `json:"RedfishVersion"` }
			if err := rc.get(ctx, "/redfish/v1/", &root); err == nil {
				return Info{Protocol: ProtocolRedfish, Vendor: "BMC", Hostname: creds.Host}, nil
			}
		}
	}

	// 2. Redfish without pin (no cert enrolled yet - probe only)
	if info, err := probeRedfish(ctx, creds); err == nil {
		return info, nil
	}

	// 3. iLO 4
	if info, err := probeILO4(ctx, creds); err == nil {
		return info, nil
	}

	// 4. IPMI
	if err := probeIPMI(creds.Host); err == nil {
		return Info{Protocol: ProtocolIPMI, Vendor: "Generic", Model: "IPMI", Hostname: creds.Host}, nil
	}

	return Info{Protocol: ProtocolNone, Hostname: creds.Host},
		fmt.Errorf("no BMC protocol responded at %s", creds.Host)
}

// GetHealth fetches hardware sensors using the best available protocol.
func GetHealth(ctx context.Context, creds Credentials, pin PinnedCert) (Health, error) {
	if pin.Fingerprint != "" {
		rc, err := newRedfishClient(creds, pin)
		if err == nil {
			if h, err := rc.GetHealth(ctx); err == nil {
				return h, nil
			}
		}
	}
	return GetHealthIPMI(creds.Host, creds.Username, creds.Password)
}

// GetEvents returns BMC system event log entries.
func GetEvents(ctx context.Context, creds Credentials, pin PinnedCert, limit int) ([]Event, error) {
	if pin.Fingerprint != "" {
		rc, err := newRedfishClient(creds, pin)
		if err == nil {
			if events, err := rc.GetEvents(ctx, limit); err == nil {
				return events, nil
			}
		}
	}
	return GetEventsIPMI(creds.Host, creds.Username, creds.Password, limit)
}

// GetPowerState returns the current chassis power state.
func GetPowerState(ctx context.Context, creds Credentials, pin PinnedCert) (PowerState, error) {
	h, err := GetHealth(ctx, creds, pin)
	if err != nil {
		return PowerUnknown, err
	}
	return h.PowerState, nil
}

// SendPowerAction sends a power management command.
func SendPowerAction(ctx context.Context, creds Credentials, pin PinnedCert, action PowerAction) error {
	if pin.Fingerprint != "" {
		rc, err := newRedfishClient(creds, pin)
		if err == nil {
			sysPath, err := rc.FindSystemPath(ctx)
			if err == nil {
				return rc.PowerAction(ctx, sysPath, action)
			}
		}
	}
	return PowerActionIPMI(creds.Host, creds.Username, creds.Password, action)
}
