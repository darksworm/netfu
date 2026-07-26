// Package fake is a scriptable in-memory Backend for tests.
package fake

import (
	"fmt"

	"github.com/ilmars/netfu/internal/backend"
	"github.com/ilmars/netfu/internal/domain"
)

type Fake struct {
	DeviceList     []domain.Device
	ConnectionList []domain.Connection
	ActiveList     []domain.ActiveConnection
	APList         []domain.AccessPoint
	SettingsByID   map[string]domain.ConnectionSettings
	HostnameValue  string
	Perms          domain.Permissions

	// Errs makes the named method fail, e.g. Errs["JoinWifi"] = err.
	Errs map[string]error
	// Calls records every mutator call for assertions.
	Calls []string
	// JoinCalls and ActivateCalls capture mutator arguments for assertions.
	JoinCalls     []domain.JoinRequest
	ActivateCalls []ActivateCall

	events chan domain.Event
}

type ActivateCall struct {
	ConnectionID string
	DeviceName   string
}

var _ backend.Backend = (*Fake)(nil)

func New() *Fake {
	return &Fake{
		SettingsByID: map[string]domain.ConnectionSettings{},
		Perms:        domain.Permissions{},
		Errs:         map[string]error{},
		events:       make(chan domain.Event, 64),
	}
}

// SeedArchLaptop mirrors the development machine: wifi connected, a wired
// device, and docker-created virtual devices.
func SeedArchLaptop() *Fake {
	f := New()
	f.DeviceList = []domain.Device{
		{Name: "wlan0", Type: domain.DeviceTypeWifi, State: domain.DeviceStateConnected, Managed: true, ActiveConnection: "Our House 1"},
		{Name: "enp0s31f6", Type: domain.DeviceTypeEthernet, State: domain.DeviceStateUnavailable, Managed: true},
		{Name: "docker0", Type: domain.DeviceTypeBridge, State: domain.DeviceStateConnected, Managed: true, ActiveConnection: "docker0"},
		{Name: "veth1a2b3c", Type: domain.DeviceTypeVeth, State: domain.DeviceStateUnmanaged, Managed: false},
	}
	f.ActiveList = []domain.ActiveConnection{
		{ID: "our-house-1", Name: "Our House 1", DeviceName: "wlan0", State: domain.DeviceStateConnected},
		{ID: "docker0", Name: "docker0", DeviceName: "docker0", State: domain.DeviceStateConnected},
	}
	f.ConnectionList = []domain.Connection{
		{ID: "our-house-1", Name: "Our House 1", Type: "802-11-wireless"},
		{ID: "our-house-5g", Name: "Our House 5G", Type: "802-11-wireless"},
		{ID: "summer-house", Name: "Summer House", Type: "802-11-wireless"},
		{ID: "docker0", Name: "docker0", Type: "bridge"},
	}
	for _, wifi := range f.ConnectionList[:3] {
		f.SettingsByID[wifi.ID] = domain.ConnectionSettings{
			"connection":      {"id": wifi.Name, "uuid": wifi.ID, "type": wifi.Type},
			"802-11-wireless": {"ssid": wifi.Name},
		}
	}
	f.APList = []domain.AccessPoint{
		{SSID: "Our House 1", Strength: 82, BSSID: "AA:BB:CC:11:11:11", Security: domain.SecurityWPA2},
		{SSID: "Neighbors", Strength: 68, BSSID: "AA:BB:CC:33:33:33", Security: domain.SecurityWPA2},
		{SSID: "Our House 1", Strength: 55, BSSID: "AA:BB:CC:22:22:22", Security: domain.SecurityWPA2},
		{SSID: "Our House 5G", Strength: 61, BSSID: "AA:BB:CC:55:55:55", Security: domain.SecurityWPA3},
		{SSID: "CafeGuest", Strength: 47, BSSID: "AA:BB:CC:44:44:44", Security: domain.SecurityOpen},
		{SSID: "", Strength: 33, BSSID: "AA:BB:CC:66:66:66", Security: domain.SecurityWPA2},
	}
	f.HostnameValue = "archbook"
	return f
}

func (f *Fake) Push(e domain.Event) {
	f.events <- e
}

func (f *Fake) record(call string) error {
	f.Calls = append(f.Calls, call)
	return f.Errs[call]
}

func (f *Fake) Devices() ([]domain.Device, error) {
	if err := f.Errs["Devices"]; err != nil {
		return nil, err
	}
	return f.DeviceList, nil
}

func (f *Fake) Connections() ([]domain.Connection, error) {
	if err := f.Errs["Connections"]; err != nil {
		return nil, err
	}
	return f.ConnectionList, nil
}

func (f *Fake) ActiveConnections() ([]domain.ActiveConnection, error) {
	if err := f.Errs["ActiveConnections"]; err != nil {
		return nil, err
	}
	return f.ActiveList, nil
}

func (f *Fake) AccessPoints() ([]domain.AccessPoint, error) {
	if err := f.Errs["AccessPoints"]; err != nil {
		return nil, err
	}
	return f.APList, nil
}

func (f *Fake) GetSettings(connectionID string) (domain.ConnectionSettings, error) {
	if err := f.Errs["GetSettings"]; err != nil {
		return nil, err
	}
	return f.SettingsByID[connectionID], nil
}

func (f *Fake) Hostname() (string, error) {
	if err := f.Errs["Hostname"]; err != nil {
		return "", err
	}
	return f.HostnameValue, nil
}

// Permissions is logged to Calls, unlike the other readers, so tests can
// assert it is queried once and cached.
func (f *Fake) Permissions() (domain.Permissions, error) {
	if err := f.record("Permissions"); err != nil {
		return nil, err
	}
	return f.Perms, nil
}

func (f *Fake) Events() <-chan domain.Event {
	return f.events
}

func (f *Fake) Activate(connectionID, deviceName string) error {
	f.ActivateCalls = append(f.ActivateCalls, ActivateCall{ConnectionID: connectionID, DeviceName: deviceName})
	f.Calls = append(f.Calls, fmt.Sprintf("Activate(%s,%s)", connectionID, deviceName))
	return f.Errs["Activate"]
}

func (f *Fake) Deactivate(activeConnectionID string) error {
	f.Calls = append(f.Calls, fmt.Sprintf("Deactivate(%s)", activeConnectionID))
	return f.Errs["Deactivate"]
}

func (f *Fake) JoinWifi(req domain.JoinRequest) error {
	f.JoinCalls = append(f.JoinCalls, req)
	return f.record("JoinWifi")
}

func (f *Fake) RequestScan() error {
	return f.record("RequestScan")
}

func (f *Fake) UpdateSettings(connectionID string, settings domain.ConnectionSettings) error {
	return f.record("UpdateSettings")
}

func (f *Fake) AddConnection(settings domain.ConnectionSettings) error {
	return f.record("AddConnection")
}

func (f *Fake) DeleteConnection(connectionID string) error {
	return f.record("DeleteConnection")
}

func (f *Fake) SetHostname(hostname string) error {
	return f.record("SetHostname")
}

func (f *Fake) SetWifiEnabled(enabled bool) error {
	return f.record("SetWifiEnabled")
}
