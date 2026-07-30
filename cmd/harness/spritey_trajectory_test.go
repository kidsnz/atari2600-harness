package main

import (
	"context"
	"testing"

	"github.com/kidsnz/atari2600-harness/internal/emu"
)

// The measured shape of roms/litmus/motion_xclamp.bin. The ROM asks for XSTART=20 and
// XMAX=100 in the SetXPos routine's units; the position the hardware actually reaches is
// 9px lower, because the offset in X(N) includes that kernel's own prologue. Per CLAUDE.md
// the verdict is the MEASURED HmovedPixel, so these are measured values, not a formula.
const (
	xcFirst   = 2   // samples 0-1 are the power-on transient (no stable frame yet)
	xcFirstX  = 13  // X at sample xcFirst
	xcStep    = 2   // px per frame during the glide
	xcClamp   = 91  // the clamp (ROM's XMAX=100)
	xcClampAt = 41  // first sample that reaches the clamp
	xcResetAt = 121 // the round ends here and the sprite snaps back
	xcResetX  = 11  // where it snaps back to (ROM's XSTART=20)
	xcYTop    = 80  // the band is fixed: Y must not move while X does
	xcYBot    = 119
)

func loadXClamp(t *testing.T) *emu.Emu {
	t.Helper()
	mu.Lock()
	defer mu.Unlock()
	e, err := emu.New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM("../../roms/litmus/motion_xclamp.bin"); err != nil {
		t.Fatal(err)
	}
	current = e // the package-level machine `get()` hands out
	return e
}

// TestSpriteYTrajectoryIsLiveInBothAxes is the witness for spritey's multi-frame mode.
//
// The mode returns a per-frame sample carrying BOTH X and Y, and has since the tool
// existed — but nothing ever checked that the X in those samples MOVES. A build that
// reported a constant X, or that reported the same number into both axes, would have
// passed every test in this repo. On 2026-07-30 the description was corrected to name the
// X trajectory; a description is not a check, so this is the check.
//
// motion_xclamp is the horizontal mirror of motion_glide (which moves Y and pins X): here
// X glides, clamps and resets while Y is a fixed band. Asserting both halves is what makes
// this non-vacuous — a cross-wired axis fails the Y half even if the X half looks alive.
func TestSpriteYTrajectoryIsLiveInBothAxes(t *testing.T) {
	loadXClamp(t)

	_, out, err := handleSpriteY(context.Background(), nil, SpriteYIn{Object: "P0", Frames: xcResetAt + 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Samples) != xcResetAt+1 {
		t.Fatalf("asked for %d samples, got %d", xcResetAt+1, len(out.Samples))
	}

	// --- X is LIVE: strictly increasing, by exactly the glide step, until the clamp.
	for i := xcFirst; i < xcClampAt; i++ {
		got, next := out.Samples[i].X, out.Samples[i+1].X
		if next-got != xcStep {
			t.Fatalf("glide broke at sample %d: X %d -> %d (want +%d). If every step is 0 the "+
				"X in a multi-frame sample is not tracking anything", i, got, next, xcStep)
		}
	}
	if got := out.Samples[xcFirst].X; got != xcFirstX {
		t.Errorf("glide starts at X=%d, want %d", got, xcFirstX)
	}

	// --- The clamp: X stops, and stays stopped, at the ROM's limit.
	for i := xcClampAt; i < xcResetAt; i++ {
		if got := out.Samples[i].X; got != xcClamp {
			t.Fatalf("sample %d: X=%d, want the clamp %d — the plateau is where a horizontal "+
				"limit is actually read off", i, got, xcClamp)
		}
	}

	// --- Y does NOT move while X does. A tool that reported one axis into the other, or
	// that recomputed Y from the moving X, fails here and passes everything above.
	for i := xcFirst; i <= xcResetAt; i++ {
		s := out.Samples[i]
		if !s.Present {
			t.Fatalf("sample %d: P0 not present — the band is drawn on every frame", i)
		}
		if s.YTop != xcYTop || s.YBot != xcYBot {
			t.Fatalf("sample %d: Y moved to %d-%d (want a fixed %d-%d) while X was gliding — "+
				"the two axes are not independent", i, s.YTop, s.YBot, xcYTop, xcYBot)
		}
	}
}

// TestSpriteYTrajectoryBeatsASingleLateRead pins the reason the multi-frame mode is worth
// preferring, as a property rather than as advice in a description.
//
// The failure it guards against was real and expensive. Outlaw's horizontal clamp was
// measured by holding "right" for 700 frames and reading the position ONCE: that returned
// x=7, near the LEFT edge, because the round had ended and the gunman was back at his
// start. No error, perfectly stable, plausible, wrong. motion_xclamp reproduces that shape
// with known constants: read it late and you measure the round that has already restarted;
// sample it and the clamp is unmistakable.
//
// Note what liveness does NOT cover here. This program is reacting the whole time — a
// liveness probe would report "responding" at every one of these frames. Liveness answers
// "is it running", not "am I still in the situation I set up", and only the second question
// is the one a late read gets wrong.
func TestSpriteYTrajectoryBeatsASingleLateRead(t *testing.T) {
	// The late single read: advance well past the end of the round, then read once.
	const late = xcResetAt + 9
	e := loadXClamp(t)
	mu.Lock()
	for i := 0; i < late; i++ {
		if _, err := e.StepFrame(); err != nil {
			mu.Unlock()
			t.Fatal(err)
		}
	}
	mu.Unlock()
	_, single, err := handleSpriteY(context.Background(), nil, SpriteYIn{Object: "P0"})
	if err != nil {
		t.Fatal(err)
	}
	if len(single.Samples) != 1 {
		t.Fatalf("frames unset should read the current frame only, got %d samples", len(single.Samples))
	}
	lateX := single.Samples[0].X

	// The trajectory over the very same span, from the same starting state.
	loadXClamp(t)
	_, traj, err := handleSpriteY(context.Background(), nil, SpriteYIn{Object: "P0", Frames: late + 1})
	if err != nil {
		t.Fatal(err)
	}
	// Only samples where the object is actually drawn count. The first version of this
	// test scanned all of them and "found" a peak of 107 — the power-on transient, before
	// the ROM has rendered a stable frame, which reports present=false and a stale X.
	// Taking a maximum over not-present samples is the same class of error as the late
	// single read: a number that is there, and means nothing.
	maxX := 0
	for _, s := range traj.Samples {
		if s.Present && s.X > maxX {
			maxX = s.X
		}
	}

	if traj.Samples[late].X != lateX {
		t.Errorf("the two paths disagree at frame %d (%d vs %d) — they must be the same machine "+
			"state, or this comparison proves nothing", late, traj.Samples[late].X, lateX)
	}
	if maxX != xcClamp {
		t.Errorf("trajectory max X = %d, want the clamp %d", maxX, xcClamp)
	}
	if lateX >= maxX {
		t.Fatalf("VACUOUS: the single late read (%d) already found the clamp (%d), so this ROM no "+
			"longer reproduces the trap it exists for", lateX, maxX)
	}
	if lateX > xcClamp/2 {
		t.Errorf("the late read returned %d; the trap is only convincing while it lands far below "+
			"the clamp %d", lateX, xcClamp)
	}
	t.Logf("single late read at frame %d: X=%d — the trajectory over the same span peaked at %d "+
		"(%d px higher)", late, lateX, maxX, maxX-lateX)
}
