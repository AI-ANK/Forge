package rng

import (
	"encoding/binary"
	"encoding/hex"
	"hash/fnv"
	"math/rand/v2"
	"time"
)

// Source produces deterministic clock ticks, UUIDs, and per-step seeds
// from a single session seed. Every call is recorded by the agent loop
// so replay produces byte-identical streams.
type Source struct {
	seed   int64
	rand   *rand.Rand
	t0     time.Time
	ticks  int64
	uuidN  uint64
}

func New(seed int64, t0 time.Time) *Source {
	var s1 [32]byte
	binary.LittleEndian.PutUint64(s1[:8], uint64(seed))
	binary.LittleEndian.PutUint64(s1[8:16], uint64(seed)^0x9E3779B97F4A7C15)
	r := rand.New(rand.NewChaCha8(s1))
	return &Source{seed: seed, rand: r, t0: t0}
}

func (s *Source) Seed() int64 { return s.seed }

// Now advances a synthetic clock by 1ms per call so every Now() is unique
// and reproducible.
func (s *Source) Now() time.Time {
	s.ticks++
	return s.t0.Add(time.Duration(s.ticks) * time.Millisecond)
}

// UUID returns a short, deterministic hex ID.
func (s *Source) UUID() string {
	s.uuidN++
	var buf [16]byte
	binary.LittleEndian.PutUint64(buf[:8], uint64(s.seed))
	binary.LittleEndian.PutUint64(buf[8:], s.uuidN)
	return hex.EncodeToString(buf[:])
}

// StepSeed returns a per-step seed derived from the session seed.
// Used for Ollama's options.seed so each LLM call is independently seeded.
func (s *Source) StepSeed(step int) int64 {
	h := fnv.New64a()
	var buf [16]byte
	binary.LittleEndian.PutUint64(buf[:8], uint64(s.seed))
	binary.LittleEndian.PutUint64(buf[8:], uint64(step))
	_, _ = h.Write(buf[:])
	return int64(h.Sum64() & 0x7FFFFFFFFFFFFFFF)
}

// SessionID returns a short ID derived from the seed.
func SessionIDFromSeed(seed int64) string {
	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], uint64(seed))
	return hex.EncodeToString(buf[:])
}
