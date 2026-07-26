package nm

import (
	"fmt"
	"sync"

	gonm "github.com/Wifx/gonetworkmanager/v3"
	"github.com/godbus/dbus/v5"

	"github.com/ilmars/netfu/internal/backend"
	"github.com/ilmars/netfu/internal/domain"
)

// Adapter implements backend.Backend over NetworkManager's D-Bus API.
// Connections are identified by their NM UUID throughout.
type Adapter struct {
	nm       gonm.NetworkManager
	settings gonm.Settings
	watcher  *watcher

	mu          sync.Mutex
	deviceNames map[dbus.ObjectPath]string
}

var _ backend.Backend = (*Adapter)(nil)

func New() (*Adapter, error) {
	manager, err := gonm.NewNetworkManager()
	if err != nil {
		return nil, fmt.Errorf("connect to NetworkManager: %w", err)
	}
	settings, err := gonm.NewSettings()
	if err != nil {
		return nil, fmt.Errorf("connect to NetworkManager settings: %w", err)
	}
	conn, err := dbus.SystemBus()
	if err != nil {
		return nil, fmt.Errorf("connect to system bus: %w", err)
	}

	a := &Adapter{
		nm:          manager,
		settings:    settings,
		deviceNames: map[dbus.ObjectPath]string{},
	}
	a.watcher, err = newWatcher(conn, a.deviceName)
	if err != nil {
		return nil, fmt.Errorf("subscribe to NetworkManager signals: %w", err)
	}
	return a, nil
}

func (a *Adapter) Events() <-chan domain.Event {
	return a.watcher.Events()
}

func (a *Adapter) deviceName(path dbus.ObjectPath) string {
	a.mu.Lock()
	name, ok := a.deviceNames[path]
	a.mu.Unlock()
	if ok {
		return name
	}
	dev, err := gonm.NewDevice(path)
	if err != nil {
		return ""
	}
	name, err = dev.GetPropertyInterface()
	if err != nil {
		return ""
	}
	a.rememberDevice(path, name)
	return name
}

func (a *Adapter) rememberDevice(path dbus.ObjectPath, name string) {
	a.mu.Lock()
	a.deviceNames[path] = name
	a.mu.Unlock()
}

func (a *Adapter) Devices() ([]domain.Device, error) {
	nmDevices, err := a.nm.GetDevices()
	if err != nil {
		return nil, err
	}
	devices := make([]domain.Device, 0, len(nmDevices))
	for _, d := range nmDevices {
		name, err := d.GetPropertyInterface()
		if err != nil {
			return nil, err
		}
		a.rememberDevice(d.GetPath(), name)
		devType, err := d.GetPropertyDeviceType()
		if err != nil {
			return nil, err
		}
		state, err := d.GetPropertyState()
		if err != nil {
			return nil, err
		}
		managed, err := d.GetPropertyManaged()
		if err != nil {
			return nil, err
		}
		activeName := ""
		if active, err := d.GetPropertyActiveConnection(); err == nil && active != nil {
			activeName, _ = active.GetPropertyID()
		}
		devices = append(devices, domain.Device{
			Name:             name,
			Type:             deviceTypeFromNM(devType),
			State:            deviceStateFromNM(state),
			Managed:          managed,
			ActiveConnection: activeName,
		})
	}
	return devices, nil
}

func (a *Adapter) Connections() ([]domain.Connection, error) {
	nmConnections, err := a.settings.ListConnections()
	if err != nil {
		return nil, err
	}
	connections := make([]domain.Connection, 0, len(nmConnections))
	for _, c := range nmConnections {
		settings, err := c.GetSettings()
		if err != nil {
			return nil, err
		}
		connections = append(connections, domain.Connection{
			ID:   stringSetting(settings, "connection", "uuid"),
			Name: stringSetting(settings, "connection", "id"),
			Type: stringSetting(settings, "connection", "type"),
		})
	}
	return connections, nil
}

func stringSetting(settings gonm.ConnectionSettings, group, key string) string {
	s, _ := settings[group][key].(string)
	return s
}

func (a *Adapter) ActiveConnections() ([]domain.ActiveConnection, error) {
	nmActive, err := a.nm.GetPropertyActiveConnections()
	if err != nil {
		return nil, err
	}
	active := make([]domain.ActiveConnection, 0, len(nmActive))
	for _, ac := range nmActive {
		uuid, err := ac.GetPropertyUUID()
		if err != nil {
			return nil, err
		}
		name, err := ac.GetPropertyID()
		if err != nil {
			return nil, err
		}
		state, err := ac.GetPropertyState()
		if err != nil {
			return nil, err
		}
		deviceName := ""
		if devices, err := ac.GetPropertyDevices(); err == nil && len(devices) > 0 {
			deviceName = a.deviceName(devices[0].GetPath())
		}
		active = append(active, domain.ActiveConnection{
			ID:         uuid,
			Name:       name,
			DeviceName: deviceName,
			State:      activeStateFromNM(state),
		})
	}
	return active, nil
}

func (a *Adapter) AccessPoints() ([]domain.AccessPoint, error) {
	wireless, err := a.wirelessDevices()
	if err != nil {
		return nil, err
	}
	var aps []domain.AccessPoint
	for _, w := range wireless {
		nmAPs, err := w.GetAllAccessPoints()
		if err != nil {
			return nil, err
		}
		for _, ap := range nmAPs {
			ssid, err := ap.GetPropertySSID()
			if err != nil {
				continue // APs can vanish between listing and reading
			}
			strength, err := ap.GetPropertyStrength()
			if err != nil {
				continue
			}
			aps = append(aps, domain.AccessPoint{SSID: ssid, Strength: strength})
		}
	}
	return aps, nil
}

