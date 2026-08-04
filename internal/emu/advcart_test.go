package emu

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// advCart is one fixture in roms/carts and everything the engine must say about it.
//
// The bank COUNT is in the table for a reason that is not cosmetic. `read_bank` and
// `CartInfo` were verified on F8/F6/F4 and on nothing else, so "the bank count is
// right" was a claim about three mappers. Here it is a claim about five more, and two
// of them (3E, 3E+) have banks that are not 4K at all — the size the rest of this
// repo used to assume.
type advCart struct {
	file      string
	mapper    string
	banks     int
	bankBytes int    // what the engine hands back per bank, NOT len(file)/banks
	origins   int    // how many places in the window a bank may appear
	notImage  bool   // does the CPU read something other than the image in the window?
	because   string // a substring of the reason, so a silent change of reason is caught
}

var advCarts = []advCart{
	{"cart_f6sc.bin", "F6SC", 4, 4096, 1, true, "superchip"},
	{"cart_f4sc.bin", "F4SC", 8, 4096, 1, true, "superchip"},
	{"cart_3e.bin", "3E", 4, 2048, 1, true, "cartridge-RAM bus"},
	{"cart_3eplus.bin", "3E+", 4, 1024, 4, true, "cartridge-RAM bus"},
	// DPC has no RAM anywhere and says so: GetBank reports IsRAM false at every
	// address and the mapper publishes no RAM bus. What it publishes is a static
	// graphics area and a register file, and $1000-$107F is the register file.
	{"cart_dpc.bin", "DPC", 2, 4096, 1, true, "static-data area"},
}

// cartsDir returns the fixture directory, or skips when the .bin are not built.
//
// CI assembles roms/**/*.asm before running tests; a fresh checkout that has not run
// DASM has the sources and no binaries. Absent is a skip WITH A REASON. PARTIAL is a
// failure: a table that silently grades 2 of 5 fixtures is the exact defect this
// repo keeps finding, and "some of the corpus" must never read as "the corpus".
func cartsDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join("..", "..", "roms", "carts")
	asms, _ := filepath.Glob(filepath.Join(dir, "*.asm"))
	if len(asms) == 0 {
		t.Skipf("%s holds no .asm — the cartridge-format fixture corpus is not present", dir)
	}
	var missing []string
	for _, c := range advCarts {
		if _, err := os.Stat(filepath.Join(dir, c.file)); err != nil {
			missing = append(missing, c.file)
		}
	}
	if len(missing) == len(advCarts) {
		t.Skipf("none of the %d cartridge fixtures are assembled (build with: "+
			"for f in %s/*.asm; do dasm \"$f\" -f3 -o\"${f%%.asm}.bin\"; done)", len(advCarts), dir)
	}
	if len(missing) > 0 {
		t.Fatalf("%d of %d cartridge fixtures are assembled and %d are missing (%v) — a partial "+
			"corpus would grade some mappers and silently skip the rest",
			len(advCarts)-len(missing), len(advCarts), len(missing), missing)
	}
	return dir
}

// TestAdvancedCartridgesLoadWithTheRightGeometry is the G1 measurement, pinned.
//
// Before it, the harness had litmus fixtures for F8, F6, F4 and one F8SC, and every
// other scheme the engine recognises had zero coverage — `read_bank` included. The
// numbers below were measured on 2026-08-04 and each one is a thing that could
// silently change: a mapper's fingerprint, its bank count, its bank size, or the
// number of origins a bank can appear at.
func TestAdvancedCartridgesLoadWithTheRightGeometry(t *testing.T) {
	dir := cartsDir(t)
	for _, c := range advCarts {
		t.Run(c.mapper, func(t *testing.T) {
			e, err := New("NTSC")
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, c.file)
			if err := e.LoadROM(path); err != nil {
				t.Fatalf("%s did not load at all: %v", c.file, err)
			}
			id, banks := e.CartInfo()
			if id != c.mapper {
				t.Errorf("%s fingerprinted as %q, want %q — the fixture no longer exercises the "+
					"scheme it is named for", c.file, id, c.mapper)
			}
			if banks != c.banks {
				t.Errorf("%s: CartInfo reports %d banks, want %d", c.file, banks, c.banks)
			}
			// read_bank must answer, and answer inside the range it just claimed.
			n, isRAM := e.Bank()
			if n < 0 || n >= banks {
				t.Errorf("%s: read_bank reports bank %d, outside the %d banks the mapper claims",
					c.file, n, banks)
			}
			if isRAM {
				t.Errorf("%s: read_bank says the boot bank is RAM; every fixture here boots from ROM",
					c.file)
			}
			contents, err := e.CopyBanks()
			if err != nil {
				t.Fatalf("%s: CopyBanks: %v", c.file, err)
			}
			if len(contents) != c.banks {
				t.Fatalf("%s: CopyBanks returned %d banks, CartInfo said %d",
					c.file, len(contents), c.banks)
			}
			for _, b := range contents {
				if len(b.Data) != c.bankBytes {
					t.Errorf("%s: bank %d is %d bytes, want %d — bank size is NOT the file length "+
						"divided by the bank count on this scheme", c.file, b.Number, len(b.Data), c.bankBytes)
				}
				if len(b.Origins) != c.origins {
					t.Errorf("%s: bank %d maps at %d origin(s) %v, want %d",
						c.file, b.Number, len(b.Origins), b.Origins, c.origins)
				}
			}
		})
	}
}

