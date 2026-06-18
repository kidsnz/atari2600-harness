package oracle

import "testing"

// TestMameOracle (gated on MAME being installed): the headless MAME oracle agrees
// with the embedded Gopher2600 oracle on a clean ROM, and the two vote unanimously.
// Proves the third independent oracle is wired in (VV-6).
func TestMameOracle(t *testing.T) {
	if !MameAvailable() {
		t.Skip("mame not installed (brew install mame) — skipping cross-oracle check")
	}
	const rom = "../../roms/litmus/smoke.bin"
	const frames = 10

	mame, err := Mame{}.DumpRAM(rom, frames)
	if err != nil {
		t.Fatal(err)
	}
	if mame[0] != 66 {
		t.Fatalf("MAME read ram.0x80 = %d, want 66", mame[0])
	}
	goph, err := Gopher{}.DumpRAM(rom, frames)
	if err != nil {
		t.Fatal(err)
	}
	if d := Diff(goph, mame); len(d) != 0 {
		t.Fatalf("Gopher2600 vs MAME disagree on a clean ROM at RAM offsets %v", d)
	}

	_, dissenters, ok, err := Vote([]Oracle{Gopher{}, Mame{}}, rom, frames)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || len(dissenters) != 0 {
		t.Fatalf("Gopher2600+MAME should agree unanimously, got ok=%v dissenters=%v", ok, dissenters)
	}
}
