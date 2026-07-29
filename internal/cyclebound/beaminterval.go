package cyclebound

import (
	"fmt"
	"sort"

	"github.com/jetsetilly/gopher2600/hardware/cpu/instructions"
	"github.com/kidsnz/atari2600-harness/internal/beamtrace"
	"github.com/kidsnz/atari2600-harness/internal/srcmap"
)

// Where does this write land on the scanline — on EVERY path?
//
// A 2600 kernel is written against the beam: a colour register written a few
// cycles late changes a different part of the line than intended, and the fault
// shows up as one wrong column on one line, which is exactly the kind of thing
// nobody finds by staring. The existing tool for this, beamtrace, answers it for
// ONE execution. That is enough while a kernel is straight-line and stops being
// enough the moment a branch decides how much work happens before the write —
// which is when it starts to matter.
//
// This carries elapsed cycles as an INTERVAL through the region's control-flow
// graph, so every TIA write comes out with the earliest and latest beam position
// it can land at over all paths. A wide interval is a real answer too: it says
// the write's position depends on the path taken, which is usually a bug in a
// kernel that meant to be cycle-exact.
//
// The critique that shaped this: an interval needs a LOWER bound as well as an
// upper one, and the package only had a longest-path solver. Reporting the
// worst-case position alone would silently claim the write always lands there.

// BeamWrite is one TIA write with its proven beam-clock window.
type BeamWrite struct {
	PC   string `json:"pc"`
	Loc  string `json:"loc,omitempty"`
	Reg  string `json:"reg"`
	Kind string `json:"kind,omitempty"`

	// MinCyc/MaxCyc are CPU cycles from the region's WSYNC through the end of
	// this instruction, over all paths.
	MinCyc int `json:"min_cyc"`
	MaxCyc int `json:"max_cyc"`

	// MinClock/MaxClock are the same window in TIA colour clocks, in the same
	// coordinates as read_row and beamtrace (HBLANK -68..-1, visible 0..159).
	MinClock int `json:"min_clock"`
	MaxClock int `json:"max_clock"`

	// MinAbs/MaxAbs are colour clocks since the region's WSYNC, NOT folded back
	// into a single line. A region may legitimately span more than one scanline
	// (a two-line kernel does), and when a window straddles a line boundary the
	// folded form comes out as nonsense like [115..-35] — the first version
	// compared in that space and reported 112 writes as violations that were
	// perfectly inside their window. The unfolded pair is the one to reason with.
	MinAbs int `json:"min_abs"`
	MaxAbs int `json:"max_abs"`

	// CrossesLine marks a window that straddles a scanline boundary: which LINE
	// the write lands on depends on the path taken. In a kernel meant to be
	// cycle-exact that is a stronger warning than a wide window within one line.
	CrossesLine bool `json:"crosses_line,omitempty"`

	// Exact marks a write whose position does not depend on the path — the state
	// a cycle-exact kernel is trying to be in.
	Exact bool `json:"exact"`
}

// BeamRegion is one WSYNC-to-WSYNC region's writes.
type BeamRegion struct {
	Start      string      `json:"start"`
	StartLoc   string      `json:"start_loc,omitempty"`
	Kind       string      `json:"kind"`
	Bounded    bool        `json:"bounded"`
	Reason     string      `json:"reason,omitempty"`
	Writes     []BeamWrite `json:"writes,omitempty"`
	PathVaries int         `json:"path_varies"` // writes whose position is not fixed
}

// BeamReport is the whole-program result.
type BeamReport struct {
	Asm       string       `json:"asm"`
	Converged bool         `json:"converged"`
	Regions   []BeamRegion `json:"regions,omitempty"`

	// Bounded/Unbounded count regions the analysis could and could not place.
	Bounded   int `json:"bounded_regions"`
	Unbounded int `json:"unbounded_regions"`

	// BankedDeclined names the reason the image was not analysed. Beam positions
	// are computed from a flat-address decode, so on a bank-switched cartridge
	// every window in this report would be a window in a program that does not
	// exist — and unlike a missing region, a confidently wrong beam clock looks
	// exactly like a right one.
	BankedDeclined string `json:"banked_declined,omitempty"`
}

// cyclesAfterWSYNC is the beam clock at which the instruction following a WSYNC
// begins. WSYNC halts the CPU until the start of the next scanline, and a
// scanline starts in HBLANK at clock -68 in this project's coordinates (the same
// ones read_row, beamtrace and GetCoords use).
//
// It is stated here as a constant and then CHECKED against the emulator rather
// than trusted: TestBeamIntervalContainsObserved requires every measured write
// clock to fall inside the interval this offset produces. If the offset were
// wrong, that check fails loudly instead of the report being quietly shifted.
const cyclesAfterWSYNC = -68

