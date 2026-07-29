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
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/jetsetilly/gopher2600/hardware/cpu/instructions"
	"github.com/jetsetilly/gopher2600/hardware/memory/memorymap"
	"github.com/kidsnz/atari2600-harness/internal/build"
	"github.com/kidsnz/atari2600-harness/internal/emu"
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

	// banked marks an image larger than the 4K the console can address at once,
	// i.e. one that switches banks at run time. Everything in this package keys
	// on a flat address, so on such an image the vectors, the decode and every
	// analysis built on them describe a program that does not exist. Callers must
	// decline rather than report — measured on banked_game.asm, the flat model
	// decoded 66 instructions from entry points $FFE0/$FFFF and produced a
	// confident finding about an address it had never decoded.
	banked bool

	// declined is the reason this image must not be analysed at all, or "" when it
	// is safe. Set once by the loader so every entry point shares one verdict.
	declined string
}

// newProgram wraps a flat cartridge image.
func newProgram(rom []byte) *program {
	return &program{rom: rom, base: uint16(0x10000 - len(rom)), banked: len(rom) > 4096}
}

// cartInfo asks the engine what cartridge this image actually is: the mapper it
// fingerprints as, and how many banks that mapper has.
func cartInfo(binPath string) (id string, banks int, err error) {
	e, err := emu.New("NTSC")
	if err != nil {
		return "", 0, err
	}
	if err := e.LoadROM(binPath); err != nil {
		return "", 0, err
	}
	id, banks = e.CartInfo()
	return id, banks, nil
}

// declineBanked returns a reason string when the image must not be analysed by
// this flat-address package, or "" when it is safe to proceed.
//
// The verdict comes from the ENGINE — it loads the cartridge and fingerprints the
// mapper — rather than from len(rom). Size is not the machine: an 8192-byte image
// is F8 only unless it fingerprints as WF8/3F/E0/E7/WD/FE/UA, and a superchip
// variant overlays RAM on part of the window whatever the size says. Guessing
// from the length means the analysis and the emulator the acceptance tests run on
// can disagree about which machine is being described.
//
// A single bank is not automatically fine either: a mapper can map cartridge RAM
// into the window, and bytes executed from there are not in the image at all, so
// any decode of them is fiction.
func (p *program) declineBanked(binPath string) string {
	id, banks, err := cartInfo(binPath)
	if err != nil {
		// Fall back to the size test rather than proceeding blind. Being unable to
		// ask the engine is a reason to be MORE careful, not less.
		if p.banked {
			return fmt.Sprintf("image is %d bytes and the cartridge could not be identified (%v); "+
				"the console addresses 4K at a time, so a flat-address analysis would describe "+
				"a program that does not exist", len(p.rom), err)
		}
		return ""
	}
	if banks > 1 {
		return fmt.Sprintf("cartridge is mapper %s with %d banks; every address in this package is a "+
			"flat 16-bit number, so the vectors, the decode and everything built on them would "+
			"describe a program that does not exist", id, banks)
	}
	return ""
}

// canon folds a CPU address to the cartridge offset it addresses, or reports
// that the address is not in cartridge space at all.
//
// The 6507 drives 13 address lines, so the cartridge answers at $1000-$1FFF and
// at every mirror of it up to $F000-$FFFF; a 2K image answers TWICE inside that
// window. Keying anything on the raw address therefore splits one byte of ROM
// across several keys, and comparing a statically-decoded address set against
// the PCs a real execution reported then subtracts sets that do not overlap —
// producing a number with no meaning rather than an answer.
//
// Cartridge space is decided by Gopher2600's own memory map rather than by a
// bit test here, and only THEN is the offset taken. Masking first would fold
// RAM, TIA and RIOT addresses into the ROM and decode whatever bytes happened to
// be there.
func (p *program) canon(addr uint16) (uint16, bool) {
	_, area := memorymap.MapAddress(addr, true)
	if area != memorymap.Cartridge || len(p.rom) == 0 {
		return 0, false
	}
	return uint16(int(addr) & (len(p.rom) - 1)), true
}

func (p *program) byteAt(addr uint16) (byte, bool) {
	off, ok := p.canon(addr)
	if !ok || int(off) >= len(p.rom) {
		return 0, false
	}
	return p.rom[off], true
}

