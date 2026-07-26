package domain

type EventKind string

const (
	EventDeviceChanged     EventKind = "device-changed"
	EventConnectionChanged EventKind = "connection-changed"
	EventAPStrength        EventKind = "ap-strength"
	EventNMStateChanged    EventKind = "nm-state-changed"
)

type Event struct {
	Kind       EventKind
	DeviceName string
	Reason     string
}
