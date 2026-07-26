// Package ethernet is one wired device's tab: the device detail plus
// activate/deactivate. Wired profile management arrives in a later packet.
package ethernet

import (
	"fmt"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/ilmars/netfu/internal/backend"
	"github.com/ilmars/netfu/internal/domain"
	"github.com/ilmars/netfu/internal/tui/components/confirm"
	"github.com/ilmars/netfu/internal/tui/keys"
	"github.com/ilmars/netfu/internal/tui/style"
)

// StatusMsg is feedback for the app's status line above the footer.
type StatusMsg string

type Model struct {
	backend backend.Backend
	device  string
	keys    keys.Ethernet
	detail  domain.Device
	active  []domain.ActiveConnection
	saved   []domain.Connection
	modal   *confirm.Model
	err     error
	width   int
	height  int
}

type loadedMsg struct {
	device string
	detail domain.Device
	active []domain.ActiveConnection
	saved  []domain.Connection
	err    error
}

func New(b backend.Backend, device string) Model {
	return Model{backend: b, device: device, keys: keys.DefaultEthernet()}
}

func (m Model) Init() tea.Cmd {
	return m.load
}

func (m Model) load() tea.Msg {
	devices, err := m.backend.Devices()
	detail := domain.Device{Name: m.device}
	for _, d := range devices {
		if d.Name == m.device {
			detail = d
		}
	}
	active, activeErr := m.backend.ActiveConnections()
	if err == nil {
		err = activeErr
	}
	saved, savedErr := m.backend.Connections()
	if err == nil {
		err = savedErr
	}
	return loadedMsg{device: m.device, detail: detail, active: active, saved: saved, err: err}
}

func (m Model) Keys() keys.Ethernet {
	return m.keys
}

// Overlay is the open confirm's view, layered by the root model over the
// dimmed detail; empty when no modal is open.
func (m Model) Overlay() string {
	if m.modal != nil {
		return m.modal.View()
	}
	return ""
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case loadedMsg:
		// Every device tab's Init runs off the same root model; only this
		// tab's own load may overwrite its detail.
		if msg.device != m.device {
			return m, nil
		}
		m.detail = msg.detail
		m.active = msg.active
		m.saved = msg.saved
		m.err = msg.err
		return m, nil
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if m.modal != nil {
		done, cmd := m.modal.Update(msg)
		if done {
			m.modal = nil
		}
		return m, cmd
	}
	switch {
	case key.Matches(msg, m.keys.Activate):
		return m.activateWiredProfile()
	case key.Matches(msg, m.keys.Deactivate):
		return m.offerDeactivate()
	}
	return m, nil
}

// activateWiredProfile brings the device up on the most recently used wired
// profile — the one NM last activated successfully is the best match.
func (m Model) activateWiredProfile() (Model, tea.Cmd) {
	if m.detail.State == domain.DeviceStateConnected {
		return m, nil
	}
	best := domain.Connection{LastUsedUnix: -1}
	for _, conn := range m.saved {
		if conn.Type == "802-3-ethernet" && conn.LastUsedUnix > best.LastUsedUnix {
			best = conn
		}
	}
	if best.ID == "" {
		return m, status("no wired profile to activate — create one on the Other tab")
	}
	mut, device := m.backend, m.device
	return m, func() tea.Msg {
		if err := mut.Activate(best.ID, device); err != nil {
			return StatusMsg(fmt.Sprintf("✗ activate %s: %v", best.Name, err))
		}
		return StatusMsg(fmt.Sprintf("Activating %s on %s…", best.Name, device))
	}
}

// offerDeactivate opens the confirm modal for the device's active
// connection; d is never destructive without it.
func (m Model) offerDeactivate() (Model, tea.Cmd) {
	if m.detail.State != domain.DeviceStateConnected {
		return m, nil
	}
	for _, ac := range m.active {
		if ac.DeviceName == m.device {
			mut := m.backend
			modal := confirm.New(
				fmt.Sprintf("Deactivate %s?", ac.Name),
				func() tea.Msg {
					if err := mut.Deactivate(ac.ID); err != nil {
						return StatusMsg(fmt.Sprintf("✗ deactivate %s: %v", ac.Name, err))
					}
					return StatusMsg(fmt.Sprintf("Deactivating %s…", ac.Name))
				},
			)
			m.modal = &modal
			return m, nil
		}
	}
	return m, nil
}

func status(format string, args ...any) tea.Cmd {
	return func() tea.Msg {
		return StatusMsg(fmt.Sprintf(format, args...))
	}
}

func (m Model) View() string {
	if m.err != nil {
		return style.NMNotRunningNotice + "\n"
	}
	activeConnection := m.detail.ActiveConnection
	if activeConnection == "" {
		activeConnection = "—"
	}
	lines := []string{
		style.Title.Render("Device " + m.device),
		fmt.Sprintf("%-20s %s", "Type:", m.detail.Type),
		fmt.Sprintf("%-20s %s", "State:", m.detail.State),
		fmt.Sprintf("%-20s %s", "Active connection:", activeConnection),
	}
	return style.Fit(lines, m.width, m.height)
}
