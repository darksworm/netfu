package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/ilmars/netfu/internal/backend"
	"github.com/ilmars/netfu/internal/domain"
	"github.com/ilmars/netfu/internal/tui/components/statusbar"
	"github.com/ilmars/netfu/internal/tui/keys"
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
	system   system.Model
	help     help.Model
	showHelp bool
	theme    style.Theme
	perms    domain.Permissions
	status   statusbar.Model
	width    int
	height   int
}

func New(b backend.Backend) tea.Model {
	return App{
		backend: b,
		keys:    keys.DefaultGlobal(),
		tab:     tabWifi,
		wifi:    wifi.New(b),
		devices: devices.New(b),
		system:  system.New(b),
		help:    help.New(),
		status:  statusbar.New(),
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
		a.wifi.Init(),
		waitForActivity(a.backend.Events()),
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
		content := tea.WindowSizeMsg{Width: msg.Width, Height: max(msg.Height-3, 0)}
		var wifiCmd, devicesCmd, systemCmd tea.Cmd
		a.wifi, wifiCmd = a.wifi.Update(content)
		a.devices, devicesCmd = a.devices.Update(content)
		a.system, systemCmd = a.system.Update(content)
		return a, tea.Batch(wifiCmd, devicesCmd, systemCmd)
	case tea.BackgroundColorMsg:
		a.theme = style.NewTheme(msg.IsDark())
		a.help.Styles = help.DefaultStyles(msg.IsDark())
		return a, nil
	case devices.StatusMsg:
		a.status = a.status.SetMessage(string(msg))
		return a, nil
	case system.StatusMsg:
		a.status = a.status.SetMessage(string(msg))
		return a, nil
	case permissionsMsg:
		a.perms = msg.perms
		a.system = a.system.WithPermissions(msg.perms)
		return a, nil
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
	if a.tab == tabSystem && a.system.CapturesInput() {
		return a.updateActiveScreen(msg)
	}
	switch {
	case key.Matches(msg, a.keys.Help):
		a.showHelp = !a.showHelp
		a.help.ShowAll = a.showHelp
		return a, nil
	case key.Matches(msg, a.keys.Quit):
		return a, tea.Quit
	case key.Matches(msg, a.keys.Tabs):
		return a.switchTab(tab(msg.Code - '1'))
	case key.Matches(msg, a.keys.NextTab):
		return a.switchTab((a.tab + 1) % tabCount)
	case key.Matches(msg, a.keys.PrevTab):
		return a.switchTab((a.tab + tabCount - 1) % tabCount)
	}
	return a.updateActiveScreen(msg)
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
	case tabSystem:
		a.system, cmd = a.system.Update(msg)
	}
	return a, cmd
}

func (a App) View() tea.View {
	sections := []string{a.headerView(), a.tabBarView(), a.activeScreenView()}
	if a.status.Message() != "" {
		sections = append(sections, a.status.View()+"\n")
	}
	sections = append(sections, a.help.View(helpKeys{screen: a.activeScreenKeys(), global: a.keys}))
	return tea.NewView(strings.Join(sections, ""))
}

func (a App) activeScreenView() string {
	switch a.tab {
	case tabWifi:
		return a.wifi.View()
	case tabDevices:
		return a.devices.View()
	case tabSystem:
		return a.system.View()
	}
	return placeholderView(tabLabels[a.tab])
}

func placeholderView(label string) string {
	return fmt.Sprintf("%s — coming soon\n", label)
}

func (a App) activeScreenKeys() help.KeyMap {
	switch a.tab {
	case tabWifi:
		return a.wifi.Keys()
	case tabDevices:
		return a.devices.Keys()
	case tabSystem:
		return a.system.Keys()
	}
	return keys.List{}
}

func (a App) headerView() string {
	header := style.Title.Render("netfu")
	return a.clipToWidth(header) + "\n"
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
	return a.clipToWidth(strings.Join(parts, " · ")) + "\n"
}

func (a App) clipToWidth(line string) string {
	if a.width <= 0 {
		return line
	}
	return lipgloss.NewStyle().MaxWidth(a.width).Render(line)
}
