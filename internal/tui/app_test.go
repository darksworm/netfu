package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/ilmars/netfu/internal/backend/fake"
)

func TestApp_StartsOnDeviceListAndQQuits(t *testing.T) {
	f := fake.SeedArchLaptop()
	model, _ := drain(t, New(f), New(f).Init())

	view := model.View().Content
	if !strings.Contains(view, "Devices") {
		t.Errorf("landing view should show the Devices screen, got:\n%s", view)
	}

	_, msgs := drain(t, model, updateCmd(t, &model, keyPress('q')))
	if !containsQuit(msgs) {
		t.Errorf("pressing q at top level should quit, got msgs: %#v", msgs)
	}
}

// keyPress builds the message bubbletea v2 delivers for a printable key.
func keyPress(r rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: r, Text: string(r)}
}

// updateCmd applies one msg to the model in place and returns the cmd.
func updateCmd(t *testing.T, m *tea.Model, msg tea.Msg) tea.Cmd {
	t.Helper()
	var cmd tea.Cmd
	*m, cmd = (*m).Update(msg)
	return cmd
}

// drain runs returned cmds synchronously, feeding their msgs back into the
// model, until no cmds remain. QuitMsg is collected but not fed back.
func drain(t *testing.T, m tea.Model, cmd tea.Cmd) (tea.Model, []tea.Msg) {
	t.Helper()
	var msgs []tea.Msg
	queue := []tea.Cmd{cmd}
	for len(queue) > 0 {
		c := queue[0]
		queue = queue[1:]
		if c == nil {
			continue
		}
		msg := c()
		if msg == nil {
			continue
		}
		if batch, ok := msg.(tea.BatchMsg); ok {
			queue = append(queue, batch...)
			continue
		}
		msgs = append(msgs, msg)
		if _, ok := msg.(tea.QuitMsg); ok {
			continue
		}
		var next tea.Cmd
		m, next = m.Update(msg)
		queue = append(queue, next)
	}
	return m, msgs
}

func containsQuit(msgs []tea.Msg) bool {
	for _, msg := range msgs {
		if _, ok := msg.(tea.QuitMsg); ok {
			return true
		}
	}
	return false
}
