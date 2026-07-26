// Package devices is the Devices tab: managed devices with state and
// active connection.
package devices

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/ilmars/netfu/internal/backend"
	"github.com/ilmars/netfu/internal/domain"
	"github.com/ilmars/netfu/internal/tui/keys"
	"github.com/ilmars/netfu/internal/tui/style"
)

type Model struct {
	reader  backend.Reader
	keys    keys.List
	devices []domain.Device
	cursor  int
	err     error
}

type devicesLoadedMsg struct {
	devices []domain.Device
	err     error
}

func New(r backend.Reader) Model {
	return Model{reader: r, keys: keys.DefaultList()}
}

func (m Model) Init() tea.Cmd {
	return m.loadDevices
}

func (m Model) loadDevices() tea.Msg {
	all, err := m.reader.Devices()
	var managed []domain.Device
	for _, d := range all {
		if d.Managed {
			managed = append(managed, d)
		}
	}
	return devicesLoadedMsg{devices: managed, err: err}
}

func (m Model) Selected() domain.Device {
	if len(m.devices) == 0 {
		return domain.Device{}
	}
	return m.devices[m.cursor]
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case devicesLoadedMsg:
		m.devices = msg.devices
		m.err = msg.err
		m.cursor = 0
		return m, nil
	case tea.KeyPressMsg:
		return m.handleKey(msg), nil
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyPressMsg) Model {
	switch {
	case key.Matches(msg, m.keys.Down):
		if m.cursor < len(m.devices)-1 {
			m.cursor++
		}
	case key.Matches(msg, m.keys.Up):
		if m.cursor > 0 {
			m.cursor--
		}
	case key.Matches(msg, m.keys.Top):
		m.cursor = 0
	case key.Matches(msg, m.keys.Bottom):
		if len(m.devices) > 0 {
			m.cursor = len(m.devices) - 1
		}
	}
	return m
}

func (m Model) View() string {
	var b strings.Builder
	b.WriteString(style.Title.Render("Devices") + "\n")
	for i, d := range m.devices {
		row := fmt.Sprintf("%s  %s  %s  %s", d.Name, d.Type, d.State, d.ActiveConnection)
		if i == m.cursor {
			b.WriteString("▸ " + style.Selected.Render(row) + "\n")
		} else {
			b.WriteString("  " + row + "\n")
		}
	}
	return b.String()
}
