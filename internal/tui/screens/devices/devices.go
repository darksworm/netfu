// Package devices is the Devices tab: managed devices with state and
// active connection, plus activate/deactivate actions.
package devices

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/ilmars/netfu/internal/backend"
	"github.com/ilmars/netfu/internal/domain"
	"github.com/ilmars/netfu/internal/tui/components/confirm"
	"github.com/ilmars/netfu/internal/tui/keys"
	"github.com/ilmars/netfu/internal/tui/style"
)

// StatusMsg is feedback for the app's status line above the footer.
type StatusMsg string

type Model struct {
	backend backend.Backend
	keys    keys.Devices
	devices []domain.Device
	active  []domain.ActiveConnection
	saved   []domain.Connection
	cursor  int
	// filter narrows the list by device name; filtering means keystrokes
	// edit the query instead of navigating.
	filter    string
	filtering bool
	// showDetail pushes the read-only device detail as a layer over the
	// list; Esc pops back.
	showDetail bool
	modal      *confirm.Model
	err        error
	width      int
	height     int
}

type devicesLoadedMsg struct {
	devices []domain.Device
	active  []domain.ActiveConnection
	saved   []domain.Connection
	err     error
}

func New(b backend.Backend) Model {
	return Model{backend: b, keys: keys.DefaultDevices()}
}

func (m Model) Init() tea.Cmd {
	return m.loadDevices
}

func (m Model) loadDevices() tea.Msg {
	all, err := m.backend.Devices()
	var managed []domain.Device
	for _, d := range all {
		if d.Managed {
			managed = append(managed, d)
		}
	}
	groups := domain.GroupDevices(managed)
	managed = append(groups.Physical, groups.Virtual...)
	active, activeErr := m.backend.ActiveConnections()
	if err == nil {
		err = activeErr
	}
	saved, savedErr := m.backend.Connections()
	if err == nil {
		err = savedErr
	}
	return devicesLoadedMsg{devices: managed, active: active, saved: saved, err: err}
}

func (m Model) Keys() keys.Devices {
	return m.keys
}

// Layered reports whether a pushed layer is open, so the app routes q here
// to pop it instead of quitting.
func (m Model) Layered() bool {
	return m.showDetail
}

// Overlay is the open modal's view, layered by the root model over the
// dimmed list; empty when no modal is open.
func (m Model) Overlay() string {
	if m.modal != nil {
		return m.modal.View()
	}
	return ""
}

// visible is the row model the cursor moves over: the managed devices that
// match the filter.
func (m Model) visible() []domain.Device {
	if m.filter == "" {
		return m.devices
	}
	var matched []domain.Device
	for _, d := range m.devices {
		if strings.Contains(strings.ToLower(d.Name), strings.ToLower(m.filter)) {
			matched = append(matched, d)
		}
	}
	return matched
}

func (m Model) clampCursor() Model {
	if visible := m.visible(); m.cursor >= len(visible) {
		m.cursor = max(len(visible)-1, 0)
	}
	return m
}

func (m Model) Selected() domain.Device {
	visible := m.visible()
	if len(visible) == 0 {
		return domain.Device{}
	}
	return visible[m.cursor]
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case devicesLoadedMsg:
		m.devices = msg.devices
		m.active = msg.active
		m.saved = msg.saved
		m.err = msg.err
		// Reloads happen live; keep the user's place, only clamping if the
		// visible set shrank.
		return m.clampCursor(), nil
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if m.modal != nil {
		done, cmd := m.modal.Update(msg)
		if done {
			m.modal = nil
		}
		return m, cmd
	}
	if m.filtering {
		return m.handleFilterKey(msg), nil
	}
	if m.showDetail {
		if msg.Code == tea.KeyEsc || msg.Text == "q" {
			m.showDetail = false
		}
		return m, nil
	}
	switch {
	case key.Matches(msg, m.keys.Info):
		if len(m.visible()) > 0 {
			m.showDetail = true
		}
	case key.Matches(msg, m.keys.Down):
		if m.cursor < len(m.visible())-1 {
			m.cursor++
		}
	case key.Matches(msg, m.keys.Up):
		if m.cursor > 0 {
			m.cursor--
		}
	case key.Matches(msg, m.keys.Top):
		m.cursor = 0
	case key.Matches(msg, m.keys.Bottom):
		if visible := m.visible(); len(visible) > 0 {
			m.cursor = len(visible) - 1
		}
	case key.Matches(msg, m.keys.Filter):
		m.filtering = true
	case msg.Code == tea.KeyEsc:
		m.filter = ""
	case key.Matches(msg, m.keys.Enter):
		// Enter is contextual and never destructive: connected devices get
		// a confirm, disconnected ones just activate.
		if m.Selected().State == domain.DeviceStateConnected {
			return m.offerDeactivate()
		}
		return m.activateSavedProfile()
	case key.Matches(msg, m.keys.Activate):
		return m.activateSavedProfile()
	case key.Matches(msg, m.keys.Deactivate):
		return m.offerDeactivate()
	}
	return m, nil
}

