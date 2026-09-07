package emu

import (
	"testing"
)

// TestPlayerAndMissileMakeOneNinePixelShape measures a 2001 answer to a problem the hardware creates
// for anyone drawing symmetrical shapes.
//
// Thomas Jentzsch, describing a shape he could not get: *"all cursors are **7 pixels wide** (has to
// be an **odd number** to make the up/down arrows look nice) and so the 'hole' in the stop cursor is
// 5 pixels wide. That means, I can only use a 4 pixel wide ball"* 〔`200102/msg00234`〕. A ball is 1,
// 2, 4 or 8 pixels and nothing else, so an odd width has no ball that fits it.
//
// Andrew Davie's answer: *"Instead of 7-wide, make the cursor **9 wide**. Use the **missile to give
// you the extra pixel** you need. (8 player + 1 missile) Then it is a simple-matter to use an 8-wide
// ball to provide the white area you need."* 〔`200102/msg00238`〕 — build the odd width as a power
// of two **plus one**, and the ball problem disappears because 8 is a size the ball has.
//
// Measured 2026-09-07 (`roms/litmus/litmus_player_missile_9px.asm`), three bands:
//
//	player alone            one run of 8 px at clock 99
//	player + missile        one run of 9 px at clock 99      <- no seam
//	missile alone           one run of 1 px at clock 107
//
// ★**The join is free.** `RESM0` on the instruction after `RESP0` is three CPU cycles, which is nine
// colour clocks; a player is eight wide, so the missile lands on the clock immediately after the
// player's last. **No fine motion, no HMOVE, one extra store.** And the missile carries `COLUP0`, so
// the two read as one object.
//
// ★★The first version of this litmus applied `HMM0 = $1` to pull the missile one clock left, on the
// reasoning that nine clocks would leave a gap. That put the missile **on** the player's last pixel
// and band B stayed eight wide. The arithmetic was off by one and the picture said so — which is why
// the bands are read rather than computed.
//
// Bands A and C are what make "9" mean "8 and 1 touching": without them a nine-pixel run could be a
// differently-sized player, or a missile somewhere else entirely.
//
// Found by the mailing-list distillation (helper-1).
func TestPlayerAndMissileMakeOneNinePixelShape(t *testing.T) {
	e, err := New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM("../../roms/litmus/litmus_player_missile_9px.bin"); err != nil {
		t.Fatal(err)
	}
	if err := e.RunFrames(6); err != nil {
		t.Fatal(err)
	}

	// lit returns the single non-background run on a line, or fails saying what it found instead.
	lit := func(line int, what string) (clock, length int) {
		runs, _, err := e.ReadRow(line)
		if err != nil {
			t.Fatalf("%s (line %d): %v", what, line, err)
		}
		if len(runs) == 0 {
			t.Fatalf("%s (line %d): the row is empty", what, line)
		}
		// The background is whatever colour clock 0 has. COLUBK=$00 renders as 060606 here, not
		// 000000, and hard-coding black made the first version of this test fail on its own fixture.
		bg := runs[0].Hex
		var found []RowRun
		for _, r := range runs {
			if r.Hex != bg {
				found = append(found, r)
			}
		}
		if len(found) != 1 {
			t.Fatalf("%s (line %d): %d lit runs, want exactly 1 — %v. Two runs would mean the "+
				"player and missile do NOT touch, which is the whole question", what, line, len(found), found)
		}
		return found[0].Clock, found[0].Len
	}

	pClock, pLen := lit(45, "player alone")
	bClock, bLen := lit(65, "player + missile")
	mClock, mLen := lit(85, "missile alone")

	if pLen != 8 {
		t.Errorf("the player alone is %d px, want 8. Every number below is relative to it", pLen)
	}
	if mLen != 1 {
		t.Errorf("the missile alone is %d px, want 1 (NUSIZ0 missile size 0)", mLen)
	}
	if bLen != 9 {
		t.Errorf("player + missile is %d px, want 9. Davie's construction is 8+1 with no seam; if "+
			"this is 8 the missile is sitting on the player, and if it is 10 something is wider "+
			"than it should be", bLen)
	}
	if bClock != pClock {
		t.Errorf("the combined shape starts at clock %d and the player alone at %d — the missile is "+
			"supposed to extend the player to the RIGHT, not move where it begins", bClock, pClock)
	}
	// The join, stated as a position rather than inferred from the length.
	if mClock != pClock+pLen {
		t.Errorf("the missile is at clock %d and the player ends at %d. Adjacency is what makes the "+
			"nine-pixel run one shape; a gap or an overlap would both still be 'a run'",
			mClock, pClock+pLen-1)
	}
}
