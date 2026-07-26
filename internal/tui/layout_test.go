package tui

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/ilmars/netfu/internal/backend/fake"
	"github.com/ilmars/netfu/internal/domain"
)

var update = flag.Bool("update", false, "rewrite the layout golden files")

// requireGolden pins the ANSI-stripped 80x24 frame: row positions, padding
// and section spacing. Regenerate with `go test ./internal/tui/ -update`.
func requireGolden(t *testing.T, name, frame string) {
	t.Helper()
	frame = ansi.Strip(frame)
	path := filepath.Join("testdata", name+".golden")
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(frame), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("missing golden %s — run with -update: %v", path, err)
	}
	if frame != string(want) {
		t.Errorf("layout drifted from %s (run with -update if intended)\ngot:\n%s\nwant:\n%s",
			path, visibleWhitespace(frame), visibleWhitespace(string(want)))
	}
}

// visibleWhitespace marks line ends so padding drift shows in the diff.
func visibleWhitespace(s string) string {
	return strings.ReplaceAll(s, "\n", "¬\n")
}

func TestLayout_WifiTabWhileScanning(t *testing.T) {
	f := fake.SeedArchLaptop()
	p := newPump(t, New(f))

	p.send(tea.WindowSizeMsg{Width: 80, Height: 24})

	requireGolden(t, "wifi_scanning_80x24", p.view())
}

func TestLayout_WifiTabIdleWithCursorOnSecondRow(t *testing.T) {
	f := fake.SeedArchLaptop()
	p := newPump(t, New(f))

	p.send(tea.WindowSizeMsg{Width: 80, Height: 24})
	f.Push(domain.Event{Kind: domain.EventAPListChanged, DeviceName: "wlan0"})
	p.deliverNext()
	p.send(keyPress('j'))

	requireGolden(t, "wifi_idle_cursor_second_row_80x24", p.view())
}

func TestLayout_EthernetTab(t *testing.T) {
	f := fake.SeedArchLaptop()
	p := newPump(t, New(f))

	p.send(tea.WindowSizeMsg{Width: 80, Height: 24})
	p.send(keyPress('2'))

	requireGolden(t, "ethernet_80x24", p.view())
}

func TestLayout_EthernetTabWithProfiles(t *testing.T) {
	f := seedWiredProfiles()
	p := newPump(t, New(f))

	p.send(tea.WindowSizeMsg{Width: 80, Height: 24})
	p.send(keyPress('2'))

	requireGolden(t, "ethernet_profiles_80x24", p.view())
}

func TestLayout_VirtualTab(t *testing.T) {
	f := fake.SeedArchLaptop()
	p := newPump(t, New(f))

	p.send(tea.WindowSizeMsg{Width: 80, Height: 24})
	p.send(keyPress('3'))

	requireGolden(t, "virtual_80x24", p.view())
}
