package system

import (
	"errors"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/ilmars/netfu/internal/backend/fake"
)

func TestSystem_SpaceOnWifiRadioRowTogglesRadioOffAndOn(t *testing.T) {
	f := fake.SeedArchLaptop()
	m := loadSystem(t, New(f))

	if view := m.View(); !strings.Contains(view, "Wi-Fi radio") || !strings.Contains(view, "on") {
		t.Fatalf("the System tab should show the Wi-Fi radio field as on, got:\n%s", view)
	}

	m = pressAndDeliver(m, keyPress('j')) // hostname → radio row
	m = pressAndDeliver(m, keyPress(' '))
	if !slices.Contains(f.Calls, "SetWifiEnabled(false)") {
		t.Errorf("space on the radio row should turn wifi off, calls: %v", f.Calls)
	}
	if view := m.View(); !strings.Contains(view, "off") {
		t.Errorf("the radio field should show off after toggling, got:\n%s", view)
	}

	m = pressAndDeliver(m, keyPress(' '))
	if !slices.Contains(f.Calls, "SetWifiEnabled(true)") {
		t.Errorf("a second space should turn wifi back on, calls: %v", f.Calls)
	}
	if view := m.View(); !strings.Contains(view, "Wi-Fi radio:             on") {
		t.Errorf("the radio field should show on again, got:\n%s", view)
	}
}

func TestSystem_ShowsNMStateLineWithStartHintWhenUnreachable(t *testing.T) {
	f := fake.SeedArchLaptop()
	m := loadSystem(t, New(f))
	if view := m.View(); !strings.Contains(view, "NetworkManager:          connected") {
		t.Errorf("the System tab should show NM's state, got:\n%s", view)
	}

	f.Errs["Hostname"] = errors.New("dbus: no such service")
	m = loadSystem(t, m)
	view := m.View()
	if !strings.Contains(view, "systemctl start NetworkManager") {
		t.Errorf("an unreachable backend should show the fix as a hint, got:\n%s", view)
	}
}

func keyPress(r rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: r, Text: string(r)}
}

// loadSystem runs the screen's Init cmd synchronously and applies its msg.
func loadSystem(t *testing.T, m Model) Model {
	t.Helper()
	cmd := m.Init()
	if cmd == nil {
		t.Fatal("Init should return a cmd that loads the system screen")
	}
	m, _ = m.Update(cmd())
	return m
}

// pressAndDeliver applies a key and runs any returned cmd's msg back in.
func pressAndDeliver(m Model, msg tea.Msg) Model {
	m, cmd := m.Update(msg)
	if cmd != nil {
		m, _ = m.Update(cmd())
	}
	return m
}

func TestSystem_ReadsRadioAndNMStateFromBackendInsteadOfAssuming(t *testing.T) {
	f := fake.SeedArchLaptop()
	f.WifiOn = false
	f.NMStateValue = "disconnected"
	m := loadSystem(t, New(f))

	view := m.View()
	if !strings.Contains(view, "Wi-Fi radio:             off") {
		t.Errorf("the radio row should show the backend's off state, got:\n%s", view)
	}
	if !strings.Contains(view, "NetworkManager:          disconnected") {
		t.Errorf("the NM line should show the backend's reported state, got:\n%s", view)
	}
}
