package tui

import (
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/ilmars/netfu/internal/backend/fake"
	"github.com/ilmars/netfu/internal/domain"
)

func TestEthernet_TabShowsDeviceDetail(t *testing.T) {
	f := seedWiredConnected()
	p := newPump(t, New(f))

	p.send(keyPress('2'))
	view := p.view()
	for _, want := range []string{
		"Device enp0s31f6",
		"Type:", "ethernet",
		"State:", "connected",
		"Active connection:", "Wired 1",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("the ethernet tab should show %q, got:\n%s", want, view)
		}
	}
}

func TestEthernet_SelectionDefaultsToTheMostRecentlyUsedProfile(t *testing.T) {
	f := fake.SeedArchLaptop()
	f.DeviceList[1].State = domain.DeviceStateDisconnected
	f.ConnectionList = append(f.ConnectionList,
		domain.Connection{ID: "wired-old", Name: "Old Wired", Type: "802-3-ethernet",
			LastUsedUnix: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).Unix()},
		domain.Connection{ID: "wired-1", Name: "Wired 1", Type: "802-3-ethernet",
			LastUsedUnix: time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC).Unix()},
	)
	p := newPump(t, New(f))

	p.send(keyPress('2'))
	p.send(keyPress('a'))

	if !slices.Contains(f.Calls, "Activate(wired-1,enp0s31f6)") {
		t.Errorf("a should activate the most recently used wired profile on the device, calls: %v", f.Calls)
	}

	// NM reports progress through a device-state event; the detail follows it.
	f.DeviceList[1].State = domain.DeviceStateConnecting
	f.Push(domain.Event{Kind: domain.EventDeviceChanged, DeviceName: "enp0s31f6"})
	p.deliverNext()

	if row := lineContaining(t, p.view(), "State:"); !strings.Contains(row, "connecting") {
		t.Errorf("the detail should show the new device state, got: %s", row)
	}
}

func TestEthernet_AWithoutAWiredProfileExplainsInsteadOfActivating(t *testing.T) {
	f := fake.SeedArchLaptop()
	f.DeviceList[1].State = domain.DeviceStateDisconnected
	p := newPump(t, New(f))

	p.send(keyPress('2'))
	p.send(keyPress('a'))

	if len(f.ActivateCalls) != 0 {
		t.Errorf("without a wired profile nothing should activate, calls: %#v", f.ActivateCalls)
	}
	if view := p.view(); !strings.Contains(view, "no wired profile") {
		t.Errorf("the status line should explain there is nothing to activate, got:\n%s", view)
	}
}

// seedWiredProfiles returns the arch laptop fixture with three wired
// profiles: one unpinned, one pinned to the built-in NIC, one pinned to an
// absent USB NIC.
func seedWiredProfiles() *fake.Fake {
	f := fake.SeedArchLaptop()
	f.DeviceList[1].State = domain.DeviceStateDisconnected
	f.ConnectionList = append(f.ConnectionList,
		domain.Connection{ID: "office-lan", Name: "Office LAN", Type: "802-3-ethernet",
			LastUsedUnix: time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC).Unix()},
		domain.Connection{ID: "dock", Name: "Dock", Type: "802-3-ethernet"},
		domain.Connection{ID: "usb-dock", Name: "USB Dock", Type: "802-3-ethernet"},
	)
	f.SettingsByID["office-lan"] = domain.ConnectionSettings{
		"connection": {"id": "Office LAN", "uuid": "office-lan", "type": "802-3-ethernet"},
	}
	f.SettingsByID["dock"] = domain.ConnectionSettings{
		"connection": {"id": "Dock", "uuid": "dock", "type": "802-3-ethernet", "interface-name": "enp0s31f6"},
	}
	f.SettingsByID["usb-dock"] = domain.ConnectionSettings{
		"connection": {"id": "USB Dock", "uuid": "usb-dock", "type": "802-3-ethernet", "interface-name": "enp5s0u1"},
	}
	return f
}

func TestEthernet_TabListsWiredProfilesUsableOnThisNIC(t *testing.T) {
	f := seedWiredProfiles()
	p := newPump(t, New(f))

	p.send(keyPress('2'))
	view := p.view()
	for _, col := range []string{"NAME", "LAST USED"} {
		if !strings.Contains(view, col) {
			t.Errorf("the profile list should have a %s column header, got:\n%s", col, view)
		}
	}
	if !strings.Contains(view, "Office LAN") {
		t.Errorf("an unpinned wired profile is usable on this NIC and should be listed, got:\n%s", view)
	}
	if row := lineContaining(t, view, "Office LAN"); !strings.Contains(row, "2026-06-30") {
		t.Errorf("the row should show the profile's last-used date, got: %s", row)
	}
	if !strings.Contains(view, "Dock") {
		t.Errorf("a profile pinned to this NIC should be listed, got:\n%s", view)
	}
	if strings.Contains(view, "USB Dock") {
		t.Errorf("a profile pinned to another interface is not usable here, got:\n%s", view)
	}
	if never := lineContaining(t, view, "Dock"); !strings.Contains(never, "never") {
		t.Errorf("a never-used profile should say so, got: %q", never)
	}
}

