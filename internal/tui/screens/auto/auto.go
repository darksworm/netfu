// Package auto is the Auto tab: every autoconnect profile in the order
// NetworkManager will actually try them (priority desc, then last used),
// reorderable without thinking in priority numbers.
package auto

import (
	"fmt"
	"slices"
	"sort"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/ilmars/netfu/internal/backend"
	"github.com/ilmars/netfu/internal/domain"
	"github.com/ilmars/netfu/internal/tui/keys"
	"github.com/ilmars/netfu/internal/tui/style"
)

// row is one profile with the autoconnect data read from its settings;
// settings ride along so a save can pass untouched keys through verbatim.
type row struct {
	conn     domain.Connection
	settings domain.ConnectionSettings
	priority int
	auto     bool
	bucket   string
}

type loadedMsg struct {
	on  []row
	off []row
	err error
}

// savedMsg is the save cmd's outcome for the status line; the list reloads
// so the written order becomes the new baseline.
type savedMsg struct {
	status string
}

type Model struct {
	backend backend.Backend
	keys    keys.Auto
	perms   domain.Permissions
	// on is the pending pick order, kept grouped by bucket; off holds the
	// autoconnect-off profiles.
	on  []row
	off []row
	// loadedOn/loadedOff snapshot the backend's order; the pending state is
	// dirty while it differs.
	loadedOn  []row
	loadedOff []row
	cursor    int
	// confirmDiscard is the leave-with-pending prompt: save, discard, or stay.
	confirmDiscard bool
	status         string
	err            error
	width          int
	height         int
}

func New(b backend.Backend) Model {
	return Model{backend: b, keys: keys.DefaultAuto()}
}

func (m Model) Init() tea.Cmd {
	return m.load
}

func (m Model) Keys() help.KeyMap {
	return m.keys
}

func (m Model) load() tea.Msg {
	conns, err := m.backend.Connections()
	if err != nil {
		return loadedMsg{err: err}
	}
	var on, off []row
	for _, c := range conns {
		settings, err := m.backend.GetSettings(c.ID)
		if err != nil {
			return loadedMsg{err: err}
		}
		r := row{
			conn:     c,
			settings: settings,
			priority: priorityOf(settings),
			auto:     autoconnectOf(settings),
			bucket:   bucketFor(c.Type),
		}
		if r.auto {
			on = append(on, r)
		} else {
			off = append(off, r)
		}
	}
	sortPickOrder(on)
	return loadedMsg{on: on, off: off}
}

// priorityOf reads connection.autoconnect-priority, tolerating the integer
// shapes a dbus round-trip produces; missing means NM's default 0.
func priorityOf(settings domain.ConnectionSettings) int {
	switch v := settings["connection"]["autoconnect-priority"].(type) {
	case int:
		return v
	case int32:
		return int(v)
	case int64:
		return int(v)
	case uint32:
		return int(v)
	}
	return 0
}

// autoconnectOf reads connection.autoconnect; NM defaults a missing key to
// true.
func autoconnectOf(settings domain.ConnectionSettings) bool {
	if b, ok := settings["connection"]["autoconnect"].(bool); ok {
		return b
	}
	return true
}

// bucketFor names the candidate set a profile competes in: NM races
// autoconnect candidates per device, so wifi and wired never contend.
func bucketFor(nmType string) string {
	switch nmType {
	case "802-11-wireless":
		return "wifi"
	case "802-3-ethernet":
		return "wired"
	}
	return nmType
}

// sortPickOrder orders rows the way NM picks them: bucket by bucket,
// priority desc, most recently used first among equals.
func sortPickOrder(rows []row) {
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].bucket != rows[j].bucket {
			return bucketRank(rows[i].bucket) < bucketRank(rows[j].bucket)
		}
		if rows[i].priority != rows[j].priority {
			return rows[i].priority > rows[j].priority
		}
		return rows[i].conn.LastUsedUnix > rows[j].conn.LastUsedUnix
	})
}

