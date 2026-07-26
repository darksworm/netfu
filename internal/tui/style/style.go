// Package style is the shared visual vocabulary: static styles plus the
// adaptive Tokyo Night-derived theme.
package style

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

var (
	Title    = lipgloss.NewStyle().Bold(true)
	Selected = lipgloss.NewStyle().Bold(true)
)

// Theme is the adaptive palette, resolved once the terminal background is
// known. Each color is the light/dark pair member matching the background.
type Theme struct {
	IsDark    bool
	Accent    color.Color
	Good      color.Color
	Attention color.Color
	Error     color.Color
	Dim       color.Color
}

func NewTheme(isDark bool) Theme {
	light := lipgloss.LightDark(isDark)
	return Theme{
		IsDark:    isDark,
		Accent:    light(lipgloss.Color("#2A5FBF"), lipgloss.Color("#7AA2F7")),
		Good:      light(lipgloss.Color("#587539"), lipgloss.Color("#9ECE6A")),
		Attention: light(lipgloss.Color("#8F5E15"), lipgloss.Color("#E0AF68")),
		Error:     light(lipgloss.Color("#C64343"), lipgloss.Color("#F7768E")),
		Dim:       light(lipgloss.Color("#848CB5"), lipgloss.Color("#565F89")),
	}
}
