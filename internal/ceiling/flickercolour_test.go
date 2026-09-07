package ceiling

import (
	"image/color"
	"testing"
)

// TestFlickerReachesColoursTheHardwareCannotHold measures flicker used as a COLOUR tool rather than
// as a way to show more objects.
//
// `docs/techniques/flicker-multiplexing.md` opens with *"**Goal:** show more than two player
// objects"*. That is one use. Manuel Polik, watching Star Ship in 2002, described the other:
//
//	"I mean the crosshair. It's done with both missiles. But the two enemy sprites have different
//	 colors. Now, the flicker is used to give the crosshair a unique look! The missiles are just
//	 swapped constantly, so the two colors *melt* into one."
//	〔stella-list `200201/msg00014`〕
//
// The same ROM uses flicker for both purposes at once — the ball multiplexes a starfield, the
// missiles compose a colour.
//
// Measured 2026-09-07 over the engine's NTSC palette (128 codes):
//
//	all pairs                8128 pairs -> 6225 distinct midpoints >16 RGB units from every static colour
//	SAME-LUMINANCE pairs      960 pairs ->  776 distinct
//
// ★**The same-luminance row is the usable one.** The TIA's luminance is D3..D1, and two codes sharing
// it differ only in hue, so the eye tracks a constant brightness and the hues melt. Pairs that differ
// in luminance flicker instead — which is the same axis `design.SameLuminance` already names for
// multiplexing, used here for the opposite purpose. **So ~776 colours are reachable that the hardware
// cannot hold in a register**, for two `COLUPx` writes a frame.
//
// ★★**What is NOT measured here: whether the eye agrees.** The midpoint is an arithmetic model of
// temporal integration. What 30 Hz alternation looks like on a television is the frontier
// `known-traps.md` names as the harness's harshest blind spot — this measurement says which colours
// are *arithmetically* out of reach, not which ones look right. Choose the pair here, judge it there.
//
// Found by the mailing-list distillation (helper-2).
func TestFlickerReachesColoursTheHardwareCannotHold(t *testing.T) {
	p, err := PaletteFor("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	static := make([]color.RGBA, PaletteSize)
	for i := range static {
		static[i] = p.RGBA(i)
	}
	dist2 := func(a, b color.RGBA) int {
		dr, dg, db := int(a.R)-int(b.R), int(a.G)-int(b.G), int(a.B)-int(b.B)
		return dr*dr + dg*dg + db*db
	}

	// count returns how many DISTINCT midpoints sit further than thr RGB units from every static
	// colour, over the pairs the filter admits.
	count := func(thr int, sameLuminance bool) (distinct, pairs int) {
		seen := map[[3]uint8]bool{}
		for i := 0; i < PaletteSize; i++ {
			for j := i + 1; j < PaletteSize; j++ {
				if sameLuminance && (p.Code(i)&0x0E) != (p.Code(j)&0x0E) {
					continue
				}
				pairs++
				a, b := static[i], static[j]
				mix := color.RGBA{
					R: uint8((int(a.R) + int(b.R)) / 2),
					G: uint8((int(a.G) + int(b.G)) / 2),
					B: uint8((int(a.B) + int(b.B)) / 2),
				}
				best := 1 << 30
				for k := range static {
					if d := dist2(mix, static[k]); d < best {
						best = d
					}
				}
				if best > thr*thr {
					seen[[3]uint8{mix.R / 4 * 4, mix.G / 4 * 4, mix.B / 4 * 4}] = true
				}
			}
		}
		return len(seen), pairs
	}

	melt, meltPairs := count(16, true)
	if meltPairs != 960 {
		t.Fatalf("%d same-luminance pairs, want 960 (8 luminance levels x C(16,2)). The palette's "+
			"shape changed and every number below is about a different set", meltPairs)
	}
	if melt < 700 || melt > 850 {
		t.Errorf("same-luminance alternation reaches %d distinct colours more than 16 RGB units "+
			"from anything static; it was 776 on 2026-09-07. flicker-multiplexing.md quotes that "+
			"number", melt)
	}

	// Two-sided. Without a threshold the count is meaningless, so a large one must shrink it a lot,
	// and the unrestricted population must be much bigger than the same-luminance one.
	far, _ := count(36, true)
	if far >= melt {
		t.Errorf("raising the threshold from 16 to 36 did not reduce the count (%d then %d) — the "+
			"measurement is not responding to distance and the numbers above mean nothing", melt, far)
	}
	all, allPairs := count(16, false)
	if allPairs <= meltPairs || all <= melt {
		t.Errorf("the unrestricted sweep (%d colours over %d pairs) is not larger than the "+
			"same-luminance one (%d over %d) — the luminance filter is not filtering",
			all, allPairs, melt, meltPairs)
	}
}
