package domain

// DeviceGroups splits the device set the way the tabs do: physical NICs
// (which get their own tabs) and virtual devices (bridge, veth, tun,
// bond, ... — the Virtual tab's rows).
type DeviceGroups struct {
	Physical []Device
	Virtual  []Device
}

// GroupDevices splits devices into physical and virtual groups,
// preserving input order within each group.
func GroupDevices(devices []Device) DeviceGroups {
	var g DeviceGroups
	for _, d := range devices {
		switch d.Type {
		case DeviceTypeWifi, DeviceTypeEthernet:
			g.Physical = append(g.Physical, d)
		default:
			g.Virtual = append(g.Virtual, d)
		}
	}
	return g
}
