// Package style is the shared visual vocabulary; the adaptive palette
// arrives with the theme work.
package style

import "charm.land/lipgloss/v2"

var (
	Title    = lipgloss.NewStyle().Bold(true)
	Selected = lipgloss.NewStyle().Bold(true)
)
