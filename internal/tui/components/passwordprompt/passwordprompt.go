// Package passwordprompt is the modal that collects a secret (or an SSID)
// for a wifi join. Screens own their prompt instance and route key presses
// to it while it is open.
package passwordprompt

import (
	"fmt"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type Model struct {
	input    textinput.Model
	title    string
	security string
	label    string
}

// New builds the password prompt for a secured network; security is the
// display badge (e.g. "WPA2").
func New(ssid, security string) Model {
	input := textinput.New()
	input.EchoMode = textinput.EchoPassword
	input.SetWidth(24)
	input.Focus()
	return Model{
		input:    input,
		title:    fmt.Sprintf("Connect to %s", ssid),
		security: security,
		label:    "Password",
	}
}

// NewSSIDEntry builds the first step of the hidden-network flow: a plain
// text prompt for the network name.
func NewSSIDEntry() Model {
	input := textinput.New()
	input.SetWidth(24)
	input.Focus()
	return Model{input: input, title: "Hidden network", label: "SSID"}
}

// Update consumes one key press. done reports the modal should close;
// submitted is true when the user confirmed rather than cancelled.
func (m Model) Update(msg tea.KeyPressMsg) (_ Model, done, submitted bool) {
	switch msg.Code {
	case tea.KeyEscape:
		return m, true, false
	case tea.KeyEnter:
		return m, true, true
	}
	m.input, _ = m.input.Update(msg)
	return m, false, false
}

func (m Model) Value() string {
	return m.input.Value()
}

func (m *Model) SetValue(value string) {
	m.input.SetValue(value)
}

var box = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	Padding(0, 1)

func (m Model) View() string {
	lines := m.title + "\n"
	if m.security != "" {
		lines += fmt.Sprintf("Security  %s\n", m.security)
	}
	lines += fmt.Sprintf("\n%s %s\n\n↵ connect · esc cancel", m.label, m.input.View())
	return box.Render(lines)
}
