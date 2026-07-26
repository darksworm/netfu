// Package ethernet is one wired device's tab: the device detail plus the
// wired profiles usable on this NIC, with activate/deactivate and profile
// management (edit, delete, new).
package ethernet

import (
	"fmt"
	"sort"
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

// actionResultMsg is a mutator's outcome for the status line; the list is
// reloaded so the outcome shows immediately.
type actionResultMsg struct {
	device string
	status string
}

type Model struct {
	backend  backend.Backend
	device   string
	keys     keys.Ethernet
	perms    domain.Permissions
	detail   domain.Device
	active   []domain.ActiveConnection
	profiles []domain.Connection
	cursor   int
	modal    *confirm.Model
	editor   *editor.Model
	status   string
	err      error
	width    int
	height   int
}

type loadedMsg struct {
	device   string
	detail   domain.Device
	active   []domain.ActiveConnection
	profiles []domain.Connection
	err      error
}

func New(b backend.Backend, device string) Model {
	return Model{backend: b, device: device, keys: keys.DefaultEthernet()}
}

func (m Model) Init() tea.Cmd {
	return m.load
}

func (m Model) load() tea.Msg {
	devices, err := m.backend.Devices()
	detail := domain.Device{Name: m.device}
	for _, d := range devices {
		if d.Name == m.device {
			detail = d
		}
	}
	active, activeErr := m.backend.ActiveConnections()
	if err == nil {
		err = activeErr
	}
	saved, savedErr := m.backend.Connections()
	if err == nil {
		err = savedErr
	}
	return loadedMsg{device: m.device, detail: detail, active: active,
		profiles: m.profilesUsableHere(saved), err: err}
}

// profilesUsableHere keeps the wired profiles this NIC can activate: not
// pinned to an interface, or pinned to this one. A failed settings read
// counts as unpinned — better to offer the profile than hide it.
func (m Model) profilesUsableHere(saved []domain.Connection) []domain.Connection {
	var out []domain.Connection
	for _, c := range saved {
		if c.Type != "802-3-ethernet" {
			continue
		}
		settings, err := m.backend.GetSettings(c.ID)
		pin := ""
		if err == nil {
			pin, _ = settings["connection"]["interface-name"].(string)
		}
		if pin == "" || pin == m.device {
			out = append(out, c)
		}
	}
	// Most recently used first, so the default selection is the profile the
	// user most likely wants to activate.
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].LastUsedUnix != out[j].LastUsedUnix {
			return out[i].LastUsedUnix > out[j].LastUsedUnix
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// Keys returns the keymap for the help footer: the editor's while it is
// pushed, the device tab's otherwise.
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
	allowed, known := m.perms[domain.PermModifySystem]
	return !known || allowed
}

// CapturesInput tells the root model to route every key here while the
// pushed editor or a modal is open; Esc pops layers, it never quits.
func (m Model) CapturesInput() bool {
	return m.editor != nil || m.modal != nil
}

// Status is the screen's line for the app's status bar.
func (m Model) Status() string {
	if m.editor != nil {
		return m.editor.Status()
	}
	return m.status
}

// Overlay is the open confirm's view, layered by the root model over the
// dimmed detail; empty when no modal is open.
func (m Model) Overlay() string {
	if m.modal != nil {
		return m.modal.View()
	}
	return ""
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
	case actionResultMsg:
		if msg.device != m.device {
			return m, nil
		}
		m.status = msg.status
		return m, m.load
	case loadedMsg:
		// Every device tab's Init runs off the same root model; only this
		// tab's own load may overwrite its detail.
		if msg.device != m.device {
			return m, nil
		}
		m.detail = msg.detail
		m.active = msg.active
		m.profiles = msg.profiles
		m.err = msg.err
		if m.cursor >= len(m.profiles) {
			m.cursor = max(len(m.profiles)-1, 0)
		}
		return m, nil
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
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
	switch {
	case key.Matches(msg, m.keys.Down):
		if m.cursor < len(m.profiles)-1 {
			m.cursor++
		}
	case key.Matches(msg, m.keys.Up):
		if m.cursor > 0 {
			m.cursor--
		}
	case key.Matches(msg, m.keys.Top):
		m.cursor = 0
	case key.Matches(msg, m.keys.Bottom):
		if len(m.profiles) > 0 {
			m.cursor = len(m.profiles) - 1
		}
	case key.Matches(msg, m.keys.Activate):
		return m.activateSelected()
	case key.Matches(msg, m.keys.Deactivate):
		return m.offerDeactivate()
	case key.Matches(msg, m.keys.Edit):
		if !m.canModify() {
			return m.denyModify()
		}
		return m.openEditor()
	case key.Matches(msg, m.keys.Delete):
		if !m.canModify() {
			return m.denyModify()
		}
		return m.offerDelete()
	case key.Matches(msg, m.keys.New):
		if !m.canModify() {
			return m.denyModify()
		}
		ed := editor.NewWiredProfile(m.backend, m.device)
		m.editor = &ed
		return m, ed.Init()
	}
	return m, nil
}

func (m Model) denyModify() (Model, tea.Cmd) {
	m.status = "🔒 not permitted (polkit)"
	return m, nil
}

// offerDelete opens the confirm modal for the selected profile; x is never
// destructive without it.
func (m Model) offerDelete() (Model, tea.Cmd) {
	conn := m.Selected()
	if conn.ID == "" {
		return m, nil
	}
	mut, device := m.backend, m.device
	modal := confirm.New(
		fmt.Sprintf("Delete %s?", conn.Name),
		func() tea.Msg {
			if err := mut.DeleteConnection(conn.ID); err != nil {
				return actionResultMsg{device: device, status: fmt.Sprintf("✗ delete %s: %v", conn.Name, err)}
			}
			return actionResultMsg{device: device, status: fmt.Sprintf("✓ deleted %s", conn.Name)}
		},
	)
	m.modal = &modal
	return m, nil
}

// openEditor pushes the selected profile's editor.
func (m Model) openEditor() (Model, tea.Cmd) {
	conn := m.Selected()
	if conn.ID == "" {
		return m, nil
	}
	ed := editor.New(m.backend, conn)
	m.editor = &ed
	return m, ed.Init()
}

// Selected returns the wired profile under the cursor.
func (m Model) Selected() domain.Connection {
	if len(m.profiles) == 0 {
		return domain.Connection{}
	}
	return m.profiles[m.cursor]
}

func (m Model) activateSelected() (Model, tea.Cmd) {
	conn := m.Selected()
	if conn.ID == "" {
		m.status = "no wired profile to activate — press n to create one"
		return m, nil
	}
	if _, alreadyActive := m.activeOn(conn); alreadyActive {
		return m, nil
	}
	mut, device := m.backend, m.device
	return m, func() tea.Msg {
		if err := mut.Activate(conn.ID, device); err != nil {
			return actionResultMsg{device: device, status: fmt.Sprintf("✗ activate %s: %v", conn.Name, err)}
		}
		return actionResultMsg{device: device, status: fmt.Sprintf("Activating %s on %s…", conn.Name, device)}
	}
}

// offerDeactivate opens the confirm modal for the device's active
// connection; d is never destructive without it.
func (m Model) offerDeactivate() (Model, tea.Cmd) {
	if m.detail.State != domain.DeviceStateConnected {
		return m, nil
	}
	for _, ac := range m.active {
		if ac.DeviceName == m.device {
			mut, device := m.backend, m.device
			modal := confirm.New(
				fmt.Sprintf("Deactivate %s?", ac.Name),
				func() tea.Msg {
					if err := mut.Deactivate(ac.ID); err != nil {
						return actionResultMsg{device: device, status: fmt.Sprintf("✗ deactivate %s: %v", ac.Name, err)}
					}
					return actionResultMsg{device: device, status: fmt.Sprintf("Deactivating %s…", ac.Name)}
				},
			)
			m.modal = &modal
			return m, nil
		}
	}
	return m, nil
}

func (m Model) View() string {
	if m.err != nil {
		return style.NMNotRunningNotice + "\n"
	}
	if m.editor != nil {
		return m.editor.View()
	}
	activeConnection := m.detail.ActiveConnection
	if activeConnection == "" {
		activeConnection = "—"
	}
	lines := []string{
		style.Title.Render("Device " + m.device),
		fmt.Sprintf("%-20s %s", "Type:", m.detail.Type),
		fmt.Sprintf("%-20s %s", "State:", m.detail.State),
		fmt.Sprintf("%-20s %s", "Active connection:", activeConnection),
	}
	if len(m.profiles) > 0 {
		lines = append(lines, "",
			style.Faint.Render(fmt.Sprintf("   %s %s", style.Cell("NAME", m.nameWidth()), "LAST USED")))
		if !m.canModify() {
			lines = append(lines, style.Faint.Render("🔒 edit · delete · new — not permitted (polkit)"))
		}
		for i, c := range m.profiles {
			lines = append(lines, m.renderRow(c, i == m.cursor))
		}
	}
	return style.Fit(lines, m.width, m.height)
}

// nameWidth is the flexible NAME column: fixed columns and gaps take the
// rest, over-long names are trimmed.
func (m Model) nameWidth() int {
	return style.FlexCell(m.width, 18, 20)
}

func (m Model) renderRow(c domain.Connection, selected bool) string {
	cursor, mark := " ", " "
	if selected {
		cursor = "▸"
	}
	if _, ok := m.activeOn(c); ok {
		mark = "✓"
	}
	row := fmt.Sprintf("%s%s %s %s", cursor, mark, style.Cell(c.Name, m.nameWidth()), lastUsed(c))
	if selected {
		return style.SelectedRow(row, m.width)
	}
	return row
}

// activeOn returns the active connection a profile is up as, if any.
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
