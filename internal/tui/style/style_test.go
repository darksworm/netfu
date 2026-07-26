package style

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestSelectedRowIsVisibleBeforeThemeResolves(t *testing.T) {
	rendered := Selected.Render("Our House 1")

	// Reverse video is the only cursor treatment guaranteed to contrast on
	// an unknown background; bold alone disappears among other bold rows.
	if !strings.Contains(rendered, "\x1b[7m") && !strings.Contains(rendered, ";7m") {
		t.Errorf("the selected row should use reverse video until the theme is known, got %q", rendered)
	}
}

func TestSelectedRowGetsAccentTintedBackgroundOnceThemeResolves(t *testing.T) {
	defer func(s lipgloss.Style) { Selected = s }(Selected)

	ResolveSelected(true)

	rendered := Selected.Render("Our House 1")
	// #33467C — the dark-scheme selection tint — as a 24-bit background.
	if !strings.Contains(rendered, "48;2;51;70;124") {
		t.Errorf("dark scheme should tint the selected row background, got %q", rendered)
	}
	if strings.Contains(rendered, "\x1b[7m") || strings.Contains(rendered, ";7m") {
		t.Errorf("the tinted background replaces reverse video, got %q", rendered)
	}
}
