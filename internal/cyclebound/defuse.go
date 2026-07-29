package cyclebound

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/jetsetilly/gopher2600/hardware/cpu/instructions"
	"github.com/kidsnz/atari2600-harness/internal/build"
	"github.com/kidsnz/atari2600-harness/internal/srcmap"
)

// Static def-use: which instruction writes which address, and what a read can
// see when it happens.
//
// Every existing tool in this harness answers this by RUNNING the ROM and
// watching, which can only ever report what the runs it did happened to do. The
// question a builder actually has — "is there ANY path where this cell is read
// before anything writes it?" — is not answerable that way, because the path
// that breaks is usually the one the tests never took.
//
// This walks the control-flow graph instead. The scope for "within a frame" is
// the WSYNC-to-WSYNC region the prover already uses, which is also the unit a
// 2600 kernel is actually written in.
//
// The analysis is a MAY/MUST pair, and the distinction is the whole point:
//   - MAY-write over-approximates. An indexed store with an unknown index may
//     have written anywhere, so it appears as a writer of everything it could
//     reach. Being wrong here means claiming a write that never happens, which
//     costs precision.
//   - MUST-write under-approximates. A store counts only where its target is
//     exact. Being wrong here means claiming a cell is initialised when it might
//     not be, which costs correctness — so must-write stays deliberately timid.
//
// The soundness claim is checkable and is checked: every write a real execution
// performs has to appear in the static may-set. See TestDefUseContainsObserved.

// AccessKind is how an instruction touches memory.
type AccessKind uint8

const (
	AccessRead AccessKind = iota
	AccessWrite
	AccessModify // read-modify-write: INC/DEC/ASL/LSR/ROL/ROR on memory
)

func (k AccessKind) String() string {
	switch k {
	case AccessRead:
		return "read"
	case AccessWrite:
		return "write"
	}
	return "modify"
}

// maxMaySet caps how many addresses a single access is willing to enumerate.
//
// 256 is not arbitrary: an 8-bit index spans at most 256 targets, so any access
// with a BOUNDED index fits. The first version capped it at 64, which quietly
// turned the RAM-clear loop every 2600 ROM opens with (`ldx #$FF / sta $00,x`)
// into an "unbounded" access — and an unbounded access contributes nothing to
// the may-write set. Dropping a write we can enumerate is not over-approximation,
// it is a hole, and it made the containment check against real execution pass
// without testing anything. Only a genuinely unknown index is unbounded now.
const maxMaySet = 256

// Access is one instruction's memory access with its target resolved as far as
// the abstract state allows.
type Access struct {
	PC   uint16     `json:"pc"`
	Kind AccessKind `json:"kind"`

	// Addrs is the may-set: every address this access might touch. Empty when
	// Unbounded.
	Addrs []uint16 `json:"addrs,omitempty"`

	// Exact marks a single, statically known target — the only case that may
	// contribute to a must-set.
	Exact bool `json:"exact"`

	// Unbounded marks an access whose target could not be pinned down at all —
	// only the indirect modes, whose pointer lives in RAM. An INDEXED access is
	// never unbounded: an 8-bit index reaches at most 256 addresses, so its
	// footprint is always enumerable even when the index itself is unknown.
	Unbounded bool `json:"unbounded"`

	// Wide marks an enumerated may-set that came from an unknown index, i.e. the
	// full 256-address reach rather than a proven window. Sound, but it carries
	// no information about WHICH cell, and a report that did not distinguish the
	// two would read as precise when it is not.
	Wide bool `json:"wide,omitempty"`
}

