package emu

import "testing"

// Grading for `roms/litmus/litmus_resmp_width.asm` — where RESMP leaves the missile.
//
// Closes `docs/fundamentals-audit.md:53`:
//
//	📖 missile-locked-to-player (RESMP D1): release leaves M centered on P (Stella PG).
//	⬜ exact lock offset.
//
// The ⬜ was harness saying it did not know the number, while
// `roms/techniques/bullets.asm:3` stated one (+4px) that nothing graded. Both are now
// resolved, and the documented word "centered" turns out to hold at one width only.
//
//	NUSIZ 1x   player  8 clocks   missile lands +4    = the centre
//	NUSIZ 2x   player 16 clocks   missile lands +6    the centre would be +8
//	NUSIZ 4x   player 32 clocks   missile lands +10   the centre would be +16
//
// The mechanism is in the vendored engine and explains why: the snap fires inside the
// player's own scan, at player-pixel 2 (1x), 4 (2x) or 5 (4x)
// (Gopher2600 `hardware/tia/video/player.go:776 triggerMissileReset`), so it tracks a
// pixel index rather than a width. A fixture that locks and releases without DRAWING the
// player never snaps at all — the first version of this one did that and read a constant.
//
// This is the number a bullet-spawn routine gets wrong when the shooter is a wide
// sprite: "centre" puts the shot two clocks right of where it appears at 2x, and six at
// 4x.

const (
	resmpInk = "FFFFFE" // COLUP0 = $0E; the player and the missile share it, which is
	// why the fixture draws them on separate lines.

	resmpBand0Row = 12 // first band's player line; pinned by TestResmpFrameGeometry
	resmpBandLen  = 7
	resmpBands    = 12 // 3 widths x 4 positions
	resmpPerWidth = 4
)

// resmpWant is the measured offset per NUSIZ width, in the order the fixture sweeps.
var resmpWant = []struct {
	name        string
	playerWidth int // clocks the single set bit covers
	offset      int
	centre      int // where "centred on the player" would put it
}{
	{"1x", 1, 4, 4},
	{"2x", 2, 6, 8},
	{"4x", 4, 10, 16},
}

func loadResmp(t *testing.T) (*Emu, int) {
	t.Helper()
	e, err := New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM("../../roms/litmus/litmus_resmp_width.bin"); err != nil {
		t.Fatal(err)
	}
	if err := e.RunFrames(10); err != nil {
		t.Fatal(err)
	}
	_, top := e.Snapshot()
	return e, top
}

// resmpAt returns the clock and width of the white run on a line, or (-1, 0).
func resmpAt(t *testing.T, e *Emu, row int) (int, int) {
	t.Helper()
	runs, _, err := e.ReadRow(row)
	if err != nil {
		return -1, 0
	}
	for _, r := range runs {
		if r.Hex == resmpInk {
			return r.Clock, r.Len
		}
	}
	return -1, 0
}

// TestResmpOffsetIsFourSixAndTen is the headline: the number the ⬜ was missing, for all
// three player widths.
func TestResmpOffsetIsFourSixAndTen(t *testing.T) {
	e, top := loadResmp(t)
	for wi, w := range resmpWant {
		for b := 0; b < resmpPerWidth; b++ {
			band := wi*resmpPerWidth + b
			pRow := top + resmpBand0Row + band*resmpBandLen
			px, pw := resmpAt(t, e, pRow)
			mx, mw := resmpAt(t, e, pRow+1)
			if px < 0 || mx < 0 {
				t.Fatalf("%s band %d: player=%d missile=%d — one of the two lines is blank, so "+
					"the offset cannot be read", w.name, b, px, mx)
			}
			if pw != w.playerWidth {
				t.Errorf("%s band %d: the player's single set bit is %d clocks wide, want %d — "+
					"NUSIZ is not what this band claims", w.name, b, pw, w.playerWidth)
			}
			if mw != 1 {
				t.Errorf("%s band %d: the missile is %d clocks wide, want 1", w.name, b, mw)
			}
			if got := mx - px; got != w.offset {
				t.Errorf("%s band %d: missile lands %+d from the player's left edge, want %+d "+
					"(player at %d, missile at %d)", w.name, b, got, w.offset, px, mx)
			}
		}
		// The missile must have MOVED with the player. Without this, a fixture whose
		// lock never fires - because the player was not drawn while it was held - can
		// still satisfy every offset above, since the missile keeps a stale position
		// and the sweep happens to line up. Measured: removing the draw-while-locked
		// left this check as the only thing between a silent pass and a real one.
		var mxs []int
		for b := 0; b < resmpPerWidth; b++ {
			pRow := top + resmpBand0Row + (wi*resmpPerWidth+b)*resmpBandLen
			if mx, _ := resmpAt(t, e, pRow+1); mx >= 0 {
				mxs = append(mxs, mx)
			}
		}
		if len(mxs) < resmpPerWidth {
			t.Errorf("%s: only %d of %d missile lines readable", w.name, len(mxs), resmpPerWidth)
		} else if span := mxs[len(mxs)-1] - mxs[0]; span < 20 {
			t.Errorf("%s: the missile moved only %d clocks while the player swept the line — "+
				"the lock is not firing, so the offsets above are reading a stale position",
				w.name, span)
		}
		t.Logf("%s: missile lands %+d; %q would put it at %+d; missile itself swept %d clocks",
			w.name, w.offset, "centred on the player", w.centre, mxs[len(mxs)-1]-mxs[0])
	}
}

