package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kidsnz/atari2600-harness/internal/emu"
)

// TestGeometryComesFromTheMapperNotTheFileLength pins the number this tool used to
// state wrongly.
//
// `len(rom)/4096` is the Atari family's arithmetic and nothing else's. Measured
// 2026-08-04 on fixtures that all load and all run:
//
//	cart_3e      (3E)    4 banks of 2048 — len/4096 said 2 banks of 4K
//	cart_3eplus  (3E+)   4 banks of 1024 at four origins — len/4096 said NOT BANKED
//	cart_dpc     (DPC)   2 banks of 4096 + 2048 bytes of graphics outside every bank
//	cart_f4sc    (F4SC)  8 banks of 4096 — the Atari case, unchanged
//
// and out of repo, on real cartridges: DeathMerchant (3E) is 24 banks of 2K where the
// old arithmetic said 12 of 4K, and a DPC+ demo is 6 banks where it said 8.
//
// The bank number was not merely printed in a header; it was attached to every table
// this tool matched. A table at file offset $1800 of a 3E cartridge was labelled "ROM
// bank 1 $F800" when it is bank 3 and a 3E bank is mapped at $1000-$17FF. Both halves
// wrong, stated as fact.
func TestGeometryComesFromTheMapperNotTheFileLength(t *testing.T) {
	cases := []struct {
		file      string
		id        string
		banks     int
		bankSize  int
		origins   int
		atari4K   bool
		oldBanks  int // what len(rom)/4096 used to claim
		describeC string
	}{
		{"cart_3e.bin", "3E", 4, 2048, 1, false, 2, "NOT the Atari"},
		{"cart_3eplus.bin", "3E+", 4, 1024, 4, false, 1, "NOT the Atari"},
		{"cart_dpc.bin", "DPC", 2, 4096, 1, true, 2, "not in any bank"},
		{"cart_f6sc.bin", "F6SC", 4, 4096, 1, true, 4, "4 banks of 4K"},
		{"cart_f4sc.bin", "F4SC", 8, 4096, 1, true, 8, "8 banks of 4K"},
	}
	dir := filepath.Join("..", "..", "roms", "carts")
	if asms, _ := filepath.Glob(filepath.Join(dir, "*.asm")); len(asms) == 0 {
		t.Skipf("%s holds no .asm — the cartridge-format fixture corpus is not present", dir)
	}
	var missing []string
	for _, c := range cases {
		if _, err := os.Stat(filepath.Join(dir, c.file)); err != nil {
			missing = append(missing, c.file)
		}
	}
	if len(missing) == len(cases) {
		t.Skipf("none of the %d cartridge fixtures are assembled", len(cases))
	}
	if len(missing) > 0 {
		t.Fatalf("%d of %d fixtures are assembled and %d are missing (%v) — a partial corpus would "+
			"check the geometries that happened to build", len(cases)-len(missing), len(cases),
			len(missing), missing)
	}

	wrongBefore := 0
	for _, c := range cases {
		path := filepath.Join(dir, c.file)
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		e, err := emu.New("NTSC")
		if err != nil {
			t.Fatal(err)
		}
		if err := e.LoadROM(path); err != nil {
			t.Fatalf("%s: %v", c.file, err)
		}
		g := readGeometry(e)
		if g.id != c.id || g.banks != c.banks || g.bankSize != c.bankSize ||
			g.origins != c.origins || g.atari4K != c.atari4K {
			t.Errorf("%s: geometry = {id:%s banks:%d bankSize:%d origins:%d atari4K:%v}, want "+
				"{id:%s banks:%d bankSize:%d origins:%d atari4K:%v}",
				c.file, g.id, g.banks, g.bankSize, g.origins, g.atari4K,
				c.id, c.banks, c.bankSize, c.origins, c.atari4K)
		}
		if d := g.describe(int(info.Size())); !strings.Contains(d, c.describeC) {
			t.Errorf("%s: the banking note is %q, which does not contain %q", c.file, d, c.describeC)
		}
		// The premise of the whole change: the old arithmetic really did disagree.
		if old := int(info.Size()) / 4096; old != c.oldBanks {
			t.Errorf("%s: len/4096 = %d, but this test was written against %d — the measurement "+
				"below is no longer the one that was taken", c.file, old, c.oldBanks)
		} else if old != c.banks {
			wrongBefore++
		}
	}
	// MEASURED, and the number is smaller than it looks: len/4096 gets the bank COUNT
	// wrong on 2 of these 5 (3E says 2 for 4, 3E+ says 1 for 4). DPC's count is right
	// BY COINCIDENCE — 10240/4096 truncates to 2, which is the true bank count — while
	// the file's last 2048 bytes belong to no bank at all, so an offset in that range
	// used to be labelled "ROM bank 2" of a two-bank cartridge. Both failures are real
	// and only one of them is visible in a bank count, which is why the DPC case is
	// asserted separately below rather than counted here.
	if wrongBefore < 2 {
		t.Errorf("only %d of %d fixtures had a bank count that len/4096 got wrong; this test exists "+
			"because that arithmetic was wrong on real cartridges, so a corpus where it is right "+
			"is not exercising the defect", wrongBefore, len(cases))
	}
	t.Logf("%d of %d fixtures had their bank count misstated by len(rom)/4096", wrongBefore, len(cases))

	// The DPC coincidence, asserted rather than assumed: there really are bytes in that
	// file outside every bank, and the note really does say so.
	dpc := filepath.Join(dir, "cart_dpc.bin")
	info, err := os.Stat(dpc)
	if err != nil {
		t.Fatal(err)
	}
	e, err := emu.New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM(dpc); err != nil {
		t.Fatal(err)
	}
	g := readGeometry(e)
	outside := int(info.Size()) - g.banks*4096
	if outside != 2048 {
		t.Errorf("cart_dpc has %d bytes outside its %d banks, want 2048 — a DPC dump is two 4K "+
			"banks plus 2K of graphics the CPU can never address, and that 2K is what the old "+
			"arithmetic attributed to a third bank", outside, g.banks)
	}
}

// TestAtariGeometryStillLabelsBanks is the negative control: the fix must not turn
// the F8/F6/F4 family — the case that was always right — into file offsets.
func TestAtariGeometryStillLabelsBanks(t *testing.T) {
	path := filepath.Join("..", "..", "roms", "litmus", "litmus_bank.bin")
	info, err := os.Stat(path)
	if err != nil {
		t.Skipf("%s is not assembled", path)
	}
	e, err := emu.New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM(path); err != nil {
		t.Fatal(err)
	}
	g := readGeometry(e)
	if !g.atari4K || g.banks != 2 || g.id != "F8" {
		t.Fatalf("litmus_bank is an 8K F8 cartridge; geometry says {id:%s banks:%d atari4K:%v}",
			g.id, g.banks, g.atari4K)
	}
	d := g.describe(int(info.Size()))
	if !strings.Contains(d, "2 banks of 4K") || strings.Contains(d, "file offsets") {
		t.Errorf("the F8 banking note is %q — the Atari family must still get bank numbers and "+
			"window addresses, or the fix has cost the case it was not about", d)
	}
}