// accessOf resolves what in touches, given the abstract state at its address.
// The addressing-mode reasoning is deliberately the same as the store kill-set's
// (see killStoreTargets): if the two disagreed, a cell could be invalidated
// without being reported, or reported without being invalidated.
func accessOf(in Instr, st State) (Access, bool) {
	d := in.Def
	var kind AccessKind
	switch d.Effect {
	case instructions.Read:
		kind = AccessRead
	case instructions.Write:
		kind = AccessWrite
	default:
		// Gopher2600 classifies INC/DEC/shifts on memory as their own effect; treat
		// anything that both reads and writes as a modify.
		switch d.Operator {
		case instructions.INC, instructions.DEC, instructions.ASL, instructions.LSR,
			instructions.ROL, instructions.ROR:
			if d.AddressingMode == instructions.Implied {
				return Access{}, false // accumulator mode: no memory touched
			}
			kind = AccessModify
		default:
			return Access{}, false
		}
	}
	// Immediate and relative touch no memory address. Implied usually does not
	// either — except for a push, whose target is the stack, and on the 2600 the
	// stack is in the same address space the console decodes. A program that aims
	// SP at a TIA register writes it with PHA; roms/litmus/litmus_stack_trick.asm
	// turns the background green that way, and until SP was tracked no analysis
	// here could see the write at all.
	switch d.AddressingMode {
	case instructions.Immediate, instructions.Relative:
		return Access{}, false
	case instructions.Implied:
		switch d.Operator {
		case instructions.PHA, instructions.PHP:
			a := Access{PC: in.Addr, Kind: AccessWrite}
			sp, ok := st.SP.konst()
			if !ok {
				a.Unbounded = true // SP unknown: the push could be anywhere in page 1
				return a, true
			}
			a.Addrs, a.Exact = []uint16{uint16(0x0100 | (sp & 0xFF))}, true
			return a, true
		}
		return Access{}, false
	}

	a := Access{PC: in.Addr, Kind: kind}
	switch d.AddressingMode {
	case instructions.Absolute:
		a.Addrs, a.Exact = []uint16{in.Operand}, true
	case instructions.AbsoluteX, instructions.AbsoluteY:
		idx := st.X
		if d.AddressingMode == instructions.AbsoluteY {
			idx = st.Y
		}
		base := int(in.Operand)
		// An index register is 8 bits, so even a completely unknown one reaches at
		// most 256 addresses — a bounded footprint, not an unknown one. Treating
		// that case as "unbounded" is what made the RAM-clear loop vanish from the
		// may-write set and left the containment check with nothing to check.
		lo, hi := idx.Lo, idx.Hi
		if idx.Top {
			lo, hi = 0, 255
		}
		zp := base < 0x100
		seen := map[uint16]bool{}
		for i := lo; i <= hi; i++ {
			t := base + i
			if zp {
				t = (base + i) & 0xFF
			}
			if !seen[uint16(t)] {
				seen[uint16(t)] = true
				a.Addrs = append(a.Addrs, uint16(t))
			}
		}
		sort.Slice(a.Addrs, func(i, j int) bool { return a.Addrs[i] < a.Addrs[j] })
		a.Exact = len(a.Addrs) == 1
		a.Wide = idx.Top
	default:
		// Indirect, (ind,X), (ind),Y: the pointer lives in RAM and is not tracked,
		// so the target is genuinely unknown.
		a.Unbounded = true
	}
	return a, true
}

// AddrFacts is what is known about one address across the analysed scope.
type AddrFacts struct {
	Addr string `json:"addr"`

	// Writers/Readers are the PCs that may write / may read it, ascending.
	Writers []string `json:"writers,omitempty"`
	Readers []string `json:"readers,omitempty"`

	// ReadBeforeWrite marks a read that some path reaches without any write to
	// this address having definitely happened first, inside the region. On the
	// 2600 that is a read of whatever the previous frame — or power-on — left
	// behind, which is the classic "passes in the emulator, fails on hardware".
	ReadBeforeWrite []string `json:"read_before_write,omitempty"`
}

// RegionDefUse is the def-use picture of one WSYNC-to-WSYNC region.
type RegionDefUse struct {
	Start    string      `json:"start"`
	StartLoc string      `json:"start_loc,omitempty"`
	Kind     string      `json:"kind"`
	Accesses int         `json:"accesses"`
	Wide     int         `json:"unbounded_accesses"`
	Addrs    []AddrFacts `json:"addrs,omitempty"`
}

