package devices

import (
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/ilmars/netfu/internal/backend/fake"
	"github.com/ilmars/netfu/internal/domain"
)

func TestDevices_ListShowsManagedDevicesWithStateAndActiveConnection(t *testing.T) {
	f := fake.SeedArchLaptop()
	m := New(f)
	m = loadDevices(t, m)

	view := m.View()
	for _, want := range []string{
		"wlan0", "wifi", "connected", "Our House 1",
		"enp0s31f6", "ethernet", "unavailable",
		"docker0", "bridge",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("device list should contain %q, got:\n%s", want, view)
		}
	}
	if strings.Contains(view, "veth1a2b3c") {
		t.Errorf("unmanaged devices should be hidden, got:\n%s", view)
	}
}

func TestDevices_JKMoveSelection_GAndShiftGJumpEnds(t *testing.T) {
	f := fake.SeedArchLaptop()
	m := New(f)
	m = loadDevices(t, m)

	assertSelected := func(step, want string) {
		t.Helper()
		if got := m.Selected().Name; got != want {
			t.Errorf("after %s: selected %q, want %q", step, got, want)
		}
	}

	assertSelected("load", "wlan0")

	m, _ = m.Update(keyPress('j'))
	assertSelected("j", "enp0s31f6")

	m, _ = m.Update(keyPress('k'))
	assertSelected("k", "wlan0")

	m, _ = m.Update(keyPress('k'))
	assertSelected("k at top", "wlan0")

	m, _ = m.Update(keyPress('G'))
	assertSelected("G", "docker0")

	m, _ = m.Update(keyPress('j'))
	assertSelected("j at bottom", "docker0")

	m, _ = m.Update(keyPress('g'))
	assertSelected("g", "wlan0")
}

func TestDevices_DOffersDeactivateConfirmAndEscCancels(t *testing.T) {
	f := fake.SeedArchLaptop()
	f.DeviceList[1].State = domain.DeviceStateConnected
	f.DeviceList[1].ActiveConnection = "Wired 1"
	f.ActiveList = append(f.ActiveList,
		domain.ActiveConnection{ID: "wired-1-active", Name: "Wired 1", DeviceName: "enp0s31f6", State: domain.DeviceStateConnected})
	m := New(f)
	m = loadDevices(t, m)
	m, _ = m.Update(keyPress('j'))

	m, _ = m.Update(keyPress('d'))
	if overlay := m.Overlay(); !strings.Contains(overlay, "Deactivate Wired 1?") {
		t.Fatalf("d on a connected device should open the deactivate confirm, got:\n%s", overlay)
	}

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if overlay := m.Overlay(); strings.Contains(overlay, "Deactivate Wired 1?") {
		t.Fatalf("esc should dismiss the confirm, got:\n%s", overlay)
	}
	if len(f.Calls) != 0 {
		t.Errorf("cancelling should not touch the backend, calls: %v", f.Calls)
	}

	m, _ = m.Update(keyPress('d'))
	m, cmd := m.Update(keyPress('y'))
	if cmd == nil {
		t.Fatal("confirming should return the deactivate cmd")
	}
	cmd()
	if !slices.Contains(f.Calls, "Deactivate(wired-1-active)") {
		t.Errorf("confirming should deactivate the active connection, calls: %v", f.Calls)
	}
}

func TestDevices_SlashFilterNarrowsListByName(t *testing.T) {
	f := fake.SeedArchLaptop()
	m := New(f)
	m = loadDevices(t, m)
	m, _ = m.Update(keyPress('G'))
	if got := m.Selected().Name; got != "docker0" {
		t.Fatalf("precondition: cursor should be on docker0, got %q", got)
	}

	m, _ = m.Update(keyPress('/'))
	m, _ = m.Update(keyPress('w'))

	view := m.View()
	if !strings.Contains(view, "wlan0") {
		t.Errorf("filter 'w' should keep wlan0 visible, got:\n%s", view)
	}
	for _, hidden := range []string{"docker0", "enp0s31f6"} {
		if strings.Contains(view, hidden) {
			t.Errorf("filter 'w' should hide %s, got:\n%s", hidden, view)
		}
	}
	if got := m.Selected().Name; got != "wlan0" {
		t.Errorf("cursor should clamp to the visible set, selected %q", got)
	}

	// A live reload must not drop the filter.
	m = loadDevices(t, m)
	if view := m.View(); strings.Contains(view, "docker0") {
		t.Errorf("filter should survive re-renders, got:\n%s", view)
	}

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if view := m.View(); !strings.Contains(view, "docker0") {
		t.Errorf("esc should clear the filter and show all rows, got:\n%s", view)
	}
}

func TestDevices_ListShowsPhysicalDevicesBeforeVirtual(t *testing.T) {
	f := fake.New()
	f.DeviceList = []domain.Device{
		{Name: "docker0", Type: domain.DeviceTypeBridge, State: domain.DeviceStateConnected, Managed: true},
		{Name: "wlan0", Type: domain.DeviceTypeWifi, State: domain.DeviceStateConnected, Managed: true},
	}
	m := New(f)
	m = loadDevices(t, m)

	view := m.View()
	if strings.Index(view, "wlan0") > strings.Index(view, "docker0") {
		t.Errorf("physical devices should be listed before virtual ones, got:\n%s", view)
	}
}

func keyPress(r rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: r, Text: string(r)}
}

// loadDevices runs the screen's Init cmd synchronously and applies its msg.
func loadDevices(t *testing.T, m Model) Model {
	t.Helper()
	cmd := m.Init()
	if cmd == nil {
		t.Fatal("Init should return a cmd that loads devices")
	}
	m, _ = m.Update(cmd())
	return m
}
