package tui

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/ilmars/netfu/internal/backend"
	"github.com/ilmars/netfu/internal/tui/keys"
	"github.com/ilmars/netfu/internal/tui/screens/devices"
)

type screen int

const (
	screenDevices screen = iota
)

type App struct {
	backend backend.Backend
	keys    keys.Global
	screen  screen
	devices devices.Model
	width   int
	height  int
}

func New(b backend.Backend) tea.Model {
	return App{
		backend: b,
		keys:    keys.DefaultGlobal(),
		screen:  screenDevices,
		devices: devices.New(b),
	}
}

func (a App) Init() tea.Cmd {
	return a.devices.Init()
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		return a, nil
	case tea.KeyPressMsg:
		if key.Matches(msg, a.keys.Quit) {
			return a, tea.Quit
		}
	}
	var cmd tea.Cmd
	a.devices, cmd = a.devices.Update(msg)
	return a, cmd
}

func (a App) View() tea.View {
	return tea.NewView(a.devices.View())
}
