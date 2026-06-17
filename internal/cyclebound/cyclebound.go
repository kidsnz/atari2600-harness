// Package cyclebound is a STATIC per-scanline cycle-budget prover (VV-2). Where
// assert_line_budget OBSERVES one execution and reports whether that run's
// WSYNC interval overran 76 cycles (∃ — an existential, one-path claim), this
// package PROVES the worst case over ALL reachable paths (∀ — the only
// universal-claim member of the verification suite). It directly attacks gap B
// (timing): a kernel that overruns only on one branch can pass a lucky run yet
// roll the screen on hardware; the static proof flags it regardless.
//
// Method: assemble the .asm (internal/build), recursive-descent decode the ROM
// from its reset/IRQ/NMI vectors (so inline data tables aren't misdecoded),
// cost each instruction from the in-tree exact cycle table
// (instructions.Definitions: Cycles + page/branch penalties), cut the CFG at
// every `STA WSYNC` ($02 store) into WSYNC-to-WSYNC regions, and prove each
// region's longest reachable path <= budget via a DAG longest-path search.
// Counted loops inside a region (ldx/ldy #N … dex/dey … bne/bpl) are folded by
// their iteration bound; anything we cannot bound (unbounded loop, JSR,
// indirect JMP) is reported honestly as "unbounded — out of scope", never
// silently passed.
//
// Scope (v1, stated honestly): single-bank flat 2K/4K kernels. Bank-switching,
// JSR-into-subroutine timing, indirect JMP, and nested/multiple intra-line
// loops are reported as unbounded rather than guessed.
//
// Provenance: implicit-path-enumeration / longest-path WCET — Li & Malik, "Performance
// analysis of embedded software using implicit path enumeration", DAC 1995;
// Ballabriga & Cassé, WCET 2008. Exact cycle table: Gopher2600
// hardware/cpu/instructions. The 76-cycle line budget: 228 color clocks / 3.
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

// DefaultBudget is one NTSC scanline = 228 color clocks / 3 = 76 CPU cycles
// (mirrors pkg/design.LineCycles; the runtime sibling assert_line_budget uses
// the same default).
const DefaultBudget = 76

// Instr is one statically decoded 6502 instruction.
type Instr struct {
	Addr    uint16
	Op      byte
	Def     instructions.Definition
	Operand uint16 // 1- or 2-byte operand, by Def.Bytes (little-endian for 3-byte)
}

func (in Instr) size() uint16 {
	if in.Def.Bytes < 1 {
		return 1
	}
	return uint16(in.Def.Bytes)
}

func (in Instr) next() uint16 { return in.Addr + in.size() }

// branchTarget resolves a relative branch's destination (Def.Bytes == 2).
func (in Instr) branchTarget() uint16 {
	return in.next() + uint16(int8(byte(in.Operand)))
}

// isWSYNC reports a store to WSYNC ($02): STA/STX/STY, non-indexed (zero-page or
// absolute — Gopher2600 tags both as AddressingMode==Absolute, distinguished by
// Bytes), operand == $0002. Indexed (AbsoluteX/Y) is excluded: `sta WSYNC` is
// never indexed, and an indexed effective address can't be resolved statically.
func (in Instr) isWSYNC() bool {
	switch in.Def.Operator {
	case instructions.STA, instructions.STX, instructions.STY:
	default:
		return false
	}
	if in.Def.AddressingMode != instructions.Absolute {
		return false
	}
	return in.Operand == 0x0002
}

// nodeCost is the worst-case cycle cost of executing this instruction, EXCLUDING
// branch taken/page penalties (those are applied on the CFG edges). A
// page-sensitive non-branch (an indexed read like LDA abs,X) is charged its +1
// worst case conservatively, since the index is generally unknown statically.
// Stores are never page-sensitive (6502 rule), so WSYNC stores cost exactly 3.
func (in Instr) nodeCost() int {
	c := in.Def.Cycles
	if in.Def.PageSensitive && !in.Def.IsBranch() {
		c++
	}
	return c
}

