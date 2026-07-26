package domain

// DeviceGroups is the devices tab's row model: physical NICs first,
// virtual devices (bridge, veth, tun, bond, ...) grouped after.
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
