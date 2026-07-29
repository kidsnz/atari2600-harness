package cyclebound

import (
	"fmt"
	"os"
	"sort"

	"github.com/jetsetilly/gopher2600/hardware/cpu/instructions"
	"github.com/kidsnz/atari2600-harness/internal/build"
	"github.com/kidsnz/atari2600-harness/internal/srcmap"
)

// Timing linter (authoring aid). Statically reads a kernel and warns — BEFORE you
// run it — about a few high-confidence TIA-timing pitfalls that the author has
// repeatedly hit. The bar is ZERO false positives on known-good kernels: a rule
// only fires when it is unambiguously a mistake. It complements the runtime
// checks (assert_line_budget / VV-10 HMOVE hazard) by being proactive.
//
// Canonical TIA write registers used here:
//
//	$20-$24 = HMP0/HMP1/HMM0/HMM1/HMBL (horizontal motion)   $2A = HMOVE strobe
//	$2B = HMCLR (clear motion)                                $25-$27 = VDELP0/P1/BL
const (
	regHMP0  = 0x20
	regHMBL  = 0x24
	regHMOVE = 0x2A
	regHMCLR = 0x2B
)

// LintWarning is one timing-lint finding.
type LintWarning struct {
	Rule string `json:"rule"`
	Loc  string `json:"loc"`  // "Label+off (file:line)"
	Msg  string `json:"msg"`  // the pitfall
	Hint string `json:"hint"` // how to fix
}

// storeTIA reports the canonical TIA write-register *base* address of a store
// instruction (STA/STX/STY to the TIA area, $00-$3F after mirror-folding), if any.
// Indexed stores (`sta HMP0,x` / `sta HMM0,y`) are included and return the BASE
// register — that is how the kernels here stage motion for several objects through
// one code path (HMP0,x with x=0..4 covers HMP0..HMBL). For the surveys below
// (does the kernel ever touch HMxx / is there a hazard) the base register is the
// right thing to reason about, and is sound: a missed index never makes us claim a
// write happened that didn't.
func storeTIA(in Instr) (uint16, bool) {
	switch in.Def.Operator {
	case instructions.STA, instructions.STX, instructions.STY:
	default:
		return 0, false
	}
	switch in.Def.AddressingMode {
	case instructions.Absolute, instructions.AbsoluteX, instructions.AbsoluteY:
	default:
		return 0, false
	}
	// DASM folds zero-page into Absolute; TIA write regs live at $00-$2C, mirrored.
	// in.Operand is the base address for indexed modes (the value comes from A/X/Y).
	if in.Operand >= 0x80 {
		return 0, false
	}
	return in.Operand & 0x3F, true
}

// storedReg returns the value range of the register feeding a store (A for STA,
// X for STX, Y for STY) in the given pre-state — i.e. the value actually written.
func storedReg(in Instr, s State) (ValueRange, bool) {
	if !s.valid {
		return ValueRange{}, false
	}
	switch in.Def.Operator {
	case instructions.STA:
		return s.A, true
	case instructions.STX:
		return s.X, true
	case instructions.STY:
		return s.Y, true
	}
	return ValueRange{}, false
}

func isHMxx(reg uint16) bool { return reg >= regHMP0 && reg <= regHMBL }