// bucketRank keeps the physical candidate sets first and everything else in
// a stable alphabetical tail.
func bucketRank(bucket string) string {
	switch bucket {
	case "wifi":
		return "0"
	case "wired":
		return "1"
	}
	return "2" + bucket
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case loadedMsg:
		// A reload triggered by backend churn must never clobber a pending
		// reorder; the user's edit wins until saved or discarded.
		if m.Dirty() {
			return m, nil
		}
		m.on = msg.on
		m.off = msg.off
		// The pending slices are mutated in place; the snapshots need their
		// own backing arrays.
		m.loadedOn = slices.Clone(msg.on)
		m.loadedOff = slices.Clone(msg.off)
		m.err = msg.err
		if total := len(m.on) + len(m.off); m.cursor >= total {
			m.cursor = max(total-1, 0)
		}
		return m, nil
	case savedMsg:
		m.status = msg.status
		// Adopt the pending state as the baseline so the reload of the just
		// written settings lands on a clean list.
		m.loadedOn = slices.Clone(m.on)
		m.loadedOff = slices.Clone(m.off)
		return m, m.load
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
	if m.confirmDiscard {
		return m.handleDiscardKey(msg)
	}
	if (msg.String() == "esc" || msg.String() == "q") && m.Dirty() {
		m.confirmDiscard = true
		return m, nil
	}
	total := len(m.on) + len(m.off)
	switch {
	case key.Matches(msg, m.keys.MoveDown):
		if !m.canModify() {
			return m.denyModify()
		}
		m.moveSelected(1)
	case key.Matches(msg, m.keys.MoveUp):
		if !m.canModify() {
			return m.denyModify()
		}
		m.moveSelected(-1)
	case key.Matches(msg, m.keys.Toggle):
		if !m.canModify() {
			return m.denyModify()
		}
		m.toggleSelected()
	case key.Matches(msg, m.keys.Save):
		if !m.canModify() {
			return m.denyModify()
		}
		return m.save()
	case key.Matches(msg, m.keys.Down):
		if m.cursor < total-1 {
			m.cursor++
		}
	case key.Matches(msg, m.keys.Up):
		if m.cursor > 0 {
			m.cursor--
		}
	case key.Matches(msg, m.keys.Top):
		m.cursor = 0
	case key.Matches(msg, m.keys.Bottom):
		if total > 0 {
			m.cursor = total - 1
		}
	}
	return m, nil
}

func (m Model) denyModify() (Model, tea.Cmd) {
	m.status = "🔒 not permitted (polkit)"
	return m, nil
}

func (m Model) handleDiscardKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	m.confirmDiscard = false
	switch msg.String() {
	case "s":
		return m.save()
	case "d":
		m.on = slices.Clone(m.loadedOn)
		m.off = slices.Clone(m.loadedOff)
	}
	return m, nil // any other key stays
}

// pendingWrite is one profile whose stored autoconnect data no longer
// matches the pending state.
type pendingWrite struct {
	name     string
	id       string
	settings domain.ConnectionSettings
}

