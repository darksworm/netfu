package domain

import "testing"

func ssids(aps []AccessPoint) []string {
	var out []string
	for _, ap := range aps {
		out = append(out, ap.SSID)
	}
	return out
}

func assertSSIDOrder(t *testing.T, got []AccessPoint, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d rows %v, want %v", len(got), ssids(got), want)
	}
	for i := range want {
		if got[i].SSID != want[i] {
			t.Fatalf("row order %v, want %v", ssids(got), want)
		}
	}
}

func TestSort_APsOrderBySignalDescending(t *testing.T) {
	aps := []AccessPoint{
		{SSID: "weak", Strength: 20},
		{SSID: "strong", Strength: 90},
		{SSID: "mid", Strength: 55},
	}

	got := BuildWifiList(aps, nil, "", "")

	assertSSIDOrder(t, got.InRange, []string{"strong", "mid", "weak"})
}

func TestSort_ActiveNetworkPinnedToTop(t *testing.T) {
	aps := []AccessPoint{
		{SSID: "neighbor", Strength: 95},
		{SSID: "home", Strength: 40},
		{SSID: "cafe", Strength: 70},
	}

	got := BuildWifiList(aps, []string{"home"}, "home", "")

	assertSSIDOrder(t, got.InRange, []string{"home", "neighbor", "cafe"})
}

func TestSort_KnownNetworksBeforeUnknownAtEqualStrength(t *testing.T) {
	aps := []AccessPoint{
		{SSID: "unknown-a", Strength: 60},
		{SSID: "saved-b", Strength: 60},
		{SSID: "unknown-c", Strength: 80},
		{SSID: "saved-d", Strength: 30},
	}

	got := BuildWifiList(aps, []string{"saved-b", "saved-d"}, "", "")

	// Saved-in-range is a whole bucket above unsaved, not a tiebreaker:
	// even the weak saved network outranks the strongest unknown one.
	assertSSIDOrder(t, got.InRange, []string{"saved-b", "saved-d", "unknown-c", "unknown-a"})
}

func TestSort_ConnectingNetworkBelowActiveAboveSaved(t *testing.T) {
	aps := []AccessPoint{
		{SSID: "saved-strong", Strength: 99},
		{SSID: "joining", Strength: 10},
		{SSID: "current", Strength: 5},
	}

	got := BuildWifiList(aps, []string{"saved-strong"}, "current", "joining")

	assertSSIDOrder(t, got.InRange, []string{"current", "joining", "saved-strong"})
}

func TestSort_SavedNetworksWithoutScannedAPListedAsOutOfRange(t *testing.T) {
	aps := []AccessPoint{
		{SSID: "home", Strength: 80},
	}

	got := BuildWifiList(aps, []string{"office", "home", "summerhouse"}, "", "")

	assertSSIDOrder(t, got.InRange, []string{"home"})
	want := []string{"office", "summerhouse"}
	if len(got.OutOfRange) != len(want) {
		t.Fatalf("OutOfRange = %v, want %v", got.OutOfRange, want)
	}
	for i := range want {
		if got.OutOfRange[i] != want[i] {
			t.Fatalf("OutOfRange = %v, want %v", got.OutOfRange, want)
		}
	}
}

// Flag values from NM's 80211ApFlags / 80211ApSecurityFlags
// (verified against networkmanager.dev nm-dbus-types).
const (
	apFlagPrivacy    uint32 = 0x1
	apSecPairCCMP    uint32 = 0x8
	apSecGroupCCMP   uint32 = 0x80
	apSecKeyMgmtPSK  uint32 = 0x100
	apSecKeyMgmt801X uint32 = 0x200
	apSecKeyMgmtSAE  uint32 = 0x400
)

