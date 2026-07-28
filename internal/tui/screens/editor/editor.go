// Package editor is the full-screen connection editor: it loads a
// profile's settings into the typed editorform, and writes back only the
// touched fields, passing every other settings key through verbatim.
package editor

import (
	"fmt"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/ilmars/netfu/internal/backend"
	"github.com/ilmars/netfu/internal/domain"
	"github.com/ilmars/netfu/internal/tui/components/editorform"
	"github.com/ilmars/netfu/internal/tui/style"
)

// ClosedMsg tells the owning screen to pop the editor without saving.
type ClosedMsg struct{}

// SavedMsg tells the owning screen the profile was written; it pops the
// editor and reloads its list.
type SavedMsg struct {
	Status string
}

type settingsLoadedMsg struct {
	settings domain.ConnectionSettings
	err      error
}

type Model struct {
	backend  backend.Backend
	conn     domain.Connection
	settings domain.ConnectionSettings
	form     editorform.Model
	isNew    bool
	// confirmDiscard is the Esc-on-dirty prompt: save, discard, or stay.
	confirmDiscard bool
	status         string
}

func New(b backend.Backend, conn domain.Connection) Model {
	return Model{backend: b, conn: conn}
}

// NewProfile is the blank-form wizard for a brand-new connection of the
// given NM type; saving calls AddConnection instead of UpdateSettings.
func NewProfile(b backend.Backend, nmType string) Model {
	m := Model{
		backend: b,
		isNew:   true,
		settings: domain.ConnectionSettings{
			"connection": {"type": nmType},
			"ipv4":       {"method": "auto"},
			"ipv6":       {"method": "auto"},
		},
	}
	m.form = editorform.New(m.sections())
	return m
}

// NewWiredProfile is the wizard for a wired profile pinned to one NIC; the
// pin rides the settings pass-through, the form never shows it.
func NewWiredProfile(b backend.Backend, device string) Model {
	m := NewProfile(b, "802-3-ethernet")
	m.settings["connection"]["interface-name"] = device
	return m
}

func (m Model) Init() tea.Cmd {
	if m.isNew {
		return nil
	}
	return func() tea.Msg {
		settings, err := m.backend.GetSettings(m.conn.ID)
		return settingsLoadedMsg{settings: settings, err: err}
	}
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case settingsLoadedMsg:
		m.settings = msg.settings
		if msg.err != nil {
			m.status = fmt.Sprintf("✗ load settings: %v", msg.err)
			return m, nil
		}
		m.form = editorform.New(m.sections())
		return m, nil
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if m.form.Editing() {
		var cmd tea.Cmd
		m.form, cmd = m.form.Update(msg)
		return m, cmd
	}
	if m.confirmDiscard {
		return m.handleDiscardKey(msg)
	}
	switch msg.String() {
	case "s":
		return m.save()
	case "esc", "q":
		if m.form.Dirty() {
			m.confirmDiscard = true
			return m, nil
		}
		return m, func() tea.Msg { return ClosedMsg{} }
	}
	var cmd tea.Cmd
	m.form, cmd = m.form.Update(msg)
	return m, cmd
}

func (m Model) handleDiscardKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	m.confirmDiscard = false
	switch msg.String() {
	case "s":
		return m.save()
	case "d":
		return m, func() tea.Msg { return ClosedMsg{} }
	}
	return m, nil // any other key stays in the editor
}

func (m Model) save() (Model, tea.Cmd) {
	if !m.form.ValidateAll() {
		m.status = "✗ fix the highlighted fields before saving"
		return m, nil
	}
	updated := m.updatedSettings()
	if m.isNew {
		return m.addNew(updated)
	}
	conn := m.conn
	mut := m.backend
	return m, func() tea.Msg {
		if err := mut.UpdateSettings(conn.ID, updated); err != nil {
			return SavedMsg{Status: fmt.Sprintf("✗ save %s: %v", conn.Name, err)}
		}
		return SavedMsg{Status: fmt.Sprintf("✓ saved %s", conn.Name)}
	}
}

func (m Model) addNew(settings domain.ConnectionSettings) (Model, tea.Cmd) {
	name, _ := settings["connection"]["id"].(string)
	if name == "" {
		m.status = "✗ the new profile needs a name"
		return m, nil
	}
	mut := m.backend
	return m, func() tea.Msg {
		if err := mut.AddConnection(settings); err != nil {
			return SavedMsg{Status: fmt.Sprintf("✗ add %s: %v", name, err)}
		}
		return SavedMsg{Status: fmt.Sprintf("✓ added %s", name)}
	}
}

func (m Model) Status() string {
	return m.status
}

