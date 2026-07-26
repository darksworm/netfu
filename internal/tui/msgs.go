package tui

import "github.com/ilmars/netfu/internal/domain"

// backendEventMsg carries one drained backend event into the root model.
type backendEventMsg domain.Event

// permissionsMsg carries the startup polkit permissions query result.
type permissionsMsg struct {
	perms domain.Permissions
	err   error
}

// radioStateMsg carries the startup wifi-radio state read from the backend.
type radioStateMsg struct {
	enabled bool
	err     error
}

// radioResultMsg reports the SetWifiEnabled call's outcome.
type radioResultMsg struct {
	err error
}

// rescanTickMsg fires the periodic wifi rescan.
type rescanTickMsg struct{}

// tabsMsg carries the device set the tab bar is derived from.
type tabsMsg struct {
	devices []domain.Device
	err     error
}
