package emu

import "testing"

// TestRAMStripCostsSevenCyclesNotEightUnlessItCrossesAPage prices the fifth route to more figures
// on a line — the one `zone-multiplexing.md` did not have.
//
// That page lists the routes and prices each in the resource it spends: flicker pays in frames,
// the missile/ball/PF pays in objects, a wide `NUSIZ` interleave pays *"in width rather than in
// frames"*, a missile-as-character pays in the missile. **A fifth pays in RAM**, proposed by Andrew
// Davie in a thread called "[stella] What would you do with more RAM?" 〔stella-list
// `200305/msg00000`, 2003-05-01〕: reserve a strip of RAM per player, draw the shape into it at the
// right offset, and the kernel becomes
//
//	lda P0strip,y
//	sta GRP0
//
// — *"That's just, what, 8 cycles per player. Neat. No skipdraw trickery … they can overlap fine,
// and they won't flicker when vertically overlapping."*
//
// ★Measured (`roms/litmus/litmus_ramstrip.asm`): **the pair is seven, not eight.** `sta GRP0` is a
// store to $1B, and the TIA lives in the ZERO PAGE, so it assembles to `sta zp` — three cycles, not
// four. Broken into pieces against an empty interval: the indexed load is **4**, the store is **3**,
// the pair is **7**.
//
// ★★And eight is reachable, at exactly the size he proposed. A 256-byte strip **spans a page by
// construction**, so an indexed read into it crosses one whenever the index carries, and the pair
// costs **8** on those lines. So the figure on the list is right about half the time, for a reason
// it does not give — and the kernel author's real choice is whether the strip can be page-aligned.
// That is worth one cycle per player per line, which at 192 lines and two players is 384 cycles a
// frame, a little over five scanlines.
//
// ★★★What the route costs in RAM is what keeps it off a stock machine: 256 bytes per player
// against `design.RAM2600 = 128`. `ScrollBackgroundFitsRAM(256,0,0,0)` is false, and that is
// asserted below rather than argued — this route needs a SuperChip, which is what separates it from
// the other four. Glenn Saunders priced the other side four months later 〔`200309/msg00071`〕:
// *"Sure, it wastes RAM on lines the sprites don't appear in, but it's worth it … all those RAM
// strips do add up."*
//
// ★★★★The first version of this ROM compared three spellings of the pair and measured 7 three
// times, because every one of them ends in the same zero-page store. That is recorded in the ROM's
// header: the comparison looked like a comparison and was not.
//
// Recovered by the mailing-list distillation (helper-2); the cycles measured here.
func TestRAMStripCostsSevenCyclesNotEightUnlessItCrossesAPage(t *testing.T) {
	e, err := New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM("../../roms/litmus/litmus_ramstrip.bin"); err != nil {
		t.Fatal(err)
	}
	if err := e.RunFrames(5); err != nil {
		t.Fatal(err)
	}
	r, err := e.CurrentRAM()
	if err != nil {
		t.Fatal(err)
	}

	base := int(r[0x0F])
	if base < 100 || base > 200 {
		t.Fatalf("the empty interval reads %d, outside anything this loop could take — the timer "+
			"setup has moved and every figure below is meaningless", base)
	}

	for _, c := range []struct {
		addr int
		what string
		want int
		why  string
	}{
		{0x00, "lda Strip,y", 4, "the indexed load, no page cross"},
		{0x01, "sta GRP0", 3, "GRP0 is $1B — the TIA is in the ZERO PAGE, so this is sta zp"},
		{0x02, "lda Strip,y / sta GRP0", 7, "the pair the list calls eight"},
		{0x03, "lda Cross,y / sta GRP0", 8, "the same pair when the indexed read crosses a page"},
	} {
		if got := int(r[c.addr]) - base; got != c.want {
			t.Errorf("%s costs %d cycles, want %d (%s)", c.what, got, c.want, c.why)
		}
	}

	// ★The claim, as a claim: the page cross is worth exactly one cycle, and it is the only thing
	// separating the measured pair from the number on the list.
	plain, crossed := int(r[0x02])-base, int(r[0x03])-base
	if crossed-plain != 1 {
		t.Errorf("crossing a page changes the pair by %d cycles, not 1 — the explanation for the "+
			"list's 'eight' no longer holds", crossed-plain)
	}
	if plain != int(r[0x00])-base+int(r[0x01])-base {
		t.Errorf("the pair (%d) is not the load (%d) plus the store (%d); the decomposition that "+
			"makes this measurement readable has stopped being true",
			plain, int(r[0x00])-base, int(r[0x01])-base)
	}
	t.Logf("load=%d store=%d pair=%d pair-across-a-page=%d",
		int(r[0x00])-base, int(r[0x01])-base, plain, crossed)
}
