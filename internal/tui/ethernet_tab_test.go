package tui

import (
	"slices"
	"strings"
	"testing"
	"time"

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

func TestEthernet_AActivatesTheBestMatchingWiredProfile(t *testing.T) {
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

func TestEthernet_DOnConnectedDeviceConfirmsBeforeDeactivating(t *testing.T) {
	f := seedWiredConnected()
	p := newPump(t, New(f))

	p.send(keyPress('2'))
	p.send(keyPress('d'))
	if view := p.view(); !strings.Contains(view, "Deactivate Wired 1?") {
		t.Fatalf("d on a connected device should open a deactivate confirm, got:\n%s", view)
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
