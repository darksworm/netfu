package tui

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/ilmars/netfu/internal/backend/fake"
	"github.com/ilmars/netfu/internal/domain"
)

// seedProfiles returns the arch laptop fixture extended with an ethernet and
// two VPN profiles, so the Other tab has grouped rows to show.
func seedProfiles() *fake.Fake {
	f := fake.SeedArchLaptop()
	f.ConnectionList = append(f.ConnectionList,
		domain.Connection{
			ID: "office-lan", Name: "Office LAN", Type: "802-3-ethernet",
			LastUsedUnix: time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC).Unix(),
		},
		domain.Connection{ID: "work-vpn", Name: "Work VPN", Type: "vpn"},
		domain.Connection{
			ID: "old-vpn", Name: "Old VPN", Type: "vpn",
			LastUsedUnix: time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC).Unix(),
		},
	)
	return f
}

func TestOther_ListsOnlyProfilesWithoutTheirOwnTab(t *testing.T) {
	f := seedProfiles()
	p := newPump(t, New(f))

	p.send(keyPress('4'))
	view := p.view()

	for _, col := range []string{"NAME", "TYPE", "DEVICE", "LAST USED"} {
		if !strings.Contains(view, col) {
			t.Errorf("list should have a %s column header, got:\n%s", col, view)
		}
	}
	for _, group := range []string{"─ VPN ─", "─ Bridge ─"} {
		if !strings.Contains(view, group) {
			t.Errorf("profiles should be grouped under %q, got:\n%s", group, view)
		}
	}
	for _, name := range []string{"Work VPN", "Old VPN", "docker0"} {
		if !strings.Contains(view, name) {
			t.Errorf("vpn and bridge profiles belong here, missing %q in:\n%s", name, view)
		}
	}
	// Wifi profiles live on the wifi device tab, wired profiles on their
	// NIC's tab — neither belongs under Other.
	for _, elsewhere := range []string{"Our House 1", "Our House 5G", "Summer House", "Office LAN", "─ Wi-Fi ─", "─ Ethernet ─"} {
		if strings.Contains(view, elsewhere) {
			t.Errorf("%q has its own tab and should not be listed under Other, got:\n%s", elsewhere, view)
		}
	}

	active := lineContaining(t, view, "docker0")
	if !strings.Contains(active, "✓") {
		t.Errorf("the active profile should show an active mark, got: %q", active)
	}
	if used := lineContaining(t, view, "Old VPN"); !strings.Contains(used, "2026-06-30") {
		t.Errorf("a profile's last successful activation date should show, got: %q", used)
	}
	if never := lineContaining(t, view, "Work VPN"); !strings.Contains(never, "never") {
		t.Errorf("a never-used profile should say so, got: %q", never)
	}
}

func TestOther_ListsWiredProfilesWhenNoEthernetNICIsPresent(t *testing.T) {
	f := seedProfiles()
	f.DeviceList = slices.Delete(f.DeviceList, 1, 2) // no enp0s31f6
	p := newPump(t, New(f))

	p.send(keyPress('3')) // Virtual moved up: wlan0, Virtual, Other, System
	if got := p.app().currentTab(); got.kind != tabKindOther {
		t.Fatalf("precondition: without a wired NIC the Other tab is third, got %+v", got)
	}
	if view := p.view(); !strings.Contains(view, "Office LAN") {
		t.Errorf("a wired profile with no NIC to live on should fall back to Other, got:\n%s", view)
	}
}

func TestOther_ListsWiredProfilesPinnedToAMissingNICEvenWhenANICIsPresent(t *testing.T) {
	f := seedWiredProfiles() // enp0s31f6 present; USB Dock pinned to absent enp5s0u1
	p := newPump(t, New(f))

	p.send(keyPress('4'))
	view := p.view()
	if !strings.Contains(view, "USB Dock") {
		t.Errorf("a wired profile pinned to a missing NIC has no tab to live on and belongs here, got:\n%s", view)
	}
	// Profiles the present NIC can host live on its tab, not here.
	for _, elsewhere := range []string{"Office LAN", "Dock "} {
		if strings.Contains(strings.ReplaceAll(view, "USB Dock", ""), elsewhere) {
			t.Errorf("%q lives on the NIC's tab and should not be listed under Other, got:\n%s", elsewhere, view)
		}
	}
}