// --- ROM image + recursive-descent decode ---

type program struct {
	rom  []byte
	base uint16 // first ROM address = 0x10000 - len(rom) (0xF000 for 4K, 0xF800 for 2K)
}

func (p *program) byteAt(addr uint16) (byte, bool) {
	off := int(addr) - int(p.base)
	if off < 0 || off >= len(p.rom) {
		return 0, false
	}
	return p.rom[off], true
}

func (p *program) decodeAt(addr uint16) (Instr, bool) {
	op, ok := p.byteAt(addr)
	if !ok {
		return Instr{}, false
	}
	in := Instr{Addr: addr, Op: op, Def: instructions.Definitions[op]}
	switch in.Def.Bytes {
	case 2:
		b1, _ := p.byteAt(addr + 1)
		in.Operand = uint16(b1)
	case 3:
		lo, _ := p.byteAt(addr + 1)
		hi, _ := p.byteAt(addr + 2)
		in.Operand = uint16(lo) | uint16(hi)<<8
	}
	return in, true
}

// decodeInto walks the CFG from entry, decoding only reachable instructions
// into instrs (recursive descent avoids misdecoding inline data tables).
func (p *program) decodeInto(instrs map[uint16]Instr, entry uint16) {
	work := []uint16{entry}
	for len(work) > 0 {
		addr := work[len(work)-1]
		work = work[:len(work)-1]
		if _, seen := instrs[addr]; seen {
			continue
		}
		in, ok := p.decodeAt(addr)
		if !ok {
			continue
		}
		instrs[addr] = in
		work = append(work, decodeSuccessors(in)...)
	}
}

// decodeSuccessors lists addresses reachable for DECODING (JSR decodes both its
// callee and its return point; an indirect JMP's target is unknown).
func decodeSuccessors(in Instr) []uint16 {
	d := in.Def
	switch d.Operator {
	case instructions.JMP:
		if d.AddressingMode == instructions.Indirect {
			return nil
		}
		return []uint16{in.Operand}
	case instructions.JSR:
		return []uint16{in.Operand, in.next()}
	case instructions.RTS, instructions.RTI, instructions.BRK, instructions.JAM:
		return nil
	}
	if d.IsBranch() {
		return []uint16{in.next(), in.branchTarget()}
	}
	return []uint16{in.next()}
}

// --- region model + worst-case (DAG longest-path) ---

// Step is one instruction (or one folded loop) on a worst-case path, with the
// cycles it is charged.
type Step struct {
	Addr uint16 `json:"addr"`
	Cyc  int    `json:"cyc"`
	Loop int    `json:"loop,omitempty"` // iteration count if this step is a folded counted loop
	Loc  string `json:"loc,omitempty"`  // srcmap "Label+off (file:line)" when available
}

// Region is one WSYNC-to-WSYNC interval and its proven worst case.
type Region struct {
	Start    uint16 `json:"start"`             // address of the WSYNC store that opens the region
	StartLoc string `json:"start_loc,omitempty"`
	Worst    int    `json:"worst"`             // proven worst-case cycles from here to the next WSYNC
	Budget   int    `json:"budget"`
	Over     bool   `json:"over"`              // Worst > Budget
	Bounded  bool   `json:"bounded"`           // false => could not prove (reported, not passed)
	Reason   string `json:"reason,omitempty"`  // why unbounded
	Path     []Step `json:"path,omitempty"`    // worst-case breakdown (filled when Over)
}

type loopInfo struct {
	cost int
	exit uint16
	n    int
}

type result struct {
	cyc  int
	path []Step
}

const (
	white = 0
	gray  = 1
	black = 2
)

type solver struct {
	nodes  map[uint16]Instr
	sinks  map[uint16]bool
	folds  map[uint16]loopInfo
	memo   map[uint16]result
	state  map[uint16]int
	cyclic bool
	sm     *srcmap.Map
}

