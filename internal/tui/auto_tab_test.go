package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/ilmars/netfu/internal/backend/fake"
	"github.com/ilmars/netfu/internal/domain"
)

func TestAuto_ListsAutoconnectProfilesInNMsPickOrder(t *testing.T) {
	f := fake.SeedAutoconnectPriorities()
	p := newPump(t, New(f))

	p.send(keyPress('5'))
	view := p.view()

	for _, col := range []string{"NAME", "PRI", "LAST USED"} {
		if !strings.Contains(view, col) {
			t.Errorf("the list should have a %s column header, got:\n%s", col, view)
		}
	}
	// Priority wins over recency; equal priorities fall back to last use.
	pinned := strings.Index(view, "Our House 5G")
	recent := strings.Index(view, "Our House 1")
	older := strings.Index(view, "Summer House")
	if pinned == -1 || recent == -1 || older == -1 {
		t.Fatalf("all autoconnect profiles should be listed, got:\n%s", view)
	}
	if !(pinned < recent && recent < older) {
		t.Errorf("pick order should be priority desc then last used desc, got:\n%s", view)
	}

	if row := lineContaining(t, view, "Our House 5G"); !strings.Contains(row, "10") {
		t.Errorf("the row should show the profile's current priority, got: %q", row)
	}
	if row := lineContaining(t, view, "Our House 1"); !strings.Contains(row, "2026-07-01") {
		t.Errorf("the row should show the profile's last-used date, got: %q", row)
	}
	if row := lineContaining(t, view, "Our House 5G"); !strings.Contains(row, "1") {
		t.Errorf("rows should carry their pick-order number, got: %q", row)
	}
}

func TestAuto_ReadsPrioritiesInEveryDbusIntegerShape(t *testing.T) {
	f := fake.SeedAutoconnectPriorities()
	f.SettingsByID["our-house-1"]["connection"] = map[string]any{
		"id": "Our House 1", "uuid": "our-house-1", "type": "802-11-wireless",
		"autoconnect-priority": int64(30),
	}
	f.SettingsByID["summer-house"]["connection"] = map[string]any{
		"id": "Summer House", "uuid": "summer-house", "type": "802-11-wireless",
		"autoconnect-priority": uint32(20),
	}
	p := newPump(t, New(f))

	p.send(keyPress('5'))
	view := p.view()

	i64 := strings.Index(view, "Our House 1")
	u32 := strings.Index(view, "Summer House")
	i32 := strings.Index(view, "Our House 5G")
	if !(i64 < u32 && u32 < i32) {
		t.Errorf("int64(30) > uint32(20) > int32(10) should order the list, got:\n%s", view)
	}
}

func TestAuto_ParksAutoconnectOffProfilesFaintAtTheBottom(t *testing.T) {
	f := fake.SeedAutoconnectPriorities()
	p := newPump(t, New(f))

	p.send(keyPress('5'))
	view := p.view()

	section := strings.Index(view, "─ autoconnect off ─")
	if section == -1 {
		t.Fatalf("opted-out profiles should sit under an autoconnect-off section, got:\n%s", view)
	}
	bridge := strings.Index(view, "docker0")
	if bridge < section {
		t.Errorf("docker0 has autoconnect off and belongs below the section header, got:\n%s", view)
	}
	if last := strings.Index(view, "Summer House"); bridge < last {
		t.Errorf("the off section belongs below the pick order, got:\n%s", view)
	}
}

func TestAuto_SplitsWifiAndWiredIntoTheirOwnCandidateSections(t *testing.T) {
	f := fake.SeedAutoconnectPriorities()
	f.ConnectionList = append(f.ConnectionList,
		domain.Connection{ID: "office-lan", Name: "Office LAN", Type: "802-3-ethernet"})
	f.SettingsByID["office-lan"] = domain.ConnectionSettings{
		"connection": {"id": "Office LAN", "uuid": "office-lan", "type": "802-3-ethernet"},
	}
	p := newPump(t, New(f))

	p.send(keyPress('5'))
	view := p.view()

	wifi := strings.Index(view, "─ wifi ─")
	wired := strings.Index(view, "─ wired ─")
	if wifi == -1 || wired == -1 {
		t.Fatalf("mixed device types race separately and should get their own sections, got:\n%s", view)
	}
	if !(wifi < wired) {
		t.Errorf("wifi candidates should come first, got:\n%s", view)
	}
	if lan := strings.Index(view, "Office LAN"); lan < wired {
		t.Errorf("the wired profile belongs in the wired section, got:\n%s", view)
	}
	// Each section restarts its order numbers: the lone wired profile is
	// NM's first pick on its device.
	if row := lineContaining(t, view, "Office LAN"); !strings.Contains(row, " 1  Office LAN") {
		t.Errorf("the wired section should restart pick-order numbering, got: %q", row)
	}
}