// DefUseReport is the whole-program result.
type DefUseReport struct {
	Asm       string `json:"asm"`
	Converged bool   `json:"converged"`

	// MayWrite is every address any reachable instruction might write, ascending.
	// This is the set a real execution's writes must be contained in.
	MayWrite []string `json:"may_write"`

	// UnboundedWriters are the PCs whose write target could not be pinned down.
	// While any of these exist, MayWrite cannot be treated as complete, and the
	// report says so rather than quietly presenting a partial set as the answer.
	UnboundedWriters []string `json:"unbounded_writers,omitempty"`

	// Writes is every reachable write/modify access with its resolved target set,
	// keyed by PC. This is the sharpest form of the soundness claim: not "the
	// program writes somewhere in this set" but "THIS instruction writes within
	// THIS set", which is what a real execution can be graded against without the
	// claim degenerating into "all of memory".
	Writes map[string]Access `json:"writes,omitempty"`

	// UninitReads are reads of RAM that no path from the reset vector has
	// definitely written first — power-on rubbish on hardware, while an emulator
	// hands out a deterministic value and the bug never surfaces in testing.
	//
	// Two earlier scopes were measured and discarded rather than shipped, because
	// each flagged so much that a report meant nothing: per WSYNC region, 515
	// addresses on one technique kernel (a kernel reads what its setup wrote);
	// whole-program but counting only exactly-targeted stores, 3783 on
	// dyn_multisprite and 506 on maze (the RAM clear every 2600 ROM opens with is
	// an INDEXED store, so the analysis never learned RAM was initialised).
	// Recognising the sweep idiom took the corpus to zero false reports while the
	// planted case in litmus_uninit_read.asm still fires.
	UninitReads []UninitRead `json:"uninit_reads,omitempty"`

	// FlatBankOnly is set when the image is larger than the 4K the console can
	// address at once. Every analysis here keys on a flat address, so on a
	// bank-switched image it would be describing a program that does not exist —
	// findings are withheld and this says why, rather than a plausible-looking
	// answer being returned.
	FlatBankOnly string `json:"flat_bank_only,omitempty"`

	Regions []RegionDefUse `json:"regions,omitempty"`
}

// UninitRead is one read that no path from reset guarantees was written first.
type UninitRead struct {
	PC   string `json:"pc"`
	Loc  string `json:"loc,omitempty"`
	Addr string `json:"addr"`
}

// mayWriteAddrs returns the may-write set as raw addresses (for the containment
// check), plus whether any writer was unbounded.
func (r *DefUseReport) mayWriteAddrs() (map[uint16]bool, bool) {
	out := map[uint16]bool{}
	for _, s := range r.MayWrite {
		var a uint16
		if _, err := fmt.Sscanf(s, "$%04X", &a); err == nil {
			out[a] = true
		}
	}
	return out, len(r.UnboundedWriters) > 0
}

func hexAddr(a uint16) string { return fmt.Sprintf("$%04X", a) }

func hexAddrs(in []uint16) []string {
	out := make([]string, 0, len(in))
	for _, a := range in {
		out = append(out, hexAddr(a))
	}
	return out
}