// clockAt converts CPU cycles elapsed since the WSYNC into a beam clock.
// Three colour clocks per CPU cycle, and the line wraps every 228.
func clockAt(cyc int) int {
	c := cyclesAfterWSYNC + 3*cyc
	for c >= 160 {
		c -= 228
	}
	return c
}

// elapsed is the interval of cycles from the region start to a program point.
type elapsed struct {
	min, max int
	set      bool
}

func (e elapsed) join(o elapsed) elapsed {
	if !e.set {
		return o
	}
	if !o.set {
		return e
	}
	return elapsed{min: imin(e.min, o.min), max: imax(e.max, o.max), set: true}
}

func (e elapsed) plus(lo, hi int) elapsed {
	if !e.set {
		return e
	}
	return elapsed{min: e.min + lo, max: e.max + hi, set: true}
}

// beamRegion computes, for one region, the elapsed-cycle interval at every
// instruction, forward from the WSYNC. It walks the SAME subgraph the prover
// analyses (collectRegion + foldLoops), so a region the prover calls unbounded
// is unbounded here too, for the same stated reason.
func beamRegion(instrs map[uint16]Instr, start Instr, states map[uint16]State, sm *srcmap.Map) BeamRegion {
	br := BeamRegion{Start: hexAddr(start.Addr), Kind: "visible", Bounded: true}
	if sm != nil {
		br.StartLoc = sm.Locate(start.Addr)
	}
	if st, ok := states[start.Addr]; ok && st.displayOff() {
		br.Kind = "blank"
	}
	s := &solver{
		nodes: map[uint16]Instr{}, sinks: map[uint16]bool{}, folds: map[uint16]loopInfo{},
		memo: map[lkey]result{}, state: map[lkey]int{}, absStates: states, sm: sm,
	}
	fail := func(reason string) BeamRegion { br.Bounded, br.Reason = false, reason; return br }
	if msg := s.collectRegion(instrs, start); msg != "" {
		return fail(msg)
	}
	if len(s.sinks) == 0 {
		return fail("no WSYNC reached from region start")
	}
	if msg := s.foldLoops(); msg != "" {
		return fail(msg)
	}

	// Forward worklist over the folded subgraph. A back edge that survived
	// folding means an instruction's elapsed time is not bounded, and the region
	// is reported as such rather than given a number from the paths that happen
	// to terminate.
	before := map[uint16]elapsed{start.next(): {min: 0, max: 0, set: true}}
	after := map[uint16]elapsed{}
	order, cyclic := topoOrder(s, start.next())
	if cyclic {
		return fail("unbounded loop in region (no counted-loop bound found)")
	}
	for _, a := range order {
		e, ok := before[a]
		if !ok || !e.set {
			continue
		}
		if lf, folded := s.folds[a]; folded {
			ex := e.plus(lf.minCost, lf.cost)
			after[a] = ex
			before[lf.exit] = before[lf.exit].join(ex)
			continue
		}
		in := s.nodes[a]
		lo, hi := instrCost(in, states[a])
		ex := e.plus(lo, hi)
		after[a] = ex
		if s.sinks[a] {
			continue
		}
		for _, nx := range beamSuccessors(in) {
			if _, known := s.nodes[nx]; !known {
				continue
			}
			before[nx] = before[nx].join(ex)
		}
	}

	for _, a := range sortedKeysU16(after) {
		in := s.nodes[a]
		if _, folded := s.folds[a]; folded {
			continue
		}
		reg, ok := tiaTarget(in, states[a])
		if !ok {
			continue
		}
		// WSYNC and RSYNC are not writes whose beam position is in question: WSYNC
		// halts the CPU until the next scanline begins, so its landing point is a
		// hardware constant rather than a consequence of the path, and RSYNC resets
		// the horizontal counter outright. Pricing them like a 3-cycle store put
		// every one of them outside its own window in the containment check —
		// 1990 of 20532 observed writes — which is how this was found. What a kernel
		// author actually wants to know about a WSYNC is how many cycles the region
		// took to reach it, and the prover already answers that.
		if reg == "WSYNC" || reg == "RSYNC" {
			continue
		}
		e := after[a]
		minAbs, maxAbs := 3*e.min, 3*e.max
		w := BeamWrite{
			PC: hexAddr(a), Reg: reg,
			MinCyc: e.min, MaxCyc: e.max,
			MinAbs: minAbs, MaxAbs: maxAbs,
			MinClock: clockAt(e.min), MaxClock: clockAt(e.max),
			Exact:       e.min == e.max,
			CrossesLine: (minAbs+68)/228 != (maxAbs+68)/228,
		}
		if sm != nil {
			w.Loc = sm.Locate(a)
		}
		if !w.Exact {
			br.PathVaries++
		}
		br.Writes = append(br.Writes, w)
	}
	return br
}

