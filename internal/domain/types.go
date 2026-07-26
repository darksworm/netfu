package domain

type DeviceType string

const (
	DeviceTypeWifi     DeviceType = "wifi"
	DeviceTypeEthernet DeviceType = "ethernet"
	DeviceTypeBridge   DeviceType = "bridge"
	DeviceTypeVeth     DeviceType = "veth"
	DeviceTypeLoopback DeviceType = "loopback"
	DeviceTypeUnknown  DeviceType = "unknown"
)

type DeviceState string

const (
	DeviceStateUnmanaged    DeviceState = "unmanaged"
	DeviceStateUnavailable  DeviceState = "unavailable"
	DeviceStateDisconnected DeviceState = "disconnected"
	DeviceStateConnecting   DeviceState = "connecting"
	DeviceStateConnected    DeviceState = "connected"
	DeviceStateDeactivating DeviceState = "deactivating"
	DeviceStateFailed       DeviceState = "failed"
)

type Device struct {
	Name             string
	Type             DeviceType
	State            DeviceState
	Managed          bool
	ActiveConnection string
}

type Connection struct {
	ID   string
	Name string
	Type string
	// LastUsedUnix is NM's connection.timestamp: seconds since epoch of the
	// last successful activation, 0 for never.
	LastUsedUnix int64
}

type ActiveConnection struct {
	ID         string
	Name       string
	DeviceName string
	State      DeviceState
}

type AccessPoint struct {
	SSID     string
	Strength uint8
	BSSID    string
	Security Security
}

// ConnectionSettings mirrors NM's a{sa{sv}} shape without leaking dbus types.
type ConnectionSettings = map[string]map[string]any

type JoinRequest struct {
	SSID     string
	Hidden   bool
	Security Security
	PSK      string
}

type Permissions map[string]bool
