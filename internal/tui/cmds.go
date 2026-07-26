package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/ilmars/netfu/internal/backend"
	"github.com/ilmars/netfu/internal/domain"
)

// rescanTick drives the periodic wifi rescan. The app re-arms it on every
// tick regardless of tab (one live chain, no storms) and only acts on it
// while the wifi tab is visible.
func rescanTick() tea.Cmd {
	return tea.Tick(15*time.Second, func(time.Time) tea.Msg {
		return rescanTickMsg{}
	})
}

// waitForActivity blocks on the backend's fan-in channel; the root model
// re-arms it after every backendEventMsg (the bubbletea realtime pattern).
func waitForActivity(events <-chan domain.Event) tea.Cmd {
	return func() tea.Msg {
		return backendEventMsg(<-events)
	}
}

// loadRadioState reads the wifi radio state at startup so the app does not
// assume the radio is on.
func loadRadioState(r backend.Reader) tea.Cmd {
	return func() tea.Msg {
		enabled, err := r.WifiEnabled()
		return radioStateMsg{enabled: enabled, err: err}
	}
}

// loadTabs reads the device set the tab bar derives from; it re-runs on
// every backend event so device churn re-derives the tabs.
func loadTabs(r backend.Reader) tea.Cmd {
	return func() tea.Msg {
		devices, err := r.Devices()
		return tabsMsg{devices: devices, err: err}
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
