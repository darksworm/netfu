package devices

import (
	"slices"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/ilmars/netfu/internal/backend/fake"
	"github.com/ilmars/netfu/internal/domain"
)

// seedVirtualZoo returns a fake with several managed virtual devices, the
// row model this screen (the Virtual tab) navigates.
func seedVirtualZoo() *fake.Fake {
	f := fake.SeedArchLaptop()
	f.DeviceList = append(f.DeviceList,
		domain.Device{Name: "br-9f1c2d", Type: domain.DeviceTypeBridge, State: domain.DeviceStateConnected, Managed: true, ActiveConnection: "br-9f1c2d"},
		domain.Device{Name: "lo", Type: domain.DeviceTypeLoopback, State: domain.DeviceStateConnected, Managed: true, ActiveConnection: "lo"},
	)
	return f
}

func TestDevices_ListShowsManagedVirtualDevicesOnly(t *testing.T) {
	f := seedVirtualZoo()
	f.DeviceList = append(f.DeviceList, domain.Device{
		Name: "p2p-dev-wlan0", Type: domain.DeviceTypeUnknown, State: domain.DeviceStateDisconnected, Managed: true,
	})
	m := New(f)
	m = loadDevices(t, m)

	view := m.View()
	for _, want := range []string{"docker0", "bridge", "connected", "br-9f1c2d", "lo", "loopback"} {
		if !strings.Contains(view, want) {
			t.Errorf("device list should contain %q, got:\n%s", want, view)
		}
	}
	if strings.Contains(view, "veth1a2b3c") {
		t.Errorf("unmanaged devices should be hidden, got:\n%s", view)
	}
	for _, physical := range []string{"wlan0", "enp0s31f6"} {
		if strings.Contains(view, physical) {
			t.Errorf("physical devices have their own tabs and should be hidden, got:\n%s", view)
		}
	}
	if strings.Contains(view, "p2p-dev-wlan0") {
		t.Errorf("p2p-dev pseudo-devices are wifi-p2p noise and should be hidden, got:\n%s", view)
	}
}

func TestDevices_ListHasColumnHeaderWithAlignedColumns(t *testing.T) {
	f := seedVirtualZoo()
	m := New(f)
	m = loadDevices(t, m)

	lines := strings.Split(ansi.Strip(m.View()), "\n")
	header := lines[0]
	for _, col := range []string{"DEVICE", "TYPE", "STATE", "CONNECTION"} {
		if !strings.Contains(header, col) {
			t.Fatalf("the first line should be a column header with %q, got %q", col, header)
		}
	}

	row := func(name string) string {
		for _, line := range lines {
			if strings.Contains(line, name) {
				return line
			}
		}
		t.Fatalf("no row for %s in:\n%s", name, m.View())
		return ""
	}
	bridge, loopback := row("br-9f1c2d"), row("lo ")
	if got, want := strings.Index(loopback, "loopback"), strings.Index(header, "TYPE"); got != want {
		t.Errorf("the type column should align with its header (col %d vs %d):\n%s\n%s", got, want, header, loopback)
	}
	if got, want := strings.LastIndex(bridge, "br-9f1c2d"), strings.Index(header, "CONNECTION"); got != want {
		t.Errorf("the connection column should align with its header (col %d vs %d):\n%s\n%s", got, want, header, bridge)
	}
}

func TestDevices_LongDeviceNamesAreTrimmedToKeepColumnsAligned(t *testing.T) {
	f := fake.New()
	f.DeviceList = []domain.Device{
		{Name: "docker0", Type: domain.DeviceTypeBridge, State: domain.DeviceStateConnected, Managed: true},
		{Name: "br-1408b82239ca-with-a-very-long-tail", Type: domain.DeviceTypeBridge, State: domain.DeviceStateConnected, Managed: true},
	}
	m := New(f)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 76, Height: 18})
	m = loadDevices(t, m)

	lines := strings.Split(ansi.Strip(m.View()), "\n")
	header, typeCol := lines[0], displayColumn(lines[0], "TYPE")
	if typeCol < 0 {
		t.Fatalf("missing TYPE header in %q", header)
	}
	sawTruncated := false
	for _, line := range lines[1:] {
		if got := displayColumn(line, "bridge"); got >= 0 && got != typeCol {
			t.Errorf("every row's type must align with the header (col %d vs %d):\n%s\n%s", got, typeCol, header, line)
		}
		if strings.Contains(line, "…") {
			sawTruncated = true
		}
	}
	if !sawTruncated {
		t.Error("an over-long device name should be trimmed with an ellipsis")
	}
}

