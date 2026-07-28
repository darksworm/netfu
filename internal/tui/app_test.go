package tui

import (
	"errors"
	"image/color"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/ilmars/netfu/internal/backend/fake"
	"github.com/ilmars/netfu/internal/domain"
	"github.com/ilmars/netfu/internal/tui/style"
)

func TestApp_RendersInTheAlternateScreenBuffer(t *testing.T) {
	f := fake.SeedArchLaptop()
	p := newPump(t, New(f))

	if !p.model.(App).View().AltScreen {
		t.Error("the app should render in the alternate screen buffer so it doesn't scroll the shell history")
	}
}

func TestApp_TabBarListsPhysicalDevicesThenVirtualOtherSystem(t *testing.T) {
	f := fake.SeedArchLaptop()
	p := newPump(t, New(f))

	view := p.view()
	for _, entry := range []string{"[1] wlan0", "[2] enp0s31f6", "[3] Virtual", "[4] Other", "[5] Auto", "[6] System"} {
		if !strings.Contains(view, entry) {
			t.Errorf("the tab bar should show %q, got:\n%s", entry, view)
		}
	}
}

func TestApp_LandsOnTheFirstWifiDeviceTabAndQQuits(t *testing.T) {
	f := fake.SeedArchLaptop()
	p := newPump(t, New(f))

	view := p.view()
	if got := p.app().currentTab(); got.kind != tabKindWifi || got.device != "wlan0" {
		t.Errorf("the app should land on the wlan0 tab, got %+v", got)
	}
	if !strings.Contains(view, "Our House 1") {
		t.Errorf("the landing tab should show the wifi scan list, got:\n%s", view)
	}
	if strings.Contains(view, "docker0") {
		t.Errorf("the wifi device tab should not render other devices, got:\n%s", view)
	}

	p.send(keyPress('q'))
	if !containsQuit(p.msgs) {
		t.Errorf("pressing q at top level should quit, got msgs: %#v", p.msgs)
	}
}

func TestApp_VirtualTabShowsVirtualDeviceListAndQQuits(t *testing.T) {
	f := fake.SeedArchLaptop()
	p := newPump(t, New(f))

	p.send(keyPress('3'))
	if view := p.view(); !strings.Contains(view, "docker0") {
		t.Errorf("the Virtual tab should show the virtual device list, got:\n%s", view)
	}

	p.send(keyPress('q'))
	if !containsQuit(p.msgs) {
		t.Errorf("pressing q at top level should quit, got msgs: %#v", p.msgs)
	}
}

func TestApp_NumberKeysAndBracketsSwitchTabs(t *testing.T) {
	f := fake.SeedArchLaptop()
	p := newPump(t, New(f))

	p.send(keyPress('4'))
	if got := p.app().currentTab(); got.kind != tabKindOther {
		t.Errorf("4 should open the Other tab, got %+v", got)
	}
	if view := p.view(); !strings.Contains(view, "LAST USED") {
		t.Errorf("the Other tab should list saved profiles, got:\n%s", view)
	}

	p.send(keyPress(']'))
	if got := p.app().currentTab(); got.kind != tabKindAuto {
		t.Errorf("] should move to the next tab, got %+v", got)
	}

	p.send(keyPress(']'))
	if got := p.app().currentTab(); got.kind != tabKindSystem {
		t.Errorf("] should move on to System, got %+v", got)
	}

	p.send(keyPress(']'))
	if got := p.app().currentTab(); got.kind != tabKindWifi || got.device != "wlan0" {
		t.Errorf("] on the last tab should wrap to the first device tab, got %+v", got)
	}

	p.send(keyPress('['))
	if got := p.app().currentTab(); got.kind != tabKindSystem {
		t.Errorf("[ on the first tab should wrap to System, got %+v", got)
	}

	p.send(keyPress('9'))
	if got := p.app().currentTab(); got.kind != tabKindSystem {
		t.Errorf("a number past the last tab should be ignored, got %+v", got)
	}
}

