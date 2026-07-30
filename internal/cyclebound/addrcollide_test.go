package cyclebound

import (
	"testing"

	"github.com/kidsnz/atari2600-harness/internal/build"
)

// This is the measurement that JUSTIFIES the (bank, address) code key, and it is kept
// so the justification stays a measurement rather than becoming a belief.
//
// Cross-bank region analysis has to MERGE the per-bank decodes into one node set, so
// "can one flat map hold two banks?" decided whether a composite key was optional or
// mandatory. The answer is not marginal: on the four- and eight-bank ROMs almost every
// decoded address is claimed by more than one bank, because each bank is a 4K image
// occupying the same $F000-$FFFF window. A merged flat map keeps whichever bank was
// inserted last — and the region set, the abstract states and the source locations all
// sit on that map.
//
// If a corpus ever stops overlapping, it is this premise that broke rather than the
// code; litmus_bank_shared_addr exists so the EXECUTED overlap is covered too, which
// address overlap in the decode (an over-approximation) does not establish.
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
