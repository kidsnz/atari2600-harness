package cyclebound

import (
	"fmt"
	"os"
	"path/filepath"

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

// LintResult is a lint run plus the DENOMINATOR it covered. A linter that prints
// "no timing warnings" tells the author nothing about how much of the program it
// actually read, and the difference is not hypothetical: on an 8K cartridge this
// package used to read 0 instructions and say exactly that. Banks and Instructions
// are what was decoded and surveyed; Declined, when set, means nothing was.
type LintResult struct {
	Warnings     []LintWarning `json:"warnings"`
	Banks        int           `json:"banks"`              // analysis units (1 for a flat image)
	Instructions int           `json:"instructions"`       // decoded across all banks
	PerBank      map[int]int   `json:"per_bank"`           // bank number -> instructions decoded
	Declined     string        `json:"declined,omitempty"` // non-empty: the ROM was not analysed at all
}

// Lint assembles asmPath and returns timing-lint warnings (proactive, static).
func Lint(asmPath string) ([]LintWarning, error) {
	r, err := LintDetail(asmPath)
	if err != nil {
		return nil, err
	}
	return r.Warnings, nil
}

// LintDetail is Lint plus the coverage the run achieved.
func LintDetail(asmPath string) (LintResult, error) {
	bin := build.BinPathFor(asmPath)
	out, lst, sym, err := build.AssembleWithListing(asmPath, bin)
	if err != nil {
		return LintResult{}, fmt.Errorf("assemble %s failed:\n%s", asmPath, out)
	}
	sm := srcmap.Parse(lst, sym, asmPath)
	rom, err := os.ReadFile(bin)
	if err != nil {
		return LintResult{}, err
	}
	if len(rom) < 6 || len(rom) > 0x10000 {
		return LintResult{}, fmt.Errorf("unexpected ROM size %d", len(rom))
	}
	// A bank-switched cartridge is linted ONE BANK AT A TIME and then surveyed as one
	// merged program keyed on (bank, address) — the same pipeline the prover runs, so
	// the two tools read the same program. Before this, the linter took the earlier
	// `declineBanked` exit and analysed NOTHING on an 8K+ cartridge: measured on
	// banked_game.asm, 0 of 133 instructions were read and the only output was the
	// refusal, i.e. every 8K game had zero static timing coverage.
	//
	// A flat ROM has exactly one unit and goes down an identical path, which is why
	// all 30 flat kernels' warnings are unchanged.
	units, decline := analysisUnits(rom, bin)
	if decline != "" {
		return LintResult{Declined: decline, Warnings: []LintWarning{{
			Rule: "not-analysed",
			Loc:  filepath.Base(asmPath),
			Msg:  decline,
			Hint: "no timing warning below should be read as an all-clear for this ROM",
		}}}, nil
	}
	decodes, instrs, entries, _ := decodeUnits(units)
	perBank := map[int]int{}
	for _, d := range decodes {
		perBank[d.bank] = len(d.instrs)
	}

	sw := switchModel{banked: len(units) > 1, banks: map[int]bool{}}
	if sw.banked {
		sw.hotspots = units[0].hotspots
		for _, u := range units {
			sw.banks[u.bank] = true
		}
	}
	// Abstract-interpret so R2 can tell a real staged motion from a defensive
	// `lda #0; sta HMP0` clear (the latter must not warn). On a banked image the
	// landing sites of a switch this analysis does not model are forced to Top, exactly
	// as the prover does — an unknown value never warns, so widening can only silence.
	widen, _ := unmodelledLandings(instrs, sw)
	states, _ := computeStates(instrs, entries, romByBank(decodes), sw, widen)

	// On a banked image NO bank gets a source location — not even bank 0.
	//
	// DASM's listing address column is the PHYSICAL ROM OFFSET, so bank 0's rows sit
	// below $1000 and srcmap.Parse drops them; that much was already known. What is
	// NOT true is the follow-on assumption that bank 0 therefore falls back safely:
	// the LABEL table comes from the symbol dump, where every bank's labels carry
	// their RORG'd $F0xx address, so the two banks' labels are interleaved in one
	// list and "the last label at or before this address" can be either bank's.
	// Measured on lint_bank_split: a warning at BANK 0 $F00A resolved to
	// "LvTab+10" — LvTab is a bank 1 label, 4K away in the image. A wrong file
	// location is worse than none, because the author goes and reads correct code.
	//
	// A per-bank line map is recoverable (bank = listing offset >> 12) and is filed
	// as a follow-up; until it exists, a banked site prints as "bank N $FFxx".
	banked := sw.banked
	loc := func(a site) string {
		if banked {
			return siteDesc(a, true)
		}
		return sm.Locate(a.addr)
	}
	var w []LintWarning

	// Survey TIA motion usage across the whole program — every bank at once, which is
	// what R1 and R2 require to stay false-positive-free on a banked cartridge. Both
	// ask an "is it EVER" question, and the answer can live in another bank: a kernel
	// that strobes HMOVE in bank 1 while staging HMxx in bank 0 is correct, and a
	// per-bank survey would warn on it twice.
	var hmoves []site
	sawHMOVE, sawHMxxAny := false, false
	hmxxNonzero, haveHmxxNonzero := site{}, false // first HMxx store proven to stage a NON-zero value
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
			if v, ok := storedReg(in, states[a]); ok && !v.Top && v.Lo > 0 {
				if !haveHmxxNonzero || a.addr < hmxxNonzero.addr {
					hmxxNonzero, haveHmxxNonzero = a, true
				}
			}
		}
	}
	sortSites(hmoves)

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
	if haveHmxxNonzero && !sawHMOVE {
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
		if hazAddr, after, hit := straightHMOVEHazard(instrs, h, sw, states); hit {
			w = append(w, LintWarning{
				Rule: "hmove-hazard", Loc: loc(hazAddr),
				Msg:  fmt.Sprintf("HMxx/HMCLR written ~%d CPU cycles after this HMOVE (<24) — motion becomes undefined", after),
				Hint: "separate the HMOVE and the next motion-register write with a WSYNC (≥24 cycles)",
			})
		}
	}
	return LintResult{Warnings: w, Banks: len(units), Instructions: len(instrs), PerBank: perBank}, nil
}

