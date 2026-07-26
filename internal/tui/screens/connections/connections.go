// Package connections is the Connections tab: every saved profile grouped
// by type, with edit/delete/new actions.
package connections

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/ilmars/netfu/internal/backend"
	"github.com/ilmars/netfu/internal/domain"
	"github.com/ilmars/netfu/internal/tui/components/confirm"
	"github.com/ilmars/netfu/internal/tui/keys"
	"github.com/ilmars/netfu/internal/tui/screens/editor"
	"github.com/ilmars/netfu/internal/tui/style"
)

// permModifySystem is the polkit permission gating profile changes.
const permModifySystem = "org.freedesktop.NetworkManager.settings.modify.system"

type Model struct {
	backend backend.Backend
	keys    keys.Connections
	perms   domain.Permissions
	conns   []domain.Connection
	active  []domain.ActiveConnection
	devices []domain.Device
	cursor  int
	modal   *confirm.Model
	editor  *editor.Model
	// picking is the new-connection type picker; pickCursor selects among
	// creatableTypes.
	picking    bool
	pickCursor int
	status     string
	err        error
	width      int
	height     int
}

// actionResultMsg is a mutator's outcome for the status line; the list is
// reloaded so the outcome shows immediately.
type actionResultMsg struct {
	status string
}

type loadedMsg struct {
	conns   []domain.Connection
	active  []domain.ActiveConnection
	devices []domain.Device
	err     error
}

func New(b backend.Backend) Model {
	return Model{backend: b, keys: keys.DefaultConnections()}
}

func (m Model) Init() tea.Cmd {
	return m.load
}

func (m Model) load() tea.Msg {
	conns, err := m.backend.Connections()
	active, activeErr := m.backend.ActiveConnections()
	if err == nil {
		err = activeErr
	}
	devices, devicesErr := m.backend.Devices()
	if err == nil {
		err = devicesErr
	}
	return loadedMsg{conns: conns, active: active, devices: devices, err: err}
}

// Keys returns the keymap for the help footer: the editor's while it is
// pushed, the list's otherwise.
func (m Model) Keys() help.KeyMap {
	if m.editor != nil {
		return keys.DefaultEditor()
	}
	return m.keys
}

// SetPermissions caches the startup polkit query result.
func (m Model) SetPermissions(perms domain.Permissions) Model {
	m.perms = perms
	return m
}

// canModify defaults to allowed while the permission is unknown; polkit
// only locks the actions when it explicitly denies them.
func (m Model) canModify() bool {
	allowed, known := m.perms[permModifySystem]
	return !known || allowed
}

// CapturesInput tells the root model to route every key here while the
// pushed editor or a modal is open; Esc pops layers, it never quits.
func (m Model) CapturesInput() bool {
	return m.editor != nil || m.modal != nil || m.picking
}

// creatableTypes is what the new-connection picker offers. VPN profiles
// cannot be created over D-Bus, so VPN is deliberately not creatable.
var creatableTypes = []struct {
	label  string
	nmType string
}{
	{"Ethernet", "802-3-ethernet"},
	{"Wi-Fi", "802-11-wireless"},
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case editor.SavedMsg:
		m.editor = nil
		m.status = msg.Status
		return m, m.load
	case editor.ClosedMsg:
		m.editor = nil
		return m, nil
	case loadedMsg:
		m.conns = msg.conns
		m.active = msg.active
		m.devices = msg.devices
		m.err = msg.err
		if rows := m.rows(); m.cursor >= len(rows) {
			m.cursor = max(len(rows)-1, 0)
		}
		return m, nil
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case actionResultMsg:
		m.status = msg.status
		return m, m.load
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}
	return m.forwardToEditor(msg)
}

// forwardToEditor routes the editor's own async msgs (settings loads, save
// results) into the pushed editor.
func (m Model) forwardToEditor(msg tea.Msg) (Model, tea.Cmd) {
	if m.editor == nil {
		return m, nil
	}
	ed, cmd := m.editor.Update(msg)
	m.editor = &ed
	return m, cmd
}

// Status is the screen's line for the app's status bar.
func (m Model) Status() string {
	if m.editor != nil {
		return m.editor.Status()
	}
	return m.status
}

