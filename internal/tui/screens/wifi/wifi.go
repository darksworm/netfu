// Package wifi is the Wi-Fi tab: the live, filterable scan list that is
// the app's home screen.
package wifi

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/ilmars/netfu/internal/backend"
	"github.com/ilmars/netfu/internal/domain"
	"github.com/ilmars/netfu/internal/tui/keys"
	"github.com/ilmars/netfu/internal/tui/style"
)

// EventMsg is a backend event the root model forwards to this screen.
type EventMsg domain.Event

// NeedsSecretMsg asks for the password modal (built in a later milestone);
// until then the root model ignores it.
type NeedsSecretMsg struct {
	AP domain.AccessPoint
}

// savedProfile ties a saved wifi connection's SSID to the profile that
// Enter activates.
type savedProfile struct {
	SSID         string
	ConnectionID string
}

type loadedMsg struct {
	aps        []domain.AccessPoint
	saved      []savedProfile
	activeSSID string
	wifiDevice string
	err        error
}

type scanRequestedMsg struct {
	err error
}

type connectResultMsg struct {
	err error
}

type Model struct {
	backend    backend.Backend
	keys       keys.Wifi
	aps        []domain.AccessPoint
	saved      []savedProfile
	activeSSID string
	wifiDevice string
	connecting string
	scanning   bool
	filtering  bool
	filter     string
	cursor     int
	err        error
	width      int
	height     int
	sized      bool
}

func New(b backend.Backend) Model {
	return Model{backend: b, keys: keys.DefaultWifi()}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.requestScan, m.load)
}

func (m Model) requestScan() tea.Msg {
	return scanRequestedMsg{err: m.backend.RequestScan()}
}

func (m Model) load() tea.Msg {
	aps, err := m.backend.AccessPoints()
	if err != nil {
		return loadedMsg{err: err}
	}
	saved, err := m.savedWifiProfiles()
	if err != nil {
		return loadedMsg{err: err}
	}
	wifiDevice, err := m.wifiDeviceName()
	if err != nil {
		return loadedMsg{err: err}
	}
	activeSSID, err := m.activeWifiSSID(wifiDevice, saved)
	if err != nil {
		return loadedMsg{err: err}
	}
	return loadedMsg{aps: aps, saved: saved, activeSSID: activeSSID, wifiDevice: wifiDevice}
}

func (m Model) savedWifiProfiles() ([]savedProfile, error) {
	connections, err := m.backend.Connections()
	if err != nil {
		return nil, err
	}
	var saved []savedProfile
	for _, c := range connections {
		if c.Type != "802-11-wireless" {
			continue
		}
		saved = append(saved, savedProfile{SSID: m.ssidOf(c), ConnectionID: c.ID})
	}
	return saved, nil
}

// ssidOf prefers the profile's stored SSID; a profile renamed away from its
// SSID still matches the scanned network.
func (m Model) ssidOf(c domain.Connection) string {
	settings, err := m.backend.GetSettings(c.ID)
	if err == nil {
		if ssid, ok := settings["802-11-wireless"]["ssid"].(string); ok && ssid != "" {
			return ssid
		}
	}
	return c.Name
}

func (m Model) wifiDeviceName() (string, error) {
	devices, err := m.backend.Devices()
	if err != nil {
		return "", err
	}
	for _, d := range devices {
		if d.Type == domain.DeviceTypeWifi {
			return d.Name, nil
		}
	}
	return "", nil
}

func (m Model) activeWifiSSID(wifiDevice string, saved []savedProfile) (string, error) {
	active, err := m.backend.ActiveConnections()
	if err != nil {
		return "", err
	}
	for _, ac := range active {
		if ac.DeviceName != wifiDevice || wifiDevice == "" {
			continue
		}
		for _, s := range saved {
			if s.ConnectionID == ac.ID {
				return s.SSID, nil
			}
		}
		return ac.Name, nil
	}
	return "", nil
}

func (m Model) Keys() keys.Wifi {
	return m.keys
}