// decodeFromVectors decodes the program reachable from the reset, NMI and
// IRQ/BRK vectors. Duplicate targets dedupe.
func (p *program) decodeFromVectors() (map[uint16]Instr, []uint16) {
	instrs := map[uint16]Instr{}
	var entries []uint16
	seen := map[uint16]bool{}
	for _, va := range []uint16{0xFFFC, 0xFFFA, 0xFFFE} {
		lo, _ := p.byteAt(va)
		hi, _ := p.byteAt(va + 1)
		t := uint16(lo) | uint16(hi)<<8
		if _, ok := p.canon(t); !ok || seen[t] {
			continue
		}
		seen[t] = true
		p.decodeInto(instrs, t)
		entries = append(entries, t)
	}
	return instrs, entries
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
	Start    uint16 `json:"start"` // address of the WSYNC store that opens the region
	StartLoc string `json:"start_loc,omitempty"`
	Kind     string `json:"kind,omitempty"` // "visible" (budget-checked) or "blank" (VSYNC/VBLANK; skipped)
	Worst    int    `json:"worst"`          // proven worst-case cycles from here to the next WSYNC
	Budget   int    `json:"budget"`
	Over     bool   `json:"over"`             // Worst > Budget
	Bounded  bool   `json:"bounded"`          // false => could not prove (reported, not passed)
	Reason   string `json:"reason,omitempty"` // why unbounded
	Path     []Step `json:"path,omitempty"`   // worst-case breakdown (filled when Over)

	// Conditional turns "cannot prove" into something a builder can act on.
	// When the ONLY obstacle is a loop whose iteration count could not be
	// established, the region's cost is still a known function of that count, so
	// the largest count that fits the budget is computable — and that number is
	// the useful one: not "unprovable" but "stays inside 76 cycles as long as
	// this loop runs at most 7 times".
	//
	// It is an OBLIGATION, not a proof. Bounded stays false and the region never
	// counts towards Certified; discharging it means proving the bound elsewhere,
	// asserting it at run time, or the author accepting it explicitly.
	Conditional *Obligation `json:"conditional,omitempty"`
}

// Obligation is a conditional bound: the region is within budget provided the
// named loop does not exceed MaxIterations.
type Obligation struct {
	LoopHeader    string `json:"loop_header"`
	LoopLoc       string `json:"loop_loc,omitempty"`
	MaxIterations int    `json:"max_iterations"`
	WorstAtMax    int    `json:"worst_at_max"`
	Budget        int    `json:"budget"`
	Statement     string `json:"statement"`
}

