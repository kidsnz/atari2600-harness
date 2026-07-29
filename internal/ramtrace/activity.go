package ramtrace

import (
	"fmt"
	"sort"

	"github.com/kidsnz/atari2600-harness/internal/behavmatch"
	"github.com/kidsnz/atari2600-harness/internal/emu"
)

// Activity is the descriptive statistics of one RAM byte across one or more
// recorded traces. It deliberately fits nothing and concludes nothing: it reports
// what the byte did. Fitting a transition family on top of these numbers is a
// later, separate step, and keeping the two apart is what stops a fitted story
// from being mistaken for an observation.
type Activity struct {
	Addr string `json:"addr"`

	Distinct      int  `json:"distinct"`       // how many different values it took
	Min           int  `json:"min"`            // lowest value observed
	Max           int  `json:"max"`            // highest value observed
	FramesChanged int  `json:"frames_changed"` // frames where it differed from the previous
	FirstChange   int  `json:"first_change"`   // frame of the first change, -1 if never
	Constant      bool `json:"constant"`

	// Deltas is the set of per-frame changes, ascending, when there are few of
	// them — a byte that only ever moves by +1 or by 0 is a very different animal
	// from one that jumps arbitrarily, and that shows up here before any model.
	Deltas    []int `json:"deltas,omitempty"`
	ManyDelta bool  `json:"many_delta,omitempty"` // too many distinct deltas to list

	// Values is the value set when small — the signature of a state/flag byte.
	Values []int `json:"values,omitempty"`

	// InputSensitive lists the input states under which this byte ever changed.
	InputSensitive []string `json:"input_sensitive,omitempty"`

	// StackReached marks a byte at or above the lowest observed stack pointer.
	StackReached bool `json:"stack_reached,omitempty"`
}

const maxSetSize = 12 // above this a set is summarised rather than listed

// ActivityReport is the per-byte map plus the facts needed to read it.
type ActivityReport struct {
	Provenance Provenance `json:"provenance"`
	Scenarios  []string   `json:"scenarios"`
	Frames     int        `json:"frames_total"`

	StackLow       string   `json:"stack_low"`       // lowest SP seen inside RAM, or "(none)"
	SPMin          string   `json:"sp_min_observed"` // lowest SP seen at all, wherever it pointed
	SPMax          string   `json:"sp_max_observed"` // highest SP seen at all
	CollisionsSeen []string `json:"collisions_seen"` // collision pairs that occurred, ever

	LiveCount int        `json:"live_count"` // bytes that changed at least once
	DeadCount int        `json:"dead_count"` // bytes that never changed
	Bytes     []Activity `json:"bytes"`
}

