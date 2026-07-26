package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/ilmars/netfu/internal/backend"
	"github.com/ilmars/netfu/internal/domain"
	"github.com/ilmars/netfu/internal/tui/components/statusbar"
	"github.com/ilmars/netfu/internal/tui/keys"
	"github.com/ilmars/netfu/internal/tui/screens/connections"
	"github.com/ilmars/netfu/internal/tui/screens/devices"
	"github.com/ilmars/netfu/internal/tui/screens/system"
	"github.com/ilmars/netfu/internal/tui/screens/wifi"
	"github.com/ilmars/netfu/internal/tui/style"
)

type tab int

const (
	tabWifi tab = iota
	tabDevices
	tabConnections
	tabSystem
	tabCount
)

var tabLabels = [tabCount]string{"Wi-Fi", "Devices", "Connections", "System"}

type App struct {
	backend  backend.Backend
	keys     keys.Global
	tab      tab
	wifi     wifi.Model
	devices  devices.Model
	conns    connections.Model
	system   system.Model
	help     help.Model
	showHelp bool
	theme    style.Theme
	perms    domain.Permissions
	status   statusbar.Model
	// radioOn is read from the backend at startup and tracked through the
	// user's toggles.
	radioOn bool
	width   int
	height  int
}

func New(b backend.Backend) tea.Model {
	return App{
		backend: b,
		keys:    keys.DefaultGlobal(),
		tab:     tabWifi,
		wifi:    wifi.New(b),
		devices: devices.New(b),
		conns:   connections.New(b),
		system:  system.New(b),
		help:    help.New(),
		status:  statusbar.New(),
		radioOn: true,
	}
}

// helpKeys joins the active screen's keymap with the global one for the
// help footer and overlay.
type helpKeys struct {
	screen help.KeyMap
	global keys.Global
}

func (h helpKeys) ShortHelp() []key.Binding {
	return append(h.screen.ShortHelp(), h.global.ShortHelp()...)
}

func (h helpKeys) FullHelp() [][]key.Binding {
	return append(h.screen.FullHelp(), h.global.FullHelp()...)
}

func (a App) Init() tea.Cmd {
	return tea.Batch(
		tea.RequestBackgroundColor,
		loadPermissions(a.backend),
		loadRadioState(a.backend),
		a.wifi.Init(),
		waitForActivity(a.backend.Events()),
		rescanTick(),
	)
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.help.SetWidth(msg.Width)
		// Header and tab bar take the first two lines and the help footer the
		// last; screens get the rest. The status line only appears while it
		// has a message.
		// Screens get the box interior: borders plus one cell of padding
		// on each side.
		content := tea.WindowSizeMsg{Width: max(msg.Width-4, 0), Height: max(msg.Height-chromeLines, 0)}
		var wifiCmd, devicesCmd, connsCmd, systemCmd tea.Cmd
		a.wifi, wifiCmd = a.wifi.Update(content)
		a.devices, devicesCmd = a.devices.Update(content)
		a.conns, connsCmd = a.conns.Update(content)
		a.system, systemCmd = a.system.Update(content)
		return a, tea.Batch(wifiCmd, devicesCmd, connsCmd, systemCmd)
	case tea.BackgroundColorMsg:
		a.theme = style.NewTheme(msg.IsDark())
		style.ResolveSelected(msg.IsDark())
		a.help.Styles = help.DefaultStyles(msg.IsDark())
		var cmd tea.Cmd
		a.wifi, cmd = a.wifi.Update(msg)
		return a, cmd
	case devices.StatusMsg:
		a.status = a.status.SetMessage(string(msg))
		return a, nil
	case system.StatusMsg:
		a.status = a.status.SetMessage(string(msg))
		return a, nil
	case radioResultMsg:
		if msg.err == nil {
			return a, nil
		}
		a.radioOn = !a.radioOn // the toggle failed; NM kept the old state
		var cmd tea.Cmd
		a.wifi, cmd = a.wifi.Update(wifi.RadioMsg{Enabled: a.radioOn, Err: msg.err})
		a.status = a.status.SetMessage(a.wifi.Status())
		return a, cmd
	case radioStateMsg:
		if msg.err != nil || msg.enabled == a.radioOn {
			return a, nil
		}
		a.radioOn = msg.enabled
		var cmd tea.Cmd
		a.wifi, cmd = a.wifi.Update(wifi.RadioMsg{Enabled: msg.enabled})
		return a, cmd
	case permissionsMsg:
		a.perms = msg.perms
		a.conns = a.conns.SetPermissions(msg.perms)
		a.system = a.system.WithPermissions(msg.perms)
		return a, nil
	case rescanTickMsg:
		cmds := []tea.Cmd{rescanTick()}
		if a.tab == tabWifi && a.radioOn {
			var cmd tea.Cmd
			a.wifi, cmd = a.wifi.Update(wifi.RescanMsg{})
			cmds = append(cmds, cmd)
		}
		return a, tea.Batch(cmds...)
	case backendEventMsg:
		var cmd tea.Cmd
		a.wifi, cmd = a.wifi.Update(wifi.EventMsg(msg))
		a.status = a.status.SetMessage(a.wifi.Status())
		return a, tea.Batch(
			waitForActivity(a.backend.Events()),
			a.devices.Init(),
			a.system.Init(),
			cmd,
		)
	case tea.KeyPressMsg:
		return a.handleKey(msg)
	}
	return a.updateActiveScreen(msg)
}