// straightHMOVEHazard walks the straight-line path after an HMOVE at addr, summing
// CPU cycles, and reports the address of the first HMxx/HMCLR write reached within
// <24 cycles (the hazard window). Stops at any branch/jump/WSYNC (can't continue
// statically) — staying silent there rather than risk a false positive.
//
// On a banked cartridge it also stops at any instruction that CAN cross banks. The
// walk follows fall-through addresses inside one bank; after a hotspot access the
// next fetch comes from the OTHER bank, so the bytes at the following address in this
// one are not the instruction that executes. Counting their cycles would measure a
// gap along a path the hardware never takes, in either direction.
func straightHMOVEHazard(instrs map[site]Instr, at site, sw switchModel, states map[site]State) (hazAddr site, after int, hit bool) {
	cyc := 0 // CPU cycles elapsed from the end of HMOVE to the START of `in`
	a := instrs[at].nextSite()
	for {
		in, ok := instrs[a]
		if !ok || in.isWSYNC() {
			return site{}, 0, false // a WSYNC safely closes the window
		}
		if edges, keep, refusal := sw.switchEdges(in, states[a]); len(edges) > 0 || !keep || refusal != "" {
			return site{}, 0, false // can cross banks — the straight line ends here
		}
		if in.Def.IsBranch() || in.Def.Operator == instructions.JMP ||
			in.Def.Operator == instructions.JSR || in.Def.Operator == instructions.RTS {
			return site{}, 0, false // can't follow statically — stay silent (no false positive)
		}
		if cyc >= 24 {
			return site{}, 0, false // this instruction starts at/after 24cy — out of the window
		}
		// Evaluate at the START of `in`: the gap is the cycles BEFORE it, so the
		// standard `sta HMOVE; ds 12 ($EA*12 = 24cy); sta HMCLR` idiom (HMCLR starts
		// at exactly 24cy) is safe, while `ds 11` (22cy) is correctly flagged.
		if reg, ok := storeTIA(in); ok && (isHMxx(reg) || reg == regHMCLR) {
			return a, cyc, true // hazard: HMxx/HMCLR write starts < 24cy after HMOVE
		}
		cyc += in.Def.Cycles
		a = in.nextSite()
	}
}
