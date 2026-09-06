package ceiling

import "testing"

// TestPALSpendsFourHuesOnGreyAndHasNoRed measures two properties of the PAL palette that
// decide what can be drawn on it, and that the NTSC palette does not share.
//
// Both were reported on the list in 1997 by someone who had burned an EPROM to check them
// on a real machine rather than an emulator 〔stella-list `199704/msg00150`, 1997-04-17〕:
//
//	Does that mean you see grey four times too, but no real orange?  Yes, the first and
//	last two colours are the same grey. What is surprising is that the TIA has many nice
//	colours but there isn't a bright, intense RED - at least in PAL.
//
// Measured here against the palette this repository derives from the renderer:
//
//   - PAL spends FOUR of its sixteen hues on the same grey (hues 0, 1, 14, 15). NTSC spends
//     one. A PAL kernel therefore chooses from twelve hues, not fifteen.
//   - The reddest colour PAL can make is an ORANGE. The same TIA code that paints a red on
//     NTSC paints an orange on PAL, so a picture whose subject IS red does not port.
//
// ★The NTSC side is the control. Every assertion below is a comparison between the two
// specs, so a palette table that had gone uniformly grey — or a comparison accidentally
// reading the same spec twice — fails rather than passes.
func TestPALSpendsFourHuesOnGreyAndHasNoRed(t *testing.T) {
	abs := func(x int) int {
		if x < 0 {
			return -x
		}
		return x
	}
	// A hue is grey when its mid-luminance entry has no channel separation worth a pixel.
	greyHues := func(p Palette) []int {
		var out []int
		for h := 0; h < 16; h++ {
			c := p.Colors[h*8+4]
			if abs(c[0]-c[1]) <= 6 && abs(c[1]-c[2]) <= 6 && abs(c[0]-c[2]) <= 6 {
				out = append(out, h)
			}
		}
		return out
	}
	// "Redness" = how far the red channel stands above the mean of the other two. The
	// reddest entry in the table is the best red the machine can draw.
	reddest := func(p Palette) (code, redness int, rgb [3]int) {
		redness = -1 << 30
		for i := 0; i < PaletteSize; i++ {
			c := p.Colors[i]
			if r := c[0] - (c[1]+c[2])/2; r > redness {
				redness, code, rgb = r, i*2, c
			}
		}
		return
	}

	ntsc, err := PaletteFor("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	pal, err := PaletteFor("PAL")
	if err != nil {
		t.Fatal(err)
	}

	gN, gP := greyHues(ntsc), greyHues(pal)
	cN, rN, rgbN := reddest(ntsc)
	cP, rP, rgbP := reddest(pal)
	t.Logf("NTSC  grey hues %v (%d/16)   reddest $%02X RGB%v redness %d", gN, len(gN), cN, rgbN, rN)
	t.Logf("PAL   grey hues %v (%d/16)   reddest $%02X RGB%v redness %d", gP, len(gP), cP, rgbP, rP)

	if len(gP) != 4 {
		t.Errorf("PAL has %d grey hues %v; the list reports four (the first two and the last two)",
			len(gP), gP)
	}
	if len(gN) >= len(gP) {
		t.Errorf("NTSC has %d grey hues and PAL %d — the specs are not being told apart, so "+
			"neither figure means anything", len(gN), len(gP))
	}
	// The claim is not "PAL's red is dimmer"; it is that PAL has no red. The reddest PAL
	// entry has a green channel high enough to read as orange.
	if rgbP[1] <= rgbP[0]/3 {
		t.Errorf("PAL's reddest colour RGB%v has little green in it — it IS a red, and the "+
			"1997 report that PAL has no intense red does not hold here", rgbP)
	}
	if rP >= rN {
		t.Errorf("PAL's reddest is %d against NTSC's %d — PAL is not the poorer of the two here",
			rP, rN)
	}

	// How many colours each spec can actually make. The table is 128 entries everywhere;
	// what differs is how many of them are the same colour twice.
	//
	// ★Asserted as an ORDER, not as three pinned numbers. The NTSC count came out 126 here
	// and 127 on another machine earlier in this project — a rounding difference in the
	// renderer's conversion, not a fact about the hardware — and pinning it turned CI red.
	// The ordering is the claim that survives the arithmetic.
	distinct := func(spec string) int {
		p, err := PaletteFor(spec)
		if err != nil {
			t.Fatalf("%s: %v", spec, err)
		}
		set := map[[3]int]bool{}
		for i := 0; i < PaletteSize; i++ {
			set[p.Colors[i]] = true
		}
		return len(set)
	}
	dN, dP, dS := distinct("NTSC"), distinct("PAL"), distinct("SECAM")
	t.Logf("distinct colours from 128 table entries: NTSC %d  PAL %d  SECAM %d", dN, dP, dS)
	if !(dN > dP && dP > dS) {
		t.Errorf("distinct-colour counts NTSC %d, PAL %d, SECAM %d are not strictly decreasing; "+
			"the specs differ in how much of the table repeats and that order is the finding",
			dN, dP, dS)
	}
	if dS > 16 {
		t.Errorf("SECAM makes %d distinct colours; it is a single-luminance spec and the count "+
			"should be tiny — a larger number means the palette is not being derived per spec", dS)
	}
}
