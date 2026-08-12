package main

import "testing"

// THE WITNESS. A zone's position comes from bx and its anchor from lx, and those are two
// different measurements: bx can read notDrawn on a line where lx still reports a leftmost
// run. Without a pin test the anchor is simply the smallest lx in the zone, so one line
// belonging to a band the zone is NOT pinned at wins it, and every GRP byte read through
// that anchor is wrong.
//
// This is the shape measured on Fishing Derby with partial following on: z1 (L27-213) is
// pinned at 134 and came out anchored at 29, the position of the band the object had
// already been retired from.
func TestTheZoneAnchorCannotBeStolenByALineTheZoneIsNotPinnedAt(t *testing.T) {
	var lx, bx [5][]int
	const n = 6
	lx[objP0] = make([]int, n)
	bx[objP0] = make([]int, n)
	for y := 0; y < n; y++ {
		lx[objP0][y] = 135 // the band this zone reproduces
		bx[objP0][y] = 134
	}
	// one line from a DIFFERENT band: further left, and not at the zone's pin
	lx[objP0][3] = 29
	bx[objP0][3] = 29

	z := zone{start: 0, end: n - 1}
	for i := range z.x {
		z.x[i] = notDrawn
	}
	z.x[objP0] = 134

	got := zoneLeftmost(lx, bx, z, objP0)
	if got == 29 {
		t.Fatal("the anchor was stolen by the x29 line: every GRP byte in this zone would be " +
			"read from a position the zone does not reproduce")
	}
	if got != 135 {
		t.Errorf("anchor %d, want 135 -- the leftmost of the lines the zone IS pinned at", got)
	}
}

// The negative control. The pin test must not throw away lines that legitimately belong to
// the zone, or a zone whose object drifts within its own band loses its anchor entirely.
func TestThePinTestKeepsEveryLineTheZoneIsPinnedAt(t *testing.T) {
	var lx, bx [5][]int
	lx[objP0] = []int{140, 136, 138}
	bx[objP0] = []int{134, 134, 134}
	z := zone{start: 0, end: 2}
	for i := range z.x {
		z.x[i] = notDrawn
	}
	z.x[objP0] = 134
	if got := zoneLeftmost(lx, bx, z, objP0); got != 136 {
		t.Errorf("anchor %d, want 136 -- all three lines are pinned at 134 and 136 is the "+
			"leftmost of them", got)
	}
}

// And a zone with no pin recorded must behave exactly as before, so the change cannot
// alter any path that does not have a pin to test against.
func TestNoPinMeansNoTest(t *testing.T) {
	var lx, bx [5][]int
	lx[objP0] = []int{140, 29, 138}
	bx[objP0] = []int{134, 29, 134}
	z := zone{start: 0, end: 2}
	for i := range z.x {
		z.x[i] = notDrawn
	}
	if got := zoneLeftmost(lx, bx, z, objP0); got != 29 {
		t.Errorf("anchor %d, want 29 -- with no pin there is nothing to test against and the "+
			"old minimum stands", got)
	}
}