type loopInfo struct {
	cost int // worst case: n iterations
	// minCost is the cost of the FEWEST iterations the loop can run. The
	// worst-case prover never needed it, but an interval does: using the
	// worst-case cost as the lower bound too claims the loop always runs its
	// maximum, which for a divide-by-15 positioning loop is only true for one
	// target X. Measured consequence of getting this wrong: 20 observed TIA
	// writes landed EARLIER than their "proven" earliest position.
	minCost int
	exit    uint16
	n       int
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

// lkey keys the longest-path memo/colour by (address, active return address). The
// return address is the interprocedural call context (2A): 0 = no active call, so
// the same subroutine address called from different sites is solved per-caller.
type lkey struct{ addr, ret uint16 }

type solver struct {
	nodes     map[uint16]Instr
	sinks     map[uint16]bool
	folds     map[uint16]loopInfo
	memo      map[lkey]result
	state     map[lkey]int
	absStates map[uint16]State // S3: abstract state per address, for page-cross precision
	cyclic    bool
	unbounded bool   // 2A: a path needs interprocedural support we don't model (nested call / RTS w/o caller)
	unbReason string // why
	sm        *srcmap.Map
	amaxHint  int // ②: `@amax N` = author-declared upper bound of a divide-loop accumulator; used by determineBound when the abstract range is Top

	// pending holds a loop whose body is perfectly well understood but whose
	// iteration count could not be established. Keeping it, instead of only
	// reporting that the region is unbounded, is what lets the caller ask the
	// question that actually helps: how many times may this run before the
	// region misses its budget?
	pending *pendingLoop
}

type pendingLoop struct {
	header       uint16
	latch        Instr
	bodyNoBranch int
	pen          int
	exit         uint16
}

// costAt returns the loop's cycle cost for exactly n iterations.
func (pl *pendingLoop) costAt(n int) int {
	if n < 1 {
		n = 1
	}
	return n*pl.bodyNoBranch + (n-1)*(pl.latch.Def.Cycles+pl.pen) + pl.latch.Def.Cycles
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
// longest returns the worst-case cycles (and breakdown) from addr to the next
// WSYNC sink, within call context ret (2A: the return address of the active
// JSR, or 0 at top level). Memoised per (addr, ret). Anything we cannot bound
// soundly (a back-edge, a nested call, or an RTS with no caller in context) sets
// s.cyclic / s.unbounded so the caller reports the region unbounded rather than
// trusting a low number.
func (s *solver) longest(addr, ret uint16) result {
	k := lkey{addr, ret}
	if r, ok := s.memo[k]; ok {
		return r
	}
	if s.state[k] == gray {
		s.cyclic = true
		return result{}
	}
	s.state[k] = gray

	var best result
	switch {
	case s.foldHit(addr):
		lf := s.folds[addr]
		sub := s.longest(lf.exit, ret)
		best = result{cyc: lf.cost + sub.cyc,
			path: prepend(Step{Addr: addr, Cyc: lf.cost, Loop: lf.n, Loc: s.loc(addr)}, sub.path)}
	default:
		in := s.nodes[addr]
		switch {
		case s.sinks[addr]:
			best = result{cyc: in.Def.Cycles, path: []Step{{Addr: addr, Cyc: in.Def.Cycles, Loc: s.loc(addr)}}}
		case in.Def.Operator == instructions.JSR:
			// 2A: follow into the callee with the return address threaded. Only one
			// level deep — a call while a call is already active is not modeled.
			if ret != 0 {
				s.unbounded = true
				s.unbReason = "nested subroutine call (single-level interprocedural only)"
				break
			}
			sub := s.longest(in.Operand, in.next())
			best = result{cyc: in.Def.Cycles + sub.cyc,
				path: prepend(Step{Addr: in.Addr, Cyc: in.Def.Cycles, Loc: s.loc(in.Addr)}, sub.path)}
		case in.Def.Operator == instructions.RTS || in.Def.Operator == instructions.RTI:
			// 2A: return to the active call site; no caller in context => cannot bound.
			if ret == 0 {
				s.unbounded = true
				s.unbReason = "RTS/RTI with no caller in context"
				break
			}
			sub := s.longest(ret, 0)
			best = result{cyc: in.Def.Cycles + sub.cyc,
				path: prepend(Step{Addr: in.Addr, Cyc: in.Def.Cycles, Loc: s.loc(in.Addr)}, sub.path)}
		case in.Def.IsBranch():
			nt := s.longest(in.next(), ret)
			tk := s.longest(in.branchTarget(), ret)
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
			cost := in.baseCost() + in.pagePenalty(s.absStates[addr]) // S3: page penalty from the index range
			sub := s.longest(theSucc(in), ret)
			best = result{cyc: cost + sub.cyc,
				path: prepend(Step{Addr: in.Addr, Cyc: cost, Loc: s.loc(in.Addr)}, sub.path)}
		}
	}

	s.state[k] = black
	s.memo[k] = best
	return best
}

// collectRegion walks the region subgraph from the instruction after start and
// fills s.nodes / s.sinks. It returns "" on success or the reason the region
// cannot be reasoned about. Shared with the beam-interval pass so both analyses
// see the same subgraph — two collectors would eventually disagree, and the
// disagreement would be invisible.
func (s *solver) collectRegion(instrs map[uint16]Instr, start Instr) string {
	return s.collectFrom(instrs, start.next())
}

// collectFrom walks the subgraph from one address, adding to whatever is already
// collected. Shared so a caller's continuation can be brought into the same
// region without a second, subtly different walker.
func (s *solver) collectFrom(instrs map[uint16]Instr, from uint16) string {
	work := []uint16{from}
	for len(work) > 0 {
		addr := work[len(work)-1]
		work = work[:len(work)-1]
		if _, ok := s.nodes[addr]; ok {
			continue
		}
		in, ok := instrs[addr]
		if !ok {
			return fmt.Sprintf("reaches undecoded address $%04X", addr)
		}
		s.nodes[addr] = in
		if in.isWSYNC() {
			s.sinks[addr] = true
			continue
		}
		switch in.Def.Operator {
		case instructions.JSR:
			// 2A: follow the callee AND the return point; the callee's own WSYNC is a
			// sink as usual, and longest() threads the return address. (Collection
			// terminates via the seen-set; longest() guards against nesting.)
			work = append(work, in.Operand, in.next())
			continue
		case instructions.RTS, instructions.RTI:
			// 2A: a leaf for collection — the return target is the call context,
			// resolved in longest() via the threaded return address.
			continue
		case instructions.BRK:
			return "BRK in region"
		case instructions.JAM:
			return "JAM (illegal/halt) in region"
		case instructions.JMP:
			if in.Def.AddressingMode == instructions.Indirect {
				return "indirect JMP in region (target not statically known)"
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
	return ""
}

// callContexts returns, for a region that begins INSIDE a subroutine, the return
// addresses that could be active when it runs.
//
// A WSYNC inside a callee opens a region whose continuation lives in the CALLER,
// so with no call context the walk hits an RTS, finds no WSYNC ahead of it, and
// the region is reported unbounded. Measured on the corpus: five regions fail as
// "no WSYNC reached" and three as "RTS with no caller in context", and
// shared_setxpos.asm fails twice because its whole positioning routine is built
// that way.
//
// Each JSR contributes its return address as a candidate whenever the region
// start is reachable from that JSR's target without first returning.
func callContexts(instrs map[uint16]Instr, regionStart uint16) []uint16 {
	var out []uint16
	seenRet := map[uint16]bool{}
	for _, in := range instrs {
		if in.Def.Operator != instructions.JSR {
			continue
		}
		ret := in.next()
		if seenRet[ret] || !reachableWithinCallee(instrs, in.Operand, regionStart) {
			continue
		}
		seenRet[ret] = true
		out = append(out, ret)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// displayPreserver returns a predicate answering "can a JSR to this address
// change VSYNC/VBLANK?" — used to keep the display bits alive across a call
// instead of dropping them with the rest of the callee's unmodeled effect.
//
// It is deliberately one-sided. Anything it cannot resolve — an indexed or
// indirect store, a store whose address folds into TIA $00/$01 through any
// mirror, a push (page 1 mirrors the console's own addresses, so SP can aim a
// PHA at VBLANK), or a nested call it has already visited — answers "not
// preserved", which restores the old behaviour for that call. Being wrong in
// this direction only loses precision; being wrong the other way would let a
// region that really does turn the display on be classified as blank and skipped.
// states may be nil on the first pass, when no index ranges are known yet; the
// caller then runs a second pass with the states the first produced, which is
// what makes `sta RESP0,X` — the ordinary way a shared positioning routine picks
// its object — resolvable at all. The second pass only ever ADDS a justified
// fact, and the justification itself comes from a sound first-pass range, so the
// fixpoint stays sound.
func displayPreserver(instrs map[uint16]Instr, states map[uint16]State) func(uint16) bool {
	memo := map[uint16]bool{}
	var walk func(entry uint16, depth int, onStack map[uint16]bool) bool
	walk = func(entry uint16, depth int, onStack map[uint16]bool) bool {
		if v, ok := memo[entry]; ok {
			return v
		}
		if depth > 8 || onStack[entry] { // recursion / deep nesting: give up, safely
			return false
		}
		onStack[entry] = true
		defer delete(onStack, entry)

		seen := map[uint16]bool{entry: true}
		work := []uint16{entry}
		for len(work) > 0 {
			a := work[len(work)-1]
			work = work[:len(work)-1]
			in, ok := instrs[a]
			if !ok {
				continue
			}
			switch in.Def.Operator {
			case instructions.RTS, instructions.RTI, instructions.BRK, instructions.JAM:
				continue
			case instructions.PHA, instructions.PHP:
				if !pushMissesDisplay(in, states) {
					memo[entry] = false
					return false
				}
			case instructions.JSR:
				if !walk(in.Operand, depth+1, onStack) {
					memo[entry] = false
					return false
				}
			case instructions.STA, instructions.STX, instructions.STY:
				if !storeMissesDisplay(in, states) {
					memo[entry] = false
					return false
				}
			}
			for _, nx := range decodeSuccessors(in) {
				if !seen[nx] {
					seen[nx] = true
					work = append(work, nx)
				}
			}
		}
		memo[entry] = true
		return true
	}
	return func(entry uint16) bool { return walk(entry, 0, map[uint16]bool{}) }
}

// storeMissesDisplay reports that this store provably cannot reach VSYNC ($00) or
// VBLANK ($01). Only a non-indexed store to a statically known address qualifies,
// and the address is folded through the engine's own memory map first so a write
// via any TIA mirror is caught rather than compared as a raw number.
func storeMissesDisplay(in Instr, states map[uint16]State) bool {
	if in.Def.AddressingMode != instructions.Absolute {
		return indexedStoreMissesDisplay(in, states)
	}
	ma, area := memorymap.MapAddress(in.Operand, false)
	if area != memorymap.TIA {
		return true
	}
	// MapAddress folds any TIA mirror onto the canonical register address, so the
	// comparison is against $00/$01 there rather than against the raw operand.
	return ma != 0x00 && ma != 0x01
}

// pushMissesDisplay reports that a PHA/PHP cannot land on VSYNC/VBLANK.
//
// A push is a store to $0100|SP, and page 1 mirrors the addresses the console
// decodes, so a program that aims SP at the TIA turns a push into a register
// write — the Stack Trick this project uses deliberately. Treating EVERY push as
// display-touching would be sound but would drag every RTS-dispatch and every
// ordinary subroutine call into the visible class. SP is tracked, so the question
// is answerable: only an SP range that can reach $0100/$0101 counts.
func pushMissesDisplay(in Instr, states map[uint16]State) bool {
	st, ok := states[in.Addr]
	if !ok || !st.valid || st.SP.Top || st.SP.Hi-st.SP.Lo > 255 {
		return false
	}
	for sp := st.SP.Lo; sp <= st.SP.Hi; sp++ {
		ma, area := memorymap.MapAddress(uint16(0x0100|(sp&0xFF)), false)
		if area == memorymap.TIA && (ma == 0x00 || ma == 0x01) {
			return false
		}
	}
	return true
}

// indexedStoreMissesDisplay resolves `sta base,X` (or ,Y) through the index range
// the previous pass proved, and reports that no address it can reach is VSYNC or
// VBLANK. An unknown or unusably wide range answers false.
func indexedStoreMissesDisplay(in Instr, states map[uint16]State) bool {
	switch in.Def.AddressingMode {
	case instructions.AbsoluteX, instructions.AbsoluteY:
	default:
		return false // indirect: no base to reason from
	}
	st, ok := states[in.Addr]
	if !ok || !st.valid {
		return false
	}
	idx := st.storeIndex(in)
	if idx.Top || idx.Hi-idx.Lo > 255 {
		return false
	}
	for i := idx.Lo; i <= idx.Hi; i++ {
		ma, area := memorymap.MapAddress(in.Operand+uint16(i), false)
		if area == memorymap.TIA && (ma == 0x00 || ma == 0x01) {
			return false
		}
	}
	return true
}

// reachableWithinCallee reports whether target is reachable from entry without
// passing through a return — i.e. whether it is part of that subroutine's body.
func reachableWithinCallee(instrs map[uint16]Instr, entry, target uint16) bool {
	seen := map[uint16]bool{entry: true}
	work := []uint16{entry}
	for len(work) > 0 {
		a := work[len(work)-1]
		work = work[:len(work)-1]
		if a == target {
			return true
		}
		in, ok := instrs[a]
		if !ok {
			continue
		}
		switch in.Def.Operator {
		case instructions.RTS, instructions.RTI, instructions.BRK, instructions.JAM:
			continue
		}
		for _, nx := range decodeSuccessors(in) {
			if !seen[nx] {
				seen[nx] = true
				work = append(work, nx)
			}
		}
	}
	return false
}

// analyzeRegionInContexts re-analyses a region once per candidate return address
// and returns the WORST result, or nil when any context fails to resolve. Every
// context must succeed: a region provable from one call site and not from another
// is not provable, and reporting the provable one would be choosing the
// flattering answer.
func analyzeRegionInContexts(instrs map[uint16]Instr, start Instr, budget, amaxHint int,
	sm *srcmap.Map, states map[uint16]State, ctxs []uint16) *Region {
	var worst *Region
	for _, ret := range ctxs {
		s := &solver{
			nodes: map[uint16]Instr{}, sinks: map[uint16]bool{}, folds: map[uint16]loopInfo{},
			memo: map[lkey]result{}, state: map[lkey]int{}, absStates: states, sm: sm, amaxHint: amaxHint,
		}
		if msg := s.collectRegion(instrs, start); msg != "" {
			return nil
		}
		// Bring the caller's continuation in; without it the walk stops at the RTS
		// and never reaches the WSYNC that ends the region.
		if msg := s.collectFrom(instrs, ret); msg != "" {
			return nil
		}
		if len(s.sinks) == 0 {
			return nil
		}
		if msg := s.foldLoops(); msg != "" {
			return nil
		}
		r := s.longest(start.next(), ret)
		if s.cyclic || s.unbounded {
			return nil
		}
		if worst == nil || r.cyc > worst.Worst {
			reg := Region{
				Start: start.Addr, StartLoc: sm.Locate(start.Addr), Budget: budget,
				Bounded: true, Kind: "visible", Worst: r.cyc, Over: r.cyc > budget,
			}
			if st, ok := states[start.Addr]; ok && st.displayOff() && !regionTouchesDisplay(s.nodes, states) {
				reg.Kind = "blank"
			}
			if reg.Over {
				reg.Path = r.path
			}
			worst = &reg
		}
	}
	return worst
}

// solveObligation answers "how many times may this loop run before the region
// misses its budget?" by folding the pending loop at successive trip counts and
// re-solving the longest path. The search is linear from 1 and stops at the
// first count that overruns, so the answer is the largest count that still fits.
//
// A count of 0 means the region cannot meet its budget even with a single
// iteration — a fact worth reporting plainly rather than as "unprovable".
func (s *solver) solveObligation(start Instr, budget int) *Obligation {
	pl := s.pending
	if pl == nil {
		return nil
	}
	const maxTrip = 256
	best := 0
	worstAtBest := 0
	for n := 1; n <= maxTrip; n++ {
		// Fresh solver state per trial: memo and colouring are per-solve.
		s.folds = map[uint16]loopInfo{pl.header: {cost: pl.costAt(n), minCost: pl.costAt(1), exit: pl.exit, n: n}}
		s.memo = map[lkey]result{}
		s.state = map[lkey]int{}
		s.cyclic, s.unbounded, s.unbReason = false, false, ""
		r := s.longest(start.next(), 0)
		if s.cyclic || s.unbounded {
			return nil // something else is wrong too; no clean obligation to state
		}
		if r.cyc > budget {
			break
		}
		best, worstAtBest = n, r.cyc
	}
	ob := &Obligation{
		LoopHeader:    fmt.Sprintf("$%04X", pl.header),
		LoopLoc:       s.loc(pl.header),
		MaxIterations: best,
		WorstAtMax:    worstAtBest,
		Budget:        budget,
	}
	if best == 0 {
		ob.Statement = fmt.Sprintf("the region misses its %d-cycle budget even at one iteration of the loop at $%04X",
			budget, pl.header)
	} else {
		ob.Statement = fmt.Sprintf("within %d cycles provided the loop at $%04X runs at most %d time(s) (worst %d there)",
			budget, pl.header, best, worstAtBest)
	}
	return ob
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

	pen := 1
	if (latch.next() >> 8) != (header >> 8) {
		pen = 2
	}
	n := determineBound(s.nodes, header, latch, s.absStates, s.amaxHint)
	if n <= 0 {
		// The body is fully understood; only the trip count is missing. Keep it so
		// the caller can still answer "how many iterations fit the budget?".
		s.pending = &pendingLoop{header: header, latch: latch, bodyNoBranch: bodyNoBranch, pen: pen, exit: latch.next()}
		return "loop bound unknown (need a counted dex/dey or sbc-divide idiom with a proven range)"
	}
	// n iterations: n bodies, (n-1) taken branches back, 1 final not-taken exit.
	loopCost := n*bodyNoBranch + (n-1)*(latch.Def.Cycles+pen) + latch.Def.Cycles
	// Fewest iterations: the back edge is a bne/bpl AFTER the body, so the body
	// runs at least once and the branch then falls through.
	minLoopCost := bodyNoBranch + latch.Def.Cycles
	s.folds[header] = loopInfo{cost: loopCost, minCost: minLoopCost, exit: latch.next(), n: n}
	return ""
}

// determineBound returns an iteration upper bound for the simple counted loop
// header..latch, or 0 if undeterminable. Two idioms:
//   - decrement-to-zero (ldx/ldy #N + dex/dey + bne/bpl), and
//   - (2B) divide / sbc-counter (sec; A reduced by a constant; bcs/bcc),
//     bounded from the proven range of A on entry to the loop header.
func determineBound(nodes map[uint16]Instr, header uint16, latch Instr, absStates map[uint16]State, amaxHint int) int {
	// 2B: divide-by-N coarse-positioning idiom — the body subtracts a constant from
	// A and loops while no borrow (BCS) / while borrow (BCC). Max iterations =
	// floor(Amax/const)+1 (with carry set); +1 more covers an unknown entry carry.
	// SOUND: over-approximates the count (more iterations = higher cost). Unknown A
	// range or non-constant subtrahend => 0 (stay unbounded, no false bound).
	if latch.Def.Operator == instructions.BCS || latch.Def.Operator == instructions.BCC {
		sub, nbody := 0, 0
		a := header
		for {
			in, ok := nodes[a]
			if !ok {
				return 0
			}
			if in.Addr == latch.Addr {
				break
			}
			nbody++
			if in.Def.Operator == instructions.SBC && in.Def.AddressingMode == instructions.Immediate {
				sub = int(in.Operand & 0xFF)
			}
			a = in.next()
		}
		// only the canonical single-instruction `sbc #const` body is modeled
		if sub == 0 || nbody != 1 {
			return 0
		}
		// A's LOOP-ENTRY upper bound. absStates[header] is the in-loop JOIN — polluted
		// to Top by the final wrapping subtraction on the back-edge — so read A from
		// the FALL-THROUGH predecessor's post-state instead (the value entering the
		// loop from above). This is where 3A's AND-mask range and 3B's array-element
		// range surface. Max over predecessors (sound). Unknown => 0 (stay unbounded).
		amax := -1
		for addr, in := range nodes {
			if in.next() == header && addr != latch.Addr && int(addr) < int(header) {
				if st, ok := absStates[addr]; ok {
					if ea := st.transfer(in).A; !ea.Top && ea.Hi > amax {
						amax = ea.Hi
					}
				}
			}
		}
		// fallback: closest immediate `lda #imm` before the loop (exact).
		if amax < 0 {
			bestAddr := -1
			for addr, in := range nodes {
				if int(addr) < int(header) && in.Def.Operator == instructions.LDA &&
					in.Def.AddressingMode == instructions.Immediate && int(addr) > bestAddr {
					bestAddr = int(addr)
					amax = int(in.Operand & 0xFF)
				}
			}
		}
		if amax < 0 && amaxHint > 0 {
			amax = amaxHint // ②: author-declared `@amax N` (the accumulator's proven upper bound) when the abstract range is Top
		}
		if amax < 0 {
			return 0
		}
		return amax/sub + 2
	}
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

// regionTouchesDisplay reports whether any node stores to VSYNC($00)/VBLANK($01),
// i.e. could change the display state within the region.
// regionTouchesDisplay reports whether anything in the region can turn the
// display on inside itself, which disqualifies it from being skipped as blank.
//
// It shares `storeMissesDisplay` with the JSR rule rather than testing the raw
// operand, because the raw test saw only non-indexed writes: `sta VSYNC,x` is
// AbsoluteX, so it returned no address at all and the region was skipped as blank
// while containing a write to VBLANK. A push counts too — page 1 mirrors the
// console's own addresses, so SP can aim a PHA at the display.
func regionTouchesDisplay(nodes map[uint16]Instr, states map[uint16]State) bool {
	for _, in := range nodes {
		switch in.Def.Operator {
		case instructions.PHA, instructions.PHP:
			if !pushMissesDisplay(in, states) {
				return true
			}
		case instructions.STA, instructions.STX, instructions.STY:
			if !storeMissesDisplay(in, states) {
				return true
			}
		}
	}
	return false
}

// analyzeRegion proves the worst case of the WSYNC-to-WSYNC region opened by the
// WSYNC store `start`. states gives the abstract state at each address, used to
// classify the region's display interval (S1).
func analyzeRegion(instrs map[uint16]Instr, start Instr, budget, amaxHint int, sm *srcmap.Map, states map[uint16]State) Region {
	reg := Region{Start: start.Addr, StartLoc: sm.Locate(start.Addr), Budget: budget, Bounded: true, Kind: "visible"}
	s := &solver{
		nodes:     map[uint16]Instr{},
		sinks:     map[uint16]bool{},
		folds:     map[uint16]loopInfo{},
		memo:      map[lkey]result{},
		state:     map[lkey]int{},
		absStates: states, // S3: page-cross penalty resolved from tracked index ranges
		sm:        sm,
		amaxHint:  amaxHint,
	}

	// Collect the region subgraph: from the instruction after `start`, follow the
	// CFG; WSYNC stores are sinks (terminators, not expanded); flow we can't
	// reason about makes the whole region unbounded.
	unbounded := func(reason string) Region { reg.Bounded = false; reg.Reason = reason; return reg }
	if msg := s.collectRegion(instrs, start); msg != "" {
		return unbounded(msg)
	}
	// S1: if the beam is provably in VSYNC/VBLANK at region entry AND the region
	// never stores to $00/$01 (so it can't turn the display on inside itself), it
	// is not a visible-scanline timing risk — skip it soundly. Its only failure
	// mode is total frame-line drift, which is a separate check (ntsc_frame_lines).
	if st, ok := states[start.Addr]; ok && st.displayOff() && !regionTouchesDisplay(s.nodes, states) {
		reg.Kind = "blank"
		// ①: fall through and STILL compute the worst-case. A blank (VSYNC/VBLANK/
		// overscan) region > budget does not tear a visible line, but it adds a
		// scanline (WSYNC halts to the next line) = frame-line drift / screen dip /
		// roll. Previously this returned Worst=0, hiding it from the ∀ proof.
	}
	ctxs := callContexts(instrs, start.Addr)
	// A region inside a subroutine called from more than one place must be walked
	// once PER CALL SITE, not as one merged graph. The flat walk follows the RTS
	// to every return address, so it happily costs a path that enters from call
	// site A and leaves through call site B — an execution that cannot happen. On
	// a two-sprite kernel that positions both players through one shared routine
	// (the ordinary way to write it) that fused path both inflated the worst case
	// and dragged in a `sta VBLANK` belonging to the other context, which flipped
	// the region's classification from blank to visible and reported a violation
	// on a routine that runs entirely inside VBLANK. Measured on a generated
	// two-call clone: 89 cycles and `certified:false`, against 74 and
	// `certified:true` for the identical kernel with one call site.
	//
	// Taking the worst across contexts stays sound — each context is a real
	// execution — while dropping the impossible ones. This used to run only as a
	// fallback when the flat walk found no WSYNC at all, which is exactly the case
	// where it does NOT help: with several call sites the flat walk reaches a sink
	// easily, and answers confidently from a path that does not exist.
	if len(ctxs) > 1 {
		if r := analyzeRegionInContexts(instrs, start, budget, amaxHint, sm, states, ctxs); r != nil {
			return *r
		}
		// Unresolvable in some context: fall through to the flat walk, which is
		// still an over-approximation and therefore still sound.
	}
	if len(s.sinks) == 0 {
		if len(ctxs) > 0 {
			if r := analyzeRegionInContexts(instrs, start, budget, amaxHint, sm, states, ctxs); r != nil {
				return *r
			}
		}
		return unbounded("no WSYNC reached from region start")
	}
	if msg := s.foldLoops(); msg != "" {
		reg = unbounded(msg)
		// If the only thing missing was the trip count, the region's cost is still
		// a known function of it, so the largest count that fits is computable.
		if s.pending != nil {
			reg.Conditional = s.solveObligation(start, budget)
		}
		return reg
	}

	r := s.longest(start.next(), 0)
	if s.cyclic {
		return unbounded("unbounded loop in region (no counted-loop bound found)")
	}
	if s.unbounded {
		if len(ctxs) > 0 {
			if rc := analyzeRegionInContexts(instrs, start, budget, amaxHint, sm, states, ctxs); rc != nil {
				return *rc
			}
		}
		return unbounded(s.unbReason)
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
	Blank      int      `json:"blank,omitempty"`      // regions skipped as VSYNC/VBLANK (not visible-line risks)
	MaxWorst   int      `json:"max_worst"`            // largest proven worst-case among bounded VISIBLE regions
	Certified  bool     `json:"certified"`            // all visible regions bounded AND <= budget
	Violations []Region `json:"violations,omitempty"` // regions whose worst case exceeds the budget
	Unbounded  []Region `json:"unbounded,omitempty"`  // regions that could not be proven (out of scope)
	Lines      []Region `json:"lines,omitempty"`      // PONG-C3: the COMPLETE per-region table (every visible region incl. passing ones, address order) — "trim by the exact margin", not guess-and-assert
	// --- blank (VSYNC/VBLANK/overscan) region accounting (VV-2b: previously skipped as worst=0) ---
	BlankLines     []Region `json:"blank_lines,omitempty"`     // every blank region with its computed worst
	BlankMaxWorst  int      `json:"blank_max_worst,omitempty"` // largest worst among BOUNDED blank regions
	BlankOver      []Region `json:"blank_over,omitempty"`      // blank regions whose worst exceeds budget×@lines (roll risk, not a visible tear)
	BlankUnbounded []Region `json:"blank_unbounded,omitempty"` // blank regions we could not statically bound (e.g. a divide loop over an un-@amax'd RAM accumulator)
	RollFree       bool     `json:"roll_free"`                 // ∀ roll-freedom: EVERY region (blank+visible) is bounded AND within its budget×@lines span

	// Converged reports that the abstract-interpretation fixpoint reached a fixed
	// point rather than stopping at its iteration cap. A capped run leaves
	// UNDER-approximated states, which every downstream consumer treats as sound,
	// so a report with Converged=false proves nothing and must not be certified.
	Converged bool `json:"converged"`

	// BankedDeclined names the reason the image was not analysed at all.
	//
	// Everything in this package keys on a flat 16-bit address, so on a
	// bank-switched cartridge the vectors, the decode and every analysis built on
	// them describe a program that does not exist. Until this field existed the
	// refusal was not real: Prove analysed the fiction anyway and was saved only by
	// the incidental fact that no STA WSYNC was reachable in whichever bank the
	// flat model happened to fold to, which tripped the "0 regions never certify"
	// backstop. That is luck, not a refusal — a banked ROM whose flat fold DID
	// contain a WSYNC would have been analysed and reported on with confidence.
	BankedDeclined string `json:"banked_declined,omitempty"`
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

	// `; @lines N` on the source line that OPENS a WSYNC region declares it spans
	// N scanlines (a 2-line kernel does ~2 lines of CPU work between WSYNCs), so
	// that region's budget is N*budget. Default 1. Greens legitimate 2-line kernels
	// without weakening the proof (an un-annotated over-76 region still flags).
	src, _ := os.ReadFile(asmPath)
	srcLines := strings.Split(string(src), "\n")
	atLinesRe := regexp.MustCompile(`@lines\s+(\d+)`)
	regionLines := func(sa uint16) int {
		ln, ok := sm.Line(sa)
		if !ok {
			return 1
		}
		// Scan the mapped line and the next: DASM maps a labeled WSYNC to its LABEL
		// line, so `@lines N` written on the `sta WSYNC` line sits one line below.
		for i := ln - 1; i <= ln && i < len(srcLines); i++ {
			if i < 0 {
				continue
			}
			if g := atLinesRe.FindStringSubmatch(srcLines[i]); g != nil {
				if n, e := strconv.Atoi(g[1]); e == nil && n >= 1 {
					return n
				}
			}
		}
		return 1
	}
	// ②: `@amax N` on the WSYNC line that opens a region declares the upper bound of
	// that region's divide-loop accumulator, so a ÷N coarse-positioner whose input is
	// a RAM byte (abstract range Top) can still be bounded. 0 = none.
	atAmaxRe := regexp.MustCompile(`@amax\s+(\d+)`)
	regionAmax := func(sa uint16) int {
		ln, ok := sm.Line(sa)
		if !ok {
			return 0
		}
		for i := ln - 1; i <= ln && i < len(srcLines); i++ {
			if i < 0 {
				continue
			}
			if g := atAmaxRe.FindStringSubmatch(srcLines[i]); g != nil {
				if n, e := strconv.Atoi(g[1]); e == nil && n >= 1 {
					return n
				}
			}
		}
		return 0
	}

	rom, err := os.ReadFile(bin)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", bin, err)
	}
	if len(rom) < 6 || len(rom) > 0x10000 {
		return nil, fmt.Errorf("unexpected ROM size %d bytes", len(rom))
	}
	p := newProgram(rom)
	if why := p.declineBanked(bin); why != "" {
		return &Report{Asm: filepath.Base(asmPath), Budget: budget, BankedDeclined: why}, nil
	}
	instrs, entries := p.decodeFromVectors()
	states, converged := computeStates(instrs, entries, p.byteAt) // S1+: VSYNC/VBLANK & value-range tracking (3D: ROM tables)

	var starts []uint16
	for a, in := range instrs {
		if in.isWSYNC() {
			starts = append(starts, a)
		}
	}
	sort.Slice(starts, func(i, j int) bool { return starts[i] < starts[j] })

	rep := &Report{Asm: filepath.Base(asmPath), Budget: budget, Converged: converged}
	for _, sa := range starts {
		reg := analyzeRegion(instrs, instrs[sa], budget*regionLines(sa), regionAmax(sa), sm, states)
		rep.Regions++
		if reg.Kind == "blank" {
			// ①: a blank region no longer vanishes as worst=0. It is not a visible-line
			// tear (so it stays OUT of Lines/MaxWorst/Violations/Certified for backward
			// compatibility), but a blank region over its budget adds a scanline = roll,
			// so surface it and feed the ③ roll_free ∀ verdict.
			rep.Blank++
			rep.BlankLines = append(rep.BlankLines, reg)
			if !reg.Bounded {
				rep.BlankUnbounded = append(rep.BlankUnbounded, reg)
			} else {
				if reg.Worst > rep.BlankMaxWorst {
					rep.BlankMaxWorst = reg.Worst
				}
				if reg.Over {
					rep.BlankOver = append(rep.BlankOver, reg)
				}
			}
			continue
		}
		rep.Lines = append(rep.Lines, reg) // PONG-C3: keep EVERY visible region, passing or not
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
	if rep.Regions == 0 {
		// No reachable STA WSYNC: a bank-switched kernel (display loop in another
		// bank we don't follow) or a non-kernel ROM. Never vacuously certify "0
		// regions, all safe" — that would be the prover lying. Report it.
		rep.Unbounded = append(rep.Unbounded, Region{Budget: budget,
			Reason: "no STA WSYNC reached from the reset/IRQ/NMI vectors — bank-switched or not a single-bank kernel (out of scope)"})
	}
	// A capped fixpoint leaves under-approximated states, so nothing derived from
	// them may be certified — the bound would rest on values the analysis never
	// finished computing.
	rep.Certified = rep.Converged && rep.Regions > 0 && len(rep.Violations) == 0 && len(rep.Unbounded) == 0
	// ③ roll_free: the ∀ roll-freedom verdict — EVERY region (blank AND visible) is
	// bounded and within its budget×@lines span. Stricter than Certified (visible-only):
	// a blank region over budget or unbounded means the frame's line total is NOT
	// statically proven here (it is delegated to the runtime ∃ ntsc_frame_lines check).
	rep.RollFree = rep.Converged && rep.Regions > 0 &&
		len(rep.Violations) == 0 && len(rep.Unbounded) == 0 &&
		len(rep.BlankOver) == 0 && len(rep.BlankUnbounded) == 0
	return rep, nil
}
