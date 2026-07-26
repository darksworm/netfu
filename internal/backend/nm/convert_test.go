package nm

import (
	"errors"
	"testing"

	gonm "github.com/Wifx/gonetworkmanager/v3"

	"github.com/ilmars/netfu/internal/domain"
)

func TestDeviceTypeFromNM_MapsKnownTypes(t *testing.T) {
	cases := []struct {
		in   gonm.NmDeviceType
		want domain.DeviceType
	}{
		{gonm.NmDeviceTypeWifi, domain.DeviceTypeWifi},
		{gonm.NmDeviceTypeEthernet, domain.DeviceTypeEthernet},
		{gonm.NmDeviceTypeBridge, domain.DeviceTypeBridge},
		{gonm.NmDeviceTypeVeth, domain.DeviceTypeVeth},
		{gonm.NmDeviceTypeBond, domain.DeviceTypeUnknown},
	}
	for _, c := range cases {
		if got := deviceTypeFromNM(c.in); got != c.want {
			t.Errorf("deviceTypeFromNM(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestDeviceTypeFromNM_MapsLoopbackUnknownToTheLibrary(t *testing.T) {
	// NM >= 1.42 reports NM_DEVICE_TYPE_LOOPBACK = 32; gonetworkmanager's
	// enum stops at 30, so the value is mapped by hand.
	if got := deviceTypeFromNM(gonm.NmDeviceType(32)); got != domain.DeviceTypeLoopback {
		t.Errorf("deviceTypeFromNM(32) = %q, want loopback", got)
	}
}

func TestDeviceStateFromNM_CollapsesActivationStagesToConnecting(t *testing.T) {
	for _, s := range []gonm.NmDeviceState{
		gonm.NmDeviceStatePrepare,
		gonm.NmDeviceStateConfig,
		gonm.NmDeviceStateNeedAuth,
		gonm.NmDeviceStateIpConfig,
		gonm.NmDeviceStateIpCheck,
		gonm.NmDeviceStateSecondaries,
	} {
		if got := deviceStateFromNM(s); got != domain.DeviceStateConnecting {
			t.Errorf("deviceStateFromNM(%d) = %q, want connecting", s, got)
		}
	}
}

func TestDeviceStateFromNM_MapsTerminalStates(t *testing.T) {
	cases := []struct {
		in   gonm.NmDeviceState
		want domain.DeviceState
	}{
		{gonm.NmDeviceStateUnmanaged, domain.DeviceStateUnmanaged},
		{gonm.NmDeviceStateUnavailable, domain.DeviceStateUnavailable},
		{gonm.NmDeviceStateDisconnected, domain.DeviceStateDisconnected},
		{gonm.NmDeviceStateActivated, domain.DeviceStateConnected},
		{gonm.NmDeviceStateDeactivating, domain.DeviceStateDeactivating},
		{gonm.NmDeviceStateFailed, domain.DeviceStateFailed},
		{gonm.NmDeviceStateUnknown, domain.DeviceStateUnavailable},
	}
	for _, c := range cases {
		if got := deviceStateFromNM(c.in); got != c.want {
			t.Errorf("deviceStateFromNM(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestActiveStateFromNM_MapsConnectionLifecycle(t *testing.T) {
	cases := []struct {
		in   gonm.NmActiveConnectionState
		want domain.DeviceState
	}{
		{gonm.NmActiveConnectionStateActivating, domain.DeviceStateConnecting},
		{gonm.NmActiveConnectionStateActivated, domain.DeviceStateConnected},
		{gonm.NmActiveConnectionStateDeactivating, domain.DeviceStateDeactivating},
		{gonm.NmActiveConnectionStateDeactivated, domain.DeviceStateDisconnected},
		{gonm.NmActiveConnectionStateUnknown, domain.DeviceStateDisconnected},
	}
	for _, c := range cases {
		if got := activeStateFromNM(c.in); got != c.want {
			t.Errorf("activeStateFromNM(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestReasonFromNM_NamesTheWrongPasswordReasonsStably(t *testing.T) {
	// The TUI's wrong-password heuristic matches on these two strings, so
	// they get stable names independent of the library's stringer output.
	if got := reasonFromNM(gonm.NmDeviceStateReasonNoSecrets); got != "no-secrets" {
		t.Errorf("no-secrets reason = %q", got)
	}
	if got := reasonFromNM(gonm.NmDeviceStateReasonSupplicantDisconnect); got != "supplicant-disconnect" {
		t.Errorf("supplicant-disconnect reason = %q", got)
	}
	if got := reasonFromNM(gonm.NmDeviceStateReasonNone); got != "" {
		t.Errorf("none reason = %q, want empty", got)
	}
}

// stubAccessPoint overrides just the properties accessPointFromNM reads;
// anything else panics via the embedded nil interface.
type stubAccessPoint struct {
	gonm.AccessPoint
	ssid     string
	strength uint8
	bssid    string
	flags    uint32
	wpaFlags uint32
	rsnFlags uint32
	err      error
}

func (s stubAccessPoint) GetPropertySSID() (string, error)      { return s.ssid, s.err }
func (s stubAccessPoint) GetPropertyStrength() (uint8, error)   { return s.strength, s.err }
func (s stubAccessPoint) GetPropertyHWAddress() (string, error) { return s.bssid, s.err }
func (s stubAccessPoint) GetPropertyFlags() (uint32, error)     { return s.flags, s.err }
func (s stubAccessPoint) GetPropertyWPAFlags() (uint32, error)  { return s.wpaFlags, s.err }
func (s stubAccessPoint) GetPropertyRSNFlags() (uint32, error)  { return s.rsnFlags, s.err }

func TestAccessPointFromNM_PopulatesBSSIDAndClassifiedSecurity(t *testing.T) {
	// NM_802_11_AP_FLAGS_PRIVACY with RSN PSK key management: a WPA2 network.
	stub := stubAccessPoint{
		ssid:     "Our House 1",
		strength: 82,
		bssid:    "AA:BB:CC:11:22:33",
		flags:    0x1,
		rsnFlags: 0x100,
	}

	ap, ok := accessPointFromNM(stub)

	if !ok {
		t.Fatal("a readable AP should convert")
	}
	want := domain.AccessPoint{
		SSID:     "Our House 1",
		Strength: 82,
		BSSID:    "AA:BB:CC:11:22:33",
		Security: domain.SecurityWPA2,
	}
	if ap != want {
		t.Errorf("accessPointFromNM = %+v, want %+v", ap, want)
	}
}

func TestAccessPointFromNM_ReportsNotOKWhenTheAPVanishedMidRead(t *testing.T) {
	stub := stubAccessPoint{ssid: "gone", err: errAPVanished}

	if _, ok := accessPointFromNM(stub); ok {
		t.Error("an AP whose properties fail to read should be skipped, not returned half-filled")
	}
}

var errAPVanished = errors.New("object does not exist")

func TestSettingsFromNM_TurnsSSIDBytesIntoAString(t *testing.T) {
	in := gonm.ConnectionSettings{
		"802-11-wireless": {"ssid": []byte("Our House 1"), "mode": "infrastructure"},
		"connection":      {"id": "Our House 1", "autoconnect": false},
	}
	out := settingsFromNM(in)
	if got := out["802-11-wireless"]["ssid"]; got != "Our House 1" {
		t.Errorf("ssid = %#v, want string", got)
	}
	if got := out["802-11-wireless"]["mode"]; got != "infrastructure" {
		t.Errorf("mode = %#v", got)
	}
	if got := out["connection"]["autoconnect"]; got != false {
		t.Errorf("autoconnect = %#v", got)
	}
}

func TestSettingsToNM_TurnsSSIDStringBackIntoBytes(t *testing.T) {
	in := domain.ConnectionSettings{
		"802-11-wireless": {"ssid": "Our House 1"},
	}
	out := settingsToNM(in)
	got, ok := out["802-11-wireless"]["ssid"].([]byte)
	if !ok || string(got) != "Our House 1" {
		t.Errorf("ssid = %#v, want []byte(\"Our House 1\")", out["802-11-wireless"]["ssid"])
	}
}

func TestSettingsToNM_RebuildsTypedArraysSoDBusSignaturesMatch(t *testing.T) {
	// GetSettings decodes typed D-Bus arrays ("as", "au", "aau") into
	// []any; marshalling []any back would produce signature "av", which
	// NM rejects. Homogeneous arrays are rebuilt with concrete types.
	in := domain.ConnectionSettings{
		"ipv4": {
			"dns-search": []any{"lan", "home"},
			"dns":        []any{uint32(16843009)},
			"addresses":  []any{[]any{uint32(1), uint32(24), uint32(2)}},
		},
	}
	out := settingsToNM(in)
	if got, ok := out["ipv4"]["dns-search"].([]string); !ok || len(got) != 2 || got[0] != "lan" {
		t.Errorf("dns-search = %#v, want []string", out["ipv4"]["dns-search"])
	}
	if got, ok := out["ipv4"]["dns"].([]uint32); !ok || got[0] != 16843009 {
		t.Errorf("dns = %#v, want []uint32", out["ipv4"]["dns"])
	}
	if got, ok := out["ipv4"]["addresses"].([][]uint32); !ok || got[0][1] != 24 {
		t.Errorf("addresses = %#v, want [][]uint32", out["ipv4"]["addresses"])
	}
}

func TestSettingsToNM_RebuildsAddressDataMapsRecursively(t *testing.T) {
	in := domain.ConnectionSettings{
		"ipv4": {
			"address-data": []any{
				map[string]any{"address": "192.168.1.5", "prefix": uint32(24)},
			},
		},
	}
	out := settingsToNM(in)
	got, ok := out["ipv4"]["address-data"].([]map[string]any)
	if !ok || got[0]["address"] != "192.168.1.5" {
		t.Errorf("address-data = %#v, want []map[string]any", out["ipv4"]["address-data"])
	}
}

func TestSettingsToNM_LeavesScalarsAndEmptyArraysAlone(t *testing.T) {
	in := domain.ConnectionSettings{
		"connection": {"id": "wired", "autoconnect-priority": int32(-1), "empty": []any{}},
	}
	out := settingsToNM(in)
	if out["connection"]["id"] != "wired" || out["connection"]["autoconnect-priority"] != int32(-1) {
		t.Errorf("scalars changed: %#v", out["connection"])
	}
	if got, ok := out["connection"]["empty"].([]any); !ok || len(got) != 0 {
		t.Errorf("empty = %#v", out["connection"]["empty"])
	}
}

func TestNMStateFromNM_CollapsesConnectedTiersAndTransitions(t *testing.T) {
	cases := []struct {
		in   gonm.NmState
		want domain.NMState
	}{
		{gonm.NmStateConnectedGlobal, domain.NMStateConnected},
		{gonm.NmStateConnectedSite, domain.NMStateConnected},
		{gonm.NmStateConnectedLocal, domain.NMStateConnected},
		{gonm.NmStateConnecting, domain.NMStateConnecting},
		{gonm.NmStateDisconnecting, domain.NMStateDisconnected},
		{gonm.NmStateDisconnected, domain.NMStateDisconnected},
		{gonm.NmStateAsleep, domain.NMStateAsleep},
		{gonm.NmStateUnknown, domain.NMStateUnknown},
	}
	for _, c := range cases {
		if got := nmStateFromNM(c.in); got != c.want {
			t.Errorf("nmStateFromNM(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}
