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
		return bytes.Contains(bts, []byte("[1] Wi-Fi")) &&
			bytes.Contains(bts, []byte("Our House 1"))
	}, teatest.WithDuration(3*time.Second))

	tm.Send(tea.KeyPressMsg{Code: 'q', Text: "q"})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}
