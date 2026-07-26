package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/ilmars/netfu/internal/backend/fake"
	"github.com/ilmars/netfu/internal/domain"
	"github.com/ilmars/netfu/internal/tui/screens/wifi"
)

func TestWifi_OpeningScreenTriggersScanAndShowsSpinner(t *testing.T) {
	f := fake.SeedArchLaptop()
	p := newPump(t, New(f))

	scanned := false
	for _, call := range f.Calls {
		if call == "RequestScan" {
			scanned = true
		}
	}
	if !scanned {
		t.Error("opening the Wi-Fi tab should request a scan")
	}

	view := p.view()
	if !strings.Contains(view, "scan ⟳") {
		t.Errorf("a pending scan should show the scan indicator, got:\n%s", view)
	}
	if !strings.Contains(view, "Our House 1") {
		t.Errorf("the screen should load and list the known APs while scanning, got:\n%s", view)
	}
}

func TestWifi_ScanResultsRenderDedupedSortedWithSignalBars(t *testing.T) {
	f := fake.SeedArchLaptop()
	p := newPump(t, New(f))
	view := p.view()

	if got := strings.Count(view, "Our House 1"); got != 1 {
		t.Errorf("duplicate BSSIDs should collapse to one row per SSID, got %d occurrences:\n%s", got, view)
	}

	// Buckets: active, then saved in range by signal, then unsaved by signal.
	inOrder := []string{"Our House 1", "Our House 5G", "Neighbors", "CafeGuest", "(hidden)"}
	last := -1
	for _, ssid := range inOrder {
		i := strings.Index(view, ssid)
		if i < 0 {
			t.Fatalf("row for %q missing, got:\n%s", ssid, view)
		}
		if i < last {
			t.Errorf("%q should render after the previous row, want order %v got:\n%s", ssid, inOrder, view)
		}
		last = i
	}

	rowAnatomy := map[string][]string{
		"Our House 1":  {"▂▄▆█", "82%", "WPA2", "✓ connected"},
		"Our House 5G": {"▂▄▆", "61%", "WPA3", "⋆ saved"},
		"Neighbors":    {"▂▄▆", "68%", "WPA2"},
		"CafeGuest":    {"▂▄", "47%", "open"},
		"(hidden)":     {"▂▄", "33%", "WPA2"},
	}
	for ssid, wants := range rowAnatomy {
		row := lineContaining(t, view, ssid)
		for _, want := range wants {
			if !strings.Contains(row, want) {
				t.Errorf("row for %q should contain %q, got: %s", ssid, want, row)
			}
		}
	}
	strongest := lineContaining(t, view, "Our House 1")
	if strings.Contains(strongest, "55%") {
		t.Errorf("dedupe should keep the strongest BSSID, got: %s", strongest)
	}
}

func TestWifi_APStrengthEventUpdatesRowWithoutLosingCursor(t *testing.T) {
	f := fake.SeedArchLaptop()
	p := newPump(t, New(f))

	p.send(keyPress('j'))
	p.send(keyPress('j'))
	if got := p.app().wifi.Selected().SSID; got != "Neighbors" {
		t.Fatalf("precondition: cursor should be on Neighbors, got %q", got)
	}

	f.APList[1].Strength = 71
	f.Push(domain.Event{Kind: domain.EventAPStrength, SSID: "Neighbors", Strength: 71})
	p.deliverNext()

	if row := lineContaining(t, p.view(), "Neighbors"); !strings.Contains(row, "71%") {
		t.Errorf("the row should show the new strength, got: %s", row)
	}
	if got := p.app().wifi.Selected().SSID; got != "Neighbors" {
		t.Errorf("a strength update should keep the cursor on Neighbors, got %q", got)
	}
}

func TestWifi_APListChangedEventRefreshesListPreservingSelection(t *testing.T) {
	f := fake.SeedArchLaptop()
	p := newPump(t, New(f))

	p.send(keyPress('G'))
	p.send(keyPress('k'))
	if got := p.app().wifi.Selected().SSID; got != "CafeGuest" {
		t.Fatalf("precondition: cursor should be on CafeGuest, got %q", got)
	}

	// A fresh scan finds a new strong network that sorts above CafeGuest.
	f.APList = append(f.APList, domain.AccessPoint{
		SSID: "PopUp Market", Strength: 75, BSSID: "AA:BB:CC:77:77:77", Security: domain.SecurityWPA2,
	})
	f.Push(domain.Event{Kind: domain.EventAPListChanged, DeviceName: "wlan0"})
	p.deliverNext()

	view := p.view()
	if !strings.Contains(view, "PopUp Market") {
		t.Errorf("the refreshed list should show the new network, got:\n%s", view)
	}
	if strings.Contains(view, "scan ⟳") {
		t.Errorf("fresh scan results should clear the scan indicator, got:\n%s", view)
	}
	if got := p.app().wifi.Selected().SSID; got != "CafeGuest" {
		t.Errorf("selection should follow the SSID, not the row index; got %q", got)
	}
}

func TestWifi_EnterOnOpenNetworkActivatesImmediately(t *testing.T) {
	f := fake.SeedArchLaptop()
	p := newPump(t, New(f))

	p.send(keyPress('G'))
	p.send(keyPress('k'))
	if got := p.app().wifi.Selected().SSID; got != "CafeGuest" {
		t.Fatalf("precondition: cursor should be on CafeGuest, got %q", got)
	}

	p.send(tea.KeyPressMsg{Code: tea.KeyEnter})

	if len(f.JoinCalls) != 1 {
		t.Fatalf("Enter on an open network should call JoinWifi once, got %d calls (%v)", len(f.JoinCalls), f.Calls)
	}
	join := f.JoinCalls[0]
	if join.SSID != "CafeGuest" || join.Security != domain.SecurityOpen || join.PSK != "" {
		t.Errorf("JoinWifi should target the open network without a PSK, got %+v", join)
	}
}

