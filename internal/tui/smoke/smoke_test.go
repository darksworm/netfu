package smoke

import (
	"bytes"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	teatest "github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/ilmars/netfu/internal/backend/fake"
	"github.com/ilmars/netfu/internal/tui"
)

func TestSmoke_BootShowsWifiHomeTabAndQExitsCleanly(t *testing.T) {
	tm := teatest.NewTestModel(t, tui.New(fake.SeedArchLaptop()),
		teatest.WithInitialTermSize(80, 24))

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("[1] wlan0")) &&
			bytes.Contains(bts, []byte("Our House 1"))
	}, teatest.WithDuration(3*time.Second))

	tm.Send(tea.KeyPressMsg{Code: 'q', Text: "q"})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestSmoke_JoiningAnOpenNetworkEndsConnectedAndQExitsCleanly(t *testing.T) {
	f := fake.SeedArchLaptop()
	f.JoinConnectsImmediately = true
	tm := teatest.NewTestModel(t, tui.New(f),
		teatest.WithInitialTermSize(80, 24))

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("CafeGuest"))
	}, teatest.WithDuration(3*time.Second))

	// Filter down to the open network and connect to it.
	for _, r := range "/Cafe" {
		tm.Send(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		row := lineWith(bts, []byte("CafeGuest"))
		return bytes.Contains(row, []byte("✓ connected"))
	}, teatest.WithDuration(3*time.Second))

	tm.Send(tea.KeyPressMsg{Code: 'q', Text: "q"})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

func lineWith(bts, want []byte) []byte {
	for _, line := range bytes.Split(bts, []byte("\n")) {
		if bytes.Contains(line, want) {
			return line
		}
	}
	return nil
}