// Analyse computes the per-byte activity across traces recorded from the same ROM.
func Analyse(prov Provenance, traces map[string]*behavmatch.Trace) *ActivityReport {
	rep := &ActivityReport{Provenance: prov, StackLow: "(none)"}
	for name := range traces {
		rep.Scenarios = append(rep.Scenarios, name)
	}
	sort.Strings(rep.Scenarios)

	var (
		values  [emu.RAMSize]map[uint8]bool
		deltas  [emu.RAMSize]map[int]bool
		changed [emu.RAMSize]int
		first   [emu.RAMSize]int
		inputs  [emu.RAMSize]map[string]bool
	)
	for i := range values {
		values[i] = map[uint8]bool{}
		deltas[i] = map[int]bool{}
		inputs[i] = map[string]bool{}
		first[i] = -1
	}
	collSeen := map[string]bool{}
	stackLow, stackOK := uint16(0xFF), false
	spMin, spMax, spSeen := uint16(0xFF), uint16(0x00), false

	for _, name := range rep.Scenarios {
		tr := traces[name]
		if tr == nil {
			continue
		}
		rep.Frames += len(tr.Samples)
		for f, s := range tr.Samples {
			for k := range collisionMap(s.Coll) {
				collSeen[k] = true
			}
			lo, hi := uint16(s.SPLow), uint16(s.SPHigh)
			if !spSeen {
				spMin, spMax, spSeen = lo, hi, true
			}
			if lo < spMin {
				spMin = lo
			}
			if hi > spMax {
				spMax = hi
			}
			if lo >= emu.RAMBase {
				stackOK = true
				if lo < stackLow {
					stackLow = lo
				}
			}
			for i := 0; i < emu.RAMSize; i++ {
				v := s.AllRAM[i]
				values[i][v] = true
				if f == 0 {
					continue
				}
				p := tr.Samples[f-1].AllRAM[i]
				if v == p {
					continue
				}
				changed[i]++
				if first[i] < 0 {
					first[i] = f
				}
				deltas[i][int(v)-int(p)] = true
				inputs[i][s.Inputs.Key()] = true
			}
		}
	}
	if stackOK {
		rep.StackLow = fmt.Sprintf("$%02X", stackLow)
	}
	rep.SPMin, rep.SPMax = "(none)", "(none)"
	if spSeen {
		rep.SPMin, rep.SPMax = fmt.Sprintf("$%02X", spMin), fmt.Sprintf("$%02X", spMax)
	}
	for k := range collSeen {
		rep.CollisionsSeen = append(rep.CollisionsSeen, k)
	}
	sort.Strings(rep.CollisionsSeen)
	if rep.CollisionsSeen == nil {
		rep.CollisionsSeen = []string{}
	}

	for i := 0; i < emu.RAMSize; i++ {
		addr := uint16(emu.RAMBase + i)
		a := Activity{
			Addr:          fmt.Sprintf("$%02X", addr),
			Distinct:      len(values[i]),
			FramesChanged: changed[i],
			FirstChange:   first[i],
			Constant:      len(values[i]) <= 1,
			StackReached:  stackOK && addr >= stackLow,
		}
		mn, mx := 256, -1
		for v := range values[i] {
			if int(v) < mn {
				mn = int(v)
			}
			if int(v) > mx {
				mx = int(v)
			}
		}
		a.Min, a.Max = mn, mx
		if len(values[i]) <= maxSetSize {
			for v := range values[i] {
				a.Values = append(a.Values, int(v))
			}
			sort.Ints(a.Values)
		}
		if len(deltas[i]) <= maxSetSize {
			for d := range deltas[i] {
				a.Deltas = append(a.Deltas, d)
			}
			sort.Ints(a.Deltas)
		} else {
			a.ManyDelta = true
		}
		for k := range inputs[i] {
			a.InputSensitive = append(a.InputSensitive, k)
		}
		sort.Strings(a.InputSensitive)
		if a.Constant {
			rep.DeadCount++
		} else {
			rep.LiveCount++
		}
		rep.Bytes = append(rep.Bytes, a)
	}
	return rep
}

// Text renders the report as a table for reading in a terminal.
func (r *ActivityReport) Text() string {
	out := fmt.Sprintf("RAM activity — %s (md5 %s)\n", r.Provenance.ROM, r.Provenance.ROMMD5)
	out += fmt.Sprintf("scenarios: %v   frames sampled: %d\n", r.Scenarios, r.Frames)
	out += fmt.Sprintf("live bytes: %d / 128    never changed: %d\n", r.LiveCount, r.DeadCount)
	out += fmt.Sprintf("stack low-water inside RAM: %s   (SP observed range %s..%s)\n",
		r.StackLow, r.SPMin, r.SPMax)
	if r.StackLow == "(none)" && r.SPMin != "(none)" {
		out += "  NOTE: SP never pointed inside RAM. On the 2600 that means the program has aimed\n" +
			"  the stack pointer somewhere else — page 1 below $80 mirrors the TIA registers, which\n" +
			"  is a deliberate technique (PHP/PHA as a register write), not a malfunction.\n"
	}
	if len(r.CollisionsSeen) == 0 {
		out += "collisions: NONE occurred in any sampled frame\n"
	} else {
		out += fmt.Sprintf("collisions that occurred: %v\n", r.CollisionsSeen)
	}
	out += "\n addr  distinct  chg  first  min  max  deltas                 values\n"
	for _, a := range r.Bytes {
		if a.Constant {
			continue
		}
		deltas := "(many)"
		if !a.ManyDelta {
			deltas = fmt.Sprint(a.Deltas)
		}
		vals := ""
		if a.Values != nil {
			vals = fmt.Sprint(a.Values)
		}
		mark := " "
		if a.StackReached {
			mark = "S"
		}
		out += fmt.Sprintf("%s%-5s %8d %4d %6d %4d %4d  %-21s  %s\n",
			mark, a.Addr, a.Distinct, a.FramesChanged, a.FirstChange, a.Min, a.Max, deltas, vals)
	}
	out += "\n(S = at or above the lowest observed stack pointer)\n"
	var dead []string
	for _, a := range r.Bytes {
		if a.Constant {
			dead = append(dead, a.Addr)
		}
	}
	out += fmt.Sprintf("never changed (%d): %v\n", len(dead), dead)
	return out
}
