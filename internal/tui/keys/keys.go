// Package keys holds per-screen KeyMap structs; they drive dispatch and,
// later, the help footer.
package keys

import "charm.land/bubbles/v2/key"

type List struct {
	Up     key.Binding
	Down   key.Binding
	Top    key.Binding
	Bottom key.Binding
}

func DefaultList() List {
	return List{
		Up: key.NewBinding(
			key.WithKeys("k", "up"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("j", "down"),
			key.WithHelp("↓/j", "down"),
		),
		Top: key.NewBinding(
			key.WithKeys("g", "home"),
			key.WithHelp("g", "top"),
		),
		Bottom: key.NewBinding(
			key.WithKeys("G", "end"),
			key.WithHelp("G", "bottom"),
		),
	}
}

func (l List) ShortHelp() []key.Binding {
	return []key.Binding{l.Up, l.Down}
}

func (l List) FullHelp() [][]key.Binding {
	return [][]key.Binding{{l.Up, l.Down, l.Top, l.Bottom}}
}

// Devices is the Devices tab keymap: list navigation plus row actions.
type Devices struct {
	List
	Enter      key.Binding
	Activate   key.Binding
	Deactivate key.Binding
	Filter     key.Binding
}

func DefaultDevices() Devices {
	return Devices{
		List: DefaultList(),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("↵", "activate/deactivate"),
		),
		Activate: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "activate"),
		),
		Deactivate: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "deactivate"),
		),
		Filter: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "filter"),
		),
	}
}

type Wifi struct {
	List
	Connect     key.Binding
	Filter      key.Binding
	ClearFilter key.Binding
}

func DefaultWifi() Wifi {
	return Wifi{
		List: DefaultList(),
		Connect: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("↵", "connect"),
		),
		Filter: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "filter"),
		),
		ClearFilter: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "clear filter"),
		),
	}
}

func (w Wifi) ShortHelp() []key.Binding {
	return append(w.List.ShortHelp(), w.Connect, w.Filter)
}

func (w Wifi) FullHelp() [][]key.Binding {
	return append(w.List.FullHelp(), []key.Binding{w.Connect, w.Filter, w.ClearFilter})
}

type Global struct {
	Help    key.Binding
	Quit    key.Binding
	Tabs    key.Binding
	NextTab key.Binding
	PrevTab key.Binding
}

func DefaultGlobal() Global {
	return Global{
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q"),
			key.WithHelp("q", "quit"),
		),
		Tabs: key.NewBinding(
			key.WithKeys("1", "2", "3", "4"),
			key.WithHelp("1-4", "tab"),
		),
		NextTab: key.NewBinding(
			key.WithKeys("]"),
			key.WithHelp("]", "next tab"),
		),
		PrevTab: key.NewBinding(
			key.WithKeys("["),
			key.WithHelp("[", "prev tab"),
		),
	}
}

func (g Global) ShortHelp() []key.Binding {
	return []key.Binding{g.Help, g.Quit}
}

func (g Global) FullHelp() [][]key.Binding {
	return [][]key.Binding{{g.Tabs, g.NextTab, g.PrevTab, g.Help, g.Quit}}
}