func (a *Adapter) wirelessDevices() ([]gonm.DeviceWireless, error) {
	nmDevices, err := a.nm.GetDevices()
	if err != nil {
		return nil, err
	}
	var wireless []gonm.DeviceWireless
	for _, d := range nmDevices {
		devType, err := d.GetPropertyDeviceType()
		if err != nil {
			return nil, err
		}
		if devType != gonm.NmDeviceTypeWifi {
			continue
		}
		w, err := gonm.NewDeviceWireless(d.GetPath())
		if err != nil {
			return nil, err
		}
		wireless = append(wireless, w)
	}
	return wireless, nil
}

func (a *Adapter) GetSettings(connectionID string) (domain.ConnectionSettings, error) {
	c, err := a.settings.GetConnectionByUUID(connectionID)
	if err != nil {
		return nil, err
	}
	settings, err := c.GetSettings()
	if err != nil {
		return nil, err
	}
	return settingsFromNM(settings), nil
}

func (a *Adapter) Hostname() (string, error) {
	return a.settings.GetPropertyHostname()
}

// Permissions queries org.freedesktop.NetworkManager.GetPermissions directly:
// gonetworkmanager declares the method name but never wraps it. NM answers
// "yes"/"no"/"auth" per permission; "auth" counts as allowed because the
// polkit agent will prompt out-of-band.
func (a *Adapter) Permissions() (domain.Permissions, error) {
	conn, err := dbus.SystemBus()
	if err != nil {
		return nil, err
	}
	var raw map[string]string
	obj := conn.Object(nmInterface, nmObjectPath)
	if err := obj.Call(nmInterface+".GetPermissions", 0).Store(&raw); err != nil {
		return nil, err
	}
	perms := domain.Permissions{}
	for name, answer := range raw {
		perms[name] = answer == "yes" || answer == "auth"
	}
	return perms, nil
}

func (a *Adapter) Activate(connectionID, deviceName string) error {
	c, err := a.settings.GetConnectionByUUID(connectionID)
	if err != nil {
		return err
	}
	device, err := a.nm.GetDeviceByIpIface(deviceName)
	if err != nil {
		return err
	}
	_, err = a.nm.ActivateConnection(c, device, nil)
	return err
}

func (a *Adapter) Deactivate(activeConnectionID string) error {
	active, err := a.nm.GetPropertyActiveConnections()
	if err != nil {
		return err
	}
	for _, ac := range active {
		uuid, err := ac.GetPropertyUUID()
		if err != nil {
			continue
		}
		if uuid == activeConnectionID {
			return a.nm.DeactivateConnection(ac)
		}
	}
	return fmt.Errorf("no active connection with UUID %s", activeConnectionID)
}

func (a *Adapter) JoinWifi(req domain.JoinRequest) error {
	wireless, err := a.wirelessDevices()
	if err != nil {
		return err
	}
	if len(wireless) == 0 {
		return fmt.Errorf("no wifi device")
	}
	device := wireless[0]

	settings := gonm.ConnectionSettings{
		"connection":      {"id": req.SSID},
		"802-11-wireless": {"ssid": []byte(req.SSID)},
	}
	if req.Hidden {
		settings["802-11-wireless"]["hidden"] = true
	}
	if keyMgmt := keyMgmtFor(req.Security); keyMgmt != "" {
		settings["802-11-wireless"]["security"] = "802-11-wireless-security"
		settings["802-11-wireless-security"] = map[string]any{
			"key-mgmt": keyMgmt,
			"psk":      req.PSK,
			// psk-flags=0 stores the PSK system-wide; no secret agent in v1.
			"psk-flags": uint32(0),
		}
	}

	if ap := findAccessPoint(device, req.SSID); ap != nil {
		_, err = a.nm.AddAndActivateWirelessConnection(settings, device, ap)
		return err
	}
	if !req.Hidden {
		return fmt.Errorf("no access point with SSID %q in range", req.SSID)
	}
	_, err = a.nm.AddAndActivateConnection(settings, device)
	return err
}

func keyMgmtFor(security string) string {
	switch security {
	case "", "open", "none":
		return ""
	case "sae", "wpa3":
		return "sae"
	default:
		return "wpa-psk"
	}
}

func findAccessPoint(device gonm.DeviceWireless, ssid string) gonm.AccessPoint {
	aps, err := device.GetAllAccessPoints()
	if err != nil {
		return nil
	}
	for _, ap := range aps {
		if s, err := ap.GetPropertySSID(); err == nil && s == ssid {
			return ap
		}
	}
	return nil
}

func (a *Adapter) RequestScan() error {
	wireless, err := a.wirelessDevices()
	if err != nil {
		return err
	}
	if len(wireless) == 0 {
		return fmt.Errorf("no wifi device")
	}
	for _, w := range wireless {
		if err := w.RequestScan(); err != nil {
			return err
		}
	}
	return nil
}

func (a *Adapter) UpdateSettings(connectionID string, settings domain.ConnectionSettings) error {
	c, err := a.settings.GetConnectionByUUID(connectionID)
	if err != nil {
		return err
	}
	return c.Update(settingsToNM(settings))
}

func (a *Adapter) AddConnection(settings domain.ConnectionSettings) error {
	_, err := a.settings.AddConnection(settingsToNM(settings))
	return err
}

func (a *Adapter) DeleteConnection(connectionID string) error {
	c, err := a.settings.GetConnectionByUUID(connectionID)
	if err != nil {
		return err
	}
	return c.Delete()
}

func (a *Adapter) SetHostname(hostname string) error {
	return a.settings.SaveHostname(hostname)
}

func (a *Adapter) SetWifiEnabled(enabled bool) error {
	return a.nm.SetPropertyWirelessEnabled(enabled)
}
