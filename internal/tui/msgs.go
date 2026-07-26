package tui

import "github.com/ilmars/netfu/internal/domain"

// backendEventMsg carries one drained backend event into the root model.
type backendEventMsg domain.Event

// permissionsMsg carries the startup polkit permissions query result.
type permissionsMsg struct {
	perms domain.Permissions
	err   error
}
