package emu

import "testing"

// Grading for `roms/litmus/litmus_vdel_cross.asm` — VDEL's write-triggered cross-copy.
//
// Closes `docs/fundamentals-audit.md:70`, marked 📖 (stated by a document, not measured
// here) and described by harness itself as "the load-bearing mechanism". The claim, from
// Stella PG §6.D:
//
//	each GRP has new+old copies. Writing GRP0 copies P1's new->old; writing GRP1 copies
//	P0's new->old, AND ALSO ENABL's new->old. VDELPx/VDELBL D0=1 displays the old copy.
//
// The vendored engine does exactly that (`hardware/tia/video/video.go:234-238`) and
// nothing asserted it. The ball half is the strange one - a write to a PLAYER register
// moving the BALL's enable - so the fixture leads with it, because it reads as a single
// bit: bands A and B differ by one instruction, GRP0 versus GRP1, and the ball is dark
// in one and lit in the other.

const (
	vdcP0Ink = "FFFFFE" // COLUP0 = $0E
	vdcP1Ink = "2FA076" // COLUP1 = $B6
	vdcBLInk = "EC3333" // COLUPF = $46

	// Each band is 8 lines; the first is spent setting up, so read the second onward.
	vdcBand0Row = 9
	vdcBandLen  = 8
	vdcRead     = 5 // lines per band that are safe to read
)

func loadVdelCross(t *testing.T) (*Emu, int) {
	t.Helper()
	e, err := New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM("../../roms/litmus/litmus_vdel_cross.bin"); err != nil {
		t.Fatal(err)
	}
	if err := e.RunFrames(10); err != nil {
		t.Fatal(err)
	}
	_, top := e.Snapshot()
	return e, top
}

// vdcSeen counts, over a band's readable lines, how many carry the given ink and how
// wide that run is.
func vdcSeen(t *testing.T, e *Emu, top, band int, ink string) (lines, width int) {
	t.Helper()
	for i := 1; i <= vdcRead; i++ {
		runs, _, err := e.ReadRow(top + vdcBand0Row + band*vdcBandLen + i)
		if err != nil {
			continue
		}
		for _, r := range runs {
			if r.Hex == ink {
				lines++
				width = r.Len
			}
		}
	}
	return
}

// TestVdelCrossGRP1LatchesTheBallAndGRP0DoesNot is the headline. Two bands that differ
// by one instruction; the ball's visibility is the whole reading.
func TestVdelCrossGRP1LatchesTheBallAndGRP0DoesNot(t *testing.T) {
	e, top := loadVdelCross(t)
	afterGRP0, _ := vdcSeen(t, e, top, 0, vdcBLInk)
	afterGRP1, w := vdcSeen(t, e, top, 1, vdcBLInk)
	if afterGRP0 != 0 {
		t.Errorf("band A: the ball is lit on %d of %d lines after a GRP0 write — GRP0 must not "+
			"copy ENABL new->old, so with VDELBL set the ball has to stay dark",
			afterGRP0, vdcRead)
	}
	if afterGRP1 != vdcRead {
		t.Errorf("band B: the ball is lit on only %d of %d lines after a GRP1 write — writing "+
			"GRP1 copies ENABL new->old, which is what makes it visible with VDELBL set",
			afterGRP1, vdcRead)
	}
	if afterGRP1 > 0 && w != 1 {
		t.Errorf("band B: the ball is %d clocks wide, want 1 (CTRLPF D4-5 are zero)", w)
	}
	t.Logf("ball dark on %d/%d lines after GRP0, lit on %d/%d after GRP1 — one instruction apart",
		afterGRP0, vdcRead, afterGRP1, vdcRead)
}

// TestVdelCrossPlayersLatchEachOther is the other half of the same mechanism, and it is
// the control that stops the ball result from being read as "GRP1 does something odd":
// the players cross-copy in exactly the same shape, each latched by the OTHER register.
func TestVdelCrossPlayersLatchEachOther(t *testing.T) {
	e, top := loadVdelCross(t)
	p0Lines, p0W := vdcSeen(t, e, top, 2, vdcP0Ink)
	p1Lines, p1W := vdcSeen(t, e, top, 3, vdcP1Ink)
	if p0Lines != vdcRead || p0W != 8 {
		t.Errorf("band C: P0 is drawn on %d of %d lines, %d clocks wide; want %d lines of 8. "+
			"GRP0 was loaded with $FF, GRP1 was written (latching it to old) and GRP0 cleared, "+
			"so VDELP0 must display the old $FF", p0Lines, vdcRead, p0W, vdcRead)
	}
	if p1Lines != vdcRead || p1W != 8 {
		t.Errorf("band D: P1 is drawn on %d of %d lines, %d clocks wide; want %d lines of 8. "+
			"The mirror of band C: GRP0's write is what latches P1's old copy",
			p1Lines, vdcRead, p1W, vdcRead)
	}
	t.Logf("P0 shows its old $FF on %d/%d lines (latched by GRP1); P1 shows its old $FF on "+
		"%d/%d (latched by GRP0)", p0Lines, vdcRead, p1Lines, vdcRead)
}

// TestVdelCrossFrameGeometry pins the window and the band indices.
func TestVdelCrossFrameGeometry(t *testing.T) {
	e, top := loadVdelCross(t)
	n, err := e.StepFrame()
	if err != nil {
		t.Fatal(err)
	}
	if n != 262 {
		t.Errorf("frame is %d scanlines, want 262", n)
	}
	// Past the four bands nothing is drawn: every band clears GRP0/GRP1/ENABL on entry
	// and the filler never sets them again.
	for _, ink := range []string{vdcP0Ink, vdcP1Ink, vdcBLInk} {
		if l, _ := vdcSeen(t, e, top, 4, ink); l != 0 {
			t.Errorf("something of colour %s is still drawn past the last band — the band count "+
				"is wrong and the readings above are off by one", ink)
		}
	}
}
