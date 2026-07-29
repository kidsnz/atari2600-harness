package ramtrace

import (
	"fmt"
	"sort"
	"strings"

	"github.com/kidsnz/atari2600-harness/internal/behavmatch"
	"github.com/kidsnz/atari2600-harness/internal/emu"
)

// The arity probe.
//
// A behavioural-reproduction system that recovers one byte's update rule at a
// time only works if each byte's next value is decided by a SMALL number of
// things: itself, the input, and one or two companion bytes. If instead a byte
// depends on a dozen others, per-byte reconstruction stops being a small
// regression and becomes an intractable search, and the whole approach has to be
// re-cut. That premise is cheap to state and easy to believe, which is exactly
// why it should be measured before anything is built on top of it.
//
// The measurement is a determinism check, not a fit. For a candidate feature set
// F, collect every observed transition (features at frame f-1) -> (byte value at
// frame f). If two transitions agree on F but disagree on the resulting value,
// F cannot determine the byte and is too small. The smallest F with no such
// conflict is the byte's observed arity.
//
// Read the negative result carefully: "unresolved" does NOT prove the arity is
// high. It means no tried feature set explained the byte, and the cause may be a
// feature this probe cannot see at all — hardware state beyond the collision
// latches, sub-frame ordering, or a value that simply was not exercised enough.
// The probe reports which case it is in rather than guessing.

// feature codes
const (
	featInputBase = 1000 // the held input state
	featCollBase  = 2000 // + collision-pair index
)

var collNames = []string{
	"p0_p1", "m0_m1", "m0_p0", "m0_p1", "m1_p0", "m1_p1",
	"p0_pf", "p0_bl", "p1_pf", "p1_bl", "m0_pf", "m0_bl", "m1_pf", "m1_bl", "bl_pf",
}

func collBit(c emu.Collisions, i int) bool {
	switch collNames[i] {
	case "p0_p1":
		return c.P0P1
	case "m0_m1":
		return c.M0M1
	case "m0_p0":
		return c.M0P0
	case "m0_p1":
		return c.M0P1
	case "m1_p0":
		return c.M1P0
	case "m1_p1":
		return c.M1P1
	case "p0_pf":
		return c.P0PF
	case "p0_bl":
		return c.P0BL
	case "p1_pf":
		return c.P1PF
	case "p1_bl":
		return c.P1BL
	case "m0_pf":
		return c.M0PF
	case "m0_bl":
		return c.M0BL
	case "m1_pf":
		return c.M1PF
	case "m1_bl":
		return c.M1BL
	case "bl_pf":
		return c.BLPF
	}
	return false
}

func featureName(code int) string {
	switch {
	case code >= featCollBase:
		return "CX." + collNames[code-featCollBase]
	case code >= featInputBase:
		return "input"
	default:
		return fmt.Sprintf("$%02X", emu.RAMBase+code)
	}
}

// transition is one observed (previous state, input) -> (byte value) pair.
type transition struct {
	trace    int    // which recording it came from
	scenario string // its name, for reporting where a conflict is
	frame    int    // frame index within that recording
	prev     [emu.RAMSize]uint8
	coll     emu.Collisions
	input    string
	next     uint8
}

// SkipFrames drops this many frames from the START of every recording before
// transitions are gathered. Power-on initialisation is a bulk RAM clear, not a
// per-byte update rule, so folding it into the same model makes bytes look
// unexplainable for a reason that has nothing to do with gameplay. Separating the
// two is how "this byte needs a feature we cannot see" stays a meaningful claim.
var SkipFrames int

