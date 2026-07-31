package cyclebound

import (
	"path/filepath"
	"testing"
)

// TestWriteIntoCodeIsReportedAndOnlyWhenItHappens covers SD-3's "a store landing in
// decoded code space is a fact, not a guess".
//
// It ships with a planted fixture rather than a corpus witness, and that is not a
// shortcut — measured before the detector existed, **133 ROMs and zero that write into
// the cartridge window at all**. A detector whose branch nothing reaches is not a check,
// so the ROM and the code arrived together.
//
// On this machine the planted store does nothing: cartridge ROM is read-only and a 4K
// image has no hotspots to trip, so litmus_smc runs identically to its twin. The fact
// being reported — this instruction's effective address is an instruction — is true
// whether or not the hardware honours the write, and it is the fact that would matter on
// a cartridge with RAM, where it would land.
func TestWriteIntoCodeIsReportedAndOnlyWhenItHappens(t *testing.T) {
	planted, err := DefUse("../../roms/litmus/litmus_smc.asm", DefaultBudget)
	if err != nil {
		t.Fatal(err)
	}
	if len(planted.WritesIntoCode) != 1 {
		t.Fatalf("litmus_smc: %d writes-into-code, want exactly the planted one: %+v",
			len(planted.WritesIntoCode), planted.WritesIntoCode)
	}
	w := planted.WritesIntoCode[0]
	if !w.Exact {
		t.Errorf("the planted store is `sta Target`, a single known address, so it must be reported "+
			"as exact — a may-set entry is a possibility, not a fact: %+v", w)
	}
	if len(w.Targets) != 1 {
		t.Errorf("expected one target, got %v", w.Targets)
	}

	// The twin differs by one operand: the store aims at RAM instead of at code.
	clean, err := DefUse("../../roms/litmus/litmus_smc_clean.asm", DefaultBudget)
	if err != nil {
		t.Fatal(err)
	}
	if len(clean.WritesIntoCode) != 0 {
		t.Errorf("litmus_smc_clean stores to RAM, yet reported %d writes-into-code — then the planted "+
			"ROM's report is not attributable to its store: %+v", len(clean.WritesIntoCode),
			clean.WritesIntoCode)
	}

	// And nothing else in the tree may fire. A detector that flags real kernels is
	// worse than none: an indexed store spans up to 256 addresses and a 4K image is
	// mostly code, so overlaps are cheap to manufacture by accident.
	var files []string
	for _, d := range []string{"../../roms/techniques", "../../roms/litmus", "../../roms/exerciser"} {
		m, _ := filepath.Glob(d + "/*.asm")
		files = append(files, m...)
	}
	scanned := 0
	for _, f := range files {
		if filepath.Base(f) == "litmus_smc.asm" {
			continue
		}
		rep, err := DefUse(f, DefaultBudget)
		if err != nil || rep == nil || rep.FlatBankOnly != "" {
			continue
		}
		scanned++
		if len(rep.WritesIntoCode) > 0 {
			t.Errorf("%s writes into decoded code: %+v", filepath.Base(f), rep.WritesIntoCode)
		}
	}
	if scanned < 50 {
		t.Fatalf("only %d ROMs were analysable — the false-positive half of this test covers too "+
			"little to mean anything", scanned)
	}
	t.Logf("planted fixture reports 1 exact write into code; %d other ROMs report none", scanned)
}
