//go:build nmintegration

// Read-only integration tests against the machine's real NetworkManager.
// Run with: go test -tags nmintegration ./internal/backend/nm/
// They scan and read but never activate, modify, or delete anything.
package nm

import (
	"testing"
	"time"

	"github.com/ilmars/netfu/internal/domain"
)

func newAdapter(t *testing.T) *Adapter {
	t.Helper()
	a, err := New()
	if err != nil {
		t.Fatalf("New(): %v", err)
	}
	return a
}

func TestDevices_IncludesTheWifiDevice(t *testing.T) {
	a := newAdapter(t)
	devices, err := a.Devices()
	if err != nil {
		t.Fatalf("Devices(): %v", err)
	}
	for _, d := range devices {
		if d.Name == "wlan0" {
			if d.Type != domain.DeviceTypeWifi {
				t.Errorf("wlan0 type = %q, want wifi", d.Type)
			}
			return
		}
	}
	t.Errorf("wlan0 not in device list: %+v", devices)
}

func TestGetSettings_RoundTripsASaneMapShape(t *testing.T) {
	a := newAdapter(t)
	connections, err := a.Connections()
	if err != nil {
		t.Fatalf("Connections(): %v", err)
	}
	if len(connections) == 0 {
		t.Skip("no saved connections on this machine")
	}
	c := connections[0]
	if c.ID == "" || c.Name == "" || c.Type == "" {
		t.Errorf("connection missing identity fields: %+v", c)
	}
	settings, err := a.GetSettings(c.ID)
	if err != nil {
		t.Fatalf("GetSettings(%q): %v", c.ID, err)
	}
	conn, ok := settings["connection"]
	if !ok {
		t.Fatalf("settings missing 'connection' group: %v", settings)
	}
	if conn["uuid"] != c.ID {
		t.Errorf("settings uuid = %v, want %q", conn["uuid"], c.ID)
	}
	if conn["id"] != c.Name {
		t.Errorf("settings id = %v, want %q", conn["id"], c.Name)
	}
	if wifi, ok := settings["802-11-wireless"]; ok {
		if _, isString := wifi["ssid"].(string); !isString {
			t.Errorf("ssid not converted to string: %#v", wifi["ssid"])
		}
	}
}

func TestAccessPoints_ReturnsResultsAfterAScan(t *testing.T) {
	a := newAdapter(t)
	if err := a.RequestScan(); err != nil {
		t.Fatalf("RequestScan(): %v", err)
	}
	deadline := time.Now().Add(15 * time.Second)
	for {
		aps, err := a.AccessPoints()
		if err != nil {
			t.Fatalf("AccessPoints(): %v", err)
		}
		if len(aps) > 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("no access points visible within 15s of a scan")
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func TestEvents_DeliverWithin15sOfAScan(t *testing.T) {
	a := newAdapter(t)
	if err := a.RequestScan(); err != nil {
		t.Fatalf("RequestScan(): %v", err)
	}
	select {
	case e := <-a.Events():
		t.Logf("event: %+v", e)
	case <-time.After(15 * time.Second):
		t.Fatal("no event within 15s of a scan")
	}
}

func TestHostnameAndPermissions_AreReadable(t *testing.T) {
	a := newAdapter(t)
	hostname, err := a.Hostname()
	if err != nil || hostname == "" {
		t.Errorf("Hostname() = %q, %v", hostname, err)
	}
	perms, err := a.Permissions()
	if err != nil {
		t.Fatalf("Permissions(): %v", err)
	}
	if len(perms) == 0 {
		t.Error("GetPermissions returned no entries")
	}
	if _, ok := perms["org.freedesktop.NetworkManager.network-control"]; !ok {
		t.Errorf("expected network-control permission key, got %v", perms)
	}
}

func TestWifiEnabledAndNMState_AreReadable(t *testing.T) {
	a := newAdapter(t)
	if _, err := a.WifiEnabled(); err != nil {
		t.Errorf("WifiEnabled(): %v", err)
	}
	state, err := a.NMState()
	if err != nil {
		t.Fatalf("NMState(): %v", err)
	}
	if state == domain.NMStateUnknown {
		t.Errorf("NMState() = %q on a running NM", state)
	}
}