func prepend(s Step, rest []Step) []Step {
	return append([]Step{s}, rest...)
}

func theSucc(in Instr) uint16 {
	if in.Def.Operator == instructions.JMP { // absolute (indirect was filtered out)
		return in.Operand
	}
	return in.next()
}

// longest returns the worst-case cycles (and breakdown) from addr to the next
// WSYNC sink. It assumes the region subgraph is a DAG (loops already folded);
// any remaining back-edge sets cyclic so the caller reports the region as
// unbounded rather than returning a bogus number.
func (s *solver) longest(addr uint16) result {
	if r, ok := s.memo[addr]; ok {
		return r
	}
	if s.state[addr] == gray {
		s.cyclic = true
		return result{}
	}
	s.state[addr] = gray

	var best result
	switch {
	case s.foldHit(addr):
		lf := s.folds[addr]
		sub := s.longest(lf.exit)
		best = result{cyc: lf.cost + sub.cyc,
			path: prepend(Step{Addr: addr, Cyc: lf.cost, Loop: lf.n, Loc: s.loc(addr)}, sub.path)}
	default:
		in := s.nodes[addr]
		switch {
		case s.sinks[addr]:
			best = result{cyc: in.Def.Cycles, path: []Step{{Addr: addr, Cyc: in.Def.Cycles, Loc: s.loc(addr)}}}
		case in.Def.IsBranch():
			nt := s.longest(in.next())
			tk := s.longest(in.branchTarget())
			pen := 1
			if (in.next() >> 8) != (in.branchTarget() >> 8) {
				pen = 2 // taken branch crossing a page costs +2 total over base
			}
			ntTot := in.Def.Cycles + nt.cyc
			tkTot := in.Def.Cycles + pen + tk.cyc
			if tkTot >= ntTot {
				best = result{cyc: tkTot, path: prepend(Step{Addr: in.Addr, Cyc: in.Def.Cycles + pen, Loc: s.loc(in.Addr)}, tk.path)}
			} else {
				best = result{cyc: ntTot, path: prepend(Step{Addr: in.Addr, Cyc: in.Def.Cycles, Loc: s.loc(in.Addr)}, nt.path)}
			}
		default:
			sub := s.longest(theSucc(in))
			best = result{cyc: in.nodeCost() + sub.cyc,
				path: prepend(Step{Addr: in.Addr, Cyc: in.nodeCost(), Loc: s.loc(in.Addr)}, sub.path)}
		}
	}

	s.state[addr] = black
	s.memo[addr] = best
	return best
}

func (s *solver) foldHit(addr uint16) bool { _, ok := s.folds[addr]; return ok }

func (s *solver) loc(addr uint16) string { return s.sm.Locate(addr) }

// foldLoops finds a single simple counted loop in the region subgraph and folds
// it into one synthetic node keyed by the loop header. Returns "" on success (or
// when there is no loop), or a reason string when a loop exists but can't be
// bounded (so the region is reported unbounded).
func (s *solver) foldLoops() string {
	var latches []Instr
	for _, in := range s.nodes {
		if in.Def.IsBranch() {
			tgt := in.branchTarget()
			// A backward branch to a WSYNC sink is the normal per-scanline loop
			// returning to the next region — a region boundary, not an intra-line
			// loop. Only branches back to a non-sink node are real loops to fold.
			if _, ok := s.nodes[tgt]; ok && tgt <= in.Addr && !s.sinks[tgt] {
				latches = append(latches, in)
			}
		}
	}
	if len(latches) == 0 {
		return ""
	}
	if len(latches) > 1 {
		return "multiple back-edges (nested/complex loops) — not modeled in v1"
	}
	latch := latches[0]
	header := latch.branchTarget()

	// Validate the body header..latch is a simple straight chain and sum its
	// non-branch cost.
	bodyNoBranch := 0
	a := header
	for {
		in, ok := s.nodes[a]
		if !ok {
			return "loop body leaves the region"
		}
		if in.Addr == latch.Addr {
			break
		}
		if in.isWSYNC() {
			return "WSYNC inside loop body"
		}
		if in.Def.IsBranch() {
			return "branch inside loop body — not a simple counted loop"
		}
		bodyNoBranch += in.nodeCost()
		a = in.next()
		if a > latch.Addr {
			return "misaligned loop body"
		}
	}

	n := determineBound(s.nodes, header, latch)
	if n <= 0 {
		return "loop bound unknown (need ldx/ldy #N + dex/dey + bne/bpl in the region)"
	}
	pen := 1
	if (latch.next() >> 8) != (header >> 8) {
		pen = 2
	}
	// n iterations: n bodies, (n-1) taken branches back, 1 final not-taken exit.
	loopCost := n*bodyNoBranch + (n-1)*(latch.Def.Cycles+pen) + latch.Def.Cycles
	s.folds[header] = loopInfo{cost: loopCost, exit: latch.next(), n: n}
	return ""
}