func TestWifi_EnterOnKnownNetworkActivatesSavedProfileWithoutPrompt(t *testing.T) {
	f := fake.SeedArchLaptop()
	p := newPump(t, New(f))

	p.send(keyPress('j'))
	if got := p.app().wifi.Selected().SSID; got != "Our House 5G" {
		t.Fatalf("precondition: cursor should be on Our House 5G, got %q", got)
	}

	p.send(tea.KeyPressMsg{Code: tea.KeyEnter})

	if len(f.ActivateCalls) != 1 {
		t.Fatalf("Enter on a saved network should call Activate once, got %d calls (%v)", len(f.ActivateCalls), f.Calls)
	}
	got := f.ActivateCalls[0]
	want := fake.ActivateCall{ConnectionID: "our-house-5g", DeviceName: "wlan0"}
	if got != want {
		t.Errorf("Activate should use the saved profile on the wifi device, got %+v want %+v", got, want)
	}
	if len(f.JoinCalls) != 0 {
		t.Errorf("a saved WPA3 network must not go through JoinWifi (no password prompt), got %+v", f.JoinCalls)
	}
}

func TestWifi_ConnectingShowsInlineActivatingStateOnRow(t *testing.T) {
	f := fake.SeedArchLaptop()
	p := newPump(t, New(f))

	p.send(keyPress('j'))
	p.send(tea.KeyPressMsg{Code: tea.KeyEnter})

	view := p.view()
	row := lineContaining(t, view, "Our House 5G")
	if !strings.Contains(row, "◌") {
		t.Errorf("the activating row should show the ◌ gutter, got: %s", row)
	}
	if !strings.Contains(row, "connecting") {
		t.Errorf("the activating row should carry a spinner tag, got: %s", row)
	}
	if !strings.Contains(view, "Connecting to Our House 5G…") {
		t.Errorf("the status line should announce the connect, got:\n%s", view)
	}
}

func TestWifi_SavedConnectionOutOfRangeListedInKnownSectionAsUnavailable(t *testing.T) {
	f := fake.SeedArchLaptop()
	p := newPump(t, New(f))
	view := p.view()

	separator := strings.Index(view, "─ out of range ─")
	if separator < 0 {
		t.Fatalf("saved networks with no scanned AP need an out-of-range section, got:\n%s", view)
	}
	summer := strings.Index(view, "Summer House")
	if summer < 0 {
		t.Fatalf("the out-of-range saved network should be listed, got:\n%s", view)
	}
	if summer < separator {
		t.Errorf("Summer House should render below the separator, got:\n%s", view)
	}
	if row := lineContaining(t, view, "Summer House"); strings.Contains(row, "%") {
		t.Errorf("an out-of-range network has no signal to show, got: %s", row)
	}
}

func TestWifi_SlashFilterMatchesSSIDCaseInsensitive(t *testing.T) {
	f := fake.SeedArchLaptop()
	p := newPump(t, New(f))

	p.send(keyPress('/'))
	p.send(keyPress('q'))
	if containsQuit(p.msgs) {
		t.Fatal("while typing a filter, q is query text, not quit")
	}
	for _, r := range "wE" {
		p.send(keyPress(r))
	}
	p.send(tea.KeyPressMsg{Code: tea.KeyBackspace})
	p.send(tea.KeyPressMsg{Code: tea.KeyBackspace})
	p.send(tea.KeyPressMsg{Code: tea.KeyBackspace})
	for _, r := range "cAfe" {
		p.send(keyPress(r))
	}

	view := p.view()
	if !strings.Contains(view, "CafeGuest") {
		t.Errorf("filter should match SSIDs case-insensitively, got:\n%s", view)
	}
	if strings.Contains(view, "Neighbors") || strings.Contains(view, "Summer House") {
		t.Errorf("non-matching networks should be hidden while filtering, got:\n%s", view)
	}

	p.send(tea.KeyPressMsg{Code: tea.KeyEscape})
	if view := p.view(); !strings.Contains(view, "Neighbors") {
		t.Errorf("esc should clear the filter and restore the full list, got:\n%s", view)
	}
}

func TestWifi_EnterOnUnknownSecuredNetworkEmitsNeedsSecret(t *testing.T) {
	f := fake.SeedArchLaptop()
	p := newPump(t, New(f))

	p.send(keyPress('j'))
	p.send(keyPress('j'))
	if got := p.app().wifi.Selected().SSID; got != "Neighbors" {
		t.Fatalf("precondition: cursor should be on Neighbors, got %q", got)
	}

	p.send(tea.KeyPressMsg{Code: tea.KeyEnter})

	if len(f.JoinCalls) != 0 || len(f.ActivateCalls) != 0 {
		t.Errorf("a secured network without a profile must not connect yet, got joins %v activates %v",
			f.JoinCalls, f.ActivateCalls)
	}
	found := false
	for _, msg := range p.msgs {
		if needs, ok := msg.(wifi.NeedsSecretMsg); ok && needs.AP.SSID == "Neighbors" {
			found = true
		}
	}
	if !found {
		t.Errorf("Enter should emit the password-modal seam msg for M5, got msgs: %#v", p.msgs)
	}
}

func lineContaining(t *testing.T, view, want string) string {
	t.Helper()
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, want) {
			return line
		}
	}
	t.Fatalf("no line containing %q in:\n%s", want, view)
	return ""
}