func TestAuto_AllZeroPrioritiesGetTheMostRecentlyUsedHint(t *testing.T) {
	f := fake.SeedAutoconnectPriorities()
	const hint = "order is most-recently-used — reorder to pin it"

	p := newPump(t, New(f))
	p.send(keyPress('5'))
	if view := p.view(); strings.Contains(view, hint) {
		t.Errorf("a pinned priority makes the order explicit; no hint needed, got:\n%s", view)
	}

	delete(f.SettingsByID["our-house-5g"]["connection"], "autoconnect-priority")
	p = newPump(t, New(f))
	p.send(keyPress('5'))
	if view := p.view(); !strings.Contains(view, hint) {
		t.Errorf("with every priority equal the shown order is only recency; the list should say so, got:\n%s", view)
	}
}

func TestAuto_ShiftJKMoveTheSelectedRowShowingThePendingOrder(t *testing.T) {
	f := fake.SeedAutoconnectPriorities()
	p := newPump(t, New(f))
	p.send(keyPress('5'))

	p.send(keyPress('J'))
	view := p.view()
	if pinned, recent := strings.Index(view, "Our House 5G"), strings.Index(view, "Our House 1"); pinned < recent {
		t.Errorf("J should move the selected row down in the pending order, got:\n%s", view)
	}
	if !strings.Contains(view, "●") {
		t.Errorf("a pending reorder should show the dirty dot, got:\n%s", view)
	}

	p.send(keyPress('K'))
	view = p.view()
	if pinned, recent := strings.Index(view, "Our House 5G"), strings.Index(view, "Our House 1"); pinned > recent {
		t.Errorf("K should move the selected row back up, got:\n%s", view)
	}
	if strings.Contains(view, "●") {
		t.Errorf("back at the backend's order nothing is pending; the dot should clear, got:\n%s", view)
	}
}

func TestAuto_ReorderingStopsAtTheSectionBoundary(t *testing.T) {
	f := fake.SeedAutoconnectPriorities()
	p := newPump(t, New(f))
	p.send(keyPress('5'))

	p.send(keyPress('j'))
	p.send(keyPress('j')) // Summer House, last of the wifi pick order
	p.send(keyPress('J'))
	view := p.view()
	if last, off := strings.Index(view, "Summer House"), strings.Index(view, "─ autoconnect off ─"); last > off {
		t.Errorf("J on the last row must not push it into the off section, got:\n%s", view)
	}
	if strings.Contains(view, "●") {
		t.Errorf("a no-op move should not mark the order dirty, got:\n%s", view)
	}

	p.send(keyPress('j')) // docker0, autoconnect off
	p.send(keyPress('K'))
	if view := p.view(); strings.Contains(view, "●") {
		t.Errorf("off rows have no pick order to move in, got:\n%s", view)
	}
}

