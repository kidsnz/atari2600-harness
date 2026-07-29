package cyclebound

import (
	"testing"

	"github.com/kidsnz/atari2600-harness/internal/build"
)

// Everything in this package keys code on a bare uint16. Stage 1 kept that
// workable by giving each bank its OWN decode map and never merging them, and
// cross-bank region analysis will have to merge — so the question "can one flat
// map hold two banks?" decides whether a composite (bank, addr) key is optional
// or mandatory. It is answerable by measurement rather than argument.
//
// Measured here, and the answer is not marginal: on the four-and-eight-bank ROMs
// almost every decoded address is claimed by more than one bank, because each bank
// is a 4K image occupying the same $F000-$FFFF window. A merged flat map would
// silently keep whichever bank happened to be inserted last.
func TestBanksShareDecodedAddresses(t *testing.T) {
	cases := []struct {
		asm      string
		minShare float64 // the fraction of decoded addresses claimed by 2+ banks
	}{
		{"../../roms/litmus/litmus_bank.asm", 0.05},
		{"../../roms/techniques/banked_game.asm", 0.05},
		{"../../roms/litmus/litmus_bank_f6.asm", 0.90},
		{"../../roms/litmus/litmus_bank_f4.asm", 0.90},
	}
	for _, c := range cases {
		bin := build.BinPathFor(c.asm)
		if out, err := build.Assemble(c.asm, bin); err != nil {
			t.Fatalf("assemble %s: %s", c.asm, out)
		}
		per, err := DecodedAddrsPerBank(bin)
		if err != nil {
			t.Fatalf("%s: %v", c.asm, err)
		}
		if len(per) < 2 {
			t.Fatalf("%s: premise broken, %d bank(s)", c.asm, len(per))
		}
		count := map[uint16]int{}
		for _, set := range per {
			for a := range set {
				count[a]++
			}
		}
		shared := 0
		for _, n := range count {
			if n > 1 {
				shared++
			}
		}
		frac := float64(shared) / float64(len(count))
		if frac < c.minShare {
			t.Errorf("%s: only %d of %d decoded addresses (%.0f%%) are shared by 2+ banks, expected at "+
				"least %.0f%% — if the banks really do not overlap then the flat key is safe and this "+
				"test's premise, not the code, is what changed",
				c.asm, shared, len(count), frac*100, c.minShare*100)
		}
		t.Logf("%s: %d banks, %d decoded addresses, %d shared by 2+ (%.0f%%)",
			c.asm, len(per), len(count), shared, frac*100)
	}
}