// save assigns spaced descending priorities per section — step 10 ending at
// 0, so the top row wins and there is room to squeeze rows in between later
// — and writes only the profiles whose stored priority or autoconnect flag
// actually changed, passing every other settings key through verbatim.
func (m Model) save() (Model, tea.Cmd) {
	var writes []pendingWrite
	for start := 0; start < len(m.on); {
		end := start
		for end < len(m.on) && m.on[end].bucket == m.on[start].bucket {
			end++
		}
		for i, r := range m.on[start:end] {
			want := 10 * (end - start - 1 - i)
			if want == r.priority && r.auto {
				continue
			}
			settings := deepCopy(r.settings)
			if want != r.priority {
				set(settings, "connection", "autoconnect-priority", want)
			}
			if !r.auto {
				set(settings, "connection", "autoconnect", true)
			}
			writes = append(writes, pendingWrite{name: r.conn.Name, id: r.conn.ID, settings: settings})
		}
		start = end
	}
	for _, r := range m.off {
		if !r.auto {
			continue
		}
		settings := deepCopy(r.settings)
		set(settings, "connection", "autoconnect", false)
		writes = append(writes, pendingWrite{name: r.conn.Name, id: r.conn.ID, settings: settings})
	}
	mut := m.backend
	return m, func() tea.Msg {
		for _, w := range writes {
			if err := mut.UpdateSettings(w.id, w.settings); err != nil {
				return savedMsg{status: fmt.Sprintf("✗ save %s: %v", w.name, err)}
			}
		}
		return savedMsg{status: fmt.Sprintf("✓ autoconnect order saved (%d profiles updated)", len(writes))}
	}
}

// deepCopy clones the two-level settings map — the pass-through guarantee:
// everything the save doesn't touch is written back verbatim.
func deepCopy(settings domain.ConnectionSettings) domain.ConnectionSettings {
	copied := domain.ConnectionSettings{}
	for section, values := range settings {
		copied[section] = map[string]any{}
		for key, value := range values {
			copied[section][key] = value
		}
	}
	return copied
}

func set(settings domain.ConnectionSettings, section, key string, value any) {
	if settings[section] == nil {
		settings[section] = map[string]any{}
	}
	settings[section][key] = value
}

// toggleSelected flips the selected profile's pending autoconnect state,
// moving it between the pick order and the off section.
func (m *Model) toggleSelected() {
	if m.cursor < len(m.on) {
		r := m.on[m.cursor]
		m.on = slices.Delete(m.on, m.cursor, m.cursor+1)
		m.off = append(m.off, r)
		return
	}
	i := m.cursor - len(m.on)
	if i >= len(m.off) {
		return
	}
	r := m.off[i]
	m.off = slices.Delete(m.off, i, i+1)
	m.insertIntoBucket(r)
}

// insertIntoBucket rejoins a re-enabled profile to the pick order: at the
// end of its section, or where its section belongs if it is the first row.
func (m *Model) insertIntoBucket(r row) {
	pos := -1
	for i, existing := range m.on {
		if existing.bucket == r.bucket {
			pos = i + 1
		}
	}
	if pos == -1 {
		pos = len(m.on)
		for i, existing := range m.on {
			if bucketRank(existing.bucket) > bucketRank(r.bucket) {
				pos = i
				break
			}
		}
	}
	m.on = slices.Insert(m.on, pos, r)
	m.cursor = pos
}

// moveSelected shifts the selected row one step in the pending pick order,
// never across its section boundary and never out of the on list.
func (m *Model) moveSelected(delta int) {
	i, j := m.cursor, m.cursor+delta
	if i >= len(m.on) || j < 0 || j >= len(m.on) || m.on[i].bucket != m.on[j].bucket {
		return
	}
	m.on[i], m.on[j] = m.on[j], m.on[i]
	m.cursor = j
}

// Dirty reports whether the pending order differs from the backend's; the
// off list rides along because membership changes reshape the on order too.
func (m Model) Dirty() bool {
	return !slices.Equal(orderIDs(m.on), orderIDs(m.loadedOn))
}

func orderIDs(rows []row) []string {
	ids := make([]string, len(rows))
	for i, r := range rows {
		ids[i] = r.conn.ID
	}
	return ids
}

// Status is the screen's line for the app's status bar.
func (m Model) Status() string {
	return m.status
}

// CapturesInput tells the root model to route every key here while the
// save/discard prompt is open.
func (m Model) CapturesInput() bool {
	return m.confirmDiscard
}

// SetPermissions caches the startup polkit query result.
func (m Model) SetPermissions(perms domain.Permissions) Model {
	m.perms = perms
	return m
}