func TestAuto_SpaceTogglesAutoconnectMovingTheProfileBetweenSections(t *testing.T) {
	f := fake.SeedAutoconnectPriorities()
	p := newPump(t, New(f))
	p.send(keyPress('5'))

	// Opt the top pick (Our House 5G) out of autoconnect.
	p.send(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	view := p.view()
	if pinned, off := strings.Index(view, "Our House 5G"), strings.Index(view, "─ autoconnect off ─"); pinned < off {
		t.Errorf("space should park the profile in the off section, got:\n%s", view)
	}
	if !strings.Contains(view, "●") {
		t.Errorf("a pending toggle should show the dirty dot, got:\n%s", view)
	}
	if len(f.UpdateCalls) != 0 {
		t.Errorf("toggling is pending until save, got writes: %#v", f.UpdateCalls)
	}

	// Opt it back in from the bottom of the off section: it rejoins its
	// section's pick order at the end.
	p.send(keyPress('G'))
	p.send(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	view = p.view()
	pinned, off := strings.Index(view, "Our House 5G"), strings.Index(view, "─ autoconnect off ─")
	if pinned > off {
		t.Errorf("space on an off row should rejoin the pick order, got:\n%s", view)
	}
	if last := strings.Index(view, "Summer House"); pinned < last {
		t.Errorf("a re-enabled profile joins at the end of its section, got:\n%s", view)
	}
}

func TestAuto_SaveSpacesPrioritiesDescendingWritingOnlyChangedProfiles(t *testing.T) {
	f := fake.SeedAutoconnectPriorities()
	p := newPump(t, New(f))
	p.send(keyPress('5'))

	// Demote the pinned profile one step: Our House 1, Our House 5G,
	// Summer House. Spaced priorities for three rows are 20, 10, 0 — only
	// Our House 1 (stored 0 → 20) actually changes.
	p.send(keyPress('J'))
	p.send(keyPress('s'))

	if len(f.UpdateCalls) != 1 {
		t.Fatalf("only the profile whose priority changed should be written, got: %#v", f.UpdateCalls)
	}
	call := f.UpdateCalls[0]
	if call.ConnectionID != "our-house-1" {
		t.Errorf("the new top pick should be written, got %q", call.ConnectionID)
	}
	if got := call.Settings["connection"]["autoconnect-priority"]; got != 20 {
		t.Errorf("the top of a three-row section should get priority 20, got %#v", got)
	}
	if got := call.Settings["802-11-wireless"]["ssid"]; got != "Our House 1" {
		t.Errorf("untouched settings keys must pass through verbatim, got %#v", call.Settings)
	}
	if _, present := call.Settings["connection"]["autoconnect"]; present {
		t.Errorf("an unchanged autoconnect flag should stay untouched, got %#v", call.Settings["connection"])
	}

	view := p.view()
	if !strings.Contains(view, "✓ autoconnect order saved (1 profiles updated)") {
		t.Errorf("the status line should report the save, got:\n%s", view)
	}
	if strings.Contains(view, "●") {
		t.Errorf("a saved order is no longer pending, got:\n%s", view)
	}
	if pinned, recent := strings.Index(view, "Our House 5G"), strings.Index(view, "Our House 1"); pinned < recent {
		t.Errorf("the saved order should survive the reload, got:\n%s", view)
	}
}

func TestAuto_SaveWritesToggledAutoconnectFlagsLeavingPrioritiesAlone(t *testing.T) {
	f := fake.SeedAutoconnectPriorities()
	// Two profiles share priority 10; recency puts Our House 1 on top.
	f.SettingsByID["our-house-1"]["connection"]["autoconnect-priority"] = int32(10)
	p := newPump(t, New(f))
	p.send(keyPress('5'))

	// Opting the top pick out leaves a two-row section whose spaced
	// priorities (10, 0) match what is stored — only the toggle writes.
	p.send(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	p.send(keyPress('s'))

	if len(f.UpdateCalls) != 1 {
		t.Fatalf("only the toggled profile should be written, got: %#v", f.UpdateCalls)
	}
	call := f.UpdateCalls[0]
	if call.ConnectionID != "our-house-1" {
		t.Errorf("the opted-out profile should be written, got %q", call.ConnectionID)
	}
	if got := call.Settings["connection"]["autoconnect"]; got != false {
		t.Errorf("the toggle should write autoconnect=false, got %#v", got)
	}
	if got := call.Settings["connection"]["autoconnect-priority"]; got != int32(10) {
		t.Errorf("an off profile's stored priority stays untouched, got %#v", got)
	}
}

func TestAuto_QWithPendingChangesAsksInsteadOfQuittingAndDCanDiscard(t *testing.T) {
	f := fake.SeedAutoconnectPriorities()
	p := newPump(t, New(f))
	p.send(keyPress('5'))
	p.send(keyPress('J'))

	p.send(keyPress('q'))
	if containsQuit(p.msgs) {
		t.Fatal("q with a pending reorder must ask, not quit")
	}
	if view := p.view(); !strings.Contains(view, "Unsaved changes") {
		t.Fatalf("q with a pending reorder should ask save/discard, got:\n%s", view)
	}

	p.send(keyPress('d'))
	view := p.view()
	if pinned, recent := strings.Index(view, "Our House 5G"), strings.Index(view, "Our House 1"); pinned > recent {
		t.Errorf("discarding should restore the backend's order, got:\n%s", view)
	}
	if strings.Contains(view, "●") {
		t.Errorf("discarding should clear the pending state, got:\n%s", view)
	}
	if len(f.UpdateCalls) != 0 {
		t.Errorf("discarding must not write anything, got: %#v", f.UpdateCalls)
	}

	p.send(keyPress('q'))
	if !containsQuit(p.msgs) {
		t.Error("with nothing pending q should quit again")
	}
}

func TestAuto_EscWithPendingChangesCanSaveFromThePrompt(t *testing.T) {
	f := fake.SeedAutoconnectPriorities()
	p := newPump(t, New(f))
	p.send(keyPress('5'))
	p.send(keyPress('J'))

	p.send(tea.KeyPressMsg{Code: tea.KeyEsc})
	if view := p.view(); !strings.Contains(view, "Unsaved changes") {
		t.Fatalf("esc with a pending reorder should ask save/discard, got:\n%s", view)
	}

	p.send(keyPress('s'))
	if len(f.UpdateCalls) != 1 {
		t.Errorf("s from the prompt should save the pending order, got: %#v", f.UpdateCalls)
	}
	if view := p.view(); !strings.Contains(view, "✓ autoconnect order saved") {
		t.Errorf("the status line should report the save, got:\n%s", view)
	}
}

func TestAuto_PolkitDenialLocksReorderToggleAndSave(t *testing.T) {
	f := fake.SeedAutoconnectPriorities()
	f.Perms = domain.Permissions{"org.freedesktop.NetworkManager.settings.modify.system": false}
	p := newPump(t, New(f))
	p.send(keyPress('5'))

	if view := p.view(); !strings.Contains(view, "🔒") {
		t.Errorf("denied modify actions should render locked, got:\n%s", view)
	}

	callsBefore := len(f.Calls)
	for _, denied := range []tea.KeyPressMsg{
		keyPress('J'), keyPress('K'), {Code: tea.KeySpace, Text: " "}, keyPress('s'),
	} {
		p.send(denied)
		view := p.view()
		if strings.Contains(view, "●") {
			t.Errorf("%q must not change the pending order when modify is denied, got:\n%s", denied.String(), view)
		}
		if !strings.Contains(view, "🔒 not permitted (polkit)") {
			t.Errorf("%q should explain the polkit denial in the status line, got:\n%s", denied.String(), view)
		}
	}
	if len(f.Calls) != callsBefore {
		t.Errorf("denied actions must not reach the backend, new calls: %v", f.Calls[callsBefore:])
	}
}

func TestAuto_BackendEventsRefreshACleanListButNeverAPendingReorder(t *testing.T) {
	f := fake.SeedAutoconnectPriorities()
	p := newPump(t, New(f))
	p.send(keyPress('5'))

	p.send(keyPress('J')) // pending: Our House 1 above Our House 5G
	f.Push(domain.Event{Kind: domain.EventConnectionChanged})
	p.deliverNext()
	view := p.view()
	if pinned, recent := strings.Index(view, "Our House 5G"), strings.Index(view, "Our House 1"); pinned < recent {
		t.Errorf("a backend event must not clobber the pending reorder, got:\n%s", view)
	}
	if !strings.Contains(view, "●") {
		t.Errorf("the pending state should survive backend churn, got:\n%s", view)
	}

	p.send(keyPress('q'))
	p.send(keyPress('d')) // discard: clean again

	f.ConnectionList = append(f.ConnectionList,
		domain.Connection{ID: "guest", Name: "Guest Net", Type: "802-11-wireless"})
	p.send(keyPress('1'))
	p.send(keyPress('5')) // any reload — churn or tab entry — may refresh now
	if view := p.view(); !strings.Contains(view, "Guest Net") {
		t.Errorf("a clean list should re-derive on reloads again, got:\n%s", view)
	}
}

func TestAuto_SingleDeviceTypeNeedsNoSectionHeaders(t *testing.T) {
	f := fake.SeedAutoconnectPriorities()
	p := newPump(t, New(f))

	p.send(keyPress('5'))
	if view := p.view(); strings.Contains(view, "─ wifi ─") {
		t.Errorf("an all-wifi pick order needs no section headers, got:\n%s", view)
	}
}
