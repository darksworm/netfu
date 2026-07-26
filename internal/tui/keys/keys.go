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

type Global struct {
	Help key.Binding
	Quit key.Binding
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
	}
}

func (g Global) ShortHelp() []key.Binding {
	return []key.Binding{g.Help, g.Quit}
}

func (g Global) FullHelp() [][]key.Binding {
	return [][]key.Binding{{g.Help, g.Quit}}
}