func TestDevices_JKMoveSelection_GAndShiftGJumpEnds(t *testing.T) {
	f := seedVirtualZoo()
	m := New(f)
	m = loadDevices(t, m)

	assertSelected := func(step, want string) {
		t.Helper()
		if got := m.Selected().Name; got != want {
			t.Errorf("after %s: selected %q, want %q", step, got, want)
		}
	}

	assertSelected("load", "docker0")

	m, _ = m.Update(keyPress('j'))
	assertSelected("j", "br-9f1c2d")

	m, _ = m.Update(keyPress('k'))
	assertSelected("k", "docker0")

	m, _ = m.Update(keyPress('k'))
	assertSelected("k at top", "docker0")

	m, _ = m.Update(keyPress('G'))
	assertSelected("G", "lo")

	m, _ = m.Update(keyPress('j'))
	assertSelected("j at bottom", "lo")

	m, _ = m.Update(keyPress('g'))
	assertSelected("g", "docker0")
}

func TestDevices_DOffersDeactivateConfirmAndEscCancels(t *testing.T) {
	f := fake.SeedArchLaptop()
	m := New(f)
	m = loadDevices(t, m)
	if got := m.Selected().Name; got != "docker0" {
		t.Fatalf("precondition: cursor should be on docker0, got %q", got)
	}

	m, _ = m.Update(keyPress('d'))
	if overlay := m.Overlay(); !strings.Contains(overlay, "Deactivate docker0?") {
		t.Fatalf("d on a connected device should open the deactivate confirm, got:\n%s", overlay)
	}

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if overlay := m.Overlay(); strings.Contains(overlay, "Deactivate docker0?") {
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
	if !slices.Contains(f.Calls, "Deactivate(docker0)") {
		t.Errorf("confirming should deactivate the active connection, calls: %v", f.Calls)
	}
}

func TestDevices_SlashFilterNarrowsListByName(t *testing.T) {
	f := seedVirtualZoo()
	m := New(f)
	m = loadDevices(t, m)
	m, _ = m.Update(keyPress('G'))
	if got := m.Selected().Name; got != "lo" {
		t.Fatalf("precondition: cursor should be on lo, got %q", got)
	}

	m, _ = m.Update(keyPress('/'))
	for _, r := range "br" {
		m, _ = m.Update(keyPress(r))
	}

	view := m.View()
	if !strings.Contains(view, "br-9f1c2d") {
		t.Errorf("filter 'br' should keep br-9f1c2d visible, got:\n%s", view)
	}
	for _, hidden := range []string{"docker0", "lo "} {
		if strings.Contains(view, hidden) {
			t.Errorf("filter 'br' should hide %s, got:\n%s", hidden, view)
		}
	}
	if got := m.Selected().Name; got != "br-9f1c2d" {
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

func TestDevices_IShowsReadOnlyDetailAndEscPops(t *testing.T) {
	f := seedVirtualZoo()
	m := New(f)
	m = loadDevices(t, m)

	m, _ = m.Update(keyPress('i'))
	view := m.View()
	for _, want := range []string{
		"Name:", "docker0",
		"Type:", "bridge",
		"State:", "connected",
		"Active connection:",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("the detail view should show %q, got:\n%s", want, view)
		}
	}
	if strings.Contains(view, "br-9f1c2d") {
		t.Errorf("the detail view should replace the list, got:\n%s", view)
	}
	if len(f.Calls) != 0 {
		t.Errorf("the detail view is read-only, backend calls: %v", f.Calls)
	}

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if view := m.View(); !strings.Contains(view, "br-9f1c2d") {
		t.Errorf("esc should pop back to the device list, got:\n%s", view)
	}
	if got := m.Selected().Name; got != "docker0" {
		t.Errorf("popping the detail should keep the cursor, got %q", got)
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

// displayColumn is the terminal cell column where sub starts — byte offsets
// lie once multibyte glyphs like … or ▸ precede it.
func displayColumn(line, sub string) int {
	i := strings.Index(line, sub)
	if i < 0 {
		return -1
	}
	return lipgloss.Width(line[:i])
}