// determineBound returns the iteration count of the simple counted loop
// header..latch (decrement-to-zero idiom), or 0 if undeterminable.
func determineBound(nodes map[uint16]Instr, header uint16, latch Instr) int {
	if latch.Def.Operator != instructions.BNE && latch.Def.Operator != instructions.BPL {
		return 0
	}
	decX, decY := false, false
	a := header
	for {
		in, ok := nodes[a]
		if !ok {
			return 0
		}
		if in.Addr == latch.Addr {
			break
		}
		switch in.Def.Operator {
		case instructions.DEX:
			decX = true
		case instructions.DEY:
			decY = true
		}
		a = in.next()
	}
	if !decX && !decY {
		return 0
	}
	wantLoad := instructions.LDY
	if decX {
		wantLoad = instructions.LDX
	}
	// Closest immediate initializer before the header within the region.
	bestAddr := -1
	bestN := 0
	for addr, in := range nodes {
		if int(addr) >= int(header) {
			continue
		}
		if in.Def.AddressingMode == instructions.Immediate && in.Def.Operator == wantLoad {
			if int(addr) > bestAddr {
				bestAddr = int(addr)
				bestN = int(in.Operand & 0xFF)
			}
		}
	}
	return bestN
}

// analyzeRegion proves the worst case of the WSYNC-to-WSYNC region opened by the
// WSYNC store `start`.
func analyzeRegion(instrs map[uint16]Instr, start Instr, budget int, sm *srcmap.Map) Region {
	reg := Region{Start: start.Addr, StartLoc: sm.Locate(start.Addr), Budget: budget, Bounded: true}
	s := &solver{
		nodes: map[uint16]Instr{},
		sinks: map[uint16]bool{},
		folds: map[uint16]loopInfo{},
		memo:  map[uint16]result{},
		state: map[uint16]int{},
		sm:    sm,
	}

	// Collect the region subgraph: from the instruction after `start`, follow the
	// CFG; WSYNC stores are sinks (terminators, not expanded); flow we can't
	// reason about makes the whole region unbounded.
	unbounded := func(reason string) Region { reg.Bounded = false; reg.Reason = reason; return reg }
	work := []uint16{start.next()}
	for len(work) > 0 {
		addr := work[len(work)-1]
		work = work[:len(work)-1]
		if _, ok := s.nodes[addr]; ok {
			continue
		}
		in, ok := instrs[addr]
		if !ok {
			return unbounded(fmt.Sprintf("reaches undecoded address $%04X", addr))
		}
		s.nodes[addr] = in
		if in.isWSYNC() {
			s.sinks[addr] = true
			continue
		}
		switch in.Def.Operator {
		case instructions.JSR:
			return unbounded("JSR in region (subroutine timing not modeled in v1)")
		case instructions.RTS, instructions.RTI:
			return unbounded("RTS/RTI in region (return target not modeled)")
		case instructions.BRK:
			return unbounded("BRK in region")
		case instructions.JAM:
			return unbounded("JAM (illegal/halt) in region")
		case instructions.JMP:
			if in.Def.AddressingMode == instructions.Indirect {
				return unbounded("indirect JMP in region (target not statically known)")
			}
			work = append(work, in.Operand)
			continue
		}
		if in.Def.IsBranch() {
			work = append(work, in.next(), in.branchTarget())
		} else {
			work = append(work, in.next())
		}
	}
	if len(s.sinks) == 0 {
		return unbounded("no WSYNC reached from region start")
	}
	if msg := s.foldLoops(); msg != "" {
		return unbounded(msg)
	}

	r := s.longest(start.next())
	if s.cyclic {
		return unbounded("unbounded loop in region (no counted-loop bound found)")
	}
	reg.Worst = r.cyc
	reg.Over = r.cyc > budget
	if reg.Over {
		reg.Path = r.path
	}
	return reg
}

