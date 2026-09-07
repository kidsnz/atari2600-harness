package ceiling

import "testing"

// TestSomeNTSCRedsAreGreenOnPAL measures the strongest form of the PAL/NTSC hue mismatch:
// a code is not merely dimmer or shifted on the other spec, it can be a DIFFERENT COLOUR.
//
// Reported from hardware in 2003 — *"the NTSC red you chose comes out as green on a PAL
// VCS"* 〔stella-list `200302/msg00246`, Eckhard Stolberg〕 — and reproduced here against
// the palette this repository derives from the renderer.
//
// The mechanism is in `visual-ceiling.md`: the two specs have sixteen hues each and **no
// guarantee that hue N means the same thing on both**. What that abstract sentence costs is
// concrete: pick `$34` for a red on NTSC and a PAL machine paints RGB(40, 140, 8).
//
// ★Asserted as "at least one code is red on one spec and green on the other", not as a list
// of codes. The exact boundary of "red" is a judgement about thresholds; the existence of a
// code that crosses the whole way is not.
func TestSomeNTSCRedsAreGreenOnPAL(t *testing.T) {
	ntsc, err := PaletteFor("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	pal, err := PaletteFor("PAL")
	if err != nil {
		t.Fatal(err)
	}
	abs := func(x int) int {
		if x < 0 {
			return -x
		}
		return x
	}
	red := func(c [3]int) bool { return c[0] > c[1]+40 && c[0] > c[2]+40 }
	green := func(c [3]int) bool { return c[1] > c[0]+40 && c[1] > c[2]+40 }

	var flipped []int
	for i := 0; i < PaletteSize; i++ {
		if red(ntsc.Colors[i]) && green(pal.Colors[i]) {
			flipped = append(flipped, i*2)
			t.Logf("$%02X  NTSC%v is red  →  PAL%v is green", i*2, ntsc.Colors[i], pal.Colors[i])
		}
	}
	if len(flipped) == 0 {
		t.Errorf("no code is red on NTSC and green on PAL — either the specs have stopped " +
			"differing or both tables are being read from the same spec")
	}
	// The control: the two tables must not be identical, and must not be uniformly shifted
	// either. If every entry moved by the same amount this would be a calibration difference
	// rather than a hue mismatch.
	same, shifts := 0, map[int]bool{}
	for i := 0; i < PaletteSize; i++ {
		n, p := ntsc.Colors[i], pal.Colors[i]
		if n == p {
			same++
		}
		shifts[abs(n[0]-p[0])+abs(n[1]-p[1])+abs(n[2]-p[2])] = true
	}
	if same == PaletteSize {
		t.Errorf("the NTSC and PAL tables are identical — the comparison above proves nothing")
	}
	if len(shifts) < 8 {
		t.Errorf("only %d distinct NTSC→PAL distances across 128 entries; that is a uniform "+
			"shift, not a hue mismatch, and the finding would be about calibration", len(shifts))
	}
}
