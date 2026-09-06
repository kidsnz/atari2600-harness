package cyclebound

import "testing"

// TestRAMFootprintPricesTechniquesAndSaysWhenItCannot pins both what the measure gives and where it
// goes blind.
//
// The catalogue was required, on 2026-09-06, to state each technique's price in every resource it
// spends. It could not state a RAM price because nothing measured one — and the obvious measure,
// `MayWrite ∩ RAM`, is **128 for every ROM in the corpus**, because they all open by clearing RAM
// through an unknown index. This is the fixed version and its limit.
func TestRAMFootprintPricesTechniquesAndSaysWhenItCannot(t *testing.T) {
	// ★Lower bounds that were measured, not declared. Held as goldens so a change to the analysis
	// that quietly moves them is visible: these are the first numbers this catalogue has for the
	// resource the archive says bends designs hardest.
	//
	// ★★They are LOWER BOUNDS and not prices, and the first version of this test called them
	// prices. All eleven ROMs measured report imprecise accesses, because the RAM-clear loop every
	// 2600 program opens with is a wide access — so `Imprecise > 0` is the normal state, not a
	// warning, and it was nearly published as though it flagged the exceptions.
	for _, c := range []struct {
		asm  string
		want int
	}{
		{"../../roms/techniques/two_line_kernel.asm", 5},
		{"../../roms/techniques/tia_pcm.asm", 4},
		{"../../roms/techniques/rpgmap.asm", 5},
		{"../../roms/techniques/sound_driver.asm", 9},
		{"../../roms/techniques/music_driver.asm", 10},
		{"../../roms/techniques/flicker_multiplex.asm", 14},
		{"../../roms/techniques/bitmap48.asm", 17},
		{"../../roms/techniques/maze.asm", 9},
		{"../../roms/techniques/score6.asm", 17},
		{"../../roms/techniques/text12.asm", 23},
	} {
		f, err := RAMFootprintOf(c.asm)
		if err != nil {
			t.Fatalf("%s: %v", c.asm, err)
		}
		if got := len(f.Bytes); got != c.want {
			t.Errorf("%s needs at least %d RAM bytes, want %d (written %d, read %d, imprecise accesses %d) — "+
				"either the technique changed or the footprint analysis did; say which before "+
				"updating the number", shortName(c.asm), got, c.want, len(f.Written), len(f.Read),
				f.Imprecise)
		}
	}

	// ★★The blind spot, asserted as its own fact. `zone_multiplex` reaches all of its RAM through
	// unknown indices, so nothing precise survives and the footprint is ZERO for a ROM that plainly
	// uses RAM. That is the shape of the measure's failure, and it is only safe to publish prices
	// because this case reports `Imprecise` rather than a confident zero.
	f, err := RAMFootprintOf("../../roms/techniques/zone_multiplex.asm")
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Bytes) != 0 {
		t.Errorf("zone_multiplex now prices at %d bytes; the documented blind spot has moved and "+
			"the warning in `RAMFootprint`'s comment no longer describes it", len(f.Bytes))
	}
	if f.Imprecise == 0 {
		t.Error("zone_multiplex prices at zero bytes AND reports no imprecise accesses — a caller " +
			"would read that as 'this technique uses no RAM', which is false. The whole safety of " +
			"publishing these numbers rests on this counter being non-zero here")
	}
	t.Logf("zone_multiplex: %d bytes priced, %d imprecise accesses (a lower bound, not a price)",
		len(f.Bytes), f.Imprecise)
}