func TestApp_VirtualTabHidesPhysicalDevicesAndP2PNoise(t *testing.T) {
	f := fake.SeedArchLaptop()
	f.DeviceList = append(f.DeviceList, domain.Device{
		Name: "p2p-dev-wlan0", Type: domain.DeviceTypeUnknown, State: domain.DeviceStateDisconnected, Managed: true,
	})
	p := newPump(t, New(f))

	p.send(keyPress('3'))
	view := p.view()
	if !strings.Contains(view, "docker0") {
		t.Errorf("the Virtual tab should list the bridge, got:\n%s", view)
	}
	// The physical names appear exactly once — in the tab bar, not as rows.
	for _, hidden := range []string{"wlan0", "enp0s31f6"} {
		if got := strings.Count(view, hidden); got != 1 {
			t.Errorf("%s has its own tab and should not appear as a Virtual row (%d occurrences), got:\n%s",
				hidden, got, view)
		}
	}
	if strings.Contains(view, "p2p-dev-wlan0") {
		t.Errorf("p2p-dev pseudo-devices are wifi-p2p noise and should be hidden, got:\n%s", view)
	}
}

func TestApp_DeviceChurnRedrivesTabsKeepingPhysicalOrderStable(t *testing.T) {
	f := fake.SeedArchLaptop()
	p := newPump(t, New(f))
	p.send(keyPress('6'))
	if got := p.app().currentTab(); got.kind != tabKindSystem {
		t.Fatalf("precondition: tab 6 should be System, got %+v", got)
	}

	// A USB NIC appears: it gets its own tab after the built-in one, and
	// System shifts to slot 7 with the user still on it.
	f.DeviceList = append(f.DeviceList, domain.Device{
		Name: "enp5s0u1", Type: domain.DeviceTypeEthernet, State: domain.DeviceStateDisconnected, Managed: true,
	})
	f.Push(domain.Event{Kind: domain.EventDeviceChanged, DeviceName: "enp5s0u1"})
	p.deliverNext()

	view := p.view()
	for _, entry := range []string{"[1] wlan0", "[2] enp0s31f6", "[3] enp5s0u1", "[4] Virtual", "[5] Other", "[6] Auto", "[7] System"} {
		if !strings.Contains(view, entry) {
			t.Errorf("after hotplug the tab bar should show %q, got:\n%s", entry, view)
		}
	}
	if got := p.app().currentTab(); got.kind != tabKindSystem {
		t.Errorf("churn elsewhere must not move the current tab off System, got %+v", got)
	}
}

func TestApp_CurrentTabsDeviceVanishingFallsBackToTheFirstTab(t *testing.T) {
	f := fake.SeedArchLaptop()
	f.DeviceList = append(f.DeviceList, domain.Device{
		Name: "enp5s0u1", Type: domain.DeviceTypeEthernet, State: domain.DeviceStateDisconnected, Managed: true,
	})
	p := newPump(t, New(f))

	p.send(keyPress('3'))
	if got := p.app().currentTab(); got.device != "enp5s0u1" {
		t.Fatalf("precondition: tab 3 should be the USB NIC, got %+v", got)
	}

	// The USB NIC is unplugged while its tab is open.
	f.DeviceList = f.DeviceList[:len(f.DeviceList)-1]
	f.Push(domain.Event{Kind: domain.EventDeviceChanged, DeviceName: "enp5s0u1"})
	p.deliverNext()

	if got := p.app().currentTab(); got.kind != tabKindWifi || got.device != "wlan0" {
		t.Errorf("a vanished device tab should fall back to the first tab, got %+v", got)
	}
	if view := p.view(); strings.Contains(view, "enp5s0u1") {
		t.Errorf("the unplugged NIC should leave the tab bar, got:\n%s", view)
	}
}

func TestApp_TabStatePersistsAcrossSwitches(t *testing.T) {
	f := fake.SeedArchLaptop()
	f.DeviceList = append(f.DeviceList, domain.Device{
		Name: "br-9f1c2d", Type: domain.DeviceTypeBridge, State: domain.DeviceStateConnected, Managed: true,
	})
	p := newPump(t, New(f))

	p.send(keyPress('3'))
	p.send(keyPress('G'))
	moved := p.app().devices.Selected().Name
	if moved == "" {
		t.Fatal("precondition: the Virtual tab should have a row to move to")
	}

	p.send(keyPress('1'))
	p.send(keyPress('3'))
	if got := p.app().devices.Selected().Name; got != moved {
		t.Errorf("returning to the Virtual tab should keep its cursor on %q, got %q", moved, got)
	}
}

func TestApp_ResizePropagatesToActiveScreenAndListsReflow(t *testing.T) {
	f := fake.SeedArchLaptop()
	p := newPump(t, New(f))

	p.send(tea.WindowSizeMsg{Width: 60, Height: 16})

	view := p.view()
	lines := strings.Split(strings.TrimRight(view, "\n"), "\n")
	if len(lines) > 16 {
		t.Errorf("view should fit in 16 rows after resize, got %d:\n%s", len(lines), view)
	}
	for _, line := range lines {
		if w := lipgloss.Width(line); w > 60 {
			t.Errorf("line %q is %d cells wide, should reflow to <= 60", line, w)
		}
	}
}

