package tui

import (
	"image/color"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/ilmars/netfu/internal/backend/fake"
	"github.com/ilmars/netfu/internal/domain"
)

func TestApp_StartsOnWifiTabAndQQuits(t *testing.T) {
	f := fake.SeedArchLaptop()
	p := newPump(t, New(f))

	view := p.view()
	if !strings.Contains(view, "[1] Wi-Fi") || !strings.Contains(view, "[2] Devices") {
		t.Errorf("landing view should show the tab bar, got:\n%s", view)
	}
	if p.app().tab != tabWifi {
		t.Errorf("the app should land on the Wi-Fi tab, got tab %d", p.app().tab)
	}
	if strings.Contains(view, "enp0s31f6") {
		t.Errorf("the Wi-Fi tab should not render the device list, got:\n%s", view)
	}

	p.send(keyPress('q'))
	if !containsQuit(p.msgs) {
		t.Errorf("pressing q at top level should quit, got msgs: %#v", p.msgs)
	}
}

func TestApp_DevicesTabShowsDeviceListAndQQuits(t *testing.T) {
	f := fake.SeedArchLaptop()
	p := newPump(t, New(f))

	p.send(keyPress('2'))
	if view := p.view(); !strings.Contains(view, "enp0s31f6") {
		t.Errorf("the Devices tab should show the device list, got:\n%s", view)
	}

	p.send(keyPress('q'))
	if !containsQuit(p.msgs) {
		t.Errorf("pressing q at top level should quit, got msgs: %#v", p.msgs)
	}
}

func TestApp_NumberKeysAndBracketsSwitchTabs(t *testing.T) {
	f := fake.SeedArchLaptop()
	p := newPump(t, New(f))

	p.send(keyPress('3'))
	if p.app().tab != tabConnections {
		t.Errorf("3 should open the Connections tab, got tab %d", p.app().tab)
	}
	if view := p.view(); !strings.Contains(view, "coming soon") {
		t.Errorf("Connections tab should show its placeholder, got:\n%s", view)
	}

	p.send(keyPress(']'))
	if p.app().tab != tabSystem {
		t.Errorf("] should move to the next tab, got tab %d", p.app().tab)
	}

	p.send(keyPress(']'))
	if p.app().tab != tabWifi {
		t.Errorf("] on the last tab should wrap to Wi-Fi, got tab %d", p.app().tab)
	}

	p.send(keyPress('['))
	if p.app().tab != tabSystem {
		t.Errorf("[ on the first tab should wrap to System, got tab %d", p.app().tab)
	}
}

func TestApp_TabStatePersistsAcrossSwitches(t *testing.T) {
	f := fake.SeedArchLaptop()
	p := newPump(t, New(f))

	p.send(keyPress('2'))
	p.send(keyPress('j'))
	if got := p.app().devices.Selected().Name; got != "enp0s31f6" {
		t.Fatalf("precondition: cursor should be on enp0s31f6, got %q", got)
	}

	p.send(keyPress('1'))
	p.send(keyPress('2'))
	if got := p.app().devices.Selected().Name; got != "enp0s31f6" {
		t.Errorf("returning to the Devices tab should keep its cursor, got %q", got)
	}
}

func TestApp_ResizePropagatesToActiveScreenAndListsReflow(t *testing.T) {
	f := fake.SeedArchLaptop()
	p := newPump(t, New(f))

	p.send(tea.WindowSizeMsg{Width: 20, Height: 3})

	view := p.view()
	lines := strings.Split(strings.TrimRight(view, "\n"), "\n")
	if len(lines) > 3 {
		t.Errorf("view should fit in 3 rows after resize, got %d:\n%s", len(lines), view)
	}
	for _, line := range lines {
		if w := lipgloss.Width(line); w > 20 {
			t.Errorf("line %q is %d cells wide, should reflow to <= 20", line, w)
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
}

func TestDevices_DeviceStateEventUpdatesRowLive(t *testing.T) {
	f := fake.SeedArchLaptop()
	p := newPump(t, New(f))

	p.send(keyPress('2'))
	p.send(keyPress('j'))
	if got := p.app().devices.Selected().Name; got != "enp0s31f6" {
		t.Fatalf("precondition: cursor should be on enp0s31f6, got %q", got)
	}

	// Cable plugged in: backend state changes and an event fires.
	f.DeviceList[1].State = domain.DeviceStateConnected
	f.Push(domain.Event{Kind: domain.EventDeviceChanged, DeviceName: "enp0s31f6"})
	p.deliverNext()

	if view := p.view(); !strings.Contains(view, "enp0s31f6  ethernet  connected") {
		t.Errorf("row should show the new device state, got:\n%s", view)
	}
	if got := p.app().devices.Selected().Name; got != "enp0s31f6" {
		t.Errorf("live update should keep the cursor on enp0s31f6, got %q", got)
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
