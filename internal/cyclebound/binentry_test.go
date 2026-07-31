package cyclebound

import (
	"os"
	"path/filepath"
	"testing"
)

// TestProveReadsARawCartridgeImage pins that the prover accepts a `.bin`.
//
// It did not, and the absence was the quiet part. SD-0c taught the DECODER to read real
// cartridges — Outlaw's 2K went from 0 instructions to 931, Combat's from 0 to 838 —
// but nothing public took a raw image: `Prove` and `timinglint` assemble their input, so
// a commercial ROM came back as "Unknown Mnemonic". The capability existed and was
// unreachable, which is the same shape as a description that denies what a tool can do,
// and it blocked the whole casebook line, where the ROMs are commercial by definition.
//
// A raw image has no listing and no symbols, so `@lines`/`@amax` and label locations are
// absent rather than wrong — `srcmap.Map` is nil-safe throughout for exactly this.
func TestProveReadsARawCartridgeImage(t *testing.T) {
	// Any 4K/2K image in the tree works; the litmus binaries are built by CI.
	bins, _ := filepath.Glob("../../roms/litmus/*.bin")
	if len(bins) == 0 {
		t.Skip("no built litmus binaries")
	}
	var pick string
	for _, b := range bins {
		if fi, err := os.Stat(b); err == nil && fi.Size() == 4096 {
			pick = b
			break
		}
	}
	if pick == "" {
		t.Skip("no 4K image available")
	}

	fromBin, err := Prove(pick, DefaultBudget)
	if err != nil {
		t.Fatalf("Prove on a raw image: %v", err)
	}
	if fromBin.Regions == 0 {
		t.Fatal("a raw image produced zero regions — that is what the old failure looked like " +
			"from the outside, except it used to be an assembler error")
	}
	if !fromBin.Converged {
		t.Errorf("%s: the analysis did not converge", pick)
	}

	// The same ROM through its source must agree on the region count: the raw path is
	// meant to lose only what SOURCE carries (labels, @lines), not change the analysis.
	asm := pick[:len(pick)-len(".bin")] + ".asm"
	if _, err := os.Stat(asm); err != nil {
		return
	}
	fromAsm, err := Prove(asm, DefaultBudget)
	if err != nil {
		t.Fatalf("Prove on %s: %v", asm, err)
	}
	if fromBin.Regions != fromAsm.Regions {
		t.Errorf("%s: %d regions from the image but %d from the source — the raw path should lose "+
			"source annotations, not regions", filepath.Base(pick), fromBin.Regions, fromAsm.Regions)
	}
	if fromBin.MaxWorst != fromAsm.MaxWorst {
		t.Errorf("%s: worst region %d from the image vs %d from the source",
			filepath.Base(pick), fromBin.MaxWorst, fromAsm.MaxWorst)
	}
	t.Logf("%s: %d regions, max_worst %d, identical through source and through the raw image",
		filepath.Base(pick), fromBin.Regions, fromBin.MaxWorst)
}