func TestApp_FooterTeachesEachTabsCoreActions(t *testing.T) {
	f := fake.SeedArchLaptop()
	p := newPump(t, New(f))
	p.send(tea.WindowSizeMsg{Width: 100, Height: 24})

	footer := func() string {
		lines := strings.Split(strings.TrimRight(p.view(), "\n"), "\n")
		return lines[len(lines)-1]
	}

	for _, want := range []string{"connect", "disconnect", "forget", "edit", "filter"} {
		if !strings.Contains(footer(), want) {
			t.Errorf("the wifi footer should teach %q, got: %s", want, footer())
		}
	}

	p.send(keyPress('2')) // ethernet tab (fake has enp0s31f6)
	for _, want := range []string{"activate", "edit", "new"} {
		if !strings.Contains(footer(), want) {
			t.Errorf("the ethernet footer should teach %q, got: %s", want, footer())
		}
	}

	p.send(keyPress('4')) // Other
	for _, want := range []string{"activate", "edit", "delete", "new"} {
		if !strings.Contains(footer(), want) {
			t.Errorf("the Other footer should teach %q, got: %s", want, footer())
		}
	}

	p.send(keyPress('5')) // Auto
	for _, want := range []string{"J/K", "reorder", "toggle", "save"} {
		if !strings.Contains(footer(), want) {
			t.Errorf("the Auto footer should teach %q, got: %s", want, footer())
		}
	}
}

func TestApp_QuestionMarkTogglesHelpOverlay(t *testing.T) {
	f := fake.SeedArchLaptop()
	p := newPump(t, New(f))

	p.send(keyPress('?'))
	view := p.view()
	for _, want := range []string{"top", "bottom", "quit", "help"} {
		if !strings.Contains(view, want) {
			t.Errorf("help overlay should list %q from the keymaps, got:\n%s", want, view)
		}
	}

	p.send(keyPress('?'))
	if view := p.view(); strings.Contains(view, "bottom") {
		t.Errorf("second ? should close the help overlay, got:\n%s", view)
	}
}

func TestApp_HelpOverlayListsTheActiveScreensKeymap(t *testing.T) {
	f := fake.SeedArchLaptop()
	p := newPump(t, New(f))

	p.send(keyPress('?'))
	view := p.view()
	for _, want := range []string{"join hidden", "disconnect", "wifi radio"} {
		if !strings.Contains(view, want) {
			t.Errorf("on the wifi tab the help overlay should list %q, got:\n%s", want, view)
		}
	}

	p.send(keyPress('3'))
	view = p.view()
	if !strings.Contains(view, "activate") || !strings.Contains(view, "deactivate") {
		t.Errorf("on the Virtual tab the help overlay should list the device actions, got:\n%s", view)
	}
	if strings.Contains(view, "join hidden") {
		t.Errorf("the Virtual tab help should not show wifi bindings, got:\n%s", view)
	}
}

func TestApp_DevicesConfirmModalOverlaysTheListInsteadOfPushingIt(t *testing.T) {
	f := fake.SeedArchLaptop()
	p := newPump(t, New(f))
	p.send(tea.WindowSizeMsg{Width: 80, Height: 24})

	p.send(keyPress('3'))
	p.send(keyPress('G'))
	linesBefore := strings.Count(p.view(), "\n")

	// Enter on the connected docker0 row asks to deactivate.
	p.send(tea.KeyPressMsg{Code: tea.KeyEnter})

	view := p.view()
	if !strings.Contains(view, "Deactivate docker0?") {
		t.Fatalf("the confirm modal should be visible, got:\n%s", view)
	}
	if linesAfter := strings.Count(view, "\n"); linesAfter != linesBefore {
		t.Errorf("the confirm should overlay the list, not push it (lines %d -> %d):\n%s",
			linesBefore, linesAfter, view)
	}
}

func TestApp_AutoRescanTickRequestsScanOnlyWhileWifiTabVisible(t *testing.T) {
	f := fake.SeedArchLaptop()
	p := newPump(t, New(f))

	scans := func() int {
		count := 0
		for _, call := range f.Calls {
			if call == "RequestScan" {
				count++
			}
		}
		return count
	}
	before := scans()

	p.send(rescanTickMsg{})
	if got := scans(); got != before+1 {
		t.Errorf("a tick on the wifi tab should request a rescan, got %d scans (was %d)", got, before)
	}

	p.send(keyPress('2'))
	base := scans()
	p.send(rescanTickMsg{})
	if got := scans(); got != base {
		t.Errorf("a tick away from the wifi tab must not rescan, got %d scans (was %d)", got, base)
	}
}