func TestSecurity_WPA2WPA3MixedAPClassifiedAsWPA2WPA3(t *testing.T) {
	ccmp := apSecPairCCMP | apSecGroupCCMP
	cases := []struct {
		name                      string
		flags, wpaFlags, rsnFlags uint32
		want                      Security
	}{
		{"mixed PSK+SAE in RSN", apFlagPrivacy, 0, ccmp | apSecKeyMgmtPSK | apSecKeyMgmtSAE, SecurityWPA2WPA3},
		{"PSK only is plain WPA2", apFlagPrivacy, 0, ccmp | apSecKeyMgmtPSK, SecurityWPA2},
		{"SAE only is plain WPA3", apFlagPrivacy, 0, ccmp | apSecKeyMgmtSAE, SecurityWPA3},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ClassifySecurity(c.flags, c.wpaFlags, c.rsnFlags); got != c.want {
				t.Errorf("ClassifySecurity(%#x, %#x, %#x) = %q, want %q", c.flags, c.wpaFlags, c.rsnFlags, got, c.want)
			}
		})
	}
}

func TestSecurity_8021XFlagsClassifiedAsEnterprise(t *testing.T) {
	ccmp := apSecPairCCMP | apSecGroupCCMP
	cases := []struct {
		name                      string
		flags, wpaFlags, rsnFlags uint32
	}{
		{"802.1X in RSN (WPA2 Enterprise)", apFlagPrivacy, 0, ccmp | apSecKeyMgmt801X},
		{"802.1X in WPA only (WPA1 Enterprise)", apFlagPrivacy, ccmp | apSecKeyMgmt801X, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ClassifySecurity(c.flags, c.wpaFlags, c.rsnFlags); got != SecurityEnterprise {
				t.Errorf("ClassifySecurity(%#x, %#x, %#x) = %q, want %q", c.flags, c.wpaFlags, c.rsnFlags, got, SecurityEnterprise)
			}
		})
	}
}

func TestSecurity_LegacyAPsClassifiedAsWEPOrWPA1(t *testing.T) {
	if got := ClassifySecurity(apFlagPrivacy, 0, 0); got != SecurityWEP {
		t.Errorf("privacy without WPA/RSN = %q, want %q", got, SecurityWEP)
	}
	if got := ClassifySecurity(apFlagPrivacy, apSecKeyMgmtPSK, 0); got != SecurityWPA {
		t.Errorf("PSK in WPA flags only = %q, want %q", got, SecurityWPA)
	}
}

func TestDedupe_HiddenAPsAreNotMergedWithEachOther(t *testing.T) {
	aps := []AccessPoint{
		{SSID: "", BSSID: "aa:aa:aa:aa:aa:01", Strength: 70},
		{SSID: "", BSSID: "bb:bb:bb:bb:bb:02", Strength: 30},
	}

	got := DedupeAPs(aps)

	if len(got) != 2 {
		t.Fatalf("got %d APs, want 2 (each hidden BSS its own row): %+v", len(got), got)
	}
	if got[0].BSSID != "aa:aa:aa:aa:aa:01" || got[1].BSSID != "bb:bb:bb:bb:bb:02" {
		t.Errorf("hidden APs merged or reordered: %+v", got)
	}
}

func TestDedupe_DuplicateSSIDsCollapseToStrongestBSSID(t *testing.T) {
	aps := []AccessPoint{
		{SSID: "home", BSSID: "aa:aa:aa:aa:aa:01", Strength: 40},
		{SSID: "home", BSSID: "aa:aa:aa:aa:aa:02", Strength: 82},
		{SSID: "cafe", BSSID: "bb:bb:bb:bb:bb:01", Strength: 60},
		{SSID: "home", BSSID: "aa:aa:aa:aa:aa:03", Strength: 55},
	}

	got := DedupeAPs(aps)

	want := []AccessPoint{
		{SSID: "home", BSSID: "aa:aa:aa:aa:aa:02", Strength: 82},
		{SSID: "cafe", BSSID: "bb:bb:bb:bb:bb:01", Strength: 60},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d APs, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("ap[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}
