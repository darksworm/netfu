// Package statusbar is the single feedback line above the help footer:
// connect spinner, green ✓ / red ✗ results, polkit locks. M1 lands the
// skeleton only; screens fill it in later milestones.
package statusbar

type Model struct {
	message string
}

func New() Model {
	return Model{}
}

func (m Model) SetMessage(message string) Model {
	m.message = message
	return m
}

func (m Model) Clear() Model {
	m.message = ""
	return m
}

func (m Model) Message() string {
	return m.message
}

func (m Model) View() string {
	return m.message
}