func (m Model) handleKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if m.editor != nil {
		return m.forwardToEditor(msg)
	}
	if m.modal != nil {
		done, cmd := m.modal.Update(msg)
		if done {
			m.modal = nil
		}
		return m, cmd
	}
	if m.picking {
		return m.handlePickerKey(msg)
	}
	rows := m.rows()
	switch {
	case key.Matches(msg, m.keys.Down):
		if m.cursor < len(rows)-1 {
			m.cursor++
		}
	case key.Matches(msg, m.keys.Up):
		if m.cursor > 0 {
			m.cursor--
		}
	case key.Matches(msg, m.keys.Top):
		m.cursor = 0
	case key.Matches(msg, m.keys.Bottom):
		if len(rows) > 0 {
			m.cursor = len(rows) - 1
		}
	case key.Matches(msg, m.keys.Delete):
		if !m.canModify() {
			return m.denyModify()
		}
		return m.offerDelete()
	case key.Matches(msg, m.keys.Edit):
		if !m.canModify() {
			return m.denyModify()
		}
		return m.openEditor()
	case key.Matches(msg, m.keys.New):
		if !m.canModify() {
			return m.denyModify()
		}
		m.picking = true
		m.pickCursor = 0
	case key.Matches(msg, m.keys.Activate):
		return m.activateSelected()
	case key.Matches(msg, m.keys.Deactivate):
		return m.offerDeactivate()
	}
	return m, nil
}

// deviceTypeFor maps NM's connection type key to the device type the
// profile activates on.
func deviceTypeFor(nmType string) domain.DeviceType {
	switch nmType {
	case "802-3-ethernet":
		return domain.DeviceTypeEthernet
	case "802-11-wireless":
		return domain.DeviceTypeWifi
	}
	return ""
}

func (m Model) activateSelected() (Model, tea.Cmd) {
	conn := m.Selected()
	if conn.ID == "" {
		return m, nil
	}
	if _, alreadyActive := m.activeOn(conn); alreadyActive {
		return m, nil
	}
	device := ""
	if want := deviceTypeFor(conn.Type); want != "" {
		for _, d := range m.devices {
			if d.Managed && d.Type == want {
				device = d.Name
				break
			}
		}
	}
	mut := m.backend
	return m, func() tea.Msg {
		if err := mut.Activate(conn.ID, device); err != nil {
			return actionResultMsg{status: fmt.Sprintf("✗ activate %s: %v", conn.Name, err)}
		}
		return actionResultMsg{status: fmt.Sprintf("Activating %s…", conn.Name)}
	}
}

func (m Model) offerDeactivate() (Model, tea.Cmd) {
	conn := m.Selected()
	ac, ok := m.activeOn(conn)
	if !ok {
		return m, nil
	}
	mut := m.backend
	modal := confirm.New(
		fmt.Sprintf("Deactivate %s?", conn.Name),
		func() tea.Msg {
			if err := mut.Deactivate(ac.ID); err != nil {
				return actionResultMsg{status: fmt.Sprintf("✗ deactivate %s: %v", conn.Name, err)}
			}
			return actionResultMsg{status: fmt.Sprintf("Deactivating %s…", conn.Name)}
		},
	)
	m.modal = &modal
	return m, nil
}

func (m Model) denyModify() (Model, tea.Cmd) {
	m.status = "🔒 not permitted (polkit)"
	return m, nil
}

func (m Model) handlePickerKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Down):
		if m.pickCursor < len(creatableTypes)-1 {
			m.pickCursor++
		}
	case key.Matches(msg, m.keys.Up):
		if m.pickCursor > 0 {
			m.pickCursor--
		}
	case msg.Code == tea.KeyEnter:
		m.picking = false
		ed := editor.NewProfile(m.backend, creatableTypes[m.pickCursor].nmType)
		m.editor = &ed
		return m, ed.Init()
	case msg.Code == tea.KeyEsc:
		m.picking = false
	}
	return m, nil
}

func (m Model) pickerView() string {
	lines := []string{style.Title.Render("New connection — choose a type")}
	for i, t := range creatableTypes {
		cursor := " "
		if i == m.pickCursor {
			cursor = "▸"
		}
		lines = append(lines, cursor+" "+t.label)
	}
	lines = append(lines, "", "↵ choose · esc cancel")
	return strings.Join(lines, "\n") + "\n"
}

func (m Model) openEditor() (Model, tea.Cmd) {
	conn := m.Selected()
	if conn.ID == "" {
		return m, nil
	}
	ed := editor.New(m.backend, conn)
	m.editor = &ed
	return m, ed.Init()
}