// Lint assembles asmPath and returns timing-lint warnings (proactive, static).
func Lint(asmPath string) ([]LintWarning, error) {
	bin := build.BinPathFor(asmPath)
	out, lst, sym, err := build.AssembleWithListing(asmPath, bin)
	if err != nil {
		return nil, fmt.Errorf("assemble %s failed:\n%s", asmPath, out)
	}
	sm := srcmap.Parse(lst, sym, asmPath)
	rom, err := os.ReadFile(bin)
	if err != nil {
		return nil, err
	}
	if len(rom) < 6 || len(rom) > 0x10000 {
		return nil, fmt.Errorf("unexpected ROM size %d", len(rom))
	}
	p := &program{rom: rom, base: uint16(0x10000 - len(rom))}
	instrs := map[uint16]Instr{}
	var entries []uint16
	for _, va := range []uint16{0xFFFC, 0xFFFA, 0xFFFE} {
		lo, _ := p.byteAt(va)
		hi, _ := p.byteAt(va + 1)
		if t := uint16(lo) | uint16(hi)<<8; t >= p.base {
			p.decodeInto(instrs, t)
			entries = append(entries, t)
		}
	}
	// Abstract-interpret so R2 can tell a real staged motion from a defensive
	// `lda #0; sta HMP0` clear (the latter must not warn).
	states := computeStates(instrs, entries, p.byteAt)

	loc := func(a uint16) string { return sm.Locate(a) }
	var w []LintWarning

	// Survey TIA motion usage across the whole program.
	var hmoves []uint16
	sawHMOVE, sawHMxxAny := false, false
	hmxxNonzero := uint16(0xFFFF) // first HMxx store proven to stage a NON-zero value
	for a, in := range instrs {
		reg, ok := storeTIA(in)
		if !ok {
			continue
		}
		switch {
		case reg == regHMOVE:
			sawHMOVE = true
			hmoves = append(hmoves, a)
		case isHMxx(reg):
			sawHMxxAny = true
			// Provably non-zero staged motion? (value range excludes 0). A defensive
			// clear (`lda #0; sta HMPx`) is provably 0 → never counted (no false warn);
			// an unknown/computed value stays silent → conservative.
			if v, ok := storedReg(in, states[a]); ok && !v.Top && v.Lo > 0 && a < hmxxNonzero {
				hmxxNonzero = a
			}
		}
	}
	sort.Slice(hmoves, func(i, j int) bool { return hmoves[i] < hmoves[j] })

	// R1: HMOVE strobed but no motion register is EVER set => HMOVE is a no-op
	// (every object's fine adjust is 0). Almost always a forgotten HMxx setup.
	if sawHMOVE && !sawHMxxAny {
		w = append(w, LintWarning{
			Rule: "hmove-without-hmxx", Loc: loc(hmoves[0]),
			Msg:  "HMOVE is strobed but no HMP0/HMP1/HMM0/HMM1/HMBL is ever written — the fine motion is always 0",
			Hint: "set the HMxx motion register(s) before the HMOVE, or drop the HMOVE",
		})
	}
	// R2: a PROVABLY NON-ZERO motion is staged but HMOVE is NEVER strobed => the
	// motion is computed but never applied. Only fires on motion proven non-zero
	// (a `lda #0; sta HMPx` clear or an unknown value never warns).
	if hmxxNonzero != 0xFFFF && !sawHMOVE {
		w = append(w, LintWarning{
			Rule: "hmxx-without-hmove", Loc: loc(hmxxNonzero),
			Msg:  "a non-zero horizontal motion (HMxx) is staged but HMOVE is never strobed — the motion is never applied",
			Hint: "strobe HMOVE (right after a WSYNC) to apply the staged motion",
		})
	}

	// R3 (static HMOVE hazard): an HMxx/HMCLR write reached within <24 CPU cycles
	// of an HMOVE on a straight-line path leaves motion undefined (Stella PG). The
	// proactive sibling of the runtime VV-10 detector.
	for _, h := range hmoves {
		if hazAddr, after, hit := straightHMOVEHazard(instrs, h); hit {
			w = append(w, LintWarning{
				Rule: "hmove-hazard", Loc: loc(hazAddr),
				Msg:  fmt.Sprintf("HMxx/HMCLR written ~%d CPU cycles after this HMOVE (<24) — motion becomes undefined", after),
				Hint: "separate the HMOVE and the next motion-register write with a WSYNC (≥24 cycles)",
			})
		}
	}
	return w, nil
}

// straightHMOVEHazard walks the straight-line path after an HMOVE at addr, summing
// CPU cycles, and reports the address of the first HMxx/HMCLR write reached within
// <24 cycles (the hazard window). Stops at any branch/jump/WSYNC (can't continue
// statically) — staying silent there rather than risk a false positive.
func straightHMOVEHazard(instrs map[uint16]Instr, addr uint16) (hazAddr uint16, after int, hit bool) {
	cyc := 0 // CPU cycles elapsed from the end of HMOVE to the START of `in`
	a := instrs[addr].next()
	for {
		in, ok := instrs[a]
		if !ok || in.isWSYNC() {
			return 0, 0, false // a WSYNC safely closes the window
		}
		if in.Def.IsBranch() || in.Def.Operator == instructions.JMP ||
			in.Def.Operator == instructions.JSR || in.Def.Operator == instructions.RTS {
			return 0, 0, false // can't follow statically — stay silent (no false positive)
		}
		if cyc >= 24 {
			return 0, 0, false // this instruction starts at/after 24cy — out of the window
		}
		// Evaluate at the START of `in`: the gap is the cycles BEFORE it, so the
		// standard `sta HMOVE; ds 12 ($EA*12 = 24cy); sta HMCLR` idiom (HMCLR starts
		// at exactly 24cy) is safe, while `ds 11` (22cy) is correctly flagged.
		if reg, ok := storeTIA(in); ok && (isHMxx(reg) || reg == regHMCLR) {
			return a, cyc, true // hazard: HMxx/HMCLR write starts < 24cy after HMOVE
		}
		cyc += in.Def.Cycles
		a = in.next()
	}
}
