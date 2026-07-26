package tui

import (
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/ilmars/netfu/internal/backend/fake"
	"github.com/ilmars/netfu/internal/domain"
)

func TestVPN_ExistingVPNProfileCanBeActivatedAndDeactivatedFromOther(t *testing.T) {
	f := fake.SeedArchLaptop()
	f.ConnectionList = append(f.ConnectionList, domain.Connection{ID: "work-vpn", Name: "Work VPN", Type: "vpn"})
	p := newPump(t, New(f))

	p.send(keyPress('4'))
	if view := p.view(); !strings.Contains(view, "Work VPN") {
		t.Fatalf("the Other tab should list the saved VPN profile, got:\n%s", view)
	}

	p.send(keyPress('g'))
	p.send(keyPress('a'))
	if len(f.ActivateCalls) != 1 || f.ActivateCalls[0].ConnectionID != "work-vpn" {
		t.Fatalf("a on the VPN row should activate it, calls: %#v", f.ActivateCalls)
	}

	// NM reports the VPN up; the event triggers a live reload.
	f.ActiveList = append(f.ActiveList,
		domain.ActiveConnection{ID: "work-vpn", Name: "Work VPN", DeviceName: "tun0", State: domain.DeviceStateConnected})
	f.Push(domain.Event{Kind: domain.EventConnectionChanged})
	p.deliverNext()

	if row := lineContaining(t, p.view(), "Work VPN"); !strings.Contains(row, "✓") {
		t.Fatalf("the active VPN row should show its active mark, got: %s", row)
	}
	p.send(keyPress('g'))
	p.send(keyPress('d'))
	p.send(keyPress('y'))
	if !slices.Contains(f.Calls, "Deactivate(work-vpn)") {
		t.Errorf("d on the active VPN row should deactivate it after confirm, calls: %v", f.Calls)
	}
}

func TestSystem_StaysPureSettingsWithoutAnActiveConnectionsSection(t *testing.T) {
	f := fake.SeedArchLaptop()
	f.ConnectionList = append(f.ConnectionList, domain.Connection{ID: "work-vpn", Name: "Work VPN", Type: "vpn"})
	p := newPump(t, New(f))

	p.send(keyPress('5'))
	view := p.view()
	if strings.Contains(view, "Active connections") {
		t.Errorf("connection activation lives on Other now; System should drop the section, got:\n%s", view)
	}
	if strings.Contains(view, "Work VPN") || strings.Contains(view, "Our House 1") {
		t.Errorf("System should show settings only, not connections, got:\n%s", view)
	}
	for _, want := range []string{"Hostname", "Wi-Fi radio", "NetworkManager:"} {
		if !strings.Contains(view, want) {
			t.Errorf("System should keep %q, got:\n%s", want, view)
		}
	}
}

func TestHostname_ShowsCurrentAndSavesNewName(t *testing.T) {
	f := fake.SeedArchLaptop()
	p := newPump(t, New(f))

	p.send(keyPress('5'))
	if view := p.view(); !strings.Contains(view, "archbook") {
		t.Fatalf("System tab should show the current hostname, got:\n%s", view)
	}

	// The digit must reach the editor, not switch tabs.
	p.send(keyPress('i'))
	for _, r := range "-2" {
		p.send(keyPress(r))
	}
	p.send(tea.KeyPressMsg{Code: tea.KeyEnter})

	if !slices.Contains(f.Calls, "SetHostname(archbook-2)") {
		t.Errorf("saving should call SetHostname with the edited name, calls: %v", f.Calls)
	}
	view := p.view()
	if !strings.Contains(view, "✓ hostname set to archbook-2") {
		t.Errorf("the status line should confirm the save, got:\n%s", view)
	}
	if !strings.Contains(view, "archbook-2") {
		t.Errorf("the hostname field should show the new name, got:\n%s", view)
	}
}

func TestHostname_ScreenGreyedOutWhenPolkitDenies(t *testing.T) {
	f := fake.SeedArchLaptop()
	f.Perms = domain.Permissions{"org.freedesktop.NetworkManager.settings.modify.hostname": false}
	p := newPump(t, New(f))

	p.send(keyPress('5'))
	if view := p.view(); !strings.Contains(view, "🔒") {
		t.Errorf("the hostname field should render locked when polkit denies, got:\n%s", view)
	}

	p.send(keyPress('i'))
	if view := p.view(); !strings.Contains(view, "not permitted (polkit)") {
		t.Errorf("an editing attempt should explain the polkit denial, got:\n%s", view)
	}
	for _, call := range f.Calls {
		if strings.HasPrefix(call, "SetHostname") {
			t.Errorf("a denied edit must not reach the backend, calls: %v", f.Calls)
		}
	}
}
