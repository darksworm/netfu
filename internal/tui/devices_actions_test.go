package tui

import (
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/ilmars/netfu/internal/backend/fake"
	"github.com/ilmars/netfu/internal/domain"
)

func TestDevices_ActivateSavedProfileOnDisconnectedEthernet(t *testing.T) {
	f := fake.SeedArchLaptop()
	f.DeviceList[1].State = domain.DeviceStateDisconnected
	f.ConnectionList = append(f.ConnectionList,
		domain.Connection{ID: "wired-1", Name: "Wired 1", Type: "802-3-ethernet"})
	p := newPump(t, New(f))

	p.send(keyPress('2'))
	p.send(keyPress('j'))
	p.send(keyPress('a'))

	if !slices.Contains(f.Calls, "Activate(wired-1,enp0s31f6)") {
		t.Errorf("a should activate the matching saved profile on the device, calls: %v", f.Calls)
	}

	// NM reports progress through a device-state event; the row follows it.
	f.DeviceList[1].State = domain.DeviceStateConnecting
	f.Push(domain.Event{Kind: domain.EventDeviceChanged, DeviceName: "enp0s31f6"})
	p.deliverNext()

	if row := lineContaining(t, p.view(), "enp0s31f6"); !strings.Contains(row, "connecting") {
		t.Errorf("row should show the activating state after the device event, got: %s", row)
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
	f.ActiveList = append(f.ActiveList,
		domain.ActiveConnection{ID: "wired-1-active", Name: "Wired 1", DeviceName: "enp0s31f6", State: domain.DeviceStateConnected})
	return f
}

func TestDevices_EnterOnConnectedEthernetOffersDeactivate(t *testing.T) {
	f := seedWiredConnected()
	p := newPump(t, New(f))

	p.send(keyPress('2'))
	p.send(keyPress('j'))
	if got := p.app().devices.Selected().Name; got != "enp0s31f6" {
		t.Fatalf("precondition: cursor should be on enp0s31f6, got %q", got)
	}

	p.send(tea.KeyPressMsg{Code: tea.KeyEnter})
	if view := p.view(); !strings.Contains(view, "Deactivate Wired 1?") {
		t.Fatalf("Enter on a connected device should open a deactivate confirm, got:\n%s", view)
	}
	if slices.Contains(f.Calls, "Deactivate(wired-1-active)") {
		t.Fatal("nothing should be deactivated before the user confirms")
	}

	p.send(keyPress('y'))
	if !slices.Contains(f.Calls, "Deactivate(wired-1-active)") {
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