func (a App) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// A screen capturing text input (the wifi filter) gets every key;
	// global and tab bindings would otherwise swallow the typed query.
	if a.tab == tabWifi && a.wifi.CapturesInput() {
		return a.updateActiveScreen(msg)
	}
	if a.tab == tabConnections && a.conns.CapturesInput() {
		return a.updateActiveScreen(msg)
	}
	if a.tab == tabSystem && a.system.CapturesInput() {
		return a.updateActiveScreen(msg)
	}
	switch {
	case key.Matches(msg, a.keys.Help):
		a.showHelp = !a.showHelp
		a.help.ShowAll = a.showHelp
		return a, nil
	case key.Matches(msg, a.keys.Quit):
		// q pops a pushed layer and only quits from top level.
		if a.tab == tabDevices && a.devices.Layered() {
			return a.updateActiveScreen(msg)
		}
		return a, tea.Quit
	case key.Matches(msg, a.keys.Tabs):
		return a.switchTab(tab(msg.Code - '1'))
	case key.Matches(msg, a.keys.NextTab):
		return a.switchTab((a.tab + 1) % tabCount)
	case key.Matches(msg, a.keys.PrevTab):
		return a.switchTab((a.tab + tabCount - 1) % tabCount)
	case key.Matches(msg, a.keys.WifiRadio):
		return a.toggleWifiRadio()
	}
	return a.updateActiveScreen(msg)
}

func (a App) toggleWifiRadio() (tea.Model, tea.Cmd) {
	a.radioOn = !a.radioOn
	radioOn := a.radioOn
	setRadio := func() tea.Msg {
		return radioResultMsg{err: a.backend.SetWifiEnabled(radioOn)}
	}
	var cmd tea.Cmd
	a.wifi, cmd = a.wifi.Update(wifi.RadioMsg{Enabled: a.radioOn})
	return a, tea.Batch(setRadio, cmd)
}

// switchTab activates a tab, re-running its entry cmd so the screen shows
// fresh data; the screen models themselves persist across switches.
func (a App) switchTab(t tab) (tea.Model, tea.Cmd) {
	a.tab = t
	switch t {
	case tabWifi:
		return a, a.wifi.Init()
	case tabDevices:
		return a, a.devices.Init()
	case tabConnections:
		return a, a.conns.Init()
	case tabSystem:
		return a, a.system.Init()
	}
	return a, nil
}

func (a App) updateActiveScreen(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch a.tab {
	case tabWifi:
		a.wifi, cmd = a.wifi.Update(msg)
		a.status = a.status.SetMessage(a.wifi.Status())
	case tabDevices:
		a.devices, cmd = a.devices.Update(msg)
	case tabConnections:
		a.conns, cmd = a.conns.Update(msg)
		a.status = a.status.SetMessage(a.conns.Status())
	case tabSystem:
		a.system, cmd = a.system.Update(msg)
	}
	return a, cmd
}

const (
	minWidth  = 60
	minHeight = 16
	// chromeLines is what the shell keeps for itself: header, tab bar,
	// reserved status line, help footer, and the content box borders.
	chromeLines = 6
)

