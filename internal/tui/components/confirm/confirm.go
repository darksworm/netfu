// Package confirm is the reusable y/esc confirmation modal. Screens own
// their modal instance and route key presses to it while it is open.
package confirm

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type Model struct {
	prompt    string
	onConfirm tea.Cmd
}

func New(prompt string, onConfirm tea.Cmd) Model {
	return Model{prompt: prompt, onConfirm: onConfirm}
}

// Update consumes one key press. done reports the modal should close;
// cmd is the confirmed action (nil on cancel or unrecognized keys).
func (m Model) Update(msg tea.KeyPressMsg) (done bool, cmd tea.Cmd) {
	switch msg.String() {
	case "y":
		return true, m.onConfirm
	case "esc", "n":
		return true, nil
	}
	return false, nil
}

var box = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	Padding(0, 1)

func (m Model) View() string {
	return box.Render(m.prompt + "\ny confirm · esc cancel")
}
