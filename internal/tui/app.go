package tui

import (
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
	"github.com/ilmars/netfu/internal/tui/style"
)

type screen int

const (
	screenDevices screen = iota
)

type App struct {
	backend  backend.Backend
	keys     keys.Global
	screen   screen
	devices  devices.Model
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
		screen:  screenDevices,
		devices: devices.New(b),
		help:    help.New(),
		status:  statusbar.New(),
	}
}

// helpKeys joins the active screen's keymap with the global one for the
// help footer and overlay.
type helpKeys struct {
	screen keys.List
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
		a.devices.Init(),
		waitForActivity(a.backend.Events()),
	)
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.help.SetWidth(msg.Width)
		// Header takes the first line and the help footer the last; screens
		// get the rest. The status line only appears while it has a message.
		content := max(msg.Height-2, 0)
		var cmd tea.Cmd
		a.devices, cmd = a.devices.Update(tea.WindowSizeMsg{Width: msg.Width, Height: content})
		return a, cmd
	case tea.BackgroundColorMsg:
		a.theme = style.NewTheme(msg.IsDark())
		a.help.Styles = help.DefaultStyles(msg.IsDark())
		return a, nil
	case permissionsMsg:
		a.perms = msg.perms
		return a, nil
	case backendEventMsg:
		return a, tea.Batch(
			waitForActivity(a.backend.Events()),
			a.devices.Init(),
		)
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, a.keys.Help):
			a.showHelp = !a.showHelp
			a.help.ShowAll = a.showHelp
			return a, nil
		case key.Matches(msg, a.keys.Quit):
			return a, tea.Quit
		}
	}
	var cmd tea.Cmd
	a.devices, cmd = a.devices.Update(msg)
	return a, cmd
}

func (a App) View() tea.View {
	sections := []string{a.headerView(), a.devices.View()}
	if a.status.Message() != "" {
		sections = append(sections, a.status.View()+"\n")
	}
	sections = append(sections, a.help.View(helpKeys{screen: a.devices.Keys(), global: a.keys}))
	return tea.NewView(strings.Join(sections, ""))
}

func (a App) headerView() string {
	header := style.Title.Render("netfu")
	if a.width > 0 {
		header = lipgloss.NewStyle().MaxWidth(a.width).Render(header)
	}
	return header + "\n"
}
