package session

import (
	"encoding/json"
	"fmt"
)

// Player walks recorded events and serves them back.
// It is the substrate for replay mode: live LLM/tool adapters are replaced
// with adapters that call Next and return recorded bytes.
type Player struct {
	events []Event
	cursor int
}

func NewPlayer(events []Event) *Player {
	return &Player{events: events}
}

func (p *Player) Len() int    { return len(p.events) }
func (p *Player) Cursor() int { return p.cursor }
func (p *Player) Done() bool  { return p.cursor >= len(p.events) }

func (p *Player) Peek() (Event, bool) {
	if p.Done() {
		return Event{}, false
	}
	return p.events[p.cursor], true
}

// Next returns the event at the cursor and advances. Returns an error if
// the event's kind does not match expected.
func (p *Player) Next(expected EventKind) (Event, error) {
	if p.Done() {
		return Event{}, fmt.Errorf("replay exhausted at step %d, expected %s", p.cursor, expected)
	}
	ev := p.events[p.cursor]
	if ev.Kind != expected {
		return Event{}, fmt.Errorf("replay mismatch at step %d: expected %s, got %s", p.cursor, expected, ev.Kind)
	}
	p.cursor++
	return ev, nil
}

// NextAs decodes the JSON payload of the next event into v.
func (p *Player) NextAs(expected EventKind, v any) error {
	ev, err := p.Next(expected)
	if err != nil {
		return err
	}
	return json.Unmarshal(ev.Payload, v)
}
