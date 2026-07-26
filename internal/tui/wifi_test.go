package tui

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

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

func TestWifi_LongSSIDIsTrimmedSoColumnsStayAligned(t *testing.T) {
	f := fake.SeedArchLaptop()
	f.APList = append(f.APList, domain.AccessPoint{
		SSID:     "DIRECT-05-HP Smart Tank Plus 650 With An Absurdly Long Broadcast Name",
		Strength: 37, BSSID: "AA:BB:CC:88:88:88", Security: domain.SecurityWPA2,
	})
	p := newPump(t, New(f))
	p.send(tea.WindowSizeMsg{Width: 80, Height: 24})

	long := lineContaining(t, p.view(), "DIRECT-05-HP")
	short := lineContaining(t, p.view(), "CafeGuest")
	if !strings.Contains(long, "…") {
		t.Errorf("an over-long SSID should be trimmed with an ellipsis, got: %s", long)
	}
	if got, want := displayColumn(long, "%"), displayColumn(short, "%"); got != want {
		t.Errorf("the signal column must stay aligned for long SSIDs (col %d vs %d):\n%s\n%s",
			got, want, short, long)
	}
}

// displayColumn is the terminal cell column where sub starts — byte offsets
// lie once multibyte glyphs like … or ▸ precede it.
func displayColumn(line, sub string) int {
	i := strings.Index(line, sub)
	if i < 0 {
		return -1
	}
	return lipgloss.Width(line[:i])
}

