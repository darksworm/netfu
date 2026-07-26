// Package system is the System tab: hostname, radio toggles, and NM state.
// Connection activation (VPN included) lives on the Other tab.
package system

import (
	"fmt"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/ilmars/netfu/internal/backend"
	"github.com/ilmars/netfu/internal/domain"
	"github.com/ilmars/netfu/internal/tui/keys"
	"github.com/ilmars/netfu/internal/tui/style"
)

// StatusMsg is feedback for the app's status line above the footer.
type StatusMsg string

const permModifyHostname = "org.freedesktop.NetworkManager.settings.modify.hostname"

type Model struct {
	backend     backend.Backend
	keys        keys.System
	hostname    string
	wifiEnabled bool
	nmState     domain.NMState
	cursor      int
	editing     bool
	// draft is the hostname editor's text; hand-rolled like the filter
	// inputs elsewhere (bubbles' textinput would drag in a new dependency).
	draft  string
	perms  domain.Permissions
	err    error
	width  int
	height int
}

type loadedMsg struct {
	hostname    string
	wifiEnabled bool
	nmState     domain.NMState
	err         error
}

type hostnameSavedMsg struct {
	name string
	err  error
}

type radioSetMsg struct {
	enabled bool
	err     error
}

type rowKind int

const (
	rowHostname rowKind = iota
	rowRadio
)

// row is one selectable settings field.
type row struct {
	kind   rowKind
	name   string
	detail string
}

func New(b backend.Backend) Model {
	return Model{backend: b, keys: keys.DefaultSystem(), wifiEnabled: true}
}

func (m Model) Init() tea.Cmd {
	return m.load
}

func (m Model) load() tea.Msg {
	hostname, err := m.backend.Hostname()
	if err != nil {
		return loadedMsg{err: err}
	}
	wifiEnabled, err := m.backend.WifiEnabled()
	if err != nil {
		return loadedMsg{err: err}
	}
	nmState, err := m.backend.NMState()
	if err != nil {
		return loadedMsg{err: err}
	}
	return loadedMsg{hostname: hostname, wifiEnabled: wifiEnabled, nmState: nmState}
}

func (m Model) Keys() keys.System {
	return m.keys
}

// WithPermissions receives the app's startup polkit query result.
func (m Model) WithPermissions(perms domain.Permissions) Model {
	m.perms = perms
	return m
}

// hostnameAllowed treats an unknown permission as allowed: only an explicit
// polkit "no" greys the field, never a missing entry.
func (m Model) hostnameAllowed() bool {
	allowed, known := m.perms[permModifyHostname]
	return allowed || !known
}

// CapturesInput tells the root model to route every key here while a field
// editor is open; digits would otherwise switch tabs.
func (m Model) CapturesInput() bool {
	return m.editing
}

func (m Model) rows() []row {
	return []row{
		{kind: rowHostname, name: "Hostname", detail: m.hostname},
		{kind: rowRadio, name: "Wi-Fi radio", detail: onOff(m.wifiEnabled)},
	}
}

func (m Model) selected() row {
	rows := m.rows()
	if len(rows) == 0 || m.cursor >= len(rows) {
		return row{}
	}
	return rows[m.cursor]
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case loadedMsg:
		m.hostname = msg.hostname
		if msg.err == nil {
			m.wifiEnabled = msg.wifiEnabled
			m.nmState = msg.nmState
		}
		m.err = msg.err
		if rows := m.rows(); m.cursor >= len(rows) {
			m.cursor = max(len(rows)-1, 0)
		}
		return m, nil
	case radioSetMsg:
		if msg.err != nil {
			return m, status("✗ wifi radio: %v", msg.err)
		}
		m.wifiEnabled = msg.enabled
		return m, status("✓ wifi radio %s", onOff(msg.enabled))
	case hostnameSavedMsg:
		if msg.err != nil {
			return m, status("✗ set hostname: %v", msg.err)
		}
		m.hostname = msg.name
		return m, status("✓ hostname set to %s", msg.name)
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func status(format string, args ...any) tea.Cmd {
	return func() tea.Msg {
		return StatusMsg(fmt.Sprintf(format, args...))
	}
}

func (m Model) handleKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if m.editing {
		return m.handleEditKey(msg)
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
	case key.Matches(msg, m.keys.Edit):
		switch m.selected().kind {
		case rowHostname:
			return m.startHostnameEdit()
		case rowRadio:
			return m, setRadio(m.backend, !m.wifiEnabled)
		}
	case key.Matches(msg, m.keys.Toggle):
		if m.selected().kind == rowRadio {
			return m, setRadio(m.backend, !m.wifiEnabled)
		}
	}
	return m, nil
}

func (m Model) startHostnameEdit() (Model, tea.Cmd) {
	if !m.hostnameAllowed() {
		return m, status("🔒 hostname: not permitted (polkit)")
	}
	m.editing = true
	m.draft = m.hostname
	return m, nil
}

func (m Model) handleEditKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case msg.Code == tea.KeyEsc:
		m.editing = false
		return m, nil
	case msg.Code == tea.KeyEnter:
		m.editing = false
		return m, saveHostname(m.backend, m.draft)
	case msg.Code == tea.KeyBackspace:
		if m.draft != "" {
			m.draft = m.draft[:len(m.draft)-1]
		}
	case msg.Text != "":
		m.draft += msg.Text
	}
	return m, nil
}

func onOff(enabled bool) string {
	if enabled {
		return "on"
	}
	return "off"
}

func setRadio(mut backend.Mutator, enabled bool) tea.Cmd {
	return func() tea.Msg {
		return radioSetMsg{enabled: enabled, err: mut.SetWifiEnabled(enabled)}
	}
}

func saveHostname(mut backend.Mutator, name string) tea.Cmd {
	return func() tea.Msg {
		return hostnameSavedMsg{name: name, err: mut.SetHostname(name)}
	}
}

func (m Model) View() string {
	if m.err != nil {
		return style.NMNotRunningNotice + "\n"
	}
	var lines []string
	lines = append(lines, style.Title.Render("System"))
	for i, r := range m.rows() {
		lines = append(lines, m.renderRow(r, i == m.cursor))
	}
	lines = append(lines, m.nmStateLine())
	return style.Fit(lines, m.width, m.height)
}

// nmStateLine is informational, not a selectable row.
func (m Model) nmStateLine() string {
	return fmt.Sprintf("  %-24s %s", "NetworkManager:", m.nmState)
}

func (m Model) renderRow(r row, selected bool) string {
	if r.kind == rowHostname && m.editing {
		return "▸ " + fmt.Sprintf("%-24s %s▏", r.name+":", m.draft)
	}
	line := fmt.Sprintf("%-24s %s", r.name+":", r.detail)
	if r.kind == rowHostname && !m.hostnameAllowed() {
		line = style.Faint.Render(line + " 🔒")
	} else if selected {
		line = style.SelectedRow(line, m.width)
	}
	if selected {
		return "▸ " + line
	}
	return "  " + line
}
