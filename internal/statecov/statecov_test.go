package statecov

import "testing"

func contains(xs []int, v int) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

// TestStateCoverageDistinguishes is the falsifiable core: the matrix must report
// MORE state coverage for a ROM that exercises a mode than for one that does
// not. If the sampler were broken (always empty, or always saturated) it could
// not separate these — so this locks its discrimination in both directions.
func TestStateCoverageDistinguishes(t *testing.T) {
	rich, err := Run("../../roms/techniques/multicolor48.bin", "NTSC", 6)
	if err != nil {
		t.Fatal(err)
	}
	poor, err := Run("../../roms/litmus/smoke.bin", "NTSC", 6)
	if err != nil {
		t.Fatal(err)
	}

	// multicolor48 drives a triple-copy NUSIZ (value 3) on both players.
	if rich.Count("nusiz0_copies") < 2 || !contains(rich.Values("nusiz0_copies"), 3) {
		t.Errorf("multicolor48 nusiz0_copies = %v, want >=2 distinct incl. 3", rich.Values("nusiz0_copies"))
	}
	// smoke exercises no sprite modes: NUSIZ stays at its single reset value.
	if poor.Count("nusiz0_copies") != 1 {
		t.Errorf("smoke nusiz0_copies = %v, want exactly 1 (blind to that mode)", poor.Values("nusiz0_copies"))
	}
	// the discrimination itself
	if rich.Count("nusiz0_copies") <= poor.Count("nusiz0_copies") {
		t.Fatal("matrix failed to distinguish a NUSIZ-exercising ROM from one that never moves NUSIZ")
	}
}

// TestBankAxis: a bank-switching ROM must show >1 distinct bank; a flat one must
// show exactly 1.
func TestBankAxis(t *testing.T) {
	banked, err := Run("../../roms/techniques/banked_game.bin", "NTSC", 6)
	if err != nil {
		t.Fatal(err)
	}
	if banked.Count("bank") < 2 {
		t.Errorf("banked_game bank distinct = %d, want >=2", banked.Count("bank"))
	}
	flat, err := Run("../../roms/litmus/smoke.bin", "NTSC", 4)
	if err != nil {
		t.Fatal(err)
	}
	if flat.Count("bank") != 1 {
		t.Errorf("smoke bank distinct = %d, want 1", flat.Count("bank"))
	}
}

// TestEmptyMatrix: a fresh matrix reports no coverage.
func TestEmptyMatrix(t *testing.T) {
	m := NewMatrix()
	for _, ax := range Axes {
		if m.Count(ax) != 0 {
			t.Errorf("fresh matrix axis %s has %d values, want 0", ax, m.Count(ax))
		}
	}
}
