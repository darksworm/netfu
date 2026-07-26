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
	"github.com/ilmars/netfu/internal/tui/components/confirm"
	"github.com/ilmars/netfu/internal/tui/components/passwordprompt"
	"github.com/ilmars/netfu/internal/tui/keys"
	"github.com/ilmars/netfu/internal/tui/style"
)

// EventMsg is a backend event the root model forwards to this screen.
type EventMsg domain.Event

// NeedsSecretMsg is the seam between the contextual Enter and the password
// modal: the screen emits it for itself and opens the prompt on receipt.
type NeedsSecretMsg struct {
	AP domain.AccessPoint
}

// RadioMsg tells the screen the wifi radio state; Err is set when a toggle
// failed and Enabled is the state the radio actually kept.
type RadioMsg struct {
	Enabled bool
	Err     error
}

// RescanMsg asks for a background rescan (the app's periodic timer).
type RescanMsg struct{}

// savedProfile ties a saved wifi connection's SSID to the profile that
// Enter activates.
type savedProfile struct {
	SSID         string
	ConnectionID string
}

type loadedMsg struct {
	aps          []domain.AccessPoint
	saved        []savedProfile
	activeSSID   string
	activeConnID string
	wifiDevice   string
	err          error
}

type scanRequestedMsg struct {
	err error
}

type connectResultMsg struct {
	err error
}

type Model struct {
	backend      backend.Backend
	keys         keys.Wifi
	aps          []domain.AccessPoint
	saved        []savedProfile
	activeSSID   string
	activeConnID string
	wifiDevice   string
	connecting   string
	prompt       *passwordprompt.Model
	confirm      *confirm.Model
	// pendingJoin is the request the open prompt completes; lastJoin is the
	// most recent one issued, kept so an auth failure can be tied back to it.
	pendingJoin domain.JoinRequest
	lastJoin    domain.JoinRequest
	failed      *domain.JoinRequest
	// ssidEntry marks the open prompt as the hidden-network SSID step.
	ssidEntry bool
	notice    string
	radioOff  bool
	theme     style.Theme
	scanning  bool
	filtering bool
	filter    string
	cursor    int
	err       error
	width     int
	height    int
}

func New(b backend.Backend) Model {
	return Model{backend: b, keys: keys.DefaultWifi(), theme: style.NewTheme(true)}
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
	activeSSID, activeConnID, err := m.activeWifi(wifiDevice, saved)
	if err != nil {
		return loadedMsg{err: err}
	}
	return loadedMsg{aps: aps, saved: saved, activeSSID: activeSSID, activeConnID: activeConnID, wifiDevice: wifiDevice}
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

func (m Model) activeWifi(wifiDevice string, saved []savedProfile) (ssid, connID string, err error) {
	active, err := m.backend.ActiveConnections()
	if err != nil {
		return "", "", err
	}
	for _, ac := range active {
		if ac.DeviceName != wifiDevice || wifiDevice == "" {
			continue
		}
		for _, s := range saved {
			if s.ConnectionID == ac.ID {
				return s.SSID, ac.ID, nil
			}
		}
		return ac.Name, ac.ID, nil
	}
	return "", "", nil
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
		return m, nil
	case scanRequestedMsg:
		m.scanning = msg.err == nil
		return m, nil
	case loadedMsg:
		selected := m.Selected()
		m.aps = msg.aps
		m.saved = msg.saved
		m.activeSSID = msg.activeSSID
		m.activeConnID = msg.activeConnID
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
	case tea.BackgroundColorMsg:
		m.theme = style.NewTheme(msg.IsDark())
		return m, nil
	case RadioMsg:
		m.radioOff = !msg.Enabled
		if msg.Err != nil {
			m.notice = fmt.Sprintf("✗ wifi radio: %v", msg.Err)
			return m, nil
		}
		if msg.Enabled {
			return m, m.Init()
		}
		return m, nil
	case RescanMsg:
		return m, m.requestScan
	case NeedsSecretMsg:
		return m.openPrompt(domain.JoinRequest{SSID: msg.AP.SSID, Security: msg.AP.Security}), nil
	case EventMsg:
		if msg.Kind == domain.EventAPListChanged {
			m.scanning = false
		}
		if m.isWrongPassword(domain.Event(msg)) {
			failed := m.lastJoin
			m.failed = &failed
			m.connecting = ""
			return m, m.deleteHalfCreatedProfile(failed.SSID)
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

// CapturesInput tells the root model to route every key here (filter or
// modal typing).
func (m Model) CapturesInput() bool {
	return m.filtering || m.prompt != nil || m.confirm != nil
}

// Overlay is the open modal's view, layered by the root model over the
// dimmed list; empty when no modal is open.
func (m Model) Overlay() string {
	if m.prompt != nil {
		return m.prompt.View()
	}
	if m.confirm != nil {
		return m.confirm.View()
	}
	return ""
}

func (m Model) handleKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if m.prompt != nil {
		return m.handlePromptKey(msg)
	}
	if m.confirm != nil {
		done, cmd := m.confirm.Update(msg)
		if done {
			m.confirm = nil
		}
		return m, cmd
	}
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
		if m.failed != nil {
			return m.openPrompt(*m.failed), nil
		}
		return m.connectSelected()
	case key.Matches(msg, m.keys.JoinHidden):
		// Started away from a scanned hidden row the security is unknown;
		// assume PSK so the flow still collects a password.
		return m.openSSIDEntry(domain.JoinRequest{Hidden: true, Security: domain.SecurityWPA2}), nil
	case key.Matches(msg, m.keys.Deactivate):
		return m.offerDeactivate()
	}
	return m, nil
}

func (m Model) openPrompt(pending domain.JoinRequest) Model {
	prompt := passwordprompt.New(pending.SSID, securityBadge(pending.Security))
	prompt.SetValue(pending.PSK)
	m.prompt = &prompt
	m.pendingJoin = pending
	m.failed = nil
	m.notice = ""
	return m
}

func (m Model) openSSIDEntry(pending domain.JoinRequest) Model {
	prompt := passwordprompt.NewSSIDEntry()
	m.prompt = &prompt
	m.pendingJoin = pending
	m.ssidEntry = true
	m.failed = nil
	m.notice = ""
	return m
}

func (m Model) handlePromptKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	prompt, done, submitted := m.prompt.Update(msg)
	m.prompt = &prompt
	if !done {
		return m, nil
	}
	m.prompt = nil
	if !submitted {
		m.ssidEntry = false
		return m, nil
	}
	if m.ssidEntry {
		m.ssidEntry = false
		m.pendingJoin.SSID = prompt.Value()
		if m.pendingJoin.Security == domain.SecurityOpen {
			return m.join(m.pendingJoin)
		}
		return m.openPrompt(m.pendingJoin), nil
	}
	req := m.pendingJoin
	req.PSK = prompt.Value()
	return m.join(req)
}

