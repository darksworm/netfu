// Package editorform is the generic NAV/EDIT form engine behind the
// connection editor: sections of typed fields, a ▸ cursor moving over
// fields (sections are skipped), Enter/i to edit a text field, space to
// cycle toggles and radios, with per-field validation and dirty tracking.
package editorform

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
)

type Kind int

const (
	Text Kind = iota
	Toggle
	Radio
)

type Field struct {
	// Key names the settings value the field edits, e.g. "connection.id".
	Key     string
	Label   string
	Kind    Kind
	Options []string // Radio choices
	Value   string   // Text content, or the selected Radio option
	On      bool     // Toggle state
	// Validate runs when a text edit commits and again on ValidateAll.
	Validate func(string) error
	Err      string
}

type Section struct {
	Title  string
	Fields []Field
}

type Model struct {
	sections []Section
	cursor   int
	editing  bool
	// input is the hand-rolled EDIT-mode buffer (append/backspace), the
	// same pattern as the list filters — bubbles' textinput would pull a
	// clipboard dependency this module doesn't carry.
	input   string
	touched map[string]bool
}

func New(sections []Section) Model {
	return Model{sections: sections, touched: map[string]bool{}}
}

// fields flattens the sections into the list the cursor moves over.
func (m *Model) fields() []*Field {
	var flat []*Field
	for s := range m.sections {
		for f := range m.sections[s].Fields {
			flat = append(flat, &m.sections[s].Fields[f])
		}
	}
	return flat
}

func (m Model) current() *Field {
	return m.fields()[m.cursor]
}

// Editing reports EDIT mode: every key belongs to the focused text input.
func (m Model) Editing() bool {
	return m.editing
}

func (m Model) Dirty() bool {
	return len(m.touched) > 0
}

func (m Model) Touched(key string) bool {
	return m.touched[key]
}

// Get returns the field with the given key; ok is false if none exists.
func (m Model) Get(key string) (Field, bool) {
	for _, f := range m.fields() {
		if f.Key == key {
			return *f, true
		}
	}
	return Field{}, false
}

// ValidateAll re-runs every field's validator and reports whether the form
// is saveable; failures stay on the fields as inline errors.
func (m *Model) ValidateAll() bool {
	ok := true
	for _, f := range m.fields() {
		if f.Validate == nil {
			continue
		}
		f.Err = ""
		if err := f.Validate(f.Value); err != nil {
			f.Err = err.Error()
			ok = false
		}
	}
	return ok
}

func (m Model) Update(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if m.editing {
		return m.handleEditKey(msg)
	}
	return m.handleNavKey(msg)
}

func (m Model) handleNavKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		if m.cursor < len(m.fields())-1 {
			m.cursor++
		}
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}
	case "enter", "i":
		if m.current().Kind == Text {
			return m.startEditing(), nil
		}
	case "space", " ":
		m.cycle()
	}
	return m, nil
}

func (m Model) startEditing() Model {
	m.input = m.current().Value
	m.editing = true
	return m
}

// cycle advances the toggle/radio under the cursor and marks it dirty.
func (m *Model) cycle() {
	f := m.current()
	switch f.Kind {
	case Toggle:
		f.On = !f.On
		m.touch(f.Key)
	case Radio:
		for i, option := range f.Options {
			if option == f.Value {
				f.Value = f.Options[(i+1)%len(f.Options)]
				break
			}
		}
		m.touch(f.Key)
	}
}

// touch marks a key dirty on a fresh map, so a shallow-copied Model never
// shares dirty state with its predecessor.
func (m *Model) touch(key string) {
	touched := make(map[string]bool, len(m.touched)+1)
	for k := range m.touched {
		touched[k] = true
	}
	touched[key] = true
	m.touched = touched
}

func (m Model) handleEditKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case msg.Code == tea.KeyEnter || msg.Code == tea.KeyEsc:
		return m.commitEdit(), nil
	case msg.Code == tea.KeyBackspace:
		if m.input != "" {
			m.input = m.input[:len(m.input)-1]
		}
	case msg.Text != "":
		m.input += msg.Text
	}
	return m, nil
}

// commitEdit writes the input back to the field, validates it, and returns
// to NAV mode.
func (m Model) commitEdit() Model {
	f := m.current()
	value := m.input
	if value != f.Value {
		f.Value = value
		m.touch(f.Key)
	}
	f.Err = ""
	if f.Validate != nil {
		if err := f.Validate(f.Value); err != nil {
			f.Err = err.Error()
		}
	}
	m.editing = false
	return m
}

func (m Model) View() string {
	var lines []string
	i := 0
	for _, section := range m.sections {
		lines = append(lines, section.Title)
		for _, f := range section.Fields {
			lines = append(lines, m.renderField(f, i))
			i++
		}
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderField(f Field, index int) string {
	cursor := " "
	if index == m.cursor {
		cursor = "▸"
	}
	value := f.Value
	switch f.Kind {
	case Text:
		if m.editing && index == m.cursor {
			value = m.input + "█"
		}
	case Toggle:
		value = "[ ]"
		if f.On {
			value = "[x]"
		}
	case Radio:
		var options []string
		for _, option := range f.Options {
			mark := "( )"
			if option == f.Value {
				mark = "(•)"
			}
			options = append(options, mark+" "+option)
		}
		value = strings.Join(options, " ")
	}
	line := fmt.Sprintf(" %s %-13s %s", cursor, f.Label, value)
	if f.Err != "" {
		line += "  ✗ " + f.Err
	}
	return line
}