func TestApp_BackendEventMsgRearmsWaitForActivity(t *testing.T) {
	f := fake.SeedArchLaptop()
	model := New(f)

	// An event is already queued, so the wait cmd from Init returns without blocking.
	f.Push(domain.Event{Kind: domain.EventDeviceChanged, DeviceName: "wlan0"})
	msgs := collectMsgs(model.Init())
	eventMsg := findBackendEvent(t, msgs)

	model, rearm := model.Update(eventMsg)
	if rearm == nil {
		t.Fatal("handling a backend event should return a cmd that waits for the next one")
	}

	f.Push(domain.Event{Kind: domain.EventDeviceChanged, DeviceName: "enp0s31f6"})
	findBackendEvent(t, collectMsgs(rearm))
	_ = model
}

func TestApp_DarkBackgroundSelectsDarkPalette(t *testing.T) {
	f := fake.SeedArchLaptop()
	model := New(f)

	requested := false
	for _, msg := range collectMsgs(model.Init()) {
		if msg == tea.RequestBackgroundColor() {
			requested = true
		}
	}
	if !requested {
		t.Error("Init should request the terminal background color")
	}

	model, _ = model.Update(tea.BackgroundColorMsg{Color: color.Black})

	theme := model.(App).theme
	if !theme.IsDark {
		t.Error("a dark background should select the dark palette")
	}
	if theme.Accent != lipgloss.Color("#7AA2F7") {
		t.Errorf("dark palette accent should be #7AA2F7, got %v", theme.Accent)
	}
	// #33467C selection tint — the cursor row must follow the resolved theme.
	if selected := style.Selected.Render("row"); !strings.Contains(selected, "48;2;51;70;124") {
		t.Errorf("resolving a dark background should tint the cursor row, got %q", selected)
	}
}

func TestDevices_DeviceStateEventUpdatesRowLive(t *testing.T) {
	f := fake.SeedArchLaptop()
	p := newPump(t, New(f))

	p.send(keyPress('3'))
	p.send(keyPress('G'))
	if got := p.app().devices.Selected().Name; got != "docker0" {
		t.Fatalf("precondition: cursor should be on docker0, got %q", got)
	}

	// The bridge loses its carrier: backend state changes and an event fires.
	f.DeviceList[2].State = domain.DeviceStateDisconnected
	f.Push(domain.Event{Kind: domain.EventDeviceChanged, DeviceName: "docker0"})
	p.deliverNext()

	if row := lineContaining(t, p.view(), "docker0"); !strings.Contains(row, "disconnected") {
		t.Errorf("row should show the new device state, got: %s", row)
	}
	if got := p.app().devices.Selected().Name; got != "docker0" {
		t.Errorf("live update should keep the cursor on docker0, got %q", got)
	}
}

func TestApp_PermissionsQueriedOnceAtStartupAndCached(t *testing.T) {
	f := fake.SeedArchLaptop()
	f.Perms = domain.Permissions{"org.freedesktop.NetworkManager.network-control": true}
	p := newPump(t, New(f))

	// Later activity must be served from the cache, not re-query.
	f.Push(domain.Event{Kind: domain.EventDeviceChanged, DeviceName: "wlan0"})
	p.deliverNext()
	p.send(keyPress('j'))

	count := 0
	for _, call := range f.Calls {
		if call == "Permissions" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("Permissions should be queried exactly once at startup, got %d calls: %v", count, f.Calls)
	}
	if !p.app().perms["org.freedesktop.NetworkManager.network-control"] {
		t.Error("the permissions result should be cached on the model")
	}
}

// keyPress builds the message bubbletea v2 delivers for a printable key.
func keyPress(r rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: r, Text: string(r)}
}

// pump runs a model's cmds the way the tea runtime does, but synchronously.
// Cmds that block (the app's armed backend wait) stay in flight; deliverNext
// awaits one after the test pushes an event, so nothing gets lost.
type pump struct {
	t        *testing.T
	model    tea.Model
	msgs     []tea.Msg
	inFlight []chan tea.Msg
}

func newPump(t *testing.T, m tea.Model) *pump {
	t.Helper()
	p := &pump{t: t, model: m}
	p.run(m.Init())
	return p
}

