package mutate

import "testing"

// TestCoveredOffsetsFollowTheBankThatRan pins that "restrict fault injection to code
// that actually executed" means the bytes that executed.
//
// A ROM offset is bank*4K plus the offset in the window. The old mapping was
// `addr & (len(rom)-1)`, which on an 8K image folds every $Fxxx into the LAST 4K
// image whichever bank ran — measured before the fix: on the exerciser all 278
// covered offsets landed in $1000-$1FFF and not one in $0000-$0FFF. So the honest
// kill rate was mutating bank 1's bytes while bank 0 was the half being executed,
// producing mutants that cannot be killed for the exact reason -covered exists to
// avoid.
//
// A 4K image is the one-bank case and must be untouched by the change.
func TestCoveredOffsetsFollowTheBankThatRan(t *testing.T) {
	flat, err := CoveredOffsets("../../roms/litmus/smoke.bin", "NTSC", 6)
	if err != nil {
		t.Fatal(err)
	}
	if len(flat) == 0 {
		t.Fatal("no covered offsets on a 4K ROM — the fixture stopped running")
	}
	for off := range flat {
		if off >= 0x1000 {
			t.Fatalf("4K image produced offset $%04X, outside the file", off)
		}
	}

	banked, err := CoveredOffsets("../../roms/exerciser/exerciser.bin", "NTSC", 6)
	if err != nil {
		t.Skipf("exerciser unavailable: %v", err)
	}
	lo, hi := 0, 0
	for off := range banked {
		if off < 0x1000 {
			lo++
		} else {
			hi++
		}
	}
	if lo == 0 {
		t.Errorf("every one of %d covered offsets landed in the upper 4K — bank 0 executed and "+
			"none of its bytes are mutable, which is the defect this test exists for", len(banked))
	}
	if hi == 0 {
		t.Errorf("no covered offset in the upper 4K out of %d: the exerciser runs code in both "+
			"banks, so a one-sided answer means the mapping moved the other way", len(banked))
	}
	t.Logf("exerciser covered offsets: %d total, %d in bank 0's image, %d in bank 1's", len(banked), lo, hi)
}