func gatherTransitions(traces map[string]*behavmatch.Trace, idx int) []transition {
	var out []transition
	names := make([]string, 0, len(traces))
	for n := range traces {
		names = append(names, n)
	}
	sort.Strings(names)
	for ti, n := range names {
		tr := traces[n]
		if tr == nil {
			continue
		}
		start := 1 + SkipFrames
		for f := start; f < len(tr.Samples); f++ {
			out = append(out, transition{
				trace:    ti,
				scenario: n,
				frame:    f,
				prev:     tr.Samples[f-1].AllRAM,
				coll:     tr.Samples[f-1].Coll,
				input:    tr.Samples[f].Inputs.Key(),
				next:     tr.Samples[f].AllRAM[idx],
			})
		}
	}
	return out
}

// frameIndexLike reports whether a RAM byte's value is distinct on every single
// transition within each recording — a free-running frame counter, for instance.
//
// Such a byte is a disaster for a naive arity search, and finding that out cost
// a failing test rather than an argument: a counter uniquely identifies the
// frame, so keying on it "explains" every other byte perfectly and the probe
// happily reports that the whole of RAM has arity 1. That is memorisation
// wearing a model's clothes — exactly the tape-recorder failure the reproduction
// system exists to avoid. Such features are not banned (a byte may genuinely
// depend on a frame counter) but they are tried last and the result is marked.
func frameIndexLike(ts []transition, code int) bool {
	if code >= featInputBase {
		return false
	}
	perTrace := map[int]map[uint8]bool{}
	counts := map[int]int{}
	for _, t := range ts {
		if perTrace[t.trace] == nil {
			perTrace[t.trace] = map[uint8]bool{}
		}
		perTrace[t.trace][t.prev[code]] = true
		counts[t.trace]++
	}
	for tid, vals := range perTrace {
		if counts[tid] < 4 {
			continue // too short for uniqueness to mean anything
		}
		// Uniqueness across the whole recording is the strict case, but a counter that
		// WRAPS is just as dangerous and does not satisfy it. Measured on Outlaw: $DA
		// takes 256 distinct values, changes on all 4266 transitions, and its only
		// deltas are +1 and -255 — a free-running frame counter that cycles. It failed
		// this test purely because the recording is longer than its period, and 27 of
		// the 35 "resolved" bytes were then explained by keying on it. A byte that
		// visits (almost) every value it can and changes every frame is a clock,
		// whether or not the trace outlives one revolution.
		if len(vals) == counts[tid] {
			continue // strictly unique: a counter by the original definition
		}
		if len(vals) >= 250 {
			continue // wraps, but visits nearly the whole byte range: still a clock
		}
		return false
	}
	return len(perTrace) > 0
}

// distinctKeys counts how many different feature keys the transitions produce.
// When it equals the number of transitions, every key was seen exactly once — the
// set may be consistent with the data while having no evidence of generalising at
// all, and the verdict says so rather than reading as a clean explanation.
func distinctKeys(ts []transition, feats []int) int {
	seen := make(map[string]bool, len(ts))
	for _, t := range ts {
		seen[keyOf(t, feats)] = true
	}
	return len(seen)
}

func keyOf(t transition, feats []int) string {
	var b strings.Builder
	for _, c := range feats {
		switch {
		case c >= featCollBase:
			if collBit(t.coll, c-featCollBase) {
				b.WriteByte('1')
			} else {
				b.WriteByte('0')
			}
		case c >= featInputBase:
			b.WriteString(t.input)
			b.WriteByte('\x00')
		default:
			b.WriteByte(t.prev[c])
		}
		b.WriteByte('\x01')
	}
	return b.String()
}

// conflicts counts transitions whose feature key is shared with a transition
// that produced a different value — the evidence that the feature set is too small.
func conflicts(ts []transition, feats []int) int {
	n, _ := conflictsWhere(ts, feats, 0)
	return n
}

