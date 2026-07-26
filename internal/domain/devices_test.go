package domain

import "testing"

func TestDevices_DockerAndVethDevicesGroupedUnderVirtual(t *testing.T) {
	devices := []Device{
		{Name: "docker0", Type: DeviceTypeBridge, Managed: true},
		{Name: "wlan0", Type: DeviceTypeWifi, Managed: true},
		{Name: "veth1a2b", Type: DeviceTypeVeth, Managed: true},
		{Name: "enp0s31f6", Type: DeviceTypeEthernet, Managed: true},
	}

	got := GroupDevices(devices)

	wantPhysical := []string{"wlan0", "enp0s31f6"}
	wantVirtual := []string{"docker0", "veth1a2b"}
	if len(got.Physical) != len(wantPhysical) {
		t.Fatalf("Physical = %+v, want names %v", got.Physical, wantPhysical)
	}
	for i, name := range wantPhysical {
		if got.Physical[i].Name != name {
			t.Errorf("Physical[%d] = %q, want %q", i, got.Physical[i].Name, name)
		}
	}
	if len(got.Virtual) != len(wantVirtual) {
		t.Fatalf("Virtual = %+v, want names %v", got.Virtual, wantVirtual)
	}
	for i, name := range wantVirtual {
		if got.Virtual[i].Name != name {
			t.Errorf("Virtual[%d] = %q, want %q", i, got.Virtual[i].Name, name)
		}
	}
}