// sections builds the typed form from the loaded settings; only the fields
// users actually edit appear, everything else stays untouched in
// m.settings and is passed through on save.
func (m Model) sections() []editorform.Section {
	sections := []editorform.Section{{
		Title: "General",
		Fields: []editorform.Field{
			{Key: "connection.id", Label: "Name", Kind: editorform.Text,
				Value: str(m.settings, "connection", "id")},
			{Key: "connection.autoconnect", Label: "Autoconnect", Kind: editorform.Toggle,
				On: boolOr(m.settings, "connection", "autoconnect", true)},
			{Key: "connection.autoconnect-priority", Label: "Priority", Kind: editorform.Text,
				Value: intStr(m.settings, "connection", "autoconnect-priority"), Validate: validatePriority},
		},
	}}
	if str(m.settings, "connection", "type") == "802-11-wireless" {
		sections = append(sections, editorform.Section{
			Title: "Wi-Fi",
			Fields: []editorform.Field{
				{Key: "802-11-wireless.ssid", Label: "SSID", Kind: editorform.Text,
					Value: str(m.settings, "802-11-wireless", "ssid")},
				{Key: "802-11-wireless-security.key-mgmt", Label: "Security", Kind: editorform.Radio,
					Options: []string{"none", "wpa-psk", "sae"},
					Value:   stringOr(str(m.settings, "802-11-wireless-security", "key-mgmt"), "none")},
			},
		})
	}
	sections = append(sections,
		editorform.Section{
			Title: "IPv4",
			Fields: []editorform.Field{
				{Key: "ipv4.method", Label: "Method", Kind: editorform.Radio,
					Options: []string{"dhcp", "static", "disabled"},
					Value:   ipv4MethodLabel(str(m.settings, "ipv4", "method"))},
				{Key: "ipv4.address", Label: "Address", Kind: editorform.Text,
					Value: firstAddress(m.settings), Validate: validateCIDR},
				{Key: "ipv4.gateway", Label: "Gateway", Kind: editorform.Text,
					Value: str(m.settings, "ipv4", "gateway")},
				{Key: "ipv4.dns", Label: "DNS", Kind: editorform.Text,
					Value: strings.Join(stringList(m.settings, "ipv4", "dns"), ",")},
			},
		},
		editorform.Section{
			Title: "IPv6",
			Fields: []editorform.Field{
				{Key: "ipv6.method", Label: "Method", Kind: editorform.Radio,
					Options: []string{"auto", "dhcp", "static", "disabled"},
					Value:   ipv6MethodLabel(str(m.settings, "ipv6", "method"))},
			},
		},
	)
	return sections
}

// updatedSettings deep-copies the loaded settings and applies only the
// touched fields on top — the pass-through guarantee.
func (m Model) updatedSettings() domain.ConnectionSettings {
	updated := domain.ConnectionSettings{}
	for section, values := range m.settings {
		updated[section] = map[string]any{}
		for key, value := range values {
			updated[section][key] = value
		}
	}
	set := func(section, key string, value any) {
		if updated[section] == nil {
			updated[section] = map[string]any{}
		}
		updated[section][key] = value
	}
	if m.form.Touched("connection.id") {
		f, _ := m.form.Get("connection.id")
		set("connection", "id", f.Value)
	}
	if m.form.Touched("connection.autoconnect") {
		f, _ := m.form.Get("connection.autoconnect")
		set("connection", "autoconnect", f.On)
	}
	if m.form.Touched("connection.autoconnect-priority") {
		f, _ := m.form.Get("connection.autoconnect-priority")
		priority, _ := strconv.Atoi(f.Value)
		set("connection", "autoconnect-priority", priority)
	}
	if m.form.Touched("802-11-wireless.ssid") {
		f, _ := m.form.Get("802-11-wireless.ssid")
		set("802-11-wireless", "ssid", f.Value)
	}
	if m.form.Touched("802-11-wireless-security.key-mgmt") {
		f, _ := m.form.Get("802-11-wireless-security.key-mgmt")
		set("802-11-wireless-security", "key-mgmt", f.Value)
	}
	if m.form.Touched("ipv4.method") {
		f, _ := m.form.Get("ipv4.method")
		set("ipv4", "method", nmIPv4Method(f.Value))
	}
	if m.form.Touched("ipv4.address") {
		f, _ := m.form.Get("ipv4.address")
		set("ipv4", "address-data", addressData(f.Value))
	}
	if m.form.Touched("ipv4.gateway") {
		f, _ := m.form.Get("ipv4.gateway")
		set("ipv4", "gateway", f.Value)
	}
	if m.form.Touched("ipv4.dns") {
		f, _ := m.form.Get("ipv4.dns")
		set("ipv4", "dns", splitList(f.Value))
	}
	if m.form.Touched("ipv6.method") {
		f, _ := m.form.Get("ipv6.method")
		set("ipv6", "method", nmIPv6Method(f.Value))
	}
	return updated
}

func (m Model) View() string {
	title := "Edit " + m.conn.Name
	if m.isNew {
		title = "New connection"
	}
	lines := []string{style.Title.Render(title), ""}
	lines = append(lines, strings.Split(m.form.View(), "\n")...)
	dirty := " "
	if m.form.Dirty() {
		dirty = "●"
	}
	if m.confirmDiscard {
		lines = append(lines, "", "Unsaved changes — s save · d discard · any other key stays")
	}
	lines = append(lines, "", dirty+" s save · esc back")
	return strings.Join(lines, "\n") + "\n"
}
