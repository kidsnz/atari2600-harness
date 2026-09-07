package emu

import (
	"os"
	"strings"
	"testing"

	"github.com/kidsnz/atari2600-harness/internal/build"
)

// TestBankSwitchCostsStoresPerLine puts a number on Andrew Davie's 2003 packing rule.
//
// His tool 'Banker' imposed a constraint and never said what it bought:
//
//	"a) For any frame, ALL of its sprites must be in a single bank
//	 b) The matrix definition for the frame must be in the same bank as the [sprites]"
//	〔stella-list `200301/msg00229`〕
//
// `bankswitching.md` carried the mechanism and no price. Measured 2026-09-07 by growing the store
// count in a kernel row until the frame stopped holding its line count:
//
//	same bank              9 stores per scanline
//	one switch per line    8            <- the switch pair costs exactly one store
//	a switch per fetch     4            <- less than half the line
//
// That is the numeric reason for rule (a). Reaching across banks per sprite halves what a scanline
// can draw; batching the switch to once per line costs one store.
//
// `roms/litmus/litmus_bank_capacity.asm` runs all three at their maxima and comes to exactly 262
// scanlines. This test adds one more store to the same-bank band and requires the frame to break, so
// the 262 above is a boundary rather than a number that happens to be true.
//
// ★**The reading that had to be corrected**: at one store past the maximum the frame grows by exactly
// **one line**, and only at two past does every kernel line start taking two (the frame grows by the
// whole band). The +1 is not the boundary — it was read as one at first, which made every maximum
// come out one too low. The signature to look for is the jump of a whole band's worth of lines.
//
// ★**A stand-in is used for the hotspot access.** A real F8 switch is `sta $1FF9`, a 4-cycle
// absolute store; the litmus is 4K and uses `lda $A0,x`, also 4 cycles and one memory access, with no
// effect on a zeroed page. What is measured is the TIME a switch costs, which is the quantity the
// rule is about. The switching itself is covered by `litmus_bank`, `litmus_bank_f4`, `litmus_bank_f6`.
//
// ★★**And the first three runs of this measurement were wrong for a reason worth keeping.** The
// variants were generated into a scratch directory outside the module and loaded through an
// environment variable, so `go test` **served cached results** across regeneration — three runs
// reported the same stale numbers while the ROMs on disk had changed twice. `-count=1` is not
// optional when the fixtures live outside what the cache can see.
//
// Found by the mailing-list distillation (helper-2).
func TestBankSwitchCostsStoresPerLine(t *testing.T) {
	lines := func(bin string) int {
		e, err := New("NTSC")
		if err != nil {
			t.Fatal(err)
		}
		if err := e.LoadROM(bin); err != nil {
			t.Fatalf("%s: %v", bin, err)
		}
		n := 0
		for i := 0; i < 10; i++ {
			m, err := e.StepFrame()
			if err != nil {
				t.Fatalf("%s: %v", bin, err)
			}
			if i < 6 {
				continue
			}
			if n == 0 {
				n = m
			} else if m != n {
				t.Fatalf("%s: frame length is not settled (%d then %d)", bin, n, m)
			}
		}
		return n
	}

	const asm = "../../roms/litmus/litmus_bank_capacity.asm"
	if got := lines("../../roms/litmus/litmus_bank_capacity.bin"); got != 262 {
		t.Fatalf("the three bands at 8 / 7 / 4 stores render %d scanlines, want 262. One of the "+
			"three capacities has moved; re-measure before trusting the numbers in "+
			"bankswitching.md", got)
	}

	// The boundary: a ninth store in the same-bank band must not fit.
	src, err := os.ReadFile(asm)
	if err != nil {
		t.Fatal(err)
	}
	marker := "        dey\n        bpl Arow\n"
	if !strings.Contains(string(src), marker) {
		t.Fatal("band A's loop tail is not where this test expects it — the litmus was restructured " +
			"and the boundary check below would be editing the wrong band")
	}
	over := strings.Replace(string(src), marker,
		"        lda Tbl,y\n        sta GRP0\n"+marker, 1)

	dir := t.TempDir()
	overAsm := dir + "/over.asm"
	if err := os.WriteFile(overAsm, []byte(over), 0o644); err != nil {
		t.Fatal(err)
	}
	overBin := dir + "/over.bin"
	if out, err := build.Assemble(overAsm, overBin); err != nil {
		t.Fatalf("the nine-store variant did not assemble: %v\n%s", err, out)
	}
	// One past the maximum grows the frame by one line; two past doubles every line in the band.
	// Either is a break, and requiring "not 262" covers both without pinning which one.
	if got := lines(overBin); got == 262 {
		t.Error("one more store in the same-bank band still renders 262 scanlines. Either the line " +
			"has more room than it did on 2026-09-07, or this check is editing a band that does " +
			"not run — and 9 would no longer be the measured maximum")
	}
}
