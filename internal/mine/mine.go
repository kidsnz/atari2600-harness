// Package mine dynamically discovers likely invariants from a driven run
// (Daikon-lite): it observes fields each frame and emits candidate invariants
// (constant / bounded range / monotonic) as a spec draft. Lowers the cost of
// writing the first scenario. docs/testing-playbook.md. Source: Ernst et al.,
// the Daikon system, SCP 2007.
package mine

import (
	"fmt"
	"math"
	"math/rand"
	"sort"

	"github.com/kidsnz/atari2600-harness/internal/emu"
	"github.com/kidsnz/atari2600-harness/internal/scenario"
)

// Candidate は発見された尤もらしい不変条件。
type Candidate struct {
	Field string
	Kind  string // "const" | "range" | "monotonic-up" | "monotonic-down"
	Value int64  // const
	Lo    int64  // range
	Hi    int64
}

// JSON は scenario の invariants/monotonic 断片として貼れる文字列を返す。
func (c Candidate) JSON() string {
	switch c.Kind {
	case "const":
		return fmt.Sprintf(`{"field": "%s", "op": "==", "value": %d}`, c.Field, c.Value)
	case "range":
		return fmt.Sprintf(`{"field": "%s", "op": "in", "lo": %d, "hi": %d}`, c.Field, c.Lo, c.Hi)
	case "monotonic-up":
		return fmt.Sprintf(`{"field": "%s", "direction": "up"}`, c.Field)
	case "monotonic-down":
		return fmt.Sprintf(`{"field": "%s", "direction": "down"}`, c.Field)
	}
	return ""
}

type stat struct {
	min, max, prev int64
	seen           bool
	monoUp         bool // 非減少
	monoDown       bool // 非増加
	changed        bool
}

// DefaultRAMFields は RAM 0x80..0xFF の全バイト（Daikon が全変数を観測するのに倣う）。
func DefaultRAMFields() []string {
	fs := make([]string, 0, 128)
	for a := 0x80; a <= 0xFF; a++ {
		fs = append(fs, fmt.Sprintf("ram.0x%02X", a))
	}
	return fs
}

// Mine は romPath を frames フレーム走らせ（actions があれば seed 付き乱数入力で駆動）、
// fields を毎フレーム観測して候補不変条件を返す。fields が空なら RAM 全域。
func Mine(romPath string, frames int, seed int64, actions []string, player int, fields []string) ([]Candidate, error) {
	if len(fields) == 0 {
		fields = DefaultRAMFields()
	}
	e, err := emu.New("NTSC")
	if err != nil {
		return nil, err
	}
	if err := e.LoadROM(romPath); err != nil {
		return nil, err
	}
	if err := e.RunFrames(2); err != nil { // warmup
		return nil, err
	}
	rng := rand.New(rand.NewSource(seed))
	st := make(map[string]*stat, len(fields))
	for _, f := range fields {
		st[f] = &stat{min: math.MaxInt64, max: math.MinInt64, monoUp: true, monoDown: true}
	}

	for i := 0; i < frames; i++ {
		if len(actions) > 0 {
			act := actions[rng.Intn(len(actions))]
			if err := e.SetInput(player, act, rng.Intn(2) == 1); err != nil {
				return nil, err
			}
		}
		if err := e.RunFrames(1); err != nil {
			return nil, err
		}
		for _, f := range fields {
			v, err := scenario.ResolveField(e, f)
			if err != nil {
				return nil, fmt.Errorf("field %q: %w", f, err)
			}
			s := st[f]
			if v < s.min {
				s.min = v
			}
			if v > s.max {
				s.max = v
			}
			if s.seen {
				if v != s.prev {
					s.changed = true
				}
				if v < s.prev {
					s.monoUp = false
				}
				if v > s.prev {
					s.monoDown = false
				}
			}
			s.prev = v
			s.seen = true
		}
	}

	// 候補化。const > monotonic > range の優先で各 field に最も強い1つを採る。
	out := make([]Candidate, 0, len(fields))
	for _, f := range fields {
		s := st[f]
		switch {
		case !s.changed: // 定数
			out = append(out, Candidate{Field: f, Kind: "const", Value: s.min})
		case s.monoUp:
			out = append(out, Candidate{Field: f, Kind: "monotonic-up", Lo: s.min, Hi: s.max})
		case s.monoDown:
			out = append(out, Candidate{Field: f, Kind: "monotonic-down", Lo: s.min, Hi: s.max})
		default: // 単に範囲が分かる
			out = append(out, Candidate{Field: f, Kind: "range", Lo: s.min, Hi: s.max})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Field < out[j].Field })
	return out, nil
}
