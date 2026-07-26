package devices

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/ilmars/netfu/internal/backend/fake"
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