// regionInstrs walks a region forward from its WSYNC start, returning the
// instruction addresses reachable before the next WSYNC. The traversal follows
// both branch edges, so what comes back is every instruction ANY path through
// the region can execute — which is what a may-analysis needs.
func regionInstrs(instrs map[uint16]Instr, start uint16) []uint16 {
	seen := map[uint16]bool{start: true}
	work := []uint16{start}
	var out []uint16
	for len(work) > 0 {
		a := work[len(work)-1]
		work = work[:len(work)-1]
		in, ok := instrs[a]
		if !ok {
			continue
		}
		out = append(out, a)
		if a != start && in.isWSYNC() {
			continue // the next region begins here
		}
		for _, s := range decodeSuccessors(in) {
			if !seen[s] {
				seen[s] = true
				work = append(work, s)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// A counted loop that walks a store's index across a proven range writes its
// whole swept range by the time it exits. Recognising that is what makes a
// must-write analysis usable on a 2600 ROM at all: every one of them opens with
// `ldx #$FF / sta $00,x / dex / bne`, and because an indexed store has no single
// exact target, an analysis that only counts exact stores never learns that RAM
// was initialised — so it reports essentially every later read as reading
// uninitialised memory. Measured before this existed: 3783 such reports on one
// technique kernel, 506 on another. Flagging everything is the same as flagging
// nothing.
//
// The fencepost is the substance of the idiom, not a detail. With `dex / bne`
// the body runs at index N down to 1 and the branch falls through once the index
// reaches zero, so index 0 is NEVER stored — the swept range is base+1 .. base+N.
// `dex / bpl` exits when the index goes negative, so that one DOES include zero.
// Getting this backwards would claim a byte is initialised that the machine
// never touched, which is the one direction a must-analysis may not be wrong in.
type sweepInfo struct {
	exit  uint16   // the loop's fall-through successor
	addrs []uint16 // everything the loop has definitely written once it gets there
}

// findSweeps locates counted sweep loops keyed by their latch (back-branch)
// address. Anything it cannot fully account for is simply not reported, so a
// missed sweep costs precision and never correctness.
func findSweeps(instrs map[uint16]Instr, states map[uint16]State) map[uint16]sweepInfo {
	out := map[uint16]sweepInfo{}
	for la, latch := range instrs {
		if !latch.Def.IsBranch() {
			continue
		}
		op := latch.Def.Operator
		if op != instructions.BNE && op != instructions.BPL {
			continue
		}
		header := latch.branchTarget()
		if header > la {
			continue // forward branch, not a loop latch
		}
		// Walk the body as a straight chain, noting which index register is
		// decremented and which indexed stores ride on it.
		decX, decY := false, false
		var stores []Instr
		a := header
		ok := true
		for guard := 0; guard < 64; guard++ {
			in, found := instrs[a]
			if !found {
				ok = false
				break
			}
			if in.Addr == la {
				break
			}
			if in.Def.IsBranch() || in.isWSYNC() {
				ok = false
				break
			}
			switch in.Def.Operator {
			case instructions.DEX:
				decX = true
			case instructions.DEY:
				decY = true
			case instructions.STA, instructions.STX, instructions.STY:
				switch in.Def.AddressingMode {
				case instructions.AbsoluteX, instructions.AbsoluteY:
					stores = append(stores, in)
				}
			}
			a = in.next()
			if a > la {
				ok = false
				break
			}
		}
		if !ok || len(stores) == 0 || (!decX && !decY) || (decX && decY) {
			continue
		}
		n := determineBound(instrs, header, latch, states, 0)
		if n <= 0 || n > 255 {
			continue
		}
		lo := 1
		if op == instructions.BPL {
			lo = 0 // dex/bpl leaves only once the index goes negative, so 0 is stored
		}
		seen := map[uint16]bool{}
		var addrs []uint16
		for _, st := range stores {
			// The store must be indexed by the register the loop decrements, or the
			// sweep says nothing about where it went.
			wantX := st.Def.AddressingMode == instructions.AbsoluteX
			if wantX != decX {
				continue
			}
			base := int(st.Operand)
			zp := base < 0x100
			for i := lo; i <= n; i++ {
				t := base + i
				if zp {
					t = (base + i) & 0xFF
				}
				if !seen[uint16(t)] {
					seen[uint16(t)] = true
					addrs = append(addrs, uint16(t))
				}
			}
		}
		if len(addrs) == 0 {
			continue
		}
		sort.Slice(addrs, func(i, j int) bool { return addrs[i] < addrs[j] })
		out[la] = sweepInfo{exit: latch.next(), addrs: addrs}
	}
	return out
}

// mustWrittenAt runs a small forward fixpoint over the region computing, at each
// instruction, the set of addresses written on EVERY path reaching it. Meet is
// intersection, which is what makes it a must-analysis: an address survives only
// if no path can arrive without it having been written.
//
// A read of an address absent from that set is a read the region does not
// guarantee to have initialised.
func mustWrittenAt(instrs map[uint16]Instr, states map[uint16]State, region []uint16, start uint16, stopAtWSYNC bool, sweeps map[uint16]sweepInfo) map[uint16]map[uint16]bool {
	inRegion := map[uint16]bool{}
	for _, a := range region {
		inRegion[a] = true
	}
	// nil means "not yet reached" (top of the must-lattice); an empty map means
	// "reached, nothing guaranteed".
	before := map[uint16]map[uint16]bool{start: {}}

	for changed, guard := true, 0; changed && guard < 100000; guard++ {
		changed = false
		for _, a := range region {
			cur, ok := before[a]
			if !ok {
				continue
			}
			in := instrs[a]
			after := map[uint16]bool{}
			for k := range cur {
				after[k] = true
			}
			if acc, ok := accessOf(in, states[a]); ok && !acc.Unbounded && acc.Exact &&
				(acc.Kind == AccessWrite || acc.Kind == AccessModify) {
				after[acc.Addrs[0]] = true
			}
			if stopAtWSYNC && in.isWSYNC() && a != start {
				continue
			}
			for _, s := range decodeSuccessors(in) {
				if !inRegion[s] {
					continue
				}
				edge := after
				// A completed sweep is known only on the way OUT. Adding it to the
				// back edge as well would claim, at the loop header, that the whole
				// range is already written — false on the first iteration, and false
				// in the direction a must-analysis may not be wrong in.
				if sw, isSweep := sweeps[a]; isSweep && s == sw.exit {
					edge = map[uint16]bool{}
					for k := range after {
						edge[k] = true
					}
					for _, t := range sw.addrs {
						edge[t] = true
					}
				}
				old, seen := before[s]
				if !seen {
					cp := map[uint16]bool{}
					for k := range edge {
						cp[k] = true
					}
					before[s] = cp
					changed = true
					continue
				}
				// meet = intersection
				for k := range old {
					if !edge[k] {
						delete(old, k)
						changed = true
					}
				}
			}
		}
	}
	return before
}

// DefUse computes the static def-use picture for an assembled source.
func DefUse(asmPath string, budget int) (*DefUseReport, error) {
	p, instrs, entries, sm, err := loadProgram(asmPath)
	if err != nil {
		return nil, err
	}
	states, converged := computeStates(instrs, entries, p.byteAt)
	rep := &DefUseReport{Asm: baseName(asmPath), Converged: converged}

	mayW := map[uint16]bool{}
	unbounded := map[uint16]bool{}
	rep.Writes = map[string]Access{}
	for a, in := range instrs {
		acc, ok := accessOf(in, states[a])
		if !ok || (acc.Kind != AccessWrite && acc.Kind != AccessModify) {
			continue
		}
		rep.Writes[hexAddr(a)] = acc
		if acc.Unbounded {
			unbounded[a] = true
			continue
		}
		for _, t := range acc.Addrs {
			mayW[t] = true
		}
	}
	rep.MayWrite = hexAddrs(sortedAddrs(mayW))
	rep.UnboundedWriters = hexAddrs(sortedAddrs(unbounded))

	// Whole-program must-write from the reset vector. Computed but not reported —
	// see UninitReads for the measured reason.
	if p.banked {
		rep.FlatBankOnly = fmt.Sprintf("image is %d bytes; the console addresses 4K at a time, so this "+
			"cartridge switches banks and a flat-address analysis does not describe it", len(p.rom))
	}
	sweeps := findSweeps(instrs, states)
	if len(entries) > 0 && !p.banked {
		all := make([]uint16, 0, len(instrs))
		for a := range instrs {
			all = append(all, a)
		}
		sort.Slice(all, func(i, j int) bool { return all[i] < all[j] })
		must := mustWrittenAt(instrs, states, all, entries[0], false, sweeps)
		for _, a := range all {
			acc, ok := accessOf(instrs[a], states[a])
			if !ok || acc.Unbounded || (acc.Kind != AccessRead && acc.Kind != AccessModify) {
				continue
			}
			m, reached := must[a]
			if !reached {
				continue // this instruction is not reachable from reset
			}
			for _, t := range acc.Addrs {
				if t < 0x80 || t > 0xFF {
					continue // only RAM has power-on rubbish; TIA/RIOT reads are another matter
				}
				if m[t] {
					continue
				}
				u := UninitRead{PC: hexAddr(a), Addr: hexAddr(t)}
				if sm != nil {
					u.Loc = sm.Locate(a)
				}
				rep.UninitReads = append(rep.UninitReads, u)
			}
		}
	}

	var starts []uint16
	for a, in := range instrs {
		if in.isWSYNC() {
			starts = append(starts, a)
		}
	}
	sort.Slice(starts, func(i, j int) bool { return starts[i] < starts[j] })

	for _, sa := range starts {
		region := regionInstrs(instrs, sa)
		must := mustWrittenAt(instrs, states, region, sa, true, sweeps)
		rd := RegionDefUse{Start: hexAddr(sa), Kind: "region"}
		if sm != nil {
			rd.StartLoc = sm.Locate(sa)
		}
		if st, ok := states[sa]; ok && st.displayOff() {
			rd.Kind = "blank"
		}
		facts := map[uint16]*AddrFacts{}
		get := func(t uint16) *AddrFacts {
			f, ok := facts[t]
			if !ok {
				f = &AddrFacts{Addr: hexAddr(t)}
				facts[t] = f
			}
			return f
		}
		for _, a := range region {
			acc, ok := accessOf(instrs[a], states[a])
			if !ok {
				continue
			}
			rd.Accesses++
			if acc.Unbounded {
				rd.Wide++
				continue
			}
			for _, t := range acc.Addrs {
				f := get(t)
				switch acc.Kind {
				case AccessWrite:
					f.Writers = append(f.Writers, hexAddr(a))
				case AccessRead:
					f.Readers = append(f.Readers, hexAddr(a))
					if m, seen := must[a]; seen && !m[t] {
						f.ReadBeforeWrite = append(f.ReadBeforeWrite, hexAddr(a))
					}
				case AccessModify:
					f.Writers = append(f.Writers, hexAddr(a))
					f.Readers = append(f.Readers, hexAddr(a))
					if m, seen := must[a]; seen && !m[t] {
						f.ReadBeforeWrite = append(f.ReadBeforeWrite, hexAddr(a))
					}
				}
			}
		}
		for _, t := range sortedAddrs(keysOf(facts)) {
			rd.Addrs = append(rd.Addrs, *facts[t])
		}
		rep.Regions = append(rep.Regions, rd)
	}
	return rep, nil
}

func keysOf(m map[uint16]*AddrFacts) map[uint16]bool {
	out := map[uint16]bool{}
	for k := range m {
		out[k] = true
	}
	return out
}

func sortedAddrs(m map[uint16]bool) []uint16 {
	out := make([]uint16, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// loadProgram assembles asmPath and recovers the decoded program: the CFG from
// the reset/NMI/IRQ vectors, the entry list, and the source map. Factored out of
// Prove so def-use analyses the same program the prover does — two decoders
// would eventually disagree, and the disagreement would be invisible.
func loadProgram(asmPath string) (*program, map[uint16]Instr, []uint16, *srcmap.Map, error) {
	bin := build.BinPathFor(asmPath)
	out, lst, sym, err := build.AssembleWithListing(asmPath, bin)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("assemble %s failed:\n%s", asmPath, out)
	}
	sm := srcmap.Parse(lst, sym, asmPath)

	rom, err := os.ReadFile(bin)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("read %s: %w", bin, err)
	}
	if len(rom) < 6 || len(rom) > 0x10000 {
		return nil, nil, nil, nil, fmt.Errorf("unexpected ROM size %d bytes", len(rom))
	}
	p := newProgram(rom)
	// Decided once, here, so every entry point through this loader inherits the
	// same verdict. Three of them used to reach their own conclusion — and two
	// reached none at all, analysing the fictional flat program and being saved
	// only by the incidental absence of a reachable WSYNC.
	p.declined = p.declineBanked(bin)
	instrs, entries := p.decodeFromVectors()
	return p, instrs, entries, sm, nil
}

func baseName(p string) string { return filepath.Base(p) }

// StaticBranches decodes a raw cartridge image from its vectors and returns every
// branch instruction the control-flow graph reaches, ascending, plus whether the
// image is bank-switched (in which case a flat decode does not describe it and
// the set is not a denominator anyone should divide by).
//
// It exists so a coverage report can be divided by the branches a program HAS
// rather than by the ones a run happened to execute. Dividing by what was
// observed makes an unreached branch vanish from the arithmetic entirely, so the
// percentage rises as the test gets worse.
func StaticBranches(romPath string) (branches []uint16, banked bool, err error) {
	rom, err := os.ReadFile(romPath)
	if err != nil {
		return nil, false, err
	}
	if len(rom) < 6 || len(rom) > 0x10000 {
		return nil, false, fmt.Errorf("unexpected ROM size %d bytes", len(rom))
	}
	p := newProgram(rom)
	instrs, _ := p.decodeFromVectors()
	for a, in := range instrs {
		if in.Def.IsBranch() {
			branches = append(branches, a)
		}
	}
	sort.Slice(branches, func(i, j int) bool { return branches[i] < branches[j] })
	return branches, p.banked, nil
}
