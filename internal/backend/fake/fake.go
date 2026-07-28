// Package fake is a scriptable in-memory Backend for tests.
package fake

import (
	"fmt"
	"sync"
	"time"

	"github.com/ilmars/netfu/internal/backend"
	"github.com/ilmars/netfu/internal/domain"
)

type Fake struct {
	// mu guards every field: bubbletea runs batched cmds concurrently.
	mu sync.Mutex

	DeviceList     []domain.Device
	ConnectionList []domain.Connection
	ActiveList     []domain.ActiveConnection
	APList         []domain.AccessPoint
	SettingsByID   map[string]domain.ConnectionSettings
	HostnameValue  string
	WifiOn         bool
	NMStateValue   domain.NMState
	Perms          domain.Permissions

	// Errs makes the named method fail, e.g. Errs["JoinWifi"] = err.
	Errs map[string]error
	// JoinConnectsImmediately scripts a successful activation: JoinWifi
	// makes the network active and pushes the change event, like NM does
	// when the credentials are right.
	JoinConnectsImmediately bool
	// Calls records every mutator call for assertions.
	Calls []string
	// The *Calls slices capture mutator arguments for assertions.
	JoinCalls           []domain.JoinRequest
	ActivateCalls       []ActivateCall
	UpdateCalls         []UpdateCall
	AddedSettings       []domain.ConnectionSettings
	DeleteCalls         []string
	SetWifiEnabledCalls []bool

	events chan domain.Event
}

type ActivateCall struct {
	ConnectionID string
	DeviceName   string
}

type UpdateCall struct {
	ConnectionID string
	Settings     domain.ConnectionSettings
}

var _ backend.Backend = (*Fake)(nil)

func New() *Fake {
	return &Fake{
		SettingsByID: map[string]domain.ConnectionSettings{},
		WifiOn:       true,
		NMStateValue: domain.NMStateConnected,
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

// SeedAutoconnectPriorities extends the arch laptop with explicit
// autoconnect data: one wifi profile pinned high, the rest on the default
// priority split by last use, and the bridge opted out of autoconnect.
func SeedAutoconnectPriorities() *Fake {
	f := SeedArchLaptop()
	f.ConnectionList[0].LastUsedUnix = time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC).Unix()  // Our House 1
	f.ConnectionList[2].LastUsedUnix = time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC).Unix() // Summer House
	f.SettingsByID["our-house-5g"]["connection"]["autoconnect-priority"] = int32(10)
	f.SettingsByID["docker0"] = domain.ConnectionSettings{
		"connection": {"id": "docker0", "uuid": "docker0", "type": "bridge", "autoconnect": false},
	}
	return f
}

func (f *Fake) Push(e domain.Event) {
	f.events <- e
}

// record appends to the call log; callers hold f.mu.
func (f *Fake) record(call string) error {
	f.Calls = append(f.Calls, call)
	return f.Errs[call]
}

func (f *Fake) Devices() ([]domain.Device, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.Errs["Devices"]; err != nil {
		return nil, err
	}
	return f.DeviceList, nil
}

func (f *Fake) Connections() ([]domain.Connection, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.Errs["Connections"]; err != nil {
		return nil, err
	}
	return f.ConnectionList, nil
}

func (f *Fake) ActiveConnections() ([]domain.ActiveConnection, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.Errs["ActiveConnections"]; err != nil {
		return nil, err
	}
	return f.ActiveList, nil
}

func (f *Fake) AccessPoints() ([]domain.AccessPoint, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.Errs["AccessPoints"]; err != nil {
		return nil, err
	}
	return f.APList, nil
}

func (f *Fake) GetSettings(connectionID string) (domain.ConnectionSettings, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.Errs["GetSettings"]; err != nil {
		return nil, err
	}
	return f.SettingsByID[connectionID], nil
}

func (f *Fake) Hostname() (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.Errs["Hostname"]; err != nil {
		return "", err
	}
	return f.HostnameValue, nil
}

// Permissions is logged to Calls, unlike the other readers, so tests can
// assert it is queried once and cached.
func (f *Fake) Permissions() (domain.Permissions, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.record("Permissions"); err != nil {
		return nil, err
	}
	return f.Perms, nil
}

func (f *Fake) WifiEnabled() (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.Errs["WifiEnabled"]; err != nil {
		return false, err
	}
	return f.WifiOn, nil
}

