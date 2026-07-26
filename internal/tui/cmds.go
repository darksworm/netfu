package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/ilmars/netfu/internal/backend"
	"github.com/ilmars/netfu/internal/domain"
)

// waitForActivity blocks on the backend's fan-in channel; the root model
// re-arms it after every backendEventMsg (the bubbletea realtime pattern).
func waitForActivity(events <-chan domain.Event) tea.Cmd {
	return func() tea.Msg {
		return backendEventMsg(<-events)
	}
}

// loadPermissions queries polkit permissions once; the result is cached on
// the root model for the whole session.
func loadPermissions(r backend.Reader) tea.Cmd {
	return func() tea.Msg {
		perms, err := r.Permissions()
		return permissionsMsg{perms: perms, err: err}
	}
}
