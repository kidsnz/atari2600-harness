package cyclebound

// The divide idiom's entry bound now comes from the edge, and the proxy holding it up
// was load-bearing.
//
// `determineBound`'s BCS/BCC path found A's entry bound with
// `in.nextSite() == header && at.addr < header.addr` — textual fall-through plus
// address order, the proxy SD-9 deleted from the dex/dey path and left alive here.
// Behind it sat a second guess: "the closest `lda #imm` below the header".
//
// The filter was wrong in both directions at once, measured on `litmus_divpred`, all
// three with `certified: true`:
//
//	MISSED   a predecessor arriving by `jmp` is not `nextSite() == header`  27 vs 87
//	PROXY    nothing adjacent, so the `lda #imm` fallback answers           28 vs 87
//	PHANTOM  a `jmp` sits before the header and is read as a predecessor    29 vs 89
//
// **And the proxy was not idle.** Its counter measured 0 uses, which was a fact about
// the eight ROMs the counting test listed. Across the corpus, NINE divide folds were
// bounded by it — and it reads `lda #80` while ignoring the `adc #XCAL` two
// instructions later, so it answered 7 iterations where the sound bound is 19. Those
// nine were above the machine by luck, not by proof.
//
// Removing the proxy exposed the precision gap that made it necessary: `adcRange`
// returned Top on wrap, and `XCAL = -5` assembles to $FB, so the ordinary calibration
// `lda #80 / clc / adc #XCAL` computes 331 and gave up. A wrapped sum is still a BYTE,
// so [0,255] is true and useful where Top is true and useless. With both changes the
// nine bound at 118 instead of 53-63 — sound instead of lucky.

import (
	"testing"

	"github.com/kidsnz/atari2600-harness/internal/emu"
)

func TestDivideEntryBoundComesFromTheEdgeState(t *testing.T) {
	const asm = "../../roms/litmus/litmus_divpred.asm"

	before := divEdgeScanned
	rep, err := Prove(asm, 76)
	if err != nil {
		t.Fatalf("prove: %v", err)
	}
	// PREMISE — the fixture must actually reach the scan under test.
	if got := divEdgeScanned - before; got < 3 {
		t.Fatalf("only %d divide folds took the edge scan; the fixture holds four and at least the "+
			"three danger rows must reach it, or it witnesses nothing", got)
	}

	region := func(name string) (Region, bool) {
		for _, r := range append(append([]Region{}, rep.Lines...), rep.Unbounded...) {
			if len(r.StartLoc) >= len(name) && r.StartLoc[:len(name)] == name {
				return r, true
			}
		}
		return Region{}, false
	}

	e, err := emu.New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM(binFor(asm)); err != nil {
		t.Fatal(err)
	}
	if err := e.RunFrames(3); err != nil {
		t.Fatal(err)
	}
	rows, _, err := e.ProfileLineWorst(12, nil)
	if err != nil {
		t.Fatal(err)
	}
	measured := map[uint16]emu.LineWorst{}
	for _, r := range rows {
		if r.Count > 0 {
			measured[r.StrobePC] = r
		}
	}

	for _, c := range []struct{ name, why string }{
		{"DivDanger", "the big-A arm reaches the header by `jmp`, which the fall-through filter could " +
			"not see, so the maximum was taken over the small arm alone"},
		{"ProxyDanger", "nothing falls through into this header, so the `lda #imm` proxy answered from " +
			"a constant belonging to the other arm"},
		{"PhantomDanger", "a `jmp` sits immediately before this header, so its nextSite() IS the header " +
			"although control never flows there — a value the machine never holds"},
		{"DivCtl", "a single entry falling through into the header: the shape every divide fold in the " +
			"corpus has, and the reason none of them was wrong. It must keep its bound"},
	} {
		r, ok := region(c.name)
		if !ok {
			t.Errorf("no %s region", c.name)
			continue
		}
		if !r.Bounded {
			t.Errorf("%s is refused (%s) — %s", c.name, r.Reason, c.why)
			continue
		}
		row, hit := measured[r.Start]
		if !hit {
			t.Errorf("%s produced no measured interval, so its bound is graded by nothing", c.name)
			continue
		}
		if r.Worst < row.WorstCycles {
			t.Errorf("%s: proven %d, machine %d — an UNDER-approximation. %s",
				c.name, r.Worst, row.WorstCycles, c.why)
		}
	}
}

// TestAdcWrapKeepsAByteRange pins the precision half. `adcRange` returned Top whenever
// the sum exceeded 255, which is numerically the same claim as [0,255] and structurally
// a much worse one: Top makes every consumer refuse, while a byte range still bounds a
// loop. `XCAL = -5` assembles to $FB, so `lda #80 / clc / adc #XCAL` hit it — and nine
// divide folds in the corpus were then rescued by the address proxy instead.
func TestAdcWrapKeepsAByteRange(t *testing.T) {
	for _, c := range []struct {
		name     string
		a, m     ValueRange
		carry    TriBool
		wantTop  bool
		wantLo   int
		wantHi   int
	}{
		{"no wrap, exact", vConst(80), vConst(10), triFalse, false, 90, 90},
		{"wrap: 80 + $FB (the -5 calibration)", vConst(80), vConst(0xFB), triFalse, false, 0, 255},
		{"wrap at the boundary", vConst(255), vConst(1), triFalse, false, 0, 255},
		{"exactly 255 does not wrap", vConst(250), vConst(5), triFalse, false, 255, 255},
		{"unknown addend stays unknown", vConst(80), vTop(), triFalse, true, 0, 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := adcRange(c.a, c.m, c.carry)
			if got.Top != c.wantTop {
				t.Fatalf("Top = %v, want %v", got.Top, c.wantTop)
			}
			if c.wantTop {
				return
			}
			if got.Lo != c.wantLo || got.Hi != c.wantHi {
				t.Errorf("range [%d,%d], want [%d,%d]", got.Lo, got.Hi, c.wantLo, c.wantHi)
			}
		})
	}
}
