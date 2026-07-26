package backend

import "github.com/ilmars/netfu/internal/domain"

type Reader interface {
	Devices() ([]domain.Device, error)
	Connections() ([]domain.Connection, error)
	ActiveConnections() ([]domain.ActiveConnection, error)
	AccessPoints() ([]domain.AccessPoint, error)
	GetSettings(connectionID string) (domain.ConnectionSettings, error)
	Hostname() (string, error)
	Permissions() (domain.Permissions, error)
}

type Watcher interface {
	// Events is the single fan-in channel the TUI drains.
	Events() <-chan domain.Event
}

type Mutator interface {
	Activate(connectionID, deviceName string) error
	Deactivate(activeConnectionID string) error
	JoinWifi(req domain.JoinRequest) error
	RequestScan() error
	UpdateSettings(connectionID string, settings domain.ConnectionSettings) error
	AddConnection(settings domain.ConnectionSettings) error
	DeleteConnection(connectionID string) error
	SetHostname(hostname string) error
	SetWifiEnabled(enabled bool) error
}

type Backend interface {
	Reader
	Watcher
	Mutator
}
