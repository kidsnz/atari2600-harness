package cyclebound

import (
	"path/filepath"
	"strings"
	"testing"
)

const multicolorAsm = "../../roms/techniques/multicolor48.asm"

// TestCertifySmoke: smoke certifies, carries hashes, and the ROM-derived core of
// the certificate is deterministic across runs (only the timestamp may differ).
func TestCertifySmoke(t *testing.T) {
	a, err := Certify(smokeAsm, 76)
	if err != nil {
		t.Fatal(err)
	}
	if !a.Certified {
		t.Fatalf("smoke must certify; got violations=%v unbounded=%v", a.Violations, a.Unbounded)
	}
	if a.Provenance.AsmSHA256 == "" || a.Provenance.BinSHA256 == "" {
		t.Fatal("certificate must carry asm+bin sha256")
	}
	if a.Provenance.ProverVersion == "" {
		t.Fatal("certificate must record the prover version")
	}
	b, err := Certify(smokeAsm, 76)
	if err != nil {
		t.Fatal(err)
	}
	if a.Certified != b.Certified || a.MaxWorst != b.MaxWorst ||
		a.Provenance.AsmSHA256 != b.Provenance.AsmSHA256 ||
		a.Provenance.BinSHA256 != b.Provenance.BinSHA256 {
		t.Fatal("certificate ROM-core not deterministic across runs")
	}
}

// TestCertifyOverrunNotCertified: a planted overrun yields a NOT-certified
// certificate (the falsifiable direction).
func TestCertifyOverrunNotCertified(t *testing.T) {
	c, err := Certify(overrunAsm, 76)
	if err != nil {
		t.Fatal(err)
	}
	if c.Certified {
		t.Fatal("litmus_overrun must NOT certify")
	}
}

// TestCertifyRecordsLemma: a kernel that relies on @lines must surface that
// declaration in the certificate (transparency about what the proof assumes).
func TestCertifyRecordsLemma(t *testing.T) {
	c, err := Certify(multicolorAsm, 76)
	if err != nil {
		t.Fatal(err)
	}
	if !c.Certified {
		t.Fatalf("multicolor48 must certify (relies on @lines 2)")
	}
	found := false
	for _, d := range c.LineDecls {
		if d.Lines == 2 {
			found = true
		}
	}
	if !found {
		t.Fatalf("certificate must record the @lines 2 declaration it relies on; got %v", c.LineDecls)
	}
}

// TestCertifyHashTiedToROM: different ROMs hash differently — tampering is
// detectable.
func TestCertifyHashTiedToROM(t *testing.T) {
	a, err := Certify(smokeAsm, 76)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Certify(overrunAsm, 76)
	if err != nil {
		t.Fatal(err)
	}
	if a.Provenance.BinSHA256 == b.Provenance.BinSHA256 {
		t.Fatal("distinct ROMs must have distinct bin hashes")
	}
}

// How many of the technique kernels the prover CERTIFIES is a headline figure the
// audit quotes in two places, and it moved without either being updated: recorded
// as 14/31, measured 15/31 after SD-6 removed the false positive that a shared
// two-call subroutine produced (which greened game_states).
//
// Pinned here so the next move is a failing test rather than two more stale
// sentences. The list is spelled out rather than counted, because "15" alone would
// pass if one ROM regressed and another improved on the same day.
func TestCertifiedTechniqueKernels(t *testing.T) {
	want := map[string]bool{
		"divtable": true, "flicker_multiplex": true, "game_states": true, "hscroll": true,
		"multicolor48": true, "pf_modes": true, "score6": true, "sfx_demo": true,
		"sprite_anim": true, "tia_pcm": true, "two_line_kernel": true, "two_line_vdel": true,
		"venetian": true, "vertical_pos": true, "vertical_pos_dcp": true,
	}
	files, err := filepath.Glob("../../roms/techniques/*.asm")
	if err != nil || len(files) == 0 {
		t.Skip("technique corpus unavailable")
	}
	got := map[string]bool{}
	for _, asm := range files {
		rep, err := Prove(asm, DefaultBudget)
		if err != nil {
			t.Fatalf("%s: %v", asm, err) // not a skip: a missing ROM would silently shrink the denominator
		}
		if rep.Certified {
			got[strings.TrimSuffix(filepath.Base(asm), ".asm")] = true
		}
	}
	for name := range want {
		if !got[name] {
			t.Errorf("%s no longer certifies — a REGRESSION, or the ROM changed", name)
		}
	}
	for name := range got {
		if !want[name] {
			t.Errorf("%s now certifies and did not before — an improvement; update this list AND the two "+
				"places docs/capability-gap-audit.md quotes the count, together", name)
		}
	}
	t.Logf("%d of %d technique kernels certify", len(got), len(files))
}