// --- top-level ---

// Report is the proof outcome over all WSYNC-to-WSYNC regions of a kernel.
type Report struct {
	Asm        string   `json:"asm"`
	Budget     int      `json:"budget"`
	Regions    int      `json:"regions"`              // WSYNC-to-WSYNC regions analyzed
	MaxWorst   int      `json:"max_worst"`            // largest proven worst-case among bounded regions
	Certified  bool     `json:"certified"`            // all regions bounded AND <= budget
	Violations []Region `json:"violations,omitempty"` // regions whose worst case exceeds the budget
	Unbounded  []Region `json:"unbounded,omitempty"`  // regions that could not be proven (out of scope)
}

// Prove assembles asmPath, statically proves every WSYNC-to-WSYNC region's
// worst-case cycle cost, and returns the report. budget<=0 defaults to 76.
func Prove(asmPath string, budget int) (*Report, error) {
	if budget <= 0 {
		budget = DefaultBudget
	}
	bin := build.BinPathFor(asmPath)
	out, lst, sym, err := build.AssembleWithListing(asmPath, bin)
	if err != nil {
		return nil, fmt.Errorf("assemble %s failed:\n%s", asmPath, out)
	}
	sm := srcmap.Parse(lst, sym, asmPath)
	rom, err := os.ReadFile(bin)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", bin, err)
	}
	if len(rom) < 6 || len(rom) > 0x10000 {
		return nil, fmt.Errorf("unexpected ROM size %d bytes (expect a flat 2K/4K image)", len(rom))
	}
	p := &program{rom: rom, base: uint16(0x10000 - len(rom))}

	// Decode from the reset, NMI and IRQ/BRK vectors (duplicates dedupe).
	instrs := map[uint16]Instr{}
	for _, va := range []uint16{0xFFFC, 0xFFFA, 0xFFFE} {
		lo, _ := p.byteAt(va)
		hi, _ := p.byteAt(va + 1)
		t := uint16(lo) | uint16(hi)<<8
		if t >= p.base {
			p.decodeInto(instrs, t)
		}
	}

	var starts []uint16
	for a, in := range instrs {
		if in.isWSYNC() {
			starts = append(starts, a)
		}
	}
	sort.Slice(starts, func(i, j int) bool { return starts[i] < starts[j] })

	rep := &Report{Asm: filepath.Base(asmPath), Budget: budget}
	for _, sa := range starts {
		reg := analyzeRegion(instrs, instrs[sa], budget, sm)
		rep.Regions++
		if !reg.Bounded {
			rep.Unbounded = append(rep.Unbounded, reg)
			continue
		}
		if reg.Worst > rep.MaxWorst {
			rep.MaxWorst = reg.Worst
		}
		if reg.Over {
			rep.Violations = append(rep.Violations, reg)
		}
	}
	rep.Certified = len(rep.Violations) == 0 && len(rep.Unbounded) == 0
	return rep, nil
}