// conflictsWhere additionally names up to maxNames of the conflicting transitions.
// A bare count says a feature set is too small; the LOCATIONS say why. When every
// conflict sits on the first frame of each recording it is a start-up transient,
// not a missing dependency — and those two look identical in a count.
func conflictsWhere(ts []transition, feats []int, maxNames int) (int, []string) {
	seen := make(map[string]uint8, len(ts))
	bad := map[string]bool{}
	for _, t := range ts {
		k := keyOf(t, feats)
		if v, ok := seen[k]; ok {
			if v != t.next {
				bad[k] = true
			}
			continue
		}
		seen[k] = t.next
	}
	if len(bad) == 0 {
		return 0, nil
	}
	n := 0
	var where []string
	for _, t := range ts {
		if !bad[keyOf(t, feats)] {
			continue
		}
		n++
		if len(where) < maxNames {
			where = append(where, fmt.Sprintf("%s:f%d", t.scenario, t.frame))
		}
	}
	return n, where
}

// ByteArity is one byte's result.
type ByteArity struct {
	Addr        string   `json:"addr"`
	Constant    bool     `json:"constant"`
	Resolved    bool     `json:"resolved"`
	Companions  int      `json:"companions"` // extra bytes/latches needed beyond self+input; -1 unresolved
	Features    []string `json:"features,omitempty"`
	NeedsInput  bool     `json:"needs_input"`
	Transitions int      `json:"transitions"`
	Method      string   `json:"method"`
	Residual    int      `json:"residual_conflicts"` // conflicts left by the best set tried

	// DistinctKeys is how many different feature keys the accepted set produced.
	DistinctKeys int `json:"distinct_keys,omitempty"`

	// Memorising marks a "resolution" in which every key was seen exactly once:
	// consistent with the data, but with no evidence that it generalises to a
	// state the scenarios never visited. It is a warning, not a verdict.
	Memorising bool `json:"memorising,omitempty"`

	// UsesFrameIndex marks a resolution that leaned on a frame-counter-like byte.
	UsesFrameIndex bool `json:"uses_frame_index,omitempty"`

	// ConflictAt names where the best feature set was contradicted (scenario:frame).
	ConflictAt []string `json:"conflict_at,omitempty"`
}

// ArityReport is the whole-RAM result plus the summary that answers the premise.
type ArityReport struct {
	Provenance Provenance `json:"provenance"`
	Scenarios  []string   `json:"scenarios"`

	Live       int         `json:"live_bytes"`
	Resolved   int         `json:"resolved"`
	Unresolved int         `json:"unresolved"`
	Memorised  int         `json:"memorised"`           // resolved, but every key seen once
	Histogram  map[int]int `json:"companion_histogram"` // companions -> byte count

	// FrameIndexLike names the bytes whose value is unique on every transition of
	// every recording. Read any resolution that uses one with suspicion.
	FrameIndexLike []string `json:"frame_index_like"`

	Bytes []ByteArity `json:"bytes"`
}

// beamWidth caps the pair search: every single companion is tried exhaustively,
// then only the most promising singles are combined. Stated in the output as the
// method, because a bounded search that reports itself as exhaustive is a lie.
const beamWidth = 16