func TestWifi_ListScrollsSoTheCursorRowStaysVisible(t *testing.T) {
	f := fake.SeedArchLaptop()
	for i := range 30 {
		f.APList = append(f.APList, domain.AccessPoint{
			SSID: fmt.Sprintf("Mesh %02d", i), Strength: uint8(30 - i%20),
			BSSID: fmt.Sprintf("AA:BB:CC:99:99:%02d", i), Security: domain.SecurityWPA2,
		})
	}
	p := newPump(t, New(f))
	p.send(tea.WindowSizeMsg{Width: 80, Height: 24})

	p.send(keyPress('G'))

	selected := p.app().wifi.Selected().SSID
	if selected == "" {
		t.Fatal("precondition: G should land on the weakest network")
	}
	if view := p.view(); !strings.Contains(view, selected) {
		t.Errorf("the list should scroll so the cursor row %q is visible, got:\n%s", selected, view)
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

func TestWifi_FailedActivationShowsTheBackendErrorOnTheStatusLine(t *testing.T) {
	f := fake.SeedArchLaptop()
	f.Errs["Activate"] = errors.New("profile is not compatible with device (mismatching interface name)")
	p := newPump(t, New(f))

	p.send(keyPress('j'))
	p.send(tea.KeyPressMsg{Code: tea.KeyEnter})

	view := p.view()
	if !strings.Contains(view, "✗ connect Our House 5G: profile is not compatible") {
		t.Errorf("a failed activation must surface NM's error, got:\n%s", view)
	}
	if strings.Contains(view, "Connecting to") {
		t.Errorf("the connecting spinner should clear on failure, got:\n%s", view)
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
	// \x1b[2m — the whole section is faint: it's context, not actionable.
	for _, marker := range []string{"out of range", "Summer House"} {
		if row := lineContaining(t, view, marker); !strings.Contains(row, "\x1b[2m") {
			t.Errorf("the out-of-range section should render greyed out, got: %s", row)
		}
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

func TestWifi_EnterOnUnknownWPA2NetworkOpensPasswordPrompt(t *testing.T) {
	f := fake.SeedArchLaptop()
	p := newPump(t, New(f))
	p.send(tea.WindowSizeMsg{Width: 80, Height: 24})

	p.send(keyPress('j'))
	p.send(keyPress('j'))
	if got := p.app().wifi.Selected().SSID; got != "Neighbors" {
		t.Fatalf("precondition: cursor should be on Neighbors, got %q", got)
	}
	linesBefore := strings.Count(p.view(), "\n")

	p.send(tea.KeyPressMsg{Code: tea.KeyEnter})

	view := p.view()
	for _, want := range []string{"Connect to Neighbors", "Security  WPA2", "Password", "↵ connect", "esc cancel"} {
		if !strings.Contains(view, want) {
			t.Errorf("the password prompt should show %q, got:\n%s", want, view)
		}
	}
	if !strings.Contains(view, "Our House 1") {
		t.Errorf("the prompt should overlay the list, not replace it, got:\n%s", view)
	}
	if linesAfter := strings.Count(view, "\n"); linesAfter != linesBefore {
		t.Errorf("the prompt should overlay the content, not push it (lines %d -> %d):\n%s",
			linesBefore, linesAfter, view)
	}

	p.send(tea.KeyPressMsg{Code: tea.KeyEscape})
	if view := p.view(); strings.Contains(view, "esc cancel") {
		t.Errorf("esc should close the prompt without connecting, got:\n%s", view)
	}
	if len(f.JoinCalls) != 0 {
		t.Errorf("opening and cancelling the prompt must not connect, got %+v", f.JoinCalls)
	}
}

func TestWifi_PasswordSubmitCallsJoinWifiWithPSKAndKeyMgmt(t *testing.T) {
	f := fake.SeedArchLaptop()
	// An unknown WPA3-only network: its join must request SAE key management.
	f.APList = append(f.APList, domain.AccessPoint{
		SSID: "Loft 6E", Strength: 40, BSSID: "AA:BB:CC:88:88:88", Security: domain.SecurityWPA3,
	})
	p := newPump(t, New(f))

	p.send(keyPress('j'))
	p.send(keyPress('j'))
	p.send(tea.KeyPressMsg{Code: tea.KeyEnter})
	for _, r := range "hunter2" {
		p.send(keyPress(r))
	}
	p.send(tea.KeyPressMsg{Code: tea.KeyEnter})

	if len(f.JoinCalls) != 1 {
		t.Fatalf("submitting the password should call JoinWifi once, got %d (%v)", len(f.JoinCalls), f.Calls)
	}
	got := f.JoinCalls[0]
	want := domain.JoinRequest{SSID: "Neighbors", Security: domain.SecurityWPA2, PSK: "hunter2"}
	if got != want {
		t.Errorf("JoinWifi request = %+v, want %+v", got, want)
	}
	if view := p.view(); !strings.Contains(view, "Connecting to Neighbors…") {
		t.Errorf("the status line should announce the join, got:\n%s", view)
	}

	// WPA3-only network → the request carries wpa3, which the NM adapter
	// maps to key-mgmt "sae".
	p.send(keyPress('/'))
	for _, r := range "Loft" {
		p.send(keyPress(r))
	}
	p.send(tea.KeyPressMsg{Code: tea.KeyEnter})
	if got := p.app().wifi.Selected().SSID; got != "Loft 6E" {
		t.Fatalf("precondition: cursor should be on Loft 6E, got %q", got)
	}
	p.send(tea.KeyPressMsg{Code: tea.KeyEnter})
	for _, r := range "sae-secret" {
		p.send(keyPress(r))
	}
	p.send(tea.KeyPressMsg{Code: tea.KeyEnter})

	if len(f.JoinCalls) != 2 {
		t.Fatalf("submitting the WPA3 password should call JoinWifi, got %d (%v)", len(f.JoinCalls), f.Calls)
	}
	got = f.JoinCalls[1]
	want = domain.JoinRequest{SSID: "Loft 6E", Security: domain.SecurityWPA3, PSK: "sae-secret"}
	if got != want {
		t.Errorf("WPA3 JoinWifi request = %+v, want %+v", got, want)
	}
}

func TestWifi_WrongPasswordShowsErrorAndReopensPromptWithSSIDPreserved(t *testing.T) {
	f := fake.SeedArchLaptop()
	p := newPump(t, New(f))

	p.send(keyPress('j'))
	p.send(keyPress('j'))
	p.send(tea.KeyPressMsg{Code: tea.KeyEnter})
	for _, r := range "wrongpass" {
		p.send(keyPress(r))
	}
	p.send(tea.KeyPressMsg{Code: tea.KeyEnter})
	if len(f.JoinCalls) != 1 {
		t.Fatalf("precondition: the join should have been issued, got %v", f.Calls)
	}

	// NM tears the activation down; the adapter reports the auth-failure
	// reason on the device state change.
	f.Push(domain.Event{
		Kind:       domain.EventDeviceChanged,
		DeviceName: "wlan0",
		Reason:     domain.ReasonNoSecrets,
	})
	p.deliverNext()

	if view := p.view(); !strings.Contains(view, "Wrong password for Neighbors — ↵ to retry") {
		t.Errorf("the status line should report the wrong password, got:\n%s", view)
	}
	if len(f.DeleteCalls) != 1 || f.DeleteCalls[0] != "joined-Neighbors" {
		t.Errorf("the half-created profile should be deleted, got deletes %v", f.DeleteCalls)
	}

	p.send(tea.KeyPressMsg{Code: tea.KeyEnter})
	view := p.view()
	if !strings.Contains(view, "Connect to Neighbors") {
		t.Fatalf("enter should reopen the prompt for the same SSID, got:\n%s", view)
	}
	if !strings.Contains(view, "*********") {
		t.Errorf("the previous input should be preserved in the reopened prompt, got:\n%s", view)
	}

	p.send(tea.KeyPressMsg{Code: tea.KeyEnter})
	if len(f.JoinCalls) != 2 || f.JoinCalls[1].PSK != "wrongpass" {
		t.Errorf("resubmitting should retry with the preserved input, got %+v", f.JoinCalls)
	}
}

func TestWifi_HiddenNetworkJoinFlowPromptsForSSIDThenSecret(t *testing.T) {
	f := fake.SeedArchLaptop()
	p := newPump(t, New(f))

	p.send(keyPress('G'))
	if got := p.app().wifi.Selected(); got.SSID != "" || got.BSSID != "AA:BB:CC:66:66:66" {
		t.Fatalf("precondition: cursor should be on the hidden row, got %+v", got)
	}

	p.send(tea.KeyPressMsg{Code: tea.KeyEnter})
	view := p.view()
	if !strings.Contains(view, "Hidden network") || !strings.Contains(view, "SSID") {
		t.Fatalf("enter on a hidden row should open the SSID entry modal, got:\n%s", view)
	}

	for _, r := range "SecretNet" {
		p.send(keyPress(r))
	}
	p.send(tea.KeyPressMsg{Code: tea.KeyEnter})
	view = p.view()
	if !strings.Contains(view, "Connect to SecretNet") || !strings.Contains(view, "Password") {
		t.Fatalf("the SSID entry should hand over to the password prompt, got:\n%s", view)
	}

	for _, r := range "hush-hush" {
		p.send(keyPress(r))
	}
	p.send(tea.KeyPressMsg{Code: tea.KeyEnter})

	if len(f.JoinCalls) != 1 {
		t.Fatalf("finishing the flow should call JoinWifi once, got %d (%v)", len(f.JoinCalls), f.Calls)
	}
	got := f.JoinCalls[0]
	want := domain.JoinRequest{SSID: "SecretNet", Hidden: true, Security: domain.SecurityWPA2, PSK: "hush-hush"}
	if got != want {
		t.Errorf("hidden JoinWifi request = %+v, want %+v", got, want)
	}

	// c starts the same flow from anywhere in the list.
	p.send(keyPress('g'))
	p.send(keyPress('c'))
	if view := p.view(); !strings.Contains(view, "Hidden network") {
		t.Errorf("c should open the hidden-network SSID entry from any row, got:\n%s", view)
	}
}

func TestWifi_EnterpriseNetworkShowsUnsupportedNoticeV1(t *testing.T) {
	f := fake.SeedArchLaptop()
	f.APList = append(f.APList, domain.AccessPoint{
		SSID: "CorpNet", Strength: 70, BSSID: "AA:BB:CC:99:99:99", Security: domain.SecurityEnterprise,
	})
	p := newPump(t, New(f))

	p.send(keyPress('/'))
	for _, r := range "Corp" {
		p.send(keyPress(r))
	}
	p.send(tea.KeyPressMsg{Code: tea.KeyEnter})
	if got := p.app().wifi.Selected().SSID; got != "CorpNet" {
		t.Fatalf("precondition: cursor should be on CorpNet, got %q", got)
	}

	p.send(tea.KeyPressMsg{Code: tea.KeyEnter})

	view := p.view()
	if !strings.Contains(view, "802.1X networks are not supported yet") {
		t.Errorf("enter on an enterprise network should explain it is unsupported, got:\n%s", view)
	}
	if strings.Contains(view, "Password") {
		t.Errorf("no password modal should open for an enterprise network, got:\n%s", view)
	}
	if len(f.JoinCalls) != 0 || len(f.ActivateCalls) != 0 {
		t.Errorf("an enterprise network must not be joined in v1, got joins %v activates %v",
			f.JoinCalls, f.ActivateCalls)
	}
}

func TestWifi_ToggleWifiRadioWithW(t *testing.T) {
	f := fake.SeedArchLaptop()
	p := newPump(t, New(f))

	p.send(keyPress('W'))

	if len(f.SetWifiEnabledCalls) != 1 || f.SetWifiEnabledCalls[0] != false {
		t.Fatalf("W should turn the radio off, got calls %v", f.SetWifiEnabledCalls)
	}
	view := p.view()
	if !strings.Contains(view, "Wi-Fi is off — press W to enable") {
		t.Errorf("the wifi tab should show the radio-off empty state, got:\n%s", view)
	}
	if strings.Contains(view, "Our House 1") {
		t.Errorf("no scan rows should render while the radio is off, got:\n%s", view)
	}

	p.send(keyPress('W'))

	if len(f.SetWifiEnabledCalls) != 2 || f.SetWifiEnabledCalls[1] != true {
		t.Fatalf("W again should turn the radio back on, got calls %v", f.SetWifiEnabledCalls)
	}
	if view := p.view(); !strings.Contains(view, "Our House 1") {
		t.Errorf("re-enabling the radio should bring the list back, got:\n%s", view)
	}
}

func TestWifi_DeactivateCurrentNetworkWithD(t *testing.T) {
	f := fake.SeedArchLaptop()
	p := newPump(t, New(f))

	if got := p.app().wifi.Selected().SSID; got != "Our House 1" {
		t.Fatalf("precondition: cursor should be on the active network, got %q", got)
	}

	p.send(keyPress('d'))
	view := p.view()
	if !strings.Contains(view, "Deactivate Our House 1?") {
		t.Fatalf("d on the active row should ask for confirmation, got:\n%s", view)
	}

	p.send(keyPress('n'))
	if containsCall(f.Calls, "Deactivate(our-house-1)") {
		t.Fatal("declining the confirm must not deactivate")
	}

	p.send(keyPress('d'))
	p.send(keyPress('y'))
	if !containsCall(f.Calls, "Deactivate(our-house-1)") {
		t.Errorf("confirming should deactivate the active wifi connection, got calls %v", f.Calls)
	}

	// d away from the active row has nothing to deactivate.
	p.send(keyPress('G'))
	p.send(keyPress('d'))
	if view := p.view(); strings.Contains(view, "Deactivate") {
		t.Errorf("d on an inactive row should not open a confirm, got:\n%s", view)
	}
}

func TestWifi_EOnSavedNetworkOpensTheProfileEditor(t *testing.T) {
	f := fake.SeedArchLaptop()
	p := newPump(t, New(f))

	if got := p.app().wifi.Selected().SSID; got != "Our House 1" {
		t.Fatalf("precondition: cursor should be on the saved active network, got %q", got)
	}

	p.send(keyPress('e'))
	view := p.view()
	if !strings.Contains(view, "Autoconnect") || !strings.Contains(view, "SSID") {
		t.Fatalf("e on a saved network should push its profile editor, got:\n%s", view)
	}
	if name := lineContaining(t, view, "Name"); !strings.Contains(name, "Our House 1") {
		t.Errorf("the editor should load the saved profile, got: %q", name)
	}

	p.send(keyPress('q'))
	if containsQuit(p.msgs) {
		t.Fatal("q inside the editor should pop it, not quit the app")
	}
	if view := p.view(); strings.Contains(view, "Autoconnect") {
		t.Errorf("q should have popped the editor back to the scan list, got:\n%s", view)
	}
}

func TestWifi_EOnAnUnsavedNetworkDoesNothing(t *testing.T) {
	f := fake.SeedArchLaptop()
	p := newPump(t, New(f))

	p.send(keyPress('j'))
	p.send(keyPress('j'))
	if got := p.app().wifi.Selected().SSID; got != "Neighbors" {
		t.Fatalf("precondition: cursor should be on Neighbors, got %q", got)
	}

	p.send(keyPress('e'))
	if view := p.view(); strings.Contains(view, "Autoconnect") {
		t.Errorf("an unsaved network has no profile to edit, got:\n%s", view)
	}
}

func TestWifi_XForgetsSavedNetworkAfterConfirm(t *testing.T) {
	f := fake.SeedArchLaptop()
	p := newPump(t, New(f))

	p.send(keyPress('j'))
	if got := p.app().wifi.Selected().SSID; got != "Our House 5G" {
		t.Fatalf("precondition: cursor should be on Our House 5G, got %q", got)
	}

	p.send(keyPress('x'))
	view := p.view()
	if !strings.Contains(view, "Forget Our House 5G?") {
		t.Fatalf("x on a saved network should ask for confirmation, got:\n%s", view)
	}

	p.send(keyPress('n'))
	if len(f.DeleteCalls) != 0 {
		t.Fatal("declining the confirm must not delete the profile")
	}

	p.send(keyPress('x'))
	p.send(keyPress('y'))
	if len(f.DeleteCalls) != 1 || f.DeleteCalls[0] != "our-house-5g" {
		t.Errorf("confirming should delete the saved profile, got deletes %v", f.DeleteCalls)
	}
	if row := lineContaining(t, p.view(), "Our House 5G"); strings.Contains(row, "⋆ saved") {
		t.Errorf("the forgotten network should lose its saved tag, got: %s", row)
	}
}

func TestWifi_XOnAnUnsavedNetworkDoesNothing(t *testing.T) {
	f := fake.SeedArchLaptop()
	p := newPump(t, New(f))

	p.send(keyPress('j'))
	p.send(keyPress('j'))
	if got := p.app().wifi.Selected().SSID; got != "Neighbors" {
		t.Fatalf("precondition: cursor should be on Neighbors, got %q", got)
	}

	p.send(keyPress('x'))
	if view := p.view(); strings.Contains(view, "Forget") {
		t.Errorf("an unsaved network has no profile to forget, got:\n%s", view)
	}
	if len(f.DeleteCalls) != 0 {
		t.Errorf("nothing should be deleted, got %v", f.DeleteCalls)
	}
}

func containsCall(calls []string, want string) bool {
	for _, c := range calls {
		if c == want {
			return true
		}
	}
	return false
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
