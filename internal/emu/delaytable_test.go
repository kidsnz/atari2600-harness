package emu

import (
	"bufio"
	"strconv"
	"strings"
	"testing"

	"github.com/kidsnz/atari2600-harness/internal/build"
)

// symbolsOf assembles the ROM and returns DASM's own symbol table, so the addresses a
// measurement is anchored to cannot drift away from the bytes being measured. Reading them
// from the assembler is the mechanical version of the project's verbatim-quoting rule: a
// hand-copied address is a number that goes stale silently.
func symbolsOf(t *testing.T, asmPath, binPath string) map[string]int {
	t.Helper()
	_, _, sym, err := build.AssembleWithListing(asmPath, binPath)
	if err != nil {
		t.Fatalf("assemble %s: %v", asmPath, err)
	}
	out := map[string]int{}
	sc := bufio.NewScanner(strings.NewReader(sym))
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) < 2 {
			continue
		}
		if v, err := strconv.ParseUint(f[1], 16, 32); err == nil {
			out[f[0]] = int(v)
		}
	}
	if len(out) == 0 {
		t.Fatal("no symbols parsed — the symbol format changed")
	}
	return out
}

// TestDelayTableIsMeasuredNotRemembered measures the 2600's "shortest code for N cycles"
// table in BOTH currencies, because the published versions of it are wrong in one currency
// and their own authors said so.
//
// The table was posted by Andrew Davie 〔stella-list `199805/msg00090`, 1998-05-09 11:07:03〕
// and he corrected one row EIGHT MINUTES later 〔`199805/msg00091`〕:
//
//	>5@2 ... STA $8000,X ...
//	That should be 5@3.  I can't figure a 2 byte non-destructive 5 cycle delay :(
//
// He fixed the row he was looking at. `11@4` contains the SAME `STA $8000,X` and was not
// fixed. Paul Slocum carried the table into the 2600 Cookbook six years later
// 〔`200404/msg00246`〕 still reading `11@4`, under a heading that says in as many words:
//
//	WASTING CYCLES
//	Christopher Tumbler, Chris Wilkson, Andrew Davie
//	 Todo: Verify Andrew's
//
// That Todo is what this test discharges. The rows below are the MEASURED values; the
// `archiveB` column is what the published table says, and the test names every row where
// the two disagree rather than quietly adopting one.
func TestDelayTableIsMeasuredNotRemembered(t *testing.T) {
	const asm = "../../roms/litmus/litmus_delaytable.asm"
	const bin = "../../roms/litmus/litmus_delaytable.bin"
	syms := symbolsOf(t, asm, bin)

	rows := []struct {
		id       string // label prefix
		what     string
		cy, by   int // MEASURED — what this test asserts
		archiveB int // bytes as published; -1 = the archive gives no byte count for this row
	}{
		// --- Davie 1998, in his order ---
		{"D01", "nop", 2, 1, 1},
		{"D02", "lda $80", 3, 2, 2},
		{"D03", "nop / nop", 4, 2, 2},
		{"D04", "sta $8000,x", 5, 3, 3}, // 3 = his own 8-minute correction
		{"D05", "lda ($80,x)", 6, 2, 2},
		{"D06", "pha / pla", 7, 2, 2},
		{"D07", "rol $8000,x", 7, 3, 3},
		{"D08", "lda ($80,x) / nop", 8, 3, 3},
		{"D09", "pha / pla / nop", 9, 3, 3},
		{"D10", "lda ($80,x) / lda $80", 9, 4, 4},
		{"D11", "rol $80 / ror $80", 10, 4, 4},
		{"D12", "sta $8000,x / lda ($80,x)", 11, 5, 4}, // ★the row never corrected
		{"D13", "jsr + rts", 12, 3, 3},                 // 3 = the CALLER's bytes; the rts is shared
		{"D14", "lda ($80,x) x2", 12, 4, 4},
		// --- the 2004 consolidation's additions ---
		{"D15", "lda $80,x  (indexing)", 4, 2, 2},
		{"D16", "lda.w $80  (forced abs)", 4, 3, 3},
		{"D17", "dop $80    (ILLEGAL)", 3, 2, 2},
		{"D18", "dec $2D", 5, 2, 2},
		{"D19", "dec $2D x2", 10, 4, 4},
		{"D20", "pha alone", 3, 1, 1},
	}

	starts, ends := map[int]string{}, map[int]string{}
	for _, r := range rows {
		s, okS := syms[r.id+"_S"]
		e, okE := syms[r.id+"_E"]
		if !okS || !okE {
			t.Fatalf("%s: missing labels in the symbol table", r.id)
		}
		starts[s], ends[e] = r.id, r.id
	}
	// pla alone is bracketed inside D21 (the pha before it balances the stack).
	starts[syms["D21_E2"]], ends[syms["D21_E"]] = "D21b", "D21b"

	e, err := New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM(bin); err != nil {
		t.Fatal(err)
	}

	var tiaBefore, tiaAfter TIARegisters
	var colubkAfterLanding byte
	got := map[string]int{}
	for i := 0; i < 4000; i++ {
		pc := int(e.VCS.CPU.PC.Address())
		if id, ok := ends[pc]; ok {
			if _, seen := got[id]; !seen {
				got[id] = int(e.CyclesSinceMark())
			}
		}
		if pc == syms["Undec_E"] {
			tiaAfter = e.ReadTIARegisters()
		}
		if id, ok := starts[pc]; ok {
			if _, seen := got[id]; !seen {
				e.MarkCycles()
			}
			_ = id
		}
		if pc == syms["Land_E"] {
			colubkAfterLanding = e.ReadTIARegisters().Playfield.BackgroundColor
		}
		if pc == syms["Undec_S"] {
			tiaBefore = e.ReadTIARegisters()
		}
		if err := e.StepInstruction(); err != nil {
			t.Fatalf("step %d: %v", i, err)
		}
	}

	t.Log("delay          measured        published   code")
	disagree := 0
	for _, r := range rows {
		cy, ok := got[r.id]
		if !ok {
			t.Errorf("%s (%s): never reached", r.id, r.what)
			continue
		}
		by := syms[r.id+"_E"] - syms[r.id+"_S"]
		note := ""
		if r.archiveB >= 0 && by != r.archiveB {
			note = "  ★archive says " + strconv.Itoa(r.archiveB) + " bytes"
			disagree++
		}
		t.Logf("%2d cy / %d B   %2d cy / %d B  %-28s%s", r.cy, r.by, cy, by, r.what, note)
		if cy != r.cy {
			t.Errorf("%s (%s): %d cycles, this test documents %d", r.id, r.what, cy, r.cy)
		}
		if by != r.by {
			t.Errorf("%s (%s): %d bytes, this test documents %d", r.id, r.what, by, r.by)
		}
	}

	// pla alone — the 2004 table's "4 cycles, 1 byte".
	if cy, by := got["D21b"], syms["D21_E"]-syms["D21_E2"]; cy != 4 || by != 1 {
		t.Errorf("pla alone: %d cy / %d B, documented 4 cy / 1 B", cy, by)
	}

	// The whole point: exactly ONE row's byte count disagrees with the published table, and
	// it is the row whose twin the author corrected in 1998 and whose own correction never
	// happened. If a future edit makes this zero, the finding has been silently absorbed.
	if disagree != 1 {
		t.Errorf("%d rows disagree with the published byte counts; expected exactly 1 (D12)", disagree)
	}
	if by := syms["D12_E"] - syms["D12_S"]; by != 5 {
		t.Errorf("D12 is %d bytes; the finding is that it is 5, published as 4", by)
	}

	// Davie's second open question, which the thread never answered: "Any comments on the
	// danger of 'writing' to ROM?" There is no ROM at $8000 on a 2600. The address bus is 13
	// bits and A12 selects the cartridge, so $8000 folds to $0000 — the TIA. The ROM writes
	// $44 through `sta $8000,x` with x = $09, and if the fold is real the BACKGROUND COLOUR
	// changes. A delay idiom that silently sets a TIA register is not non-destructive.
	if colubkAfterLanding != 0x44 {
		t.Errorf("`sta $8000,x` with x=$09 left COLUBK at $%02X; the fold to TIA predicts $44",
			colubkAfterLanding)
	}

	// The 2004 claim: "locations $2D-$3F do nothin and aren't decoded". A WRITE claim, and a
	// different axis from the read-side folding. Writing $FF to seven of them must move no
	// write-only TIA register.
	if tiaBefore != tiaAfter {
		t.Errorf("writes to $2D-$3F changed TIA state:\n before %+v\n after  %+v", tiaBefore, tiaAfter)
	}
}