func TestOther_TreatsAWiredProfileWithUnreadableSettingsAsUnpinned(t *testing.T) {
	f := seedWiredProfiles()
	f.Errs["GetSettings"] = errors.New("dbus timeout")
	p := newPump(t, New(f))

	p.send(keyPress('4'))
	if view := p.view(); strings.Contains(view, "USB Dock") {
		t.Errorf("with unreadable settings the pin is unknown; the profile should stay off Other, got:\n%s", view)
	}
}

func TestEditor_DeleteConfirmOverlaysEvenWhenTheListOverflowsThePane(t *testing.T) {
	f := seedProfiles()
	for i := range 30 {
		f.ConnectionList = append(f.ConnectionList,
			domain.Connection{ID: fmt.Sprintf("vpn-%d", i), Name: fmt.Sprintf("Exit Node %d", i), Type: "vpn"})
	}
	p := newPump(t, New(f))
	p.send(tea.WindowSizeMsg{Width: 80, Height: 24})

	p.send(keyPress('4'))
	p.send(keyPress('x'))

	if view := p.view(); !strings.Contains(view, "Delete Work VPN?") {
		t.Fatalf("the delete confirm must overlay the list, not render below the fold, got:\n%s", view)
	}
}

func TestEditor_ListScrollsSoTheCursorRowStaysVisible(t *testing.T) {
	f := seedProfiles()
	for i := range 30 {
		f.ConnectionList = append(f.ConnectionList,
			domain.Connection{ID: fmt.Sprintf("vpn-%d", i), Name: fmt.Sprintf("Exit Node %d", i), Type: "vpn"})
	}
	p := newPump(t, New(f))
	p.send(tea.WindowSizeMsg{Width: 80, Height: 24})

	p.send(keyPress('4'))
	p.send(keyPress('G'))

	if got := p.app().conns.Selected().Name; got != "docker0" {
		t.Fatalf("precondition: G should select the last profile, got %q", got)
	}
	if view := p.view(); !strings.Contains(view, "docker0") {
		t.Errorf("the list should scroll so the cursor row is visible, got:\n%s", view)
	}
}

func TestEditor_DeleteConnectionAsksConfirmThenCallsDelete(t *testing.T) {
	f := seedProfiles()
	p := newPump(t, New(f))

	p.send(keyPress('4'))
	p.send(keyPress('x'))
	if view := p.view(); !strings.Contains(view, "Delete Work VPN?") {
		t.Fatalf("x should ask for confirmation first, got:\n%s", view)
	}
	if slices.Contains(f.Calls, "DeleteConnection(work-vpn)") {
		t.Fatal("nothing should be deleted before the user confirms")
	}

	p.send(keyPress('y'))
	if !slices.Contains(f.Calls, "DeleteConnection(work-vpn)") {
		t.Errorf("confirming should delete the profile, calls: %v", f.Calls)
	}
	if view := p.view(); strings.Contains(view, "Delete Work VPN?") {
		t.Errorf("confirming should close the modal, got:\n%s", view)
	}
}

// seedStaticWifiProfile enriches "Our House 1" with the settings the typed
// form covers, plus a proxy section the editor knows nothing about.
func seedStaticWifiProfile() *fake.Fake {
	f := seedProfiles()
	f.SettingsByID["our-house-1"] = domain.ConnectionSettings{
		"connection": {
			"id": "Our House 1", "uuid": "our-house-1",
			"type": "802-11-wireless", "autoconnect": false,
		},
		"802-11-wireless":          {"ssid": "Our House 1", "mode": "infrastructure"},
		"802-11-wireless-security": {"key-mgmt": "wpa-psk"},
		"ipv4": {
			"method":       "manual",
			"address-data": []map[string]any{{"address": "192.168.1.50", "prefix": 24}},
			"gateway":      "192.168.1.1",
			"dns":          []string{"1.1.1.1"},
		},
		"ipv6":  {"method": "auto"},
		"proxy": {"browser-only": false},
	}
	return f
}