// Arity measures, for every RAM byte, the smallest feature set that determines
// its next value across the recorded traces.
func Arity(prov Provenance, traces map[string]*behavmatch.Trace) *ArityReport {
	rep := &ArityReport{Provenance: prov, Histogram: map[int]int{}}
	for n := range traces {
		rep.Scenarios = append(rep.Scenarios, n)
	}
	sort.Strings(rep.Scenarios)

	// Candidate companions: every RAM byte plus every collision latch.
	var candidates []int
	for i := 0; i < emu.RAMSize; i++ {
		candidates = append(candidates, i)
	}
	for i := range collNames {
		candidates = append(candidates, featCollBase+i)
	}

	frameIdx := map[int]bool{}

	for idx := 0; idx < emu.RAMSize; idx++ {
		addr := fmt.Sprintf("$%02X", emu.RAMBase+idx)
		ts := gatherTransitions(traces, idx)
		ba := ByteArity{Addr: addr, Transitions: len(ts), Companions: -1}

		allSame := true
		for _, t := range ts {
			if t.next != ts[0].next {
				allSame = false
				break
			}
		}
		if len(ts) == 0 || allSame {
			ba.Constant, ba.Resolved, ba.Companions, ba.Method = true, true, 0, "constant"
			rep.Bytes = append(rep.Bytes, ba)
			continue
		}
		rep.Live++
		if frameIndexLike(ts, idx) {
			frameIdx[idx] = true
		}

		// accept records a resolution together with the evidence for how much it
		// is worth: how many distinct keys it produced, whether every key was seen
		// only once, and whether it leaned on a frame-counter-like byte.
		accept := func(companions int, feats []int, needsInput bool, method string) {
			ba.Resolved, ba.Companions, ba.NeedsInput, ba.Method = true, companions, needsInput, method
			ba.Features = nil
			for _, f := range feats {
				ba.Features = append(ba.Features, featureName(f))
				if f != idx && frameIndexLike(ts, f) {
					ba.UsesFrameIndex = true
				}
			}
			ba.DistinctKeys = distinctKeys(ts, feats)
			ba.Memorising = ba.DistinctKeys >= len(ts)
			if frameIndexLike(ts, idx) {
				ba.UsesFrameIndex = true
			}
		}

		// Level 1: the byte's own previous value alone.
		self := []int{idx}
		if conflicts(ts, self) == 0 {
			accept(0, self, false, "self")
			rep.finish(&ba)
			rep.Bytes = append(rep.Bytes, ba)
			continue
		}

		// Level 2: self + input.
		selfIn := []int{idx, featInputBase}
		best := conflicts(ts, selfIn)
		if best == 0 {
			accept(0, selfIn, true, "self+input")
			rep.finish(&ba)
			rep.Bytes = append(rep.Bytes, ba)
			continue
		}

		// Level 3: self + input + one companion. Ordinary bytes are tried before
		// frame-counter-like ones, so a genuine dependency is preferred over the
		// counter that could key anything.
		type scored struct {
			c    int
			left int
			fidx bool
		}
		var ranked []scored
		found, foundFidx := -1, true
		for _, c := range candidates {
			if c == idx {
				continue
			}
			left := conflicts(ts, []int{idx, featInputBase, c})
			fi := frameIndexLike(ts, c)
			ranked = append(ranked, scored{c, left, fi})
			if left != 0 {
				continue
			}
			if found < 0 || (foundFidx && !fi) { // prefer a non-counter explanation
				found, foundFidx = c, fi
			}
		}
		if found >= 0 {
			accept(1, []int{idx, featInputBase, found}, true, "self+input+1 (exhaustive)")
			rep.finish(&ba)
			rep.Bytes = append(rep.Bytes, ba)
			continue
		}

		// Level 4: pairs, drawn from the singles that reduced conflicts the most.
		sort.Slice(ranked, func(i, j int) bool {
			if ranked[i].fidx != ranked[j].fidx {
				return !ranked[i].fidx // non-counter candidates first
			}
			if ranked[i].left != ranked[j].left {
				return ranked[i].left < ranked[j].left
			}
			return ranked[i].c < ranked[j].c
		})
		top := ranked
		if len(top) > beamWidth {
			top = top[:beamWidth]
		}
		for _, r := range ranked {
			if r.left < best {
				best = r.left
			}
		}
		for i := 0; i < len(top) && !ba.Resolved; i++ {
			for j := i + 1; j < len(top); j++ {
				feats := []int{idx, featInputBase, top[i].c, top[j].c}
				left := conflicts(ts, feats)
				if left < best {
					best = left
				}
				if left == 0 {
					accept(2, feats, true, fmt.Sprintf("self+input+2 (beam %d)", beamWidth))
					break
				}
			}
		}
		if !ba.Resolved {
			ba.Method = fmt.Sprintf("unresolved with <=2 companions (beam %d)", beamWidth)
			// Re-run the best single-companion set to report WHERE it broke.
			bestFeats := []int{idx, featInputBase}
			if len(ranked) > 0 {
				bestFeats = append(bestFeats, ranked[0].c)
			}
			ba.Residual, ba.ConflictAt = conflictsWhere(ts, bestFeats, 8)
			if ba.Residual == 0 || ba.Residual > best {
				ba.Residual = best
			}
		}
		rep.finish(&ba)
		rep.Bytes = append(rep.Bytes, ba)
	}

	for i := range frameIdx {
		rep.FrameIndexLike = append(rep.FrameIndexLike, featureName(i))
	}
	sort.Strings(rep.FrameIndexLike)
	if rep.FrameIndexLike == nil {
		rep.FrameIndexLike = []string{}
	}
	return rep
}