// Selected returns the AP under the cursor.
func (m Model) Selected() domain.AccessPoint {
	rows := m.list().InRange
	if len(rows) == 0 || m.cursor >= len(rows) {
		return domain.AccessPoint{}
	}
	return rows[m.cursor]
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.sized = true
		return m, nil
	case scanRequestedMsg:
		m.scanning = msg.err == nil
		return m, nil
	case loadedMsg:
		selected := m.Selected()
		m.aps = msg.aps
		m.saved = msg.saved
		m.activeSSID = msg.activeSSID
		m.wifiDevice = msg.wifiDevice
		m.err = msg.err
		if m.connecting != "" && m.connecting == msg.activeSSID {
			m.connecting = ""
		}
		m.keepCursorOn(selected)
		return m, nil
	case connectResultMsg:
		if msg.err != nil {
			m.connecting = ""
		}
		return m, nil
	case EventMsg:
		if msg.Kind == domain.EventAPListChanged {
			m.scanning = false
		}
		return m, m.load
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

// keepCursorOn re-finds the previously selected network after a reload, so
// live updates never move the user's cursor. Hidden rows share an empty
// SSID, so they are matched by BSSID.
func (m *Model) keepCursorOn(selected domain.AccessPoint) {
	rows := m.list().InRange
	for i, ap := range rows {
		if ap.SSID == selected.SSID && (ap.SSID != "" || ap.BSSID == selected.BSSID) {
			m.cursor = i
			return
		}
	}
	if m.cursor >= len(rows) {
		m.cursor = max(len(rows)-1, 0)
	}
}

// CapturesInput tells the root model to route every key here (filter typing).
func (m Model) CapturesInput() bool {
	return m.filtering
}

func (m Model) handleKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if m.filtering {
		return m.handleFilterKey(msg), nil
	}
	rows := m.list().InRange
	switch {
	case key.Matches(msg, m.keys.Filter):
		m.filtering = true
		m.filter = ""
		m.cursor = 0
		return m, nil
	case key.Matches(msg, m.keys.ClearFilter):
		m.filter = ""
		m.cursor = 0
		return m, nil
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
	case key.Matches(msg, m.keys.Connect):
		return m.connectSelected()
	}
	return m, nil
}

func (m Model) handleFilterKey(msg tea.KeyPressMsg) Model {
	switch {
	case key.Matches(msg, m.keys.ClearFilter):
		m.filtering = false
		m.filter = ""
	case msg.Code == tea.KeyEnter:
		m.filtering = false // keep the query, stop capturing keys
	case msg.Code == tea.KeyBackspace:
		if len(m.filter) > 0 {
			m.filter = m.filter[:len(m.filter)-1]
		}
	case msg.Text != "":
		m.filter += msg.Text
	}
	m.cursor = 0
	return m
}

// connectSelected is the contextual, never-destructive Enter.
func (m Model) connectSelected() (Model, tea.Cmd) {
	ap := m.Selected()
	if ap.SSID != "" && ap.SSID == m.activeSSID {
		return m, nil // already connected; detail view is a later milestone
	}
	if profile, ok := m.savedProfileFor(ap.SSID); ok {
		m.connecting = ap.SSID
		return m, func() tea.Msg {
			return connectResultMsg{err: m.backend.Activate(profile.ConnectionID, m.wifiDevice)}
		}
	}
	if ap.Security == domain.SecurityOpen {
		m.connecting = ap.SSID
		return m, func() tea.Msg {
			return connectResultMsg{err: m.backend.JoinWifi(domain.JoinRequest{
				SSID:     ap.SSID,
				Security: domain.SecurityOpen,
			})}
		}
	}
	if ap.SSID != "" {
		return m, func() tea.Msg { return NeedsSecretMsg{AP: ap} }
	}
	return m, nil
}

// Status is the screen's line for the app's status bar.
func (m Model) Status() string {
	if m.connecting != "" {
		return fmt.Sprintf("Connecting to %s…", m.connecting)
	}
	return ""
}

