package cyclebound

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The G1 refusal table. Every advanced cartridge scheme the harness has a fixture
// for must be DECLINED, and the decline must name the mapper the engine
// fingerprinted and say why — because a decline that does not say why is one nobody
// can check, and a wrong number here is worse than no number at all.
//
// The `wantReason` column is the part that is not decoration. Before 2026-08-04, DPC
// was declined by the LAST guard in the chain — "this mapper is not in the table of
// checked bank-switch rules" — and that refusal was one plausible source-reading away
// from being edited out. Measured: DPC's banks are 2 × 4096 at origin $F000, it
// publishes $1FF8:BANK0 / $1FF9:BANK1, `GetBank` says IsRAM false everywhere, and
// `mapper_dpc.go bankswitch` takes the target from the address alone — which IS the
// rule this package models. Anyone who checked the switch would have been right about
// the switch and wrong about the cartridge, because $1000-$107F is the data-fetcher /
// RNG / music register file and the image's bytes there are never what the CPU reads.
// The refusal now rests on the window not being the image, which is the fundamental
// objection and the one that cannot be argued away by reading the switch.
var advCartDeclines = []struct {
	file       string
	mapper     string
	wantReason string
}{
	{"cart_f6sc.bin", "F6SC", "a superchip overlays 128 bytes of RAM"},
	{"cart_f4sc.bin", "F4SC", "a superchip overlays 128 bytes of RAM"},
	{"cart_3e.bin", "3E", "publishes a cartridge-RAM bus"},
	{"cart_3eplus.bin", "3E+", "publishes a cartridge-RAM bus"},
	// DPC's window is NOT RAM — $1000-$107F is the data-fetcher/RNG/music
	// register file. The old flattened message called it RAM, which sent a reader
	// hunting for a RAM overlay that does not exist; the decline now repeats the
	// bus interface's own words.
	{"cart_dpc.bin", "DPC", "static-data area the CPU reads through registers"},
}

// TestAdvancedCartridgesAreDeclinedByNameAndReason grades the whole G1 fixture
// corpus in one place, so "N of M schemes are refused" is a number this suite
// produces rather than a claim in a document.
func TestAdvancedCartridgesAreDeclinedByNameAndReason(t *testing.T) {
	dir := filepath.Join("..", "..", "roms", "carts")
	if asms, _ := filepath.Glob(filepath.Join(dir, "*.asm")); len(asms) == 0 {
		t.Skipf("%s holds no .asm — the cartridge-format fixture corpus is not present", dir)
	}
	var present, missing []string
	for _, c := range advCartDeclines {
		if _, err := os.Stat(filepath.Join(dir, c.file)); err == nil {
			present = append(present, c.file)
		} else {
			missing = append(missing, c.file)
		}
	}
	if len(present) == 0 {
		t.Skipf("none of the %d cartridge fixtures are assembled", len(advCartDeclines))
	}
	if len(missing) > 0 {
		t.Fatalf("%d of %d fixtures are assembled and %d are missing (%v) — grading a subset would "+
			"report the schemes that happened to build as though they were all of them",
			len(present), len(advCartDeclines), len(missing), missing)
	}

	declined := 0
	for _, c := range advCartDeclines {
		rep, err := Prove(filepath.Join(dir, c.file), 76)
		if err != nil {
			t.Errorf("%s: Prove returned an error rather than a verdict: %v", c.file, err)
			continue
		}
		if rep.BankedDeclined == "" {
			t.Errorf("%s (mapper %s) was ANALYSED. Its cartridge window is not the image, so every "+
				"value range folded out of it bounds a loop on data the hardware never holds — "+
				"regions=%d certified=%v max_worst=%d",
				c.file, c.mapper, rep.Regions, rep.Certified, rep.MaxWorst)
			continue
		}
		declined++
		if !strings.Contains(rep.BankedDeclined, c.mapper) {
			t.Errorf("%s: the decline does not name the mapper the ENGINE fingerprinted (%s), so a "+
				"size-based guess and the emulator could disagree about which machine is described; "+
				"got %q", c.file, c.mapper, rep.BankedDeclined)
		}
		if !strings.Contains(rep.BankedDeclined, c.wantReason) {
			t.Errorf("%s: the decline reason is %q, which does not contain %q. A refusal that has "+
				"moved to a weaker guard still refuses today and can be argued away tomorrow",
				c.file, rep.BankedDeclined, c.wantReason)
		}
		if rep.Certified {
			t.Errorf("%s: a declined image certified", c.file)
		}
	}
	if declined != len(advCartDeclines) {
		t.Errorf("%d of %d advanced-cartridge schemes were refused; the other %d produced a number",
			declined, len(advCartDeclines), len(advCartDeclines)-declined)
	}
	t.Logf("%d of %d advanced-cartridge fixtures refused, each naming its mapper and its reason",
		declined, len(advCartDeclines))
}