func TestEditor_OpenProfileLoadsSettingsIntoTypedForm(t *testing.T) {
	f := seedStaticWifiProfile()
	p := newPump(t, New(f))

	// e on the wifi tab's saved active network pushes its profile editor.
	p.send(keyPress('e'))
	view := p.view()

	if strings.Contains(view, "▂▄▆█") {
		t.Errorf("the editor should replace the scan list, got:\n%s", view)
	}
	for _, section := range []string{"General", "Wi-Fi", "IPv4", "IPv6"} {
		if !strings.Contains(view, section) {
			t.Errorf("form should have a %s section, got:\n%s", section, view)
		}
	}

	name := lineContaining(t, view, "Name")
	if !strings.Contains(name, "Our House 1") {
		t.Errorf("Name should be loaded from connection.id, got: %q", name)
	}
	if !strings.Contains(name, "▸") {
		t.Errorf("the NAV cursor should start on the first field, got: %q", name)
	}
	if autoconnect := lineContaining(t, view, "Autoconnect"); !strings.Contains(autoconnect, "[ ]") {
		t.Errorf("Autoconnect should show the loaded off state, got: %q", autoconnect)
	}
	if ssid := lineContaining(t, view, "SSID"); !strings.Contains(ssid, "Our House 1") {
		t.Errorf("SSID should be loaded, got: %q", ssid)
	}
	if security := lineContaining(t, view, "Security"); !strings.Contains(security, "(•) wpa-psk") {
		t.Errorf("Security should show the loaded key-mgmt, got: %q", security)
	}

	method := lineContaining(t, view, "Method")
	for _, option := range []string{"(•) static", "( ) dhcp", "( ) disabled"} {
		if !strings.Contains(method, option) {
			t.Errorf("IPv4 Method should be a radio with %q, got: %q", option, method)
		}
	}
	if address := lineContaining(t, view, "Address"); !strings.Contains(address, "192.168.1.50/24") {
		t.Errorf("Address should be loaded from ipv4.address-data, got: %q", address)
	}
	if gateway := lineContaining(t, view, "Gateway"); !strings.Contains(gateway, "192.168.1.1") {
		t.Errorf("Gateway should be loaded, got: %q", gateway)
	}
	if dns := lineContaining(t, view, "DNS"); !strings.Contains(dns, "1.1.1.1") {
		t.Errorf("DNS should be loaded, got: %q", dns)
	}
	if !strings.Contains(view, "(•) auto") {
		t.Errorf("IPv6 method should show the loaded auto, got:\n%s", view)
	}
}

func typeText(p *pump, text string) {
	for _, r := range text {
		p.send(keyPress(r))
	}
}

func TestEditor_EditingAutoconnectAndNameWritesBackViaUpdateSettings(t *testing.T) {
	f := seedStaticWifiProfile()
	p := newPump(t, New(f))

	p.send(keyPress('e'))

	// Rename: EDIT mode on the Name field, append to the existing value.
	p.send(keyPress('i'))
	typeText(p, " Basement")
	p.send(tea.KeyPressMsg{Code: tea.KeyEnter})
	if view := p.view(); !strings.Contains(view, "●") {
		t.Errorf("a touched form should show the dirty dot, got:\n%s", view)
	}

	// Flip autoconnect off→on.
	p.send(keyPress('j'))
	p.send(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})

	p.send(keyPress('s'))

	if len(f.UpdateCalls) != 1 {
		t.Fatalf("s should save via UpdateSettings exactly once, got %d calls", len(f.UpdateCalls))
	}
	call := f.UpdateCalls[0]
	if call.ConnectionID != "our-house-1" {
		t.Errorf("save should target the edited profile, got %q", call.ConnectionID)
	}

	want := seedStaticWifiProfile().SettingsByID["our-house-1"]
	want["connection"]["id"] = "Our House 1 Basement"
	want["connection"]["autoconnect"] = true
	if !reflect.DeepEqual(call.Settings, want) {
		t.Errorf("only the touched keys should change, everything else verbatim.\ngot:  %#v\nwant: %#v",
			call.Settings, want)
	}

	view := p.view()
	if !strings.Contains(view, "▂▄▆█") {
		t.Errorf("a successful save should pop back to the scan list, got:\n%s", view)
	}
	if !strings.Contains(view, "✓ saved Our House 1") {
		t.Errorf("the status line should report the save, got:\n%s", view)
	}
}