// instrCost is the [best, worst] cycle cost of one instruction. The upper bound
// reuses the prover's own costing (including the page-cross penalty resolved
// from the tracked index range); the lower bound is the same minus penalties
// that a path can avoid. A branch is priced by its two outcomes.
func instrCost(in Instr, st State) (lo, hi int) {
	base := in.Def.Cycles
	if in.Def.IsBranch() {
		pen := 1
		if (in.next() >> 8) != (in.branchTarget() >> 8) {
			pen = 2
		}
		return base, base + pen
	}
	hi = in.baseCost() + in.pagePenalty(st)
	lo = in.baseCost()
	if lo > hi {
		lo = hi
	}
	return lo, hi
}

func beamSuccessors(in Instr) []uint16 {
	switch in.Def.Operator {
	case instructions.JSR:
		return []uint16{in.Operand}
	case instructions.RTS, instructions.RTI, instructions.BRK, instructions.JAM:
		return nil
	case instructions.JMP:
		if in.Def.AddressingMode == instructions.Indirect {
			return nil
		}
		return []uint16{in.Operand}
	}
	if in.Def.IsBranch() {
		return []uint16{in.next(), in.branchTarget()}
	}
	return []uint16{in.next()}
}

// topoOrder returns a topological order of the folded subgraph, or cyclic=true
// if a back edge survived. Folded loop headers are single nodes.
func topoOrder(s *solver, entry uint16) (order []uint16, cyclic bool) {
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := map[uint16]int{}
	var visit func(uint16)
	visit = func(a uint16) {
		if cyclic {
			return
		}
		switch color[a] {
		case gray:
			cyclic = true
			return
		case black:
			return
		}
		color[a] = gray
		if lf, folded := s.folds[a]; folded {
			if _, known := s.nodes[lf.exit]; known {
				visit(lf.exit)
			}
		} else if in, ok := s.nodes[a]; ok && !s.sinks[a] {
			for _, nx := range beamSuccessors(in) {
				if _, known := s.nodes[nx]; known {
					visit(nx)
				}
			}
		}
		color[a] = black
		order = append(order, a)
	}
	visit(entry)
	// reverse post-order
	for i, j := 0, len(order)-1; i < j; i, j = i+1, j-1 {
		order[i], order[j] = order[j], order[i]
	}
	return order, cyclic
}

// tiaTarget reports the TIA register an instruction writes, resolved through the
// effective address rather than the base operand — the same correction the
// emulator-side observation needed.
func tiaTarget(in Instr, st State) (string, bool) {
	acc, ok := accessOf(in, st)
	if !ok || acc.Unbounded || !acc.Exact {
		return "", false
	}
	if acc.Kind != AccessWrite && acc.Kind != AccessModify {
		return "", false
	}
	t := acc.Addrs[0]
	if t >= 0x0100 && t < 0x0180 {
		t &= 0xFF // page-1 mirror: a push aimed at TIA space
	}
	if t >= 0x40 {
		return "", false // not a TIA write register
	}
	a := t & 0x3F
	if n, _ := beamtrace.RegInfo(a); n != "" {
		return n, true
	}
	return fmt.Sprintf("$%02X", a), true
}

func sortedKeysU16(m map[uint16]elapsed) []uint16 {
	out := make([]uint16, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// BeamIntervals proves, for every TIA write in every region, the beam-clock
// window it can land in over all paths.
func BeamIntervals(asmPath string) (*BeamReport, error) {
	p, instrs, entries, sm, err := loadProgram(asmPath)
	if err != nil {
		return nil, err
	}
	if p.declined != "" {
		return &BeamReport{Asm: baseName(asmPath), BankedDeclined: p.declined}, nil
	}
	states, converged := computeStates(instrs, entries, p.byteAt)
	rep := &BeamReport{Asm: baseName(asmPath), Converged: converged}

	var starts []uint16
	for a, in := range instrs {
		if in.isWSYNC() {
			starts = append(starts, a)
		}
	}
	sort.Slice(starts, func(i, j int) bool { return starts[i] < starts[j] })
	for _, sa := range starts {
		br := beamRegion(instrs, instrs[sa], states, sm)
		if br.Bounded {
			rep.Bounded++
		} else {
			rep.Unbounded++
		}
		rep.Regions = append(rep.Regions, br)
	}
	return rep, nil
}
