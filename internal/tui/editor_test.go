package tui

import (
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
// a VPN profile, so the Connections tab has every group to show.
func seedProfiles() *fake.Fake {
	f := fake.SeedArchLaptop()
	f.ConnectionList = append(f.ConnectionList,
		domain.Connection{
			ID: "office-lan", Name: "Office LAN", Type: "802-3-ethernet",
			LastUsedUnix: time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC).Unix(),
		},
		domain.Connection{ID: "work-vpn", Name: "Work VPN", Type: "vpn"},
	)
	return f
}

func TestEditor_ListShowsAllProfilesGroupedByType(t *testing.T) {
	f := seedProfiles()
	p := newPump(t, New(f))

	p.send(keyPress('3'))
	view := p.view()

	for _, col := range []string{"NAME", "TYPE", "DEVICE", "LAST USED"} {
		if !strings.Contains(view, col) {
			t.Errorf("list should have a %s column header, got:\n%s", col, view)
		}
	}
	for _, group := range []string{"─ Wi-Fi ─", "─ Ethernet ─", "─ VPN ─"} {
		if !strings.Contains(view, group) {
			t.Errorf("profiles should be grouped under %q, got:\n%s", group, view)
		}
	}
	for _, name := range []string{"Our House 1", "Our House 5G", "Summer House", "Office LAN", "Work VPN"} {
		if !strings.Contains(view, name) {
			t.Errorf("all profiles should be listed, missing %q in:\n%s", name, view)
		}
	}

	active := lineContaining(t, view, "Our House 1")
	if !strings.Contains(active, "wlan0") || !strings.Contains(active, "✓") {
		t.Errorf("the active profile should show its device and an active mark, got: %q", active)
	}
	ethernet := lineContaining(t, view, "Office LAN")
	if !strings.Contains(ethernet, "2026-06-30") {
		t.Errorf("a profile's last successful activation date should show, got: %q", ethernet)
	}
	if never := lineContaining(t, view, "Work VPN"); !strings.Contains(never, "never") {
		t.Errorf("a never-used profile should say so, got: %q", never)
	}
}

func TestEditor_DeleteConnectionAsksConfirmThenCallsDelete(t *testing.T) {
	f := seedProfiles()
	p := newPump(t, New(f))

	p.send(keyPress('3'))
	p.send(keyPress('x'))
	if view := p.view(); !strings.Contains(view, "Delete Our House 1?") {
		t.Fatalf("x should ask for confirmation first, got:\n%s", view)
	}
	if slices.Contains(f.Calls, "DeleteConnection(our-house-1)") {
		t.Fatal("nothing should be deleted before the user confirms")
	}

	p.send(keyPress('y'))
	if !slices.Contains(f.Calls, "DeleteConnection(our-house-1)") {
		t.Errorf("confirming should delete the profile, calls: %v", f.Calls)
	}
	if view := p.view(); strings.Contains(view, "Delete Our House 1?") {
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

	p.send(keyPress('3'))
	p.send(keyPress('e'))
	view := p.view()

	if strings.Contains(view, "LAST USED") {
		t.Errorf("the editor should replace the list, got:\n%s", view)
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

	p.send(keyPress('3'))
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
	if !strings.Contains(view, "LAST USED") {
		t.Errorf("a successful save should pop back to the list, got:\n%s", view)
	}
	if !strings.Contains(view, "✓ saved Our House 1") {
		t.Errorf("the status line should report the save, got:\n%s", view)
	}
}

func TestEditor_InvalidIPv4StaticAddressBlocksSaveWithFieldError(t *testing.T) {
	f := seedStaticWifiProfile()
	p := newPump(t, New(f))

	p.send(keyPress('3'))
	p.send(keyPress('e'))

	// Down to the Address field: Name, Autoconnect, SSID, Security, Method.
	for range 5 {
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

	p.send(keyPress('3'))
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

	p.send(keyPress('3'))
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

	p.send(keyPress('3'))
	p.send(keyPress('e'))
	p.send(keyPress('j'))
	p.send(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}) // dirty: flip autoconnect

	p.send(tea.KeyPressMsg{Code: tea.KeyEsc})
	view := p.view()
	if strings.Contains(view, "LAST USED") {
		t.Fatalf("esc on a dirty form must not silently discard, got:\n%s", view)
	}
	if !strings.Contains(view, "Unsaved changes") {
		t.Fatalf("esc on a dirty form should ask save/discard, got:\n%s", view)
	}

	p.send(keyPress('d'))
	if view := p.view(); !strings.Contains(view, "LAST USED") {
		t.Errorf("discarding should pop back to the list, got:\n%s", view)
	}
	if len(f.UpdateCalls) != 0 {
		t.Errorf("discarding must not write anything, got: %#v", f.UpdateCalls)
	}
}

func TestEditor_ActivateAndDeactivateFromConnectionsList(t *testing.T) {
	f := seedProfiles()
	p := newPump(t, New(f))

	p.send(keyPress('3'))
	// Down to Office LAN, past the three wifi profiles.
	for range 3 {
		p.send(keyPress('j'))
	}
	p.send(keyPress('a'))
	if !slices.Contains(f.Calls, "Activate(office-lan,enp0s31f6)") {
		t.Errorf("a should activate the profile on the matching device, calls: %v", f.Calls)
	}

	p.send(keyPress('g'))
	p.send(keyPress('d'))
	if view := p.view(); !strings.Contains(view, "Deactivate Our House 1?") {
		t.Fatalf("d should confirm before deactivating, got:\n%s", view)
	}
	p.send(keyPress('y'))
	if !slices.Contains(f.Calls, "Deactivate(our-house-1)") {
		t.Errorf("confirming should deactivate the profile, calls: %v", f.Calls)
	}
}