func (m Model) offerDelete() (Model, tea.Cmd) {
	conn := m.Selected()
	if conn.ID == "" {
		return m, nil
	}
	modal := confirm.New(
		fmt.Sprintf("Delete %s?", conn.Name),
		deleteConnection(m.backend, conn),
	)
	m.modal = &modal
	return m, nil
}

func deleteConnection(mut backend.Mutator, conn domain.Connection) tea.Cmd {
	return func() tea.Msg {
		if err := mut.DeleteConnection(conn.ID); err != nil {
			return actionResultMsg{status: fmt.Sprintf("✗ delete %s: %v", conn.Name, err)}
		}
		return actionResultMsg{status: fmt.Sprintf("✓ deleted %s", conn.Name)}
	}
}

// typeLabel maps NM connection types to the group headers users know.
func typeLabel(nmType string) string {
	switch nmType {
	case "802-11-wireless":
		return "Wi-Fi"
	case "802-3-ethernet":
		return "Ethernet"
	case "vpn", "wireguard":
		return "VPN"
	case "bridge":
		return "Bridge"
	}
	return nmType
}

type group struct {
	label string
	conns []domain.Connection
}

// groups buckets the profiles by type label, well-known types first, any
// remaining labels in first-seen order.
func (m Model) groups() []group {
	byLabel := map[string][]domain.Connection{}
	order := []string{"Wi-Fi", "Ethernet", "VPN"}
	for _, c := range m.conns {
		label := typeLabel(c.Type)
		if _, seen := byLabel[label]; !seen && !slices.Contains(order, label) {
			order = append(order, label)
		}
		byLabel[label] = append(byLabel[label], c)
	}
	var groups []group
	for _, label := range order {
		if len(byLabel[label]) > 0 {
			groups = append(groups, group{label: label, conns: byLabel[label]})
		}
	}
	return groups
}

// rows is the flat selectable list the cursor moves over: the grouped
// profiles without their headers.
func (m Model) rows() []domain.Connection {
	var rows []domain.Connection
	for _, g := range m.groups() {
		rows = append(rows, g.conns...)
	}
	return rows
}

func (m Model) Selected() domain.Connection {
	rows := m.rows()
	if len(rows) == 0 {
		return domain.Connection{}
	}
	return rows[m.cursor]
}

// activeOn returns the device a profile is currently active on, if any.
func (m Model) activeOn(c domain.Connection) (domain.ActiveConnection, bool) {
	for _, ac := range m.active {
		if ac.ID == c.ID {
			return ac, true
		}
	}
	return domain.ActiveConnection{}, false
}

func lastUsed(c domain.Connection) string {
	if c.LastUsedUnix == 0 {
		return "never"
	}
	return time.Unix(c.LastUsedUnix, 0).UTC().Format("2006-01-02")
}

func (m Model) renderRow(c domain.Connection, selected bool) string {
	cursor, mark, device := " ", " ", "—"
	if selected {
		cursor = "▸"
	}
	if ac, ok := m.activeOn(c); ok {
		mark = "✓"
		device = ac.DeviceName
	}
	row := fmt.Sprintf("%s%s %-20s %-10s %-10s %s",
		cursor, mark, c.Name, strings.ToLower(typeLabel(c.Type)), device, lastUsed(c))
	if selected {
		return style.Selected.Render(row)
	}
	return row
}

func (m Model) View() string {
	if m.err != nil {
		return style.NMNotRunningNotice + "\n"
	}
	if m.editor != nil {
		return m.editor.View()
	}
	if m.picking {
		return m.pickerView()
	}
	lines := []string{
		style.Title.Render("Connections"),
		fmt.Sprintf("   %-20s %-10s %-10s %s", "NAME", "TYPE", "DEVICE", "LAST USED"),
	}
	if !m.canModify() {
		lines = append(lines, style.Faint.Render("🔒 edit · delete · new — not permitted (polkit)"))
	}
	selected := m.Selected()
	for _, g := range m.groups() {
		lines = append(lines, "─ "+g.label+" ─")
		for _, c := range g.conns {
			lines = append(lines, m.renderRow(c, c.ID == selected.ID))
		}
	}
	if m.modal != nil {
		lines = append(lines, strings.Split(m.modal.View(), "\n")...)
	}
	return style.Fit(lines, m.width, m.height)
}