// padToHeight fills the content pane with blank lines so the status line
// and footer sit at the bottom of the terminal.
func padToHeight(content string, height int) string {
	if height <= 0 {
		return content
	}
	lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
	for len(lines) < height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n") + "\n"
}

func (a App) View() tea.View {
	if a.width > 0 && (a.width < minWidth || a.height < minHeight) {
		return tea.NewView(fmt.Sprintf("Terminal too small (%dx%d, need %dx%d)",
			a.width, a.height, minWidth, minHeight))
	}
	content := a.activeScreenView()
	if a.height > 0 {
		content = padToHeight(content, a.height-chromeLines)
	}
	// The status line lives on the box's last interior row — always
	// reserved so async feedback never shifts the content above it.
	content = a.boxed(strings.TrimSuffix(content, "\n") + "\n" + a.status.View())
	sections := []string{a.headerView(), a.tabBarView(), content,
		a.help.View(helpKeys{screen: a.activeScreenKeys(), global: a.keys})}
	base := strings.Join(sections, "")
	if overlay := a.activeOverlay(); overlay != "" {
		base = layerCentered(base, overlay)
	}
	view := tea.NewView(base)
	view.AltScreen = true
	return view
}

// boxed frames the content pane; the borders account for two of the
// chromeLines rows and two columns of the pane width. Width includes the
// frame in lipgloss v2, so the box spans the full terminal width.
func (a App) boxed(content string) string {
	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
	if a.width > 0 {
		box = box.Width(a.width)
	}
	if a.theme.Dim != nil {
		box = box.BorderForeground(a.theme.Dim)
	}
	// The content arrives with its trailing newline already stripped by
	// the caller; an empty last line is the reserved status row.
	return box.Render(content) + "\n"
}

func (a App) activeOverlay() string {
	switch a.tab {
	case tabWifi:
		return a.wifi.Overlay()
	case tabDevices:
		return a.devices.Overlay()
	}
	return ""
}

// layerCentered composites the modal over the base view instead of pushing
// content down; the base keeps its footprint. The backdrop drops its own
// styling and renders faint so the modal reads as the only active layer.
func layerCentered(base, overlay string) string {
	width, height := lipgloss.Width(base), lipgloss.Height(base)
	backdrop := style.Faint.Render(ansi.Strip(base))
	return lipgloss.NewCompositor(
		lipgloss.NewLayer(backdrop),
		lipgloss.NewLayer(overlay).
			X(max((width-lipgloss.Width(overlay))/2, 0)).
			Y(max((height-lipgloss.Height(overlay))/2, 0)).
			Z(1),
	).Render()
}

func (a App) activeScreenView() string {
	switch a.tab {
	case tabWifi:
		return a.wifi.View()
	case tabDevices:
		return a.devices.View()
	case tabConnections:
		return a.conns.View()
	case tabSystem:
		return a.system.View()
	}
	return ""
}

func (a App) activeScreenKeys() help.KeyMap {
	switch a.tab {
	case tabWifi:
		return a.wifi.Keys()
	case tabDevices:
		return a.devices.Keys()
	case tabConnections:
		return a.conns.Keys()
	case tabSystem:
		return a.system.Keys()
	}
	return keys.List{}
}

func (a App) headerView() string {
	logoStyle := style.Logo
	if a.theme.Dim != nil {
		logoStyle = logoStyle.Foreground(a.theme.Dim)
	}
	logo := logoStyle.Render("netfu")
	if a.width > 0 {
		logo = lipgloss.NewStyle().Width(a.width).AlignHorizontal(lipgloss.Right).Render(logo)
	}
	return a.clipToWidth(logo) + "\n"
}

func (a App) tabBarView() string {
	parts := make([]string, tabCount)
	for i, label := range tabLabels {
		entry := fmt.Sprintf("[%d] %s", i+1, label)
		if tab(i) == a.tab {
			entry = style.Selected.Render(entry)
		}
		parts[i] = entry
	}
	bar := strings.Join(parts, " · ")
	if a.wifi.Scanning() {
		bar += "   " + style.Faint.Render("scan ⟳")
	}
	return a.clipToWidth(bar) + "\n"
}

func (a App) clipToWidth(line string) string {
	if a.width <= 0 {
		return line
	}
	return lipgloss.NewStyle().MaxWidth(a.width).Render(line)
}
