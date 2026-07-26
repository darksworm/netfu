package nm

import (
	"sync"
	"time"

	gonm "github.com/Wifx/gonetworkmanager/v3"
	"github.com/godbus/dbus/v5"

	"github.com/ilmars/netfu/internal/domain"
)

// strengthGate coalesces AP strength events to at most one per second per AP,
// measured from the last event it let through.
type strengthGate struct {
	lastEmitted map[string]time.Time
}

func newStrengthGate() *strengthGate {
	return &strengthGate{lastEmitted: map[string]time.Time{}}
}

func (g *strengthGate) allow(apKey string, now time.Time) bool {
	last, seen := g.lastEmitted[apKey]
	if seen && now.Sub(last) < time.Second {
		return false
	}
	g.lastEmitted[apKey] = now
	return true
}

// eventBuffer is a FIFO that, when at capacity, evicts the oldest strength
// event to make room. State events are never dropped: if the buffer is full
// of them it grows past capacity instead.
type eventBuffer struct {
	capacity int
	events   []domain.Event
}

func newEventBuffer(capacity int) *eventBuffer {
	return &eventBuffer{capacity: capacity}
}

func (b *eventBuffer) push(e domain.Event) {
	if len(b.events) >= b.capacity {
		for i, queued := range b.events {
			if queued.Kind == domain.EventAPStrength {
				b.events = append(b.events[:i], b.events[i+1:]...)
				break
			}
		}
	}
	b.events = append(b.events, e)
}

func (b *eventBuffer) pop() (domain.Event, bool) {
	if len(b.events) == 0 {
		return domain.Event{}, false
	}
	e := b.events[0]
	b.events = b.events[1:]
	return e, true
}

const (
	nmObjectPath        = "/org/freedesktop/NetworkManager"
	nmInterface         = "org.freedesktop.NetworkManager"
	deviceInterface     = nmInterface + ".Device"
	activeConnInterface = nmInterface + ".Connection.Active"
	apInterface         = nmInterface + ".AccessPoint"
	wirelessInterface   = nmInterface + ".Device.Wireless"
	propertiesInterface = "org.freedesktop.DBus.Properties"
)

// watcher fans every NM D-Bus signal the app cares about into one
// domain.Event channel, applying the coalescing and drop policies above.
type watcher struct {
	conn   *dbus.Conn
	out    chan domain.Event
	gate   *strengthGate
	buffer *eventBuffer

	mu       sync.Mutex
	nonEmpty chan struct{}

	deviceNames func(path dbus.ObjectPath) string
}

func newWatcher(conn *dbus.Conn, deviceNames func(dbus.ObjectPath) string) (*watcher, error) {
	w := &watcher{
		conn:        conn,
		out:         make(chan domain.Event, 64),
		gate:        newStrengthGate(),
		buffer:      newEventBuffer(64),
		nonEmpty:    make(chan struct{}, 1),
		deviceNames: deviceNames,
	}

	// One raw match per signal family, all paths at once, so devices and
	// active connections that appear after startup are covered too.
	matches := [][]dbus.MatchOption{
		{dbus.WithMatchInterface(deviceInterface), dbus.WithMatchMember("StateChanged")},
		{dbus.WithMatchInterface(activeConnInterface), dbus.WithMatchMember("StateChanged")},
		{dbus.WithMatchInterface(nmInterface), dbus.WithMatchMember("StateChanged"), dbus.WithMatchObjectPath(nmObjectPath)},
		{dbus.WithMatchInterface(propertiesInterface), dbus.WithMatchMember("PropertiesChanged"), dbus.WithMatchPathNamespace(nmObjectPath)},
	}
	for _, m := range matches {
		if err := conn.AddMatchSignal(m...); err != nil {
			return nil, err
		}
	}

	signals := make(chan *dbus.Signal, 128)
	conn.Signal(signals)
	go w.dispatch(signals)
	go w.pump()
	return w, nil
}

func (w *watcher) Events() <-chan domain.Event {
	return w.out
}

func (w *watcher) dispatch(signals <-chan *dbus.Signal) {
	for s := range signals {
		e, ok := w.eventFor(s)
		if !ok {
			continue
		}
		if e.Kind == domain.EventAPStrength && !w.gate.allow(string(s.Path), time.Now()) {
			continue
		}
		w.mu.Lock()
		w.buffer.push(e)
		w.mu.Unlock()
		select {
		case w.nonEmpty <- struct{}{}:
		default:
		}
	}
	close(w.out)
}

// pump moves buffered events to the out channel; blocking here (a slow TUI)
// backs pressure into the buffer where the drop policy applies, instead of
// blocking the dbus dispatch goroutine.
func (w *watcher) pump() {
	for range w.nonEmpty {
		for {
			w.mu.Lock()
			e, ok := w.buffer.pop()
			w.mu.Unlock()
			if !ok {
				break
			}
			w.out <- e
		}
	}
}

func (w *watcher) eventFor(s *dbus.Signal) (domain.Event, bool) {
	switch s.Name {
	case deviceInterface + ".StateChanged":
		if len(s.Body) < 2 {
			return domain.Event{}, false
		}
		reason, _ := s.Body[1].(uint32)
		return domain.Event{
			Kind:       domain.EventDeviceChanged,
			DeviceName: w.deviceNames(s.Path),
			Reason:     reasonFromNM(gonm.NmDeviceStateReason(reason)),
		}, true
	case activeConnInterface + ".StateChanged":
		return domain.Event{Kind: domain.EventConnectionChanged}, true
	case nmInterface + ".StateChanged":
		return domain.Event{Kind: domain.EventNMStateChanged}, true
	case propertiesInterface + ".PropertiesChanged":
		return w.eventForPropertiesChanged(s)
	default:
		return domain.Event{}, false
	}
}

func (w *watcher) eventForPropertiesChanged(s *dbus.Signal) (domain.Event, bool) {
	if len(s.Body) < 2 {
		return domain.Event{}, false
	}
	iface, _ := s.Body[0].(string)
	changed, _ := s.Body[1].(map[string]dbus.Variant)
	switch iface {
	case apInterface:
		if _, ok := changed["Strength"]; ok {
			return domain.Event{Kind: domain.EventAPStrength}, true
		}
	case wirelessInterface:
		// A LastScan bump means fresh scan results are ready to re-read.
		if _, ok := changed["LastScan"]; ok {
			return domain.Event{
				Kind:       domain.EventAPStrength,
				DeviceName: w.deviceNames(s.Path),
				Reason:     "last-scan",
			}, true
		}
	}
	return domain.Event{}, false
}