// TestResmpIsNotTheCentreExceptAtOneX is the correction the document needs, stated as a
// test so it cannot drift back. "Release leaves M centered on P" is true at 1x and false
// at both wider settings.
func TestResmpIsNotTheCentreExceptAtOneX(t *testing.T) {
	agree, disagree := 0, 0
	for _, w := range resmpWant {
		if w.offset == w.centre {
			agree++
		} else {
			disagree++
		}
	}
	if agree != 1 || disagree != 2 {
		t.Errorf("the measured offsets now agree with \"centred\" on %d of %d widths; the "+
			"documented claim held at 1x only, and if that has changed the doc line has to "+
			"change with it", agree, len(resmpWant))
	}
	t.Logf("\"centred on P\" holds at 1x (+4) and fails at 2x (+6, centre +8) and 4x "+
		"(+10, centre +16)")
}

// TestResmpOffsetDoesNotDependOnWhereThePlayerIs is the control. The offset is a
// property of the lock, not of the position, so sweeping the player across the line must
// leave the difference untouched — otherwise the numbers above would be four
// coincidences per width.
func TestResmpOffsetDoesNotDependOnWhereThePlayerIs(t *testing.T) {
	e, top := loadResmp(t)
	for wi, w := range resmpWant {
		var players []int
		var diffs []int
		for b := 0; b < resmpPerWidth; b++ {
			pRow := top + resmpBand0Row + (wi*resmpPerWidth+b)*resmpBandLen
			px, _ := resmpAt(t, e, pRow)
			mx, _ := resmpAt(t, e, pRow+1)
			if px < 0 || mx < 0 {
				continue
			}
			players = append(players, px)
			diffs = append(diffs, mx-px)
		}
		if len(players) < resmpPerWidth {
			t.Errorf("%s: only %d of %d bands readable", w.name, len(players), resmpPerWidth)
			continue
		}
		if players[len(players)-1]-players[0] < 20 {
			t.Errorf("%s: the player only moved %d clocks across the four bands; a control that "+
				"does not move controls nothing", w.name, players[len(players)-1]-players[0])
		}
		for i, d := range diffs {
			if d != diffs[0] {
				t.Errorf("%s: band %d has offset %+d but band 0 has %+d — the lock offset must "+
					"not depend on where the player sits", w.name, i, d, diffs[0])
			}
		}
		t.Logf("%s: player swept %d..%d, offset %+d on all %d bands",
			w.name, players[0], players[len(players)-1], diffs[0], len(diffs))
	}
}

// TestResmpFrameGeometry pins the window the band indices are counted from.
func TestResmpFrameGeometry(t *testing.T) {
	e, top := loadResmp(t)
	n, err := e.StepFrame()
	if err != nil {
		t.Fatal(err)
	}
	if n != 262 {
		t.Errorf("frame is %d scanlines, want 262", n)
	}
	// The line before band 0's player line carries the full-width bar drawn while the
	// lock is held; it must be wider than the single-pixel player line that follows.
	_, w0 := resmpAt(t, e, top+resmpBand0Row-1)
	_, w1 := resmpAt(t, e, top+resmpBand0Row)
	if w0 <= w1 {
		t.Errorf("the line above band 0 is %d clocks wide and band 0's player line is %d — the "+
			"lock-held bar should be the wider of the two, so the index is off", w0, w1)
	}
}
