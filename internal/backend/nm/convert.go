// Package nm adapts NetworkManager's D-Bus API to the app's Backend seam.
// It is the only package that imports gonetworkmanager/godbus.
package nm

import (
	gonm "github.com/Wifx/gonetworkmanager/v3"

	"github.com/ilmars/netfu/internal/domain"
)

// NM >= 1.42 reports loopback devices (type 32); gonetworkmanager's enum
// stops at 30, so the value is mapped by hand.
const nmDeviceTypeLoopback gonm.NmDeviceType = 32

func deviceTypeFromNM(t gonm.NmDeviceType) domain.DeviceType {
	switch t {
	case gonm.NmDeviceTypeWifi:
		return domain.DeviceTypeWifi
	case gonm.NmDeviceTypeEthernet:
		return domain.DeviceTypeEthernet
	case gonm.NmDeviceTypeBridge:
		return domain.DeviceTypeBridge
	case gonm.NmDeviceTypeVeth:
		return domain.DeviceTypeVeth
	case nmDeviceTypeLoopback:
		return domain.DeviceTypeLoopback
	default:
		return domain.DeviceTypeUnknown
	}
}

func deviceStateFromNM(s gonm.NmDeviceState) domain.DeviceState {
	switch s {
	case gonm.NmDeviceStateUnmanaged:
		return domain.DeviceStateUnmanaged
	case gonm.NmDeviceStateDisconnected:
		return domain.DeviceStateDisconnected
	case gonm.NmDeviceStatePrepare, gonm.NmDeviceStateConfig, gonm.NmDeviceStateNeedAuth,
		gonm.NmDeviceStateIpConfig, gonm.NmDeviceStateIpCheck, gonm.NmDeviceStateSecondaries:
		return domain.DeviceStateConnecting
	case gonm.NmDeviceStateActivated:
		return domain.DeviceStateConnected
	case gonm.NmDeviceStateDeactivating:
		return domain.DeviceStateDeactivating
	case gonm.NmDeviceStateFailed:
		return domain.DeviceStateFailed
	default:
		return domain.DeviceStateUnavailable
	}
}

func activeStateFromNM(s gonm.NmActiveConnectionState) domain.DeviceState {
	switch s {
	case gonm.NmActiveConnectionStateActivating:
		return domain.DeviceStateConnecting
	case gonm.NmActiveConnectionStateActivated:
		return domain.DeviceStateConnected
	case gonm.NmActiveConnectionStateDeactivating:
		return domain.DeviceStateDeactivating
	default:
		return domain.DeviceStateDisconnected
	}
}

// accessPointFromNM reads one scanned AP's properties; not-ok means the AP
// vanished between listing and reading and should be skipped.
func accessPointFromNM(ap gonm.AccessPoint) (domain.AccessPoint, bool) {
	ssid, err := ap.GetPropertySSID()
	if err != nil {
		return domain.AccessPoint{}, false
	}
	strength, err := ap.GetPropertyStrength()
	if err != nil {
		return domain.AccessPoint{}, false
	}
	bssid, err := ap.GetPropertyHWAddress()
	if err != nil {
		return domain.AccessPoint{}, false
	}
	flags, err := ap.GetPropertyFlags()
	if err != nil {
		return domain.AccessPoint{}, false
	}
	wpaFlags, err := ap.GetPropertyWPAFlags()
	if err != nil {
		return domain.AccessPoint{}, false
	}
	rsnFlags, err := ap.GetPropertyRSNFlags()
	if err != nil {
		return domain.AccessPoint{}, false
	}
	return domain.AccessPoint{
		SSID:     ssid,
		Strength: strength,
		BSSID:    bssid,
		Security: domain.ClassifySecurity(flags, wpaFlags, rsnFlags),
	}, true
}

// settingsFromNM converts a decoded NM settings map to the domain shape.
// NM sends SSIDs as byte arrays ("ay") because SSIDs are not guaranteed to
// be UTF-8; the app treats them as strings.
func settingsFromNM(in gonm.ConnectionSettings) domain.ConnectionSettings {
	out := domain.ConnectionSettings{}
	for group, values := range in {
		out[group] = map[string]any{}
		for key, value := range values {
			if group == "802-11-wireless" && key == "ssid" {
				if b, ok := value.([]byte); ok {
					value = string(b)
				}
			}
			out[group][key] = value
		}
	}
	return out
}

// settingsToNM converts domain settings back to what NM's Update/AddConnection
// expect. Two coercions matter: the SSID goes back to bytes, and homogeneous
// []any arrays (produced by GetSettings' variant decoding) are rebuilt with
// concrete element types — godbus would otherwise marshal them as "av",
// which NM rejects against typed schemas like "as"/"au"/"aau".
func settingsToNM(in domain.ConnectionSettings) gonm.ConnectionSettings {
	out := gonm.ConnectionSettings{}
	for group, values := range in {
		out[group] = map[string]any{}
		for key, value := range values {
			if group == "802-11-wireless" && key == "ssid" {
				if s, ok := value.(string); ok {
					value = []byte(s)
				}
			}
			out[group][key] = encodeValue(value)
		}
	}
	return out
}

func encodeValue(value any) any {
	switch v := value.(type) {
	case []any:
		return encodeArray(v)
	case map[string]any:
		encoded := map[string]any{}
		for k, e := range v {
			encoded[k] = encodeValue(e)
		}
		return encoded
	default:
		return value
	}
}

func encodeArray(in []any) any {
	if len(in) == 0 {
		return in
	}
	switch in[0].(type) {
	case string:
		out := make([]string, 0, len(in))
		for _, e := range in {
			s, ok := e.(string)
			if !ok {
				return in
			}
			out = append(out, s)
		}
		return out
	case uint32:
		out := make([]uint32, 0, len(in))
		for _, e := range in {
			u, ok := e.(uint32)
			if !ok {
				return in
			}
			out = append(out, u)
		}
		return out
	case []any:
		out := make([][]uint32, 0, len(in))
		for _, e := range in {
			inner, ok := e.([]any)
			if !ok {
				return in
			}
			row, ok := encodeArray(inner).([]uint32)
			if !ok {
				return in
			}
			out = append(out, row)
		}
		return out
	case map[string]any:
		out := make([]map[string]any, 0, len(in))
		for _, e := range in {
			m, ok := e.(map[string]any)
			if !ok {
				return in
			}
			out = append(out, encodeValue(m).(map[string]any))
		}
		return out
	default:
		return in
	}
}

// reasonFromNM names the reasons the TUI's wrong-password heuristic matches
// on with stable strings; everything else keeps the library's stringer name.
func reasonFromNM(r gonm.NmDeviceStateReason) string {
	switch r {
	case gonm.NmDeviceStateReasonNone:
		return ""
	case gonm.NmDeviceStateReasonNoSecrets:
		return domain.ReasonNoSecrets
	case gonm.NmDeviceStateReasonSupplicantDisconnect:
		return domain.ReasonSupplicantDisconnect
	default:
		return r.String()
	}
}

func nmStateFromNM(s gonm.NmState) domain.NMState {
	switch s {
	case gonm.NmStateAsleep:
		return domain.NMStateAsleep
	case gonm.NmStateDisconnected, gonm.NmStateDisconnecting:
		return domain.NMStateDisconnected
	case gonm.NmStateConnecting:
		return domain.NMStateConnecting
	case gonm.NmStateConnectedLocal, gonm.NmStateConnectedSite, gonm.NmStateConnectedGlobal:
		return domain.NMStateConnected
	}
	return domain.NMStateUnknown
}
