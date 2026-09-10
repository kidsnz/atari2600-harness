package emu

import "testing"

// TestHmoveMidCyclesAgreeWith1998Table closes the question litmus_hmove_mid left
// open. Its own comment recorded that only one of the three mid-line strobes moves
// P0, and said the reason was unknown.
//
// The reason is a measurement from 1998. Bradford W. Mott (the author of Stella)
// swept the HMOVE strobe across a whole scanline and published the resulting
// shift for every strobe cycle and every HMPx high nibble:
//
//	reference/stella-list/199804/msg00198.html  ("games that do bad things to hmove...")
//
// Two facts in that table explain what this ROM sees, both read off the HM=0 column:
//
//   - cycles 21..54 hold 0 for all sixteen nibble values. A strobe landing there
//     moves nothing, whatever HMPx says. Brad's own summary of the run is that
//     "hitting HMOVE at cycle 73 or cycle 74 should be fairly useful in creating a
//     playfield where no HMOVE blanks occur".
//   - the column leaves 0 at cycle 64 and reaches -5 at cycles 69 and 70:
//     64:-1  65:-2  66:-2  67:-3  68:-4  69:-5  70:-5  71:-6  72:-7  73:-8  74:-8
//
// litmus_hmove_mid pads with NOPs to three strobe positions. Measured here, they
// land at CPU cycles 27, 51 and 70 — two inside the dead band and one on the -5
// step. That is the whole of the "only one of three shifts" observation.
//
// This test is a cross-check of Gopher2600 against a 1998 measurement of real
// hardware, so it fails if either the emulator's HMOVE timing or the ROM's NOP
// padding moves. It is deliberately written against the TABLE (bands, not three
// magic numbers) so that a strobe drifting into a different band is reported as
// such.
func TestHmoveMidCyclesAgreeWith1998Table(t *testing.T) {
	// The HM=0 column of the 1998 table, for the cycles it covers past the dead band.
	// Provenance: 199804/msg00198, Bradford W. Mott.
	hm0 := map[int]int{
		55: 0, 56: 0, 57: 0, 58: 0, 59: 0, 60: 0, 61: 0, 62: 0, 63: 0,
		64: -1, 65: -2, 66: -2, 67: -3, 68: -4, 69: -5, 70: -5,
		71: -6, 72: -7, 73: -8, 74: -8,
	}
	const deadLo, deadHi = 21, 54 // the table's 34-cycle band where every nibble reads 0

	e, err := New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM("../../roms/litmus/litmus_hmove_mid.bin"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ { // the scenario's warmup
		if _, err := e.StepFrame(); err != nil {
			t.Fatal(err)
		}
	}

	type strobe struct {
		cycle int
		shift int
	}
	var seen []strobe
	for f := 0; f < 4; f++ {
		if _, err := e.StepFrame(); err != nil {
			t.Fatal(err)
		}
		latches := 0
		var second HmoveSpan
		for sl := 0; sl < 262; sl++ {
			sp, err := e.HmoveOnScanline(sl)
			if err != nil || !sp.Recorded || !sp.Latched {
				continue
			}
			latches++
			if latches == 2 {
				second = sp
			}
		}
		if latches != 2 {
			continue // the control frame, covered by TestHmoveMidStrobesAllFireButOnlyOneShifts
		}
		x, ok := e.ObjectX("P0")
		if !ok {
			t.Fatal("ObjectX(\"P0\") is not available")
		}
		// StrobeClock is in read_row beam coordinates (HBLANK -68..-1, visible 0..159),
		// so +68 gives colour clocks from the start of the line and /3 gives CPU cycles.
		cc := second.StrobeClock + 68
		if cc%3 != 0 {
			t.Errorf("strobe clock %d (+68 = %d) is not on a CPU cycle boundary", second.StrobeClock, cc)
		}
		seen = append(seen, strobe{cycle: cc / 3, shift: x - 60})
	}

	if len(seen) != 3 {
		t.Fatalf("expected 3 mid-line strobes across the 4-frame parity cycle, got %d", len(seen))
	}

	moved := 0
	for _, s := range seen {
		switch {
		case s.cycle >= deadLo && s.cycle <= deadHi:
			if s.shift != 0 {
				t.Errorf("strobe at CPU cycle %d is inside the 1998 table's dead band %d..%d, "+
					"where every HMPx nibble reads 0, but P0 moved by %d",
					s.cycle, deadLo, deadHi, s.shift)
			}
		default:
			want, ok := hm0[s.cycle]
			if !ok {
				t.Errorf("strobe at CPU cycle %d is outside both the dead band %d..%d and the part "+
					"of the 1998 table this test carries (%d..%d) — the ROM's NOP padding has moved, "+
					"so this test no longer measures what it claims",
					s.cycle, deadLo, deadHi, 55, 74)
				continue
			}
			if s.shift != want {
				t.Errorf("strobe at CPU cycle %d: P0 moved %d, but the 1998 table's HM=0 column "+
					"says %d (199804/msg00198)", s.cycle, s.shift, want)
			}
			if s.shift != 0 {
				moved++
			}
		}
	}

	if moved != 1 {
		t.Errorf("expected exactly one of the three strobes to move P0 (two land in the dead band), "+
			"got %d; strobes were %+v", moved, seen)
	}
}
