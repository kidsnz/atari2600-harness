package emu

import "testing"

// TestASecondHMOVEStrobeDependsOnWhenItLands fills a hole this repository marked itself:
// `fundamentals-audit.md` carries **⬜ double-strobe behavior unmeasured**, while
// `known-traps.md` warns that positioning each object with its own `sta HMOVE` "re-applies every
// current HMxx to ALL objects on every strobe" so the sprite "never settles".
//
// The AtariAge corpus (`198577`, 2012) says something that would make that an emulator artefact:
// *"on real hardware a repeated HMOVE strobe only counts once (emulators appear to accumulate —
// which is wrong)"*. The two were being compared as if they described the same experiment.
//
// ★They do not. Measured here with HMP0 = $70 (one strobe moves P0 by -7), same RESP0 every time,
// reading `Player0.HmovedPixel` one scanline after each treatment settles:
//
//	one strobe                       3 -> 156   (-7, the baseline)
//	two back to back                 3 ->   4   (+1 — NOT one strobe, and NOT two)
//	two 24 cycles apart              3 -> 156   (-7 — the second added nothing)
//	two 24 cycles apart, HMCLR first 3 -> 156   (-7 — the control: HMCLR emptied HMxx)
//
// ★★So within one scanline this engine does NOT accumulate: the spaced pair lands exactly where
// one strobe lands, which is what the AtariAge report describes. And a strobe fired while the
// previous ripple is still running is a **third** outcome, not a doubling — +1 rather than -7 or
// -14, which is the "strange cycle HMOVE" family the archive keeps naming.
//
// ★★★What this does NOT settle, said plainly because the gap is the whole point: the
// `known-traps.md` warning is about strobes on DIFFERENT scanlines with positioning code between
// them, and that is a different experiment from either case here. This ROM measures the
// within-a-line question the ⬜ was about, and leaves the across-lines one open.
//
// Found by the mailing-list distillation (helper-2), cross-checking the two corpora.
func TestASecondHMOVEStrobeDependsOnWhenItLands(t *testing.T) {
	e, err := New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM("../../roms/litmus/litmus_hmove_double.bin"); err != nil {
		t.Fatal(err)
	}
	if err := e.RunFrames(2); err != nil {
		t.Fatal(err)
	}
	if _, err := e.StepFrame(); err != nil {
		t.Fatal(err)
	}
	// The four treatments settle five scanlines apart; sample each one line after it completes.
	at := map[int]struct {
		want int
		what string
	}{
		40: {156, "ONE strobe — the baseline, HMP0 $70 moves P0 by -7 (3 -> 156 with the wrap)"},
		45: {4, "TWO back to back — neither -7 nor -14 but +1: a strobe landing inside the previous " +
			"ripple is a third outcome, not a doubling"},
		50: {156, "TWO 24 cycles apart — the same as one strobe, so this engine does NOT accumulate " +
			"within a line, which is what the AtariAge report describes"},
		55: {156, "TWO 24 cycles apart with HMCLR between — the control: with HMxx emptied the second " +
			"strobe must add nothing, and -7 is the first strobe's own move"},
	}
	for line := 1; line <= 56; line++ {
		if err := e.StepScanline(); err != nil {
			t.Fatalf("scanline %d: %v", line, err)
		}
		tc, ok := at[line]
		if !ok {
			continue
		}
		got := int(e.VCS.TIA.Video.Player0.HmovedPixel)
		if got != tc.want {
			t.Errorf("at scanline %d P0 sits at %d, want %d — %s", line, got, tc.want, tc.what)
		}
	}
	if got := int(e.VCS.TIA.Video.Player0.ResetPixel); got != 3 {
		t.Errorf("RESP0 landed P0 at %d rather than 3, so every figure above is measured from a "+
			"different starting point than the one they were pinned at", got)
	}
}