func TestEditor_PriorityFieldLoadsValidatesRangeAndWritesBack(t *testing.T) {
	f := seedStaticWifiProfile()
	f.SettingsByID["our-house-1"]["connection"]["autoconnect-priority"] = int32(10)
	p := newPump(t, New(f))

	p.send(keyPress('e'))
	if priority := lineContaining(t, p.view(), "Priority"); !strings.Contains(priority, "10") {
		t.Errorf("Priority should load from connection.autoconnect-priority, got: %q", priority)
	}

	// Down to Priority (Name, Autoconnect, Priority) and type an
	// out-of-range value.
	p.send(keyPress('j'))
	p.send(keyPress('j'))
	p.send(keyPress('i'))
	for range len("10") {
		p.send(tea.KeyPressMsg{Code: tea.KeyBackspace})
	}
	typeText(p, "1200")
	p.send(tea.KeyPressMsg{Code: tea.KeyEnter})
	p.send(keyPress('s'))

	if len(f.UpdateCalls) != 0 {
		t.Errorf("a priority outside -999..999 must block the save, got calls: %#v", f.UpdateCalls)
	}
	if priority := lineContaining(t, p.view(), "Priority"); !strings.Contains(priority, "✗") {
		t.Errorf("the invalid field should carry an inline error, got: %q", priority)
	}

	p.send(keyPress('i'))
	for range len("1200") {
		p.send(tea.KeyPressMsg{Code: tea.KeyBackspace})
	}
	typeText(p, "-50")
	p.send(tea.KeyPressMsg{Code: tea.KeyEnter})
	p.send(keyPress('s'))

	if len(f.UpdateCalls) != 1 {
		t.Fatalf("a valid priority should save via UpdateSettings once, got %d calls", len(f.UpdateCalls))
	}
	call := f.UpdateCalls[0]
	if got := call.Settings["connection"]["autoconnect-priority"]; got != -50 {
		t.Errorf("the edited priority should write back as an integer, got %#v", got)
	}
	if got := call.Settings["proxy"]["browser-only"]; got != false {
		t.Errorf("untouched settings sections must pass through verbatim, got %#v", call.Settings)
	}
}

func TestEditor_InvalidIPv4StaticAddressBlocksSaveWithFieldError(t *testing.T) {
	f := seedStaticWifiProfile()
	p := newPump(t, New(f))

	p.send(keyPress('e'))

	// Down to the Address field: Name, Autoconnect, Priority, SSID,
	// Security, Method.
	for range 6 {
		p.send(keyPress('j'))
	}
	p.send(keyPress('i'))
	for range len("192.168.1.50/24") {
		p.send(tea.KeyPressMsg{Code: tea.KeyBackspace})
	}
	typeText(p, "999.168.1.50/24")
	p.send(tea.KeyPressMsg{Code: tea.KeyEnter})

	p.send(keyPress('s'))

	if len(f.UpdateCalls) != 0 {
		t.Errorf("an invalid address must block the save, got calls: %#v", f.UpdateCalls)
	}
	address := lineContaining(t, p.view(), "Address")
	if !strings.Contains(address, "✗") {
		t.Errorf("the invalid field should carry an inline error, got: %q", address)
	}
	if view := p.view(); !strings.Contains(view, "fix the highlighted fields") {
		t.Errorf("the status line should explain why the save is blocked, got:\n%s", view)
	}
}

func TestEditor_NewEthernetProfileWizardCallsAddConnection(t *testing.T) {
	f := seedProfiles()
	p := newPump(t, New(f))

	p.send(keyPress('4'))
	p.send(keyPress('n'))

	view := p.view()
	if !strings.Contains(view, "New connection") {
		t.Fatalf("n should open the type picker, got:\n%s", view)
	}
	for _, creatable := range []string{"Ethernet", "Wi-Fi"} {
		if !strings.Contains(view, creatable) {
			t.Errorf("the picker should offer %s, got:\n%s", creatable, view)
		}
	}
	for _, line := range strings.Split(view, "\n") {
		if option := strings.TrimLeft(line, "▸ "); option == "VPN" {
			t.Errorf("VPN must not be offered as creatable, got:\n%s", view)
		}
	}

	// Choose Ethernet (first option) and name the blank profile.
	p.send(tea.KeyPressMsg{Code: tea.KeyEnter})
	if view := p.view(); !strings.Contains(view, "IPv4") {
		t.Fatalf("choosing ethernet should open a blank form, got:\n%s", view)
	}
	p.send(keyPress('i'))
	typeText(p, "Office wired")
	p.send(tea.KeyPressMsg{Code: tea.KeyEnter})
	p.send(keyPress('s'))

	if len(f.AddedSettings) != 1 {
		t.Fatalf("saving the wizard should call AddConnection once, got %d", len(f.AddedSettings))
	}
	added := f.AddedSettings[0]
	if got := added["connection"]["id"]; got != "Office wired" {
		t.Errorf("connection.id should be the typed name, got %#v", got)
	}
	if got := added["connection"]["type"]; got != "802-3-ethernet" {
		t.Errorf("connection.type should be ethernet, got %#v", got)
	}
	if got := added["ipv4"]["method"]; got != "auto" {
		t.Errorf("ipv4.method should default to auto, got %#v", got)
	}
}