func (f *Fake) NMState() (domain.NMState, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.Errs["NMState"]; err != nil {
		return domain.NMStateUnknown, err
	}
	return f.NMStateValue, nil
}

func (f *Fake) Events() <-chan domain.Event {
	return f.events
}

func (f *Fake) Activate(connectionID, deviceName string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.ActivateCalls = append(f.ActivateCalls, ActivateCall{ConnectionID: connectionID, DeviceName: deviceName})
	f.Calls = append(f.Calls, fmt.Sprintf("Activate(%s,%s)", connectionID, deviceName))
	return f.Errs["Activate"]
}

func (f *Fake) Deactivate(activeConnectionID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.Calls = append(f.Calls, fmt.Sprintf("Deactivate(%s)", activeConnectionID))
	return f.Errs["Deactivate"]
}

// JoinWifi mirrors NM's AddAndActivateConnection: the profile is created
// even when the activation later fails on a wrong password.
func (f *Fake) JoinWifi(req domain.JoinRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.JoinCalls = append(f.JoinCalls, req)
	if err := f.record("JoinWifi"); err != nil {
		return err
	}
	id := "joined-" + req.SSID
	f.ConnectionList = append(f.ConnectionList, domain.Connection{ID: id, Name: req.SSID, Type: "802-11-wireless"})
	f.SettingsByID[id] = domain.ConnectionSettings{
		"connection":      {"id": req.SSID, "uuid": id, "type": "802-11-wireless"},
		"802-11-wireless": {"ssid": req.SSID},
	}
	if f.JoinConnectsImmediately {
		f.activateOnWifiDevice(id, req.SSID)
	}
	return nil
}

func (f *Fake) activateOnWifiDevice(connID, name string) {
	wifiDevice := ""
	for _, d := range f.DeviceList {
		if d.Type == domain.DeviceTypeWifi {
			wifiDevice = d.Name
			break
		}
	}
	var active []domain.ActiveConnection
	for _, ac := range f.ActiveList {
		if ac.DeviceName != wifiDevice {
			active = append(active, ac)
		}
	}
	f.ActiveList = append(active, domain.ActiveConnection{
		ID: connID, Name: name, DeviceName: wifiDevice, State: domain.DeviceStateConnected,
	})
	f.Push(domain.Event{Kind: domain.EventConnectionChanged})
}

func (f *Fake) RequestScan() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.record("RequestScan")
}

func (f *Fake) UpdateSettings(connectionID string, settings domain.ConnectionSettings) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.UpdateCalls = append(f.UpdateCalls, UpdateCall{ConnectionID: connectionID, Settings: settings})
	if err := f.record("UpdateSettings"); err != nil {
		return err
	}
	// NM applies the update, so reloads see the written settings.
	f.SettingsByID[connectionID] = settings
	return nil
}

func (f *Fake) AddConnection(settings domain.ConnectionSettings) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.AddedSettings = append(f.AddedSettings, settings)
	return f.record("AddConnection")
}

func (f *Fake) DeleteConnection(connectionID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.DeleteCalls = append(f.DeleteCalls, connectionID)
	f.Calls = append(f.Calls, fmt.Sprintf("DeleteConnection(%s)", connectionID))
	if err := f.Errs["DeleteConnection"]; err != nil {
		return err
	}
	for i, c := range f.ConnectionList {
		if c.ID == connectionID {
			f.ConnectionList = append(f.ConnectionList[:i], f.ConnectionList[i+1:]...)
			break
		}
	}
	delete(f.SettingsByID, connectionID)
	return nil
}

func (f *Fake) SetHostname(hostname string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.Calls = append(f.Calls, fmt.Sprintf("SetHostname(%s)", hostname))
	if err := f.Errs["SetHostname"]; err != nil {
		return err
	}
	f.HostnameValue = hostname
	return nil
}

func (f *Fake) SetWifiEnabled(enabled bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.SetWifiEnabledCalls = append(f.SetWifiEnabledCalls, enabled)
	f.Calls = append(f.Calls, fmt.Sprintf("SetWifiEnabled(%t)", enabled))
	if err := f.Errs["SetWifiEnabled"]; err != nil {
		return err
	}
	f.WifiOn = enabled
	return nil
}
