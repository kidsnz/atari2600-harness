package emu

import "testing"

// Grading for `roms/litmus/litmus_missile_novdel.asm`.
//
// Closes the 📖 in `docs/fundamentals-audit.md`: "Missiles have **no** vertical delay
// (so in a 2LK they start only on even lines)". Layers (1), (2) and (3) all returned 0
// for it, so nothing in the tree checked it.
//
// The claim only means something next to an object that DOES have the delay. The ball
// does: `litmus_vdel_cross` measured that with VDELBL set, ENABL's new copy is invisible
// until a GRP1 write latches it. If missiles shared that path there would be a VDELM
// register and the same wait.
//
// Three bands under identical enables:
//
//	A  every VDEL bit set, both enabled, no GRP write   missile LIT, ball DARK
//	B  VDELBL clear, both enabled                       both LIT   (the fixture does enable both)
//	C  VDELBL set, then GRP1 written                    both LIT   (the ball was waiting, not off)
//
// B and C are what make A readable. Without B, "ball dark" could mean the fixture never
// enabled it; without C, it could mean the ball is broken rather than delayed.

const (
	mnvMissileInk = "FFFFFE" // COLUP0 = $0E
	mnvBallInk    = "EC3333" // COLUPF = $46

	mnvBand0Row = 9
	mnvBandLen  = 10
	mnvRead     = 6
)

func loadMissileNoVdel(t *testing.T) (*Emu, int) {
	t.Helper()
	e, err := New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM("../../roms/litmus/litmus_missile_novdel.bin"); err != nil {
		t.Fatal(err)
	}
	if err := e.RunFrames(10); err != nil {
		t.Fatal(err)
	}
	_, top := e.Snapshot()
	return e, top
}

func mnvLit(t *testing.T, e *Emu, top, band int, ink string) int {
	t.Helper()
	n := 0
	for i := 0; i < mnvRead; i++ {
		runs, _, err := e.ReadRow(top + mnvBand0Row + band*mnvBandLen + i)
		if err != nil {
			continue
		}
		for _, r := range runs {
			if r.Hex == ink {
				n++
			}
		}
	}
	return n
}

// TestMissileHasNoDelayPathWhileTheBallDoes is the measurement. One band, two objects,
// identical enables, opposite answers.
func TestMissileHasNoDelayPathWhileTheBallDoes(t *testing.T) {
	e, top := loadMissileNoVdel(t)
	m := mnvLit(t, e, top, 0, mnvMissileInk)
	b := mnvLit(t, e, top, 0, mnvBallInk)
	if m != mnvRead {
		t.Errorf("band A: the missile is lit on %d of %d lines, want all — it has no new/old pair, "+
			"so setting ENAM0 shows it at once no matter what the VDEL bits say", m, mnvRead)
	}
	if b != 0 {
		t.Errorf("band A: the ball is lit on %d of %d lines, want none — with VDELBL set its new "+
			"copy waits for a GRP write, and this band makes none", b, mnvRead)
	}
	t.Logf("band A: missile lit %d/%d, ball lit %d/%d — same enables, opposite answers",
		m, mnvRead, b, mnvRead)
}

// TestMissileNoVdelControlsShowTheBallIsMerelyWaiting is what makes band A mean
// something. B proves the fixture enables the ball at all; C proves band A's darkness is
// a pending latch rather than a broken ball.
func TestMissileNoVdelControlsShowTheBallIsMerelyWaiting(t *testing.T) {
	e, top := loadMissileNoVdel(t)
	for _, c := range []struct {
		band int
		what string
	}{
		{1, "VDELBL clear"},
		{2, "VDELBL set, then a GRP1 write"},
	} {
		m := mnvLit(t, e, top, c.band, mnvMissileInk)
		b := mnvLit(t, e, top, c.band, mnvBallInk)
		if m != mnvRead || b != mnvRead {
			t.Errorf("band %d (%s): missile %d/%d, ball %d/%d — both must be lit here, or band A's "+
				"dark ball cannot be read as a delay", c.band, c.what, m, mnvRead, b, mnvRead)
		}
	}
	t.Logf("with VDELBL clear, and with VDELBL set plus a GRP1 write, both objects are lit — so the "+
		"ball in band A is waiting, not off")
}

// TestMissileNoVdelFrameGeometry pins the window.
func TestMissileNoVdelFrameGeometry(t *testing.T) {
	e, top := loadMissileNoVdel(t)
	n, err := e.StepFrame()
	if err != nil {
		t.Fatal(err)
	}
	if n != 262 {
		t.Errorf("frame is %d scanlines, want 262", n)
	}
	if m := mnvLit(t, e, top, 3, mnvMissileInk); m != 0 {
		t.Errorf("the missile is still lit on %d lines past the last band — the band count is wrong", m)
	}
}
