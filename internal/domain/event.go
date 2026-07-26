package domain

type EventKind string

const (
	EventDeviceChanged     EventKind = "device-changed"
	EventConnectionChanged EventKind = "connection-changed"
	EventAPStrength        EventKind = "ap-strength"
	EventAPListChanged     EventKind = "ap-list-changed"
	EventNMStateChanged    EventKind = "nm-state-changed"
)

type Event struct {
	Kind       EventKind
	DeviceName string
	Reason     string
	// SSID and Strength identify the AP an ap-strength event is about.
	SSID     string
	Strength uint8
}
