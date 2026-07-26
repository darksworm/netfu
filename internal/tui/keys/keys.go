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

// Connections is the Connections tab keymap: list navigation plus
// profile actions.
type Connections struct {
	List
	Edit       key.Binding
	Delete     key.Binding
	New        key.Binding
	Activate   key.Binding
	Deactivate key.Binding
}

func DefaultConnections() Connections {
	return Connections{
		List: DefaultList(),
		Edit: key.NewBinding(
			key.WithKeys("e", "enter"),
			key.WithHelp("e/↵", "edit"),
		),
		Delete: key.NewBinding(
			key.WithKeys("x"),
			key.WithHelp("x", "delete"),
		),
		New: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n", "new"),
		),
		Activate: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "activate"),
		),
		Deactivate: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "deactivate"),
		),
	}
}

func (c Connections) ShortHelp() []key.Binding {
	return append(c.List.ShortHelp(), c.Edit, c.New)
}

func (c Connections) FullHelp() [][]key.Binding {
	return append(c.List.FullHelp(),
		[]key.Binding{c.Edit, c.Delete, c.New, c.Activate, c.Deactivate})
}

// Editor is the connection editor keymap: NAV-mode field navigation and
// form actions.
type Editor struct {
	Up        key.Binding
	Down      key.Binding
	EditField key.Binding
	Cycle     key.Binding
	Save      key.Binding
	Back      key.Binding
}

func DefaultEditor() Editor {
	return Editor{
		Up: key.NewBinding(
			key.WithKeys("k", "up"),
			key.WithHelp("↑/k", "prev field"),
		),
		Down: key.NewBinding(
			key.WithKeys("j", "down"),
			key.WithHelp("↓/j", "next field"),
		),
		EditField: key.NewBinding(
			key.WithKeys("enter", "i"),
			key.WithHelp("↵/i", "edit field"),
		),
		Cycle: key.NewBinding(
			key.WithKeys("space"),
			key.WithHelp("␣", "cycle"),
		),
		Save: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "save"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc", "q"),
			key.WithHelp("esc", "back"),
		),
	}
}

func (e Editor) ShortHelp() []key.Binding {
	return []key.Binding{e.Down, e.EditField, e.Save, e.Back}
}

func (e Editor) FullHelp() [][]key.Binding {
	return [][]key.Binding{{e.Up, e.Down, e.EditField, e.Cycle, e.Save, e.Back}}
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