// canModify defaults to allowed while the permission is unknown; polkit
// only locks the actions when it explicitly denies them.
func (m Model) canModify() bool {
	allowed, known := m.perms[domain.PermModifySystem]
	return !known || allowed
}

func lastUsed(c domain.Connection) string {
	if c.LastUsedUnix == 0 {
		return "never"
	}
	return time.Unix(c.LastUsedUnix, 0).UTC().Format("2006-01-02")
}

// nameWidth is the flexible NAME column: fixed columns and gaps take the
// rest, over-long names are trimmed.
func (m Model) nameWidth() int {
	return style.FlexCell(m.width, 30, 20)
}

// buckets lists the distinct candidate sets among the pending on rows, in
// row order.
func (m Model) buckets() []string {
	var buckets []string
	for _, r := range m.on {
		if len(buckets) == 0 || buckets[len(buckets)-1] != r.bucket {
			buckets = append(buckets, r.bucket)
		}
	}
	return buckets
}

// allPrioritiesEqual means the shown order is pure recency — nothing is
// pinned, so NM would follow whatever gets used next.
func (m Model) allPrioritiesEqual() bool {
	for _, r := range m.on {
		if r.priority != m.on[0].priority {
			return false
		}
	}
	return true
}

func (m Model) renderRow(r row, order int, selected bool) string {
	cursor := " "
	if selected {
		cursor = "▸"
	}
	priority := fmt.Sprintf("%4d", r.priority)
	if !selected {
		priority = style.Faint.Render(priority)
	}
	line := fmt.Sprintf("%s %2d  %s %s  %s",
		cursor, order, style.Cell(r.conn.Name, m.nameWidth()), priority, lastUsed(r.conn))
	if selected {
		return style.SelectedRow(line, m.width)
	}
	return line
}

func (m Model) renderOffRow(r row, selected bool) string {
	cursor := " "
	if selected {
		cursor = "▸"
	}
	line := fmt.Sprintf("%s     %s", cursor, style.Cell(r.conn.Name, m.nameWidth()))
	if selected {
		return style.SelectedRow(line, m.width)
	}
	return style.Faint.Render(line)
}

func (m Model) View() string {
	if m.err != nil {
		return style.NMNotRunningNotice + "\n"
	}
	lines := []string{
		style.Faint.Render(fmt.Sprintf("  %2s  %s %4s  %s",
			"#", style.Cell("NAME", m.nameWidth()), "PRI", "LAST USED")),
	}
	if !m.canModify() {
		lines = append(lines, style.Faint.Render("🔒 reorder · toggle · save — not permitted (polkit)"))
	}
	if len(m.on) > 0 && m.allPrioritiesEqual() {
		lines = append(lines, style.Faint.Render("order is most-recently-used — reorder to pin it"))
	}
	focus := 0
	sectioned := len(m.buckets()) > 1
	order, index := 0, 0
	previousBucket := ""
	for _, r := range m.on {
		if r.bucket != previousBucket {
			previousBucket = r.bucket
			order = 0
			if sectioned {
				lines = append(lines, style.Faint.Render("─ "+r.bucket+" ─"))
			}
		}
		order++
		if index == m.cursor {
			focus = len(lines)
		}
		lines = append(lines, m.renderRow(r, order, index == m.cursor))
		index++
	}
	if len(m.off) > 0 {
		lines = append(lines, style.Faint.Render("─ autoconnect off ─"))
		for _, r := range m.off {
			if index == m.cursor {
				focus = len(lines)
			}
			lines = append(lines, m.renderOffRow(r, index == m.cursor))
			index++
		}
	}
	if m.confirmDiscard {
		lines = append(lines, "", "Unsaved changes — s save · d discard · any other key stays")
	} else if m.Dirty() {
		lines = append(lines, "", "● unsaved order — s save")
	}
	return style.FitScrolled(lines, m.width, m.height, focus)
}