// TestAdvancedCartridgeWindowsAreNotTheImage is the soundness half.
//
// Every one of these fixtures puts something other than image bytes in $1000-$1FFF,
// and the whole value of saying so is that a static analysis folding cartridge bytes
// into a value range must refuse them. The REASON is asserted as well as the verdict:
// DPC's window is not RAM, it is a register file, and a guard that answered "yes,
// RAM" for the wrong reason would still be answering by luck.
func TestAdvancedCartridgeWindowsAreNotTheImage(t *testing.T) {
	dir := cartsDir(t)
	for _, c := range advCarts {
		t.Run(c.mapper, func(t *testing.T) {
			e, err := New("NTSC")
			if err != nil {
				t.Fatal(err)
			}
			if err := e.LoadROM(filepath.Join(dir, c.file)); err != nil {
				t.Fatal(err)
			}
			notImage, why, err := e.CartridgeWindowNotImage()
			if err != nil {
				t.Fatalf("%s: CartridgeWindowNotImage: %v", c.file, err)
			}
			if notImage != c.notImage {
				t.Fatalf("%s: window-not-image = %v (%q), want %v", c.file, notImage, why, c.notImage)
			}
			if !strings.Contains(why, c.because) {
				t.Errorf("%s: the reason is %q, which does not mention %q — the verdict may be right "+
					"for a reason that has moved", c.file, why, c.because)
			}
			// The legacy bool must agree, because internal/cyclebound refuses through it.
			legacy, err := e.MapsCartridgeRAM()
			if err != nil {
				t.Fatal(err)
			}
			if legacy != notImage {
				t.Errorf("%s: MapsCartridgeRAM=%v disagrees with CartridgeWindowNotImage=%v; the "+
					"refusal cyclebound prints would then not match the reason", c.file, legacy, notImage)
			}
		})
	}
}

// TestPlainCartridgeWindowsAreStillTheImage is the negative control for the guard
// above, and it is the one that stops it from spreading.
//
// A predicate that refuses everything is sound and useless. The guard's new arms ask
// the engine's bus interfaces rather than a list of mapper IDs, so the risk it takes
// on is over-refusal: a mapper that publishes some bus for an unrelated reason would
// start being declined, and every ROM in this repo that the static analysis currently
// handles would stop being analysed without anything saying so.
//
// The strongest available case is measured OUTSIDE this repository and is recorded
// here rather than asserted, because no image of that mapper is in it: Parker Bros
// (E0) maps three 1K segments — exotic enough to trip a careless predicate — and on
// the three E0 cartridges in the umbrella's reference archive it answers no on all
// four buses, which is correct, because an E0 window really is image bytes.
func TestPlainCartridgeWindowsAreStillTheImage(t *testing.T) {
	controls := []string{
		"../../roms/litmus/litmus_bank.bin",    // F8
		"../../roms/litmus/litmus_bank_f6.bin", // F6
		"../../roms/litmus/litmus_bank_f4.bin", // F4
		"../../roms/litmus/smoke.bin",          // 4K, not banked at all
		"../../roms/techniques/banked_game.bin",
	}
	var present []string
	for _, f := range controls {
		if _, err := os.Stat(f); err == nil {
			present = append(present, f)
		}
	}
	if len(present) == 0 {
		t.Skipf("none of the %d control ROMs are assembled; the corpus is not present", len(controls))
	}
	if len(present) != len(controls) {
		t.Fatalf("%d of %d control ROMs are assembled — a partial control is a control over "+
			"whichever mappers happened to build", len(present), len(controls))
	}
	for _, f := range present {
		e, err := New("NTSC")
		if err != nil {
			t.Fatal(err)
		}
		if err := e.LoadROM(f); err != nil {
			t.Fatal(err)
		}
		notImage, why, err := e.CartridgeWindowNotImage()
		if err != nil {
			t.Fatal(err)
		}
		if notImage {
			id, _ := e.CartInfo()
			t.Errorf("%s (mapper %s) is now refused as \"not the image\" because %q — this ROM's "+
				"window IS its image, so the guard has spread to cartridges the static analysis "+
				"was handling", filepath.Base(f), id, why)
		}
	}
}
