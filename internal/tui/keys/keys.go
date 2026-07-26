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
	Info       key.Binding
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
		Info: key.NewBinding(
			key.WithKeys("i"),
			key.WithHelp("i", "info"),
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

func (d Devices) ShortHelp() []key.Binding {
	return []key.Binding{d.Enter, d.Activate, d.Deactivate, d.Filter}
}

func (d Devices) FullHelp() [][]key.Binding {
	return append(d.List.FullHelp(), []key.Binding{d.Enter, d.Info, d.Activate, d.Deactivate, d.Filter})
}

type Wifi struct {
	List
	Connect     key.Binding
	JoinHidden  key.Binding
	Deactivate  key.Binding
	Edit        key.Binding
	Forget      key.Binding
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
		JoinHidden: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "join hidden"),
		),
		Deactivate: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "disconnect"),
		),
		Edit: key.NewBinding(
			key.WithKeys("e"),
			key.WithHelp("e", "edit"),
		),
		Forget: key.NewBinding(
			key.WithKeys("x"),
			key.WithHelp("x", "forget"),
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

// ShortHelp is the footer: the actions that teach the tab, navigation
// stays in the ? overlay.
func (w Wifi) ShortHelp() []key.Binding {
	return []key.Binding{w.Connect, w.Deactivate, w.Forget, w.Edit, w.Filter}
}

func (w Wifi) FullHelp() [][]key.Binding {
	return append(w.List.FullHelp(),
		[]key.Binding{w.Connect, w.JoinHidden, w.Deactivate, w.Edit, w.Forget, w.Filter, w.ClearFilter})
}

// Ethernet is a wired device tab's keymap: the device detail plus its
// wired profile list with the profile actions.
type Ethernet struct {
	List
	Activate   key.Binding
	Deactivate key.Binding
	Edit       key.Binding
	Delete     key.Binding
	New        key.Binding
}

func DefaultEthernet() Ethernet {
	return Ethernet{
		List: DefaultList(),
		Activate: key.NewBinding(
			key.WithKeys("a", "enter"),
			key.WithHelp("a/↵", "activate"),
		),
		Deactivate: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "deactivate"),
		),
		Edit: key.NewBinding(
			key.WithKeys("e"),
			key.WithHelp("e", "edit"),
		),
		Delete: key.NewBinding(
			key.WithKeys("x"),
			key.WithHelp("x", "delete"),
		),
		New: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n", "new"),
		),
	}
}

func (e Ethernet) ShortHelp() []key.Binding {
	return []key.Binding{e.Activate, e.Deactivate, e.Edit, e.Delete, e.New}
}

func (e Ethernet) FullHelp() [][]key.Binding {
	return append(e.List.FullHelp(),
		[]key.Binding{e.Activate, e.Deactivate, e.Edit, e.Delete, e.New})
}

// System is the System tab keymap: list navigation over the settings
// fields plus their contextual actions.
type System struct {
	List
	Edit   key.Binding
	Toggle key.Binding
}

func DefaultSystem() System {
	return System{
		List: DefaultList(),
		Edit: key.NewBinding(
			key.WithKeys("i", "enter"),
			key.WithHelp("i/↵", "edit"),
		),
		Toggle: key.NewBinding(
			key.WithKeys("space"),
			key.WithHelp("space", "toggle"),
		),
	}
}

func (s System) ShortHelp() []key.Binding {
	return []key.Binding{s.Edit, s.Toggle}
}

func (s System) FullHelp() [][]key.Binding {
	return append(s.List.FullHelp(), []key.Binding{s.Edit, s.Toggle})
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
	return []key.Binding{c.Activate, c.Deactivate, c.Edit, c.Delete, c.New}
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
	Help      key.Binding
	Quit      key.Binding
	Tabs      key.Binding
	NextTab   key.Binding
	PrevTab   key.Binding
	WifiRadio key.Binding
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
			key.WithKeys("1", "2", "3", "4", "5", "6", "7", "8", "9"),
			key.WithHelp("1-9", "tab"),
		),
		NextTab: key.NewBinding(
			key.WithKeys("]"),
			key.WithHelp("]", "next tab"),
		),
		PrevTab: key.NewBinding(
			key.WithKeys("["),
			key.WithHelp("[", "prev tab"),
		),
		WifiRadio: key.NewBinding(
			key.WithKeys("W"),
			key.WithHelp("W", "wifi radio"),
		),
	}
}

func (g Global) ShortHelp() []key.Binding {
	return []key.Binding{g.Help, g.Quit}
}

func (g Global) FullHelp() [][]key.Binding {
	return [][]key.Binding{{g.Tabs, g.NextTab, g.PrevTab, g.WifiRadio, g.Help, g.Quit}}
}
