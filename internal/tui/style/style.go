// Package style is the shared visual vocabulary: static styles plus the
// adaptive Tokyo Night-derived theme.
package style

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
)

var (
	Title    = lipgloss.NewStyle().Bold(true)
	Selected = lipgloss.NewStyle().Bold(true)
	Faint    = lipgloss.NewStyle().Faint(true)
)

// Fit renders a screen's lines into its pane: truncated to height and
// clipped to width, zero meaning unconstrained.
func Fit(lines []string, width, height int) string {
	if height > 0 && len(lines) > height {
		lines = lines[:height]
	}
	if width > 0 {
		clip := lipgloss.NewStyle().MaxWidth(width)
		for i, line := range lines {
			lines[i] = clip.Render(line)
		}
	}
	return strings.Join(lines, "\n") + "\n"
}

// NMNotRunningNotice is the edge state every tab shows while the backend is
// unreachable; the fix ships as part of the message (k9s convention).
const NMNotRunningNotice = "NetworkManager is not running — systemctl start NetworkManager"

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