// isWrongPassword is the auth-failure heuristic: while a join we initiated
// is activating, NM tearing the device down for missing/rejected secrets
// means the PSK was wrong.
func (m Model) isWrongPassword(e domain.Event) bool {
	if e.Kind != domain.EventDeviceChanged {
		return false
	}
	if m.connecting == "" || m.connecting != m.lastJoin.SSID {
		return false
	}
	return e.Reason == domain.ReasonNoSecrets || e.Reason == domain.ReasonSupplicantDisconnect
}

// deleteHalfCreatedProfile removes the profile the failed join created, so
// the network does not masquerade as saved with a wrong PSK stored.
func (m Model) deleteHalfCreatedProfile(ssid string) tea.Cmd {
	return func() tea.Msg {
		profiles, err := m.savedWifiProfiles()
		if err != nil {
			return loadedMsg{err: err}
		}
		for _, p := range profiles {
			if p.SSID == ssid {
				if err := m.backend.DeleteConnection(p.ConnectionID); err != nil {
					return loadedMsg{err: err}
				}
			}
		}
		return m.load()
	}
}

func (m Model) join(req domain.JoinRequest) (Model, tea.Cmd) {
	m.connecting = req.SSID
	m.lastJoin = req
	m.notice = ""
	return m, func() tea.Msg {
		return connectResultMsg{err: m.backend.JoinWifi(req)}
	}
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

// offerDeactivate opens the confirm modal for the active network; d is
// never destructive without it.
func (m Model) offerDeactivate() (Model, tea.Cmd) {
	ap := m.Selected()
	if ap.SSID == "" || ap.SSID != m.activeSSID || m.activeConnID == "" {
		return m, nil
	}
	backend, activeConnID := m.backend, m.activeConnID
	modal := confirm.New(
		fmt.Sprintf("Deactivate %s?", ap.SSID),
		func() tea.Msg {
			return connectResultMsg{err: backend.Deactivate(activeConnID)}
		},
	)
	m.confirm = &modal
	return m, nil
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
		return m.join(domain.JoinRequest{SSID: ap.SSID, Security: domain.SecurityOpen})
	}
	if ap.Security == domain.SecurityEnterprise {
		m.notice = fmt.Sprintf("🔒 802.1X networks are not supported yet — configure %s in nmtui/nmcli for now", ap.SSID)
		return m, nil
	}
	if ap.SSID != "" {
		return m, func() tea.Msg { return NeedsSecretMsg{AP: ap} }
	}
	if ap.BSSID != "" {
		return m.openSSIDEntry(domain.JoinRequest{Hidden: true, Security: ap.Security}), nil
	}
	return m, nil
}

// Scanning reports whether a requested scan is still pending, for the
// app's tab-bar indicator.
func (m Model) Scanning() bool {
	return m.scanning
}

// Status is the screen's line for the app's status bar.
func (m Model) Status() string {
	if m.failed != nil {
		return fmt.Sprintf("✗ Wrong password for %s — ↵ to retry", m.failed.SSID)
	}
	if m.connecting != "" {
		return fmt.Sprintf("Connecting to %s…", m.connecting)
	}
	return m.notice
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
	if m.err != nil {
		return style.NMNotRunningNotice + "\n"
	}
	if m.radioOff {
		return lipgloss.NewStyle().Foreground(m.theme.Attention).
			Render("Wi-Fi is off — press W to enable") + "\n"
	}
	var lines []string
	if m.filtering || m.filter != "" {
		lines = append(lines, "/"+m.filter)
	}
	list := m.list()
	for i, ap := range list.InRange {
		row := m.renderRow(ap)
		if i == m.cursor {
			row = style.SelectedRow(row, m.width)
		}
		lines = append(lines, row)
	}
	if len(list.OutOfRange) > 0 {
		section := []string{style.Faint.Render("─ out of range ─")}
		for _, ssid := range list.OutOfRange {
			section = append(section, style.Faint.Render(fmt.Sprintf("  %-24s ⋆ saved", ssid)))
		}
		// Bottom-align the section in the pane; at least one blank line
		// keeps it separated when the pane is crowded.
		for len(lines)+len(section)+1 < m.height {
			lines = append(lines, "")
		}
		lines = append(lines, "")
		lines = append(lines, section...)
	}
	if len(lines) == 0 {
		return ""
	}
	return style.Fit(lines, m.width, m.height)
}