func (r *ArityReport) finish(ba *ByteArity) {
	if ba.Resolved {
		r.Resolved++
		r.Histogram[ba.Companions]++
		if ba.Memorising {
			r.Memorised++
		}
	} else {
		r.Unresolved++
	}
}

// Text renders the verdict on the premise.
func (r *ArityReport) Text() string {
	var b strings.Builder
	fmt.Fprintf(&b, "RAM arity — %s (md5 %s)\n", r.Provenance.ROM, r.Provenance.ROMMD5)
	fmt.Fprintf(&b, "scenarios: %v\n\n", r.Scenarios)
	fmt.Fprintf(&b, "live bytes: %d   resolved: %d   unresolved: %d   (of the resolved, %d only ever saw each key once = no evidence of generalising)\n",
		r.Live, r.Resolved, r.Unresolved, r.Memorised)
	if len(r.FrameIndexLike) > 0 {
		fmt.Fprintf(&b, "frame-counter-like bytes (unique value every frame — can key anything): %v\n", r.FrameIndexLike)
	}
	keys := make([]int, 0, len(r.Histogram))
	for k := range r.Histogram {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	for _, k := range keys {
		fmt.Fprintf(&b, "  %d companion(s): %d bytes\n", k, r.Histogram[k])
	}
	fmt.Fprintf(&b, "\n addr   companions  needs_input  transitions  method / features\n")
	for _, x := range r.Bytes {
		if x.Constant {
			continue
		}
		comp := fmt.Sprint(x.Companions)
		if !x.Resolved {
			comp = "?"
		}
		feats := strings.Join(x.Features, ", ")
		if !x.Resolved {
			feats = fmt.Sprintf("%s — %d conflicting transitions remain", x.Method, x.Residual)
			if len(x.ConflictAt) > 0 {
				feats += "  at " + strings.Join(x.ConflictAt, " ")
			}
		} else {
			feats = fmt.Sprintf("%s — {%s}", x.Method, feats)
			if x.Memorising {
				feats += fmt.Sprintf("  [MEMORISING: %d keys / %d transitions]", x.DistinctKeys, x.Transitions)
			}
			if x.UsesFrameIndex {
				feats += "  [uses a frame-counter-like byte]"
			}
		}
		fmt.Fprintf(&b, " %-5s %10s  %11v  %11d  %s\n", x.Addr, comp, x.NeedsInput, x.Transitions, feats)
	}
	b.WriteString("\nNOTE: 'unresolved' does not prove high arity. It means no tried feature set\n")
	b.WriteString("explained the byte — the missing feature may be outside what this probe can see\n")
	b.WriteString("(sub-frame ordering, hardware state beyond the collision latches, or a regime the\n")
	b.WriteString("scenarios never entered).\n")
	b.WriteString("NOTE: a MEMORISING resolution saw every key exactly once. It is consistent with the\n")
	b.WriteString("data and proves nothing about unvisited states — treat it as unmeasured, not as\n")
	b.WriteString("low arity, and lengthen or vary the scenarios until keys repeat.\n")
	return b.String()
}
