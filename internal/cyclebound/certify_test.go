package cyclebound

import "testing"

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