func (m Model) savedProfileFor(ssid string) (savedProfile, bool) {
	if ssid == "" {
		return savedProfile{}, false
	}
	for _, s := range m.saved {
		if s.SSID == ssid {
			return s, true
		}
	}
	return savedProfile{}, false
}

func (m Model) renderRow(ap domain.AccessPoint) string {
	gutter := " "
	switch {
	case ap.SSID != "" && ap.SSID == m.connecting:
		gutter = "◌"
	case ap.SSID != "" && ap.SSID == m.activeSSID:
		gutter = "▸"
	}
	ssid := ap.SSID
	if ssid == "" {
		ssid = "(hidden)"
	}
	return strings.TrimRight(fmt.Sprintf("%s %-24s %s %3d%%  %-7s %s",
		gutter, ssid, signalBars(ap.Strength), ap.Strength, securityBadge(ap.Security), m.tagFor(ap)), " ")
}

func (m Model) tagFor(ap domain.AccessPoint) string {
	switch {
	case ap.SSID != "" && ap.SSID == m.connecting:
		return "◐ connecting"
	case ap.SSID != "" && ap.SSID == m.activeSSID:
		return "✓ connected"
	case ap.SSID == "":
		return "hidden"
	default:
		if _, ok := m.savedProfileFor(ap.SSID); ok {
			return "⋆ saved"
		}
		return ""
	}
}

// signalBars maps strength quartiles to a fixed-width ▂▄▆█ ramp.
func signalBars(strength uint8) string {
	switch {
	case strength >= 75:
		return "▂▄▆█"
	case strength >= 50:
		return "▂▄▆ "
	case strength >= 25:
		return "▂▄  "
	default:
		return "▂   "
	}
}

func securityBadge(s domain.Security) string {
	switch s {
	case domain.SecurityOpen:
		return "open"
	case domain.SecurityWEP:
		return "WEP!"
	case domain.SecurityWPA:
		return "WPA"
	case domain.SecurityWPA2:
		return "WPA2"
	case domain.SecurityWPA3:
		return "WPA3"
	case domain.SecurityWPA2WPA3:
		return "WPA2/3"
	case domain.SecurityEnterprise:
		return "802.1X"
	}
	return string(s)
}

func (m Model) list() domain.WifiList {
	savedSSIDs := make([]string, len(m.saved))
	for i, s := range m.saved {
		savedSSIDs[i] = s.SSID
	}
	list := domain.BuildWifiList(domain.DedupeAPs(m.aps), savedSSIDs, m.activeSSID, m.connecting)
	if m.filter == "" {
		return list
	}
	query := strings.ToLower(m.filter)
	var inRange []domain.AccessPoint
	for _, ap := range list.InRange {
		if strings.Contains(strings.ToLower(ap.SSID), query) {
			inRange = append(inRange, ap)
		}
	}
	var outOfRange []string
	for _, ssid := range list.OutOfRange {
		if strings.Contains(strings.ToLower(ssid), query) {
			outOfRange = append(outOfRange, ssid)
		}
	}
	return domain.WifiList{InRange: inRange, OutOfRange: outOfRange}
}

func (m Model) View() string {
	var lines []string
	if m.filtering || m.filter != "" {
		lines = append(lines, "/"+m.filter)
	}
	if m.scanning {
		lines = append(lines, style.Title.Render("scan ⟳"))
	}
	list := m.list()
	for i, ap := range list.InRange {
		row := m.renderRow(ap)
		if i == m.cursor {
			row = style.Selected.Render(row)
		}
		lines = append(lines, row)
	}
	if len(list.OutOfRange) > 0 {
		lines = append(lines, "─ out of range ─")
		for _, ssid := range list.OutOfRange {
			lines = append(lines, fmt.Sprintf("  %-24s ⋆ saved", ssid))
		}
	}
	if m.sized && len(lines) > m.height {
		lines = lines[:m.height]
	}
	if len(lines) == 0 {
		return ""
	}
	if m.width > 0 {
		clip := lipgloss.NewStyle().MaxWidth(m.width)
		for i, line := range lines {
			lines[i] = clip.Render(line)
		}
	}
	return strings.Join(lines, "\n") + "\n"
}