func (m Model) handleFilterKey(msg tea.KeyPressMsg) Model {
	switch msg.Code {
	case tea.KeyEsc:
		m.filter = ""
		m.filtering = false
	case tea.KeyEnter:
		m.filtering = false
	case tea.KeyBackspace:
		if m.filter != "" {
			m.filter = m.filter[:len(m.filter)-1]
		}
	default:
		m.filter += msg.Text
	}
	return m.clampCursor()
}

// connectionTypeFor maps a device type to NM's connection type key, used to
// find a saved profile that fits the device.
func connectionTypeFor(t domain.DeviceType) string {
	switch t {
	case domain.DeviceTypeEthernet:
		return "802-3-ethernet"
	case domain.DeviceTypeWifi:
		return "802-11-wireless"
	default:
		return ""
	}
}

func (m Model) activateSavedProfile() (Model, tea.Cmd) {
	device := m.Selected()
	if device.State != domain.DeviceStateDisconnected {
		return m, nil
	}
	wantType := connectionTypeFor(device.Type)
	for _, conn := range m.saved {
		if conn.Type == wantType {
			return m, activate(m.backend, conn, device)
		}
	}
	return m, nil
}

func activate(mut backend.Mutator, conn domain.Connection, device domain.Device) tea.Cmd {
	return func() tea.Msg {
		if err := mut.Activate(conn.ID, device.Name); err != nil {
			return StatusMsg(fmt.Sprintf("✗ activate %s: %v", conn.Name, err))
		}
		return StatusMsg(fmt.Sprintf("Activating %s on %s…", conn.Name, device.Name))
	}
}

// offerDeactivate opens the confirm modal for the selected device's active
// connection; Enter is never destructive without it.
func (m Model) offerDeactivate() (Model, tea.Cmd) {
	device := m.Selected()
	if device.State != domain.DeviceStateConnected {
		return m, nil
	}
	for _, ac := range m.active {
		if ac.DeviceName == device.Name {
			modal := confirm.New(
				fmt.Sprintf("Deactivate %s?", ac.Name),
				deactivate(m.backend, ac),
			)
			m.modal = &modal
			return m, nil
		}
	}
	return m, nil
}

func deactivate(mut backend.Mutator, ac domain.ActiveConnection) tea.Cmd {
	return func() tea.Msg {
		if err := mut.Deactivate(ac.ID); err != nil {
			return StatusMsg(fmt.Sprintf("✗ deactivate %s: %v", ac.Name, err))
		}
		return StatusMsg(fmt.Sprintf("Deactivating %s…", ac.Name))
	}
}

func (m Model) View() string {
	if m.err != nil {
		return style.NMNotRunningNotice + "\n"
	}
	if m.showDetail {
		return m.detailView()
	}
	// Fixed columns: gutter, TYPE, STATE and their gaps; DEVICE and
	// CONNECTION split what remains, DEVICE trimmed when it doesn't fit.
	nameWidth := style.FlexCell(m.width, 48, 12)
	row := func(name, typ, state, conn string) string {
		return fmt.Sprintf("%s  %-10s  %-14s  %s", style.Cell(name, nameWidth), typ, state, conn)
	}
	var lines []string
	lines = append(lines, style.Faint.Render("  "+row("DEVICE", "TYPE", "STATE", "CONNECTION")))
	if m.filtering || m.filter != "" {
		lines = append(lines, "/"+m.filter)
	}
	for i, d := range m.visible() {
		row := row(d.Name, string(d.Type), string(d.State), d.ActiveConnection)
		if i == m.cursor {
			lines = append(lines, style.SelectedRow("▸ "+row, m.width))
		} else {
			lines = append(lines, "  "+row)
		}
	}
	return style.Fit(lines, m.width, m.height)
}

func (m Model) detailView() string {
	d := m.Selected()
	lines := []string{
		style.Title.Render("Device " + d.Name),
		fmt.Sprintf("%-20s %s", "Name:", d.Name),
		fmt.Sprintf("%-20s %s", "Type:", d.Type),
		fmt.Sprintf("%-20s %s", "State:", d.State),
		fmt.Sprintf("%-20s %s", "Active connection:", d.ActiveConnection),
		"esc back",
	}
	return style.Fit(lines, m.width, m.height)
}