func TestEthernet_JMovesTheSelectionAndAActivatesTheSelectedProfile(t *testing.T) {
	f := seedWiredProfiles()
	p := newPump(t, New(f))

	p.send(keyPress('2'))
	// Rows sort by last used, so Office LAN (2026) precedes never-used Dock.
	p.send(keyPress('j'))
	p.send(keyPress('a'))

	if !slices.Contains(f.Calls, "Activate(dock,enp0s31f6)") {
		t.Errorf("a should activate the selected profile on this NIC, calls: %v", f.Calls)
	}
	if view := p.view(); !strings.Contains(view, "Activating Dock") {
		t.Errorf("the status line should report the activation, got:\n%s", view)
	}
}

func TestEthernet_EnterActivatesTheSelectedProfile(t *testing.T) {
	f := seedWiredProfiles()
	p := newPump(t, New(f))

	p.send(keyPress('2'))
	p.send(tea.KeyPressMsg{Code: tea.KeyEnter})

	if !slices.Contains(f.Calls, "Activate(office-lan,enp0s31f6)") {
		t.Errorf("enter should activate the selected profile, calls: %v", f.Calls)
	}
}

func TestEthernet_ActivatingTheAlreadyActiveProfileDoesNothing(t *testing.T) {
	f := seedWiredConnected()
	p := newPump(t, New(f))

	p.send(keyPress('2'))
	p.send(keyPress('a'))

	if len(f.ActivateCalls) != 0 {
		t.Errorf("the selected profile is already active; nothing should activate, calls: %#v", f.ActivateCalls)
	}
}

func TestEthernet_EOpensTheSelectedProfilesEditorAndQPopsIt(t *testing.T) {
	f := seedWiredProfiles()
	p := newPump(t, New(f))

	p.send(keyPress('2'))
	p.send(keyPress('e'))
	view := p.view()
	if !strings.Contains(view, "Autoconnect") || !strings.Contains(view, "IPv4") {
		t.Fatalf("e should push the selected profile's editor, got:\n%s", view)
	}
	if name := lineContaining(t, view, "Name"); !strings.Contains(name, "Office LAN") {
		t.Errorf("the editor should load the selected profile, got: %q", name)
	}

	p.send(keyPress('q'))
	if containsQuit(p.msgs) {
		t.Fatal("q inside the editor should pop it, not quit the app")
	}
	if view := p.view(); strings.Contains(view, "Autoconnect") {
		t.Errorf("q should have popped the editor back to the device tab, got:\n%s", view)
	}
}

func TestEthernet_SavingTheEditorWritesTheProfileAndPopsBack(t *testing.T) {
	f := seedWiredProfiles()
	p := newPump(t, New(f))

	p.send(keyPress('2'))
	p.send(keyPress('e'))
	p.send(keyPress('s'))

	if len(f.UpdateCalls) != 1 || f.UpdateCalls[0].ConnectionID != "office-lan" {
		t.Fatalf("s should save the edited profile, got %#v", f.UpdateCalls)
	}
	view := p.view()
	if strings.Contains(view, "Autoconnect") {
		t.Errorf("saving should pop the editor back to the device tab, got:\n%s", view)
	}
	if !strings.Contains(view, "✓ saved Office LAN") {
		t.Errorf("the status line should report the save, got:\n%s", view)
	}
}

func TestEthernet_XDeletesTheSelectedProfileAfterConfirm(t *testing.T) {
	f := seedWiredProfiles()
	p := newPump(t, New(f))

	p.send(keyPress('2'))
	p.send(keyPress('x'))
	if view := p.view(); !strings.Contains(view, "Delete Office LAN?") {
		t.Fatalf("x should open a delete confirm for the selected profile, got:\n%s", view)
	}

	p.send(keyPress('n'))
	if len(f.DeleteCalls) != 0 {
		t.Fatal("declining the confirm must not delete the profile")
	}

	p.send(keyPress('x'))
	p.send(keyPress('y'))
	if len(f.DeleteCalls) != 1 || f.DeleteCalls[0] != "office-lan" {
		t.Errorf("confirming should delete the selected profile, got deletes %v", f.DeleteCalls)
	}
	view := p.view()
	if !strings.Contains(view, "✓ deleted Office LAN") {
		t.Errorf("the status line should report the delete, got:\n%s", view)
	}
	if row := lineContaining(t, view, "Office LAN"); strings.Contains(row, "2026-06-30") {
		t.Errorf("the deleted profile should leave the list, got row: %s", row)
	}
}