// run executes cmds, feeding their msgs back into the model until none
// remain. Blocking cmds move to inFlight; QuitMsg is collected, not fed back.
func (p *pump) run(cmd tea.Cmd) {
	queue := []tea.Cmd{cmd}
	for len(queue) > 0 {
		c := queue[0]
		queue = queue[1:]
		if c == nil {
			continue
		}
		done := make(chan tea.Msg, 1)
		go func() { done <- c() }()
		select {
		case msg := <-done:
			queue = append(queue, p.apply(msg)...)
		case <-time.After(50 * time.Millisecond):
			p.inFlight = append(p.inFlight, done)
		}
	}
}

func (p *pump) apply(msg tea.Msg) []tea.Cmd {
	if msg == nil {
		return nil
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		return batch
	}
	p.msgs = append(p.msgs, msg)
	if _, ok := msg.(tea.QuitMsg); ok {
		return nil
	}
	var next tea.Cmd
	p.model, next = p.model.Update(msg)
	return []tea.Cmd{next}
}

func (p *pump) send(msg tea.Msg) {
	for _, c := range p.apply(msg) {
		p.run(c)
	}
}

// deliverNext waits for the oldest in-flight cmd to yield and feeds its msg
// into the model.
func (p *pump) deliverNext() {
	p.t.Helper()
	if len(p.inFlight) == 0 {
		p.t.Fatal("no in-flight cmd to deliver from")
	}
	done := p.inFlight[0]
	p.inFlight = p.inFlight[1:]
	select {
	case msg := <-done:
		for _, c := range p.apply(msg) {
			p.run(c)
		}
	case <-time.After(time.Second):
		p.t.Fatal("in-flight cmd did not yield within 1s")
	}
}

func (p *pump) view() string {
	return p.model.View().Content
}

func (p *pump) app() App {
	return p.model.(App)
}

// collectMsgs runs cmds (expanding batches) and returns their msgs without
// feeding them back into any model. Blocking cmds are dropped.
func collectMsgs(cmd tea.Cmd) []tea.Msg {
	var msgs []tea.Msg
	queue := []tea.Cmd{cmd}
	for len(queue) > 0 {
		c := queue[0]
		queue = queue[1:]
		if c == nil {
			continue
		}
		done := make(chan tea.Msg, 1)
		go func() { done <- c() }()
		var msg tea.Msg
		select {
		case msg = <-done:
		case <-time.After(50 * time.Millisecond):
			continue
		}
		if batch, ok := msg.(tea.BatchMsg); ok {
			queue = append(queue, batch...)
			continue
		}
		if msg != nil {
			msgs = append(msgs, msg)
		}
	}
	return msgs
}

func findBackendEvent(t *testing.T, msgs []tea.Msg) tea.Msg {
	t.Helper()
	for _, msg := range msgs {
		if _, ok := msg.(backendEventMsg); ok {
			return msg
		}
	}
	t.Fatalf("expected a backendEventMsg, got: %#v", msgs)
	return nil
}

func containsQuit(msgs []tea.Msg) bool {
	for _, msg := range msgs {
		if _, ok := msg.(tea.QuitMsg); ok {
			return true
		}
	}
	return false
}

func TestApp_InitializesWifiRadioStateFromBackend(t *testing.T) {
	f := fake.SeedArchLaptop()
	f.WifiOn = false
	p := newPump(t, New(f))

	if p.app().radioOn {
		t.Error("the app should adopt the backend's radio state at startup, not assume on")
	}
	if view := p.view(); !strings.Contains(view, "Wi-Fi is off — press W to enable") {
		t.Errorf("the wifi tab should show the radio-off state, got:\n%s", view)
	}

	// W from the off state must turn the radio on, not off.
	p.send(keyPress('W'))
	if calls := f.SetWifiEnabledCalls; len(calls) == 0 || !calls[len(calls)-1] {
		t.Errorf("W should enable the radio, got calls %v", f.SetWifiEnabledCalls)
	}
}

func TestApp_QPopsDeviceDetailLayerAndOnlyQuitsFromTopLevel(t *testing.T) {
	f := fake.SeedArchLaptop()
	p := newPump(t, New(f))

	p.send(keyPress('3'))
	p.send(keyPress('G'))
	p.send(keyPress('i'))
	if view := p.view(); !strings.Contains(view, "Device docker0") {
		t.Fatalf("precondition: i should push the device detail, got:\n%s", view)
	}

	p.send(keyPress('q'))
	if containsQuit(p.msgs) {
		t.Fatal("q on a pushed layer should pop it, not quit the app")
	}
	if view := p.view(); strings.Contains(view, "Device docker0") {
		t.Errorf("q should have popped the detail back to the list, got:\n%s", view)
	}

	p.send(keyPress('q'))
	if !containsQuit(p.msgs) {
		t.Error("q at top level should quit")
	}
}

