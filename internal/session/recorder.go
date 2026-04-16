package session

import (
	"encoding/json"
	"time"
)

// Recorder appends events to a session in order.
type Recorder struct {
	store     *Store
	sessionID string
	step      int
	now       func() time.Time
}

func NewRecorder(store *Store, sessionID string) (*Recorder, error) {
	step, err := store.NextStep(sessionID)
	if err != nil {
		return nil, err
	}
	return &Recorder{store: store, sessionID: sessionID, step: step, now: time.Now}, nil
}

func (r *Recorder) Step() int { return r.step }

// Record serializes payload as JSON and appends.
func (r *Recorder) Record(kind EventKind, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	ev := Event{
		Step:    r.step,
		Kind:    kind,
		Payload: data,
		TS:      r.now().UTC(),
	}
	if err := r.store.AppendEvent(r.sessionID, ev); err != nil {
		return err
	}
	r.step++
	return nil
}

// RecordRaw appends raw bytes (e.g. captured LLM response body).
func (r *Recorder) RecordRaw(kind EventKind, payload []byte) error {
	ev := Event{
		Step:    r.step,
		Kind:    kind,
		Payload: payload,
		TS:      r.now().UTC(),
	}
	if err := r.store.AppendEvent(r.sessionID, ev); err != nil {
		return err
	}
	r.step++
	return nil
}