func TestEthernet_NCreatesAWiredProfilePinnedToThisNIC(t *testing.T) {
	f := seedWiredProfiles()
	p := newPump(t, New(f))

	p.send(keyPress('2'))
	p.send(keyPress('n'))
	view := p.view()
	if !strings.Contains(view, "New connection") || !strings.Contains(view, "IPv4") {
		t.Fatalf("n should open the new-profile wizard directly (the type is known), got:\n%s", view)
	}

	p.send(keyPress('i'))
	for _, r := range "Desk dock" {
		p.send(keyPress(r))
	}
	p.send(tea.KeyPressMsg{Code: tea.KeyEnter})
	p.send(keyPress('s'))

	if len(f.AddedSettings) != 1 {
		t.Fatalf("saving the wizard should call AddConnection once, got %d", len(f.AddedSettings))
	}
	added := f.AddedSettings[0]
	if got := added["connection"]["id"]; got != "Desk dock" {
		t.Errorf("connection.id should be the typed name, got %#v", got)
	}
	if got := added["connection"]["type"]; got != "802-3-ethernet" {
		t.Errorf("connection.type should be ethernet, got %#v", got)
	}
	if got := added["connection"]["interface-name"]; got != "enp0s31f6" {
		t.Errorf("the new profile should be pinned to this NIC, got %#v", got)
	}
}

func TestEthernet_ModifyActionsLockedWhenModifySystemPermissionDenied(t *testing.T) {
	f := seedWiredProfiles()
	f.Perms = domain.Permissions{"org.freedesktop.NetworkManager.settings.modify.system": false}
	p := newPump(t, New(f))

	p.send(keyPress('2'))
	if view := p.view(); !strings.Contains(view, "🔒") {
		t.Errorf("denied modify actions should render locked, got:\n%s", view)
	}

	callsBefore := len(f.Calls)
	for _, denied := range []rune{'e', 'x', 'n'} {
		p.send(keyPress(denied))
		view := p.view()
		if !strings.Contains(view, "Device enp0s31f6") {
			t.Errorf("%q must not leave the device tab when modify is denied, got:\n%s", denied, view)
		}
		if strings.Contains(view, "Delete") || strings.Contains(view, "New connection") {
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

func TestEthernet_ActiveProfileRowShowsActiveMark(t *testing.T) {
	f := seedWiredConnected()
	p := newPump(t, New(f))

	p.send(keyPress('2'))
	marked := false
	for _, line := range strings.Split(p.view(), "\n") {
		if strings.Contains(line, "Wired 1") && strings.Contains(line, "✓") {
			marked = true
		}
	}
	if !marked {
		t.Errorf("the active profile's row should carry the active mark, got:\n%s", p.view())
	}
}

// seedWiredConnected returns the arch laptop fixture with the ethernet
// device up on a saved "Wired 1" profile.
func seedWiredConnected() *fake.Fake {
	f := fake.SeedArchLaptop()
	f.DeviceList[1].State = domain.DeviceStateConnected
	f.DeviceList[1].ActiveConnection = "Wired 1"
	f.ConnectionList = append(f.ConnectionList,
		domain.Connection{ID: "wired-1", Name: "Wired 1", Type: "802-3-ethernet"})
	// The active connection carries the profile's UUID, like the NM adapter.
	f.ActiveList = append(f.ActiveList,
		domain.ActiveConnection{ID: "wired-1", Name: "Wired 1", DeviceName: "enp0s31f6", State: domain.DeviceStateConnected})
	return f
}

func TestEthernet_DOnConnectedDeviceConfirmsBeforeDeactivating(t *testing.T) {
	f := seedWiredConnected()
	p := newPump(t, New(f))

	p.send(keyPress('2'))
	p.send(keyPress('d'))
	if view := p.view(); !strings.Contains(view, "Deactivate Wired 1?") {
		t.Fatalf("d on a connected device should open a deactivate confirm, got:\n%s", view)
	}
	if slices.Contains(f.Calls, "Deactivate(wired-1)") {
		t.Fatal("nothing should be deactivated before the user confirms")
	}

	p.send(keyPress('y'))
	if !slices.Contains(f.Calls, "Deactivate(wired-1)") {
		t.Errorf("confirming should deactivate the device's active connection, calls: %v", f.Calls)
	}
	view := p.view()
	if strings.Contains(view, "Deactivate Wired 1?") {
		t.Errorf("confirming should close the modal, got:\n%s", view)
	}
	if !strings.Contains(view, "Deactivating Wired 1") {
		t.Errorf("the status line should report the deactivation, got:\n%s", view)
	}
}
