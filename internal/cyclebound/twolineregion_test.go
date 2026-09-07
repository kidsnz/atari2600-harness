package cyclebound

import (
	"testing"
)

// TestTwoLineRegionHidesAPerLineOverrun is a negative control for this package's own prover.
//
// `roms/techniques/score6.asm`'s six-store choreography must finish inside ONE scanline — the
// source says so at the branch: `bpl Krow ; 72 (<76 で行内完結)`. `roms/litmus/litmus_store7_overrun.asm`
// is that file with one extra `lda (zp),y` + `sta GRPn`, eight cycles the line does not have.
//
// Measured 2026-09-07:
//
//	                    Krow worst / budget      Prove says       rendered frame
//	score6                93 / 152               certified        262 scanlines
//	litmus_store7_overrun 102 / 152               CERTIFIED        269 scanlines
//
// **The prover certifies a kernel that adds seven scanlines to the frame.** The region begins with
// `sta WSYNC`, so it is treated as spanning two lines and given 152 cycles; an eight-cycle overrun of
// the *inner* line vanishes into that allowance. Nothing here is broken by accident — a two-line
// region is a real thing and 152 is the right budget for one — but the consequence is that
// `prove_line_budget` **cannot be the only check on a loop whose region starts with a WSYNC**. The
// frame-length check (`ntsc_frame_lines`, `frame_lines_stable`) is what catches this, and this test
// exists so that fact is measured rather than assumed.
//
// The seventh store also buys nothing: the rendered band is 46 px wide either way. The "6" in
// "six-store choreography" was never a cycle budget — it is two players times three NUSIZ copies, and
// there is no seventh place to put a seventh image. That is the answer to the question this litmus
// was built for, raised by the mailing-list distillation (helper-2): *"is there room in the 76 cycles
// for a 7th store?"* There is room, and it does not help.
func TestTwoLineRegionHidesAPerLineOverrun(t *testing.T) {
	krow := func(asm string) (worst, budget int, certified bool) {
		r, err := Prove(asm, 76)
		if err != nil {
			t.Fatalf("%s: %v", asm, err)
		}
		for _, l := range r.Lines {
			if len(l.StartLoc) >= 4 && l.StartLoc[:4] == "Krow" {
				return l.Worst, l.Budget, r.Certified
			}
		}
		t.Fatalf("%s: no Krow region in the report — the kernel was renamed and this control is "+
			"pointing at nothing", asm)
		return
	}

	w6, b6, c6 := krow("../../roms/techniques/score6.asm")
	w7, b7, c7 := krow("../../roms/litmus/litmus_store7_overrun.asm")

	if b6 != 152 || b7 != 152 {
		t.Fatalf("the Krow regions no longer get a two-line budget (%d and %d, want 152 each). "+
			"That allowance is the whole subject of this test", b6, b7)
	}
	if w7 <= w6 {
		t.Errorf("the seven-store kernel measures %d cycles against the six-store kernel's %d — "+
			"the extra load and store are supposed to cost about eight", w7, w6)
	}
	if !c6 {
		t.Error("score6 is no longer certified, so it has stopped being the control this test " +
			"compares against")
	}
	// The point: the prover says yes to a kernel the machine says no to.
	if !c7 {
		t.Error("the prover now REJECTS the seven-store kernel. That is better than it was on " +
			"2026-09-07, when it certified a kernel that renders 269 scanlines — but it means the " +
			"gap this control documents has been closed, and the note in " +
			"litmus_store7_overrun.asm should be rewritten rather than left describing a hole " +
			"that no longer exists")
	}
}