func TestApp_QPopsConnectionEditorLayerInsteadOfQuitting(t *testing.T) {
	f := fake.SeedArchLaptop()
	p := newPump(t, New(f))

	p.send(keyPress('4'))
	p.send(keyPress('e'))
	if view := p.view(); !strings.Contains(view, "Autoconnect") {
		t.Fatalf("precondition: e should push the editor, got:\n%s", view)
	}

	p.send(keyPress('q'))
	if containsQuit(p.msgs) {
		t.Fatal("q inside the editor should pop it, not quit the app")
	}
	if view := p.view(); strings.Contains(view, "Autoconnect") {
		t.Errorf("q should have popped the editor back to the list, got:\n%s", view)
	}
}

func TestApp_BelowMinimumSizeShowsTooSmallScreenAndRecoversOnResize(t *testing.T) {
	f := fake.SeedArchLaptop()
	p := newPump(t, New(f))

	p.send(tea.WindowSizeMsg{Width: 59, Height: 16})
	view := p.view()
	if !strings.Contains(view, "Terminal too small (59x16, need 60x16)") {
		t.Errorf("a too-narrow terminal should show the too-small screen, got:\n%s", view)
	}
	if strings.Contains(view, "[1] wlan0") {
		t.Errorf("the too-small screen should replace the whole pane, got:\n%s", view)
	}

	p.send(tea.WindowSizeMsg{Width: 80, Height: 24})
	if view := p.view(); !strings.Contains(view, "[1] wlan0") {
		t.Errorf("growing the terminal should restore the app, got:\n%s", view)
	}
}

func TestApp_BackendConnectionErrorShowsNMNotRunningOnEveryTabUntilALoadSucceeds(t *testing.T) {
	f := fake.SeedArchLaptop()
	down := errors.New("dbus: connection refused")
	for _, call := range []string{"Devices", "Connections", "ActiveConnections", "AccessPoints", "Hostname"} {
		f.Errs[call] = down
	}
	p := newPump(t, New(f))

	const notice = "NetworkManager is not running — systemctl start NetworkManager"
	for _, tab := range []rune{'1', '2', '3'} {
		p.send(keyPress(tab))
		if view := p.view(); !strings.Contains(view, notice) {
			t.Errorf("tab %c should show the NM-not-running notice, got:\n%s", tab, view)
		}
	}

	// NM comes back: the device event re-derives the tabs and the next
	// successful reload clears the notice.
	f.Errs = map[string]error{}
	f.Push(domain.Event{Kind: domain.EventDeviceChanged, DeviceName: "wlan0"})
	p.deliverNext()
	for _, tab := range []rune{'1', '2', '3', '4', '5'} {
		p.send(keyPress(tab))
		if view := p.view(); strings.Contains(view, notice) {
			t.Errorf("tab %c should recover after a successful reload, got:\n%s", tab, view)
		}
	}
}

func TestApp_RadioToggleFailureRevertsStateAndReportsError(t *testing.T) {
	f := fake.SeedArchLaptop()
	f.Errs["SetWifiEnabled"] = errors.New("blocked by rfkill")
	p := newPump(t, New(f))

	p.send(keyPress('W'))
	if !p.app().radioOn {
		t.Error("a failed radio toggle should revert to the backend's actual state")
	}
	if view := p.view(); !strings.Contains(view, "blocked by rfkill") {
		t.Errorf("the failure should surface on the status line, got:\n%s", view)
	}
}

func TestApp_ModalDimsTheBackdropBehindIt(t *testing.T) {
	f := fake.SeedArchLaptop()
	p := newPump(t, New(f))
	p.send(tea.WindowSizeMsg{Width: 80, Height: 24})

	p.send(keyPress('3'))
	p.send(keyPress('G'))
	p.send(tea.KeyPressMsg{Code: tea.KeyEnter}) // confirm modal over the list

	view := p.view()
	if !strings.Contains(view, "Deactivate docker0?") {
		t.Fatalf("precondition: the confirm modal should be open, got:\n%s", view)
	}
	if !strings.Contains(view, "\x1b[2m") {
		t.Error("the backdrop under a modal should render faint")
	}
}