func TestEditor_EditGreyedOutWhenModifySystemPermissionDenied(t *testing.T) {
	f := seedProfiles()
	f.Perms = domain.Permissions{"org.freedesktop.NetworkManager.settings.modify.system": false}
	p := newPump(t, New(f))

	p.send(keyPress('4'))
	if view := p.view(); !strings.Contains(view, "🔒") {
		t.Errorf("denied modify actions should render locked, got:\n%s", view)
	}

	callsBefore := len(f.Calls)
	for _, denied := range []rune{'e', 'x', 'n'} {
		p.send(keyPress(denied))
		view := p.view()
		if !strings.Contains(view, "LAST USED") {
			t.Errorf("%q must not leave the list when modify is denied, got:\n%s", denied, view)
		}
		if strings.Contains(view, "Delete") || strings.Contains(view, "choose a type") {
			t.Errorf("%q must not open its modal when modify is denied, got:\n%s", denied, view)
		}
		if !strings.Contains(view, "🔒 not permitted (polkit)") {
			t.Errorf("%q should explain the polkit denial in the status line, got:\n%s", denied, view)
		}
	}
	if len(f.Calls) != callsBefore {
		t.Errorf("denied actions must not reach the backend, new calls: %v", f.Calls[callsBefore:])
	}
}

func TestEditor_EscWithDirtyFormAsksSaveOrDiscard(t *testing.T) {
	f := seedStaticWifiProfile()
	p := newPump(t, New(f))

	p.send(keyPress('e'))
	p.send(keyPress('j'))
	p.send(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}) // dirty: flip autoconnect

	p.send(tea.KeyPressMsg{Code: tea.KeyEsc})
	view := p.view()
	if strings.Contains(view, "▂▄▆█") {
		t.Fatalf("esc on a dirty form must not silently discard, got:\n%s", view)
	}
	if !strings.Contains(view, "Unsaved changes") {
		t.Fatalf("esc on a dirty form should ask save/discard, got:\n%s", view)
	}

	p.send(keyPress('d'))
	if view := p.view(); !strings.Contains(view, "▂▄▆█") {
		t.Errorf("discarding should pop back to the scan list, got:\n%s", view)
	}
	if len(f.UpdateCalls) != 0 {
		t.Errorf("discarding must not write anything, got: %#v", f.UpdateCalls)
	}
}

func TestEditor_ActivateAndDeactivateFromOtherList(t *testing.T) {
	f := seedProfiles()
	p := newPump(t, New(f))

	p.send(keyPress('4'))
	if got := p.app().conns.Selected().Name; got != "Work VPN" {
		t.Fatalf("precondition: cursor should be on Work VPN, got %q", got)
	}
	p.send(keyPress('a'))
	if !slices.Contains(f.Calls, "Activate(work-vpn,)") {
		t.Errorf("a should activate the VPN profile letting NM pick the device, calls: %v", f.Calls)
	}

	p.send(keyPress('G'))
	p.send(keyPress('d'))
	if view := p.view(); !strings.Contains(view, "Deactivate docker0?") {
		t.Fatalf("d should confirm before deactivating, got:\n%s", view)
	}
	p.send(keyPress('y'))
	if !slices.Contains(f.Calls, "Deactivate(docker0)") {
		t.Errorf("confirming should deactivate the profile, calls: %v", f.Calls)
	}
}
