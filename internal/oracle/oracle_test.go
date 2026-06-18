package oracle

import "testing"

// TestGopherOracle: the embedded Gopher2600 oracle is deterministic and reads the
// expected RAM (smoke.bin holds ram.0x80 == 66 from power-on).
func TestGopherOracle(t *testing.T) {
	g := Gopher{}
	a, err := g.DumpRAM("../../roms/litmus/smoke.bin", 5)
	if err != nil {
		t.Fatal(err)
	}
	if a[0] != 66 {
		t.Fatalf("smoke ram.0x80 = %d, want 66", a[0])
	}
	b, err := g.DumpRAM("../../roms/litmus/smoke.bin", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(Diff(a, b)) != 0 {
		t.Fatalf("oracle not deterministic: diffs at %v", Diff(a, b))
	}
}

type fakeOracle struct {
	name string
	dump RAMDump
}

func (f fakeOracle) Name() string                         { return f.name }
func (f fakeOracle) DumpRAM(string, int) (RAMDump, error) { return f.dump, nil }

// TestVoteDissent: a lone dissenter is surfaced against the majority — the
// cross-oracle catch (planted discrepancy: one oracle disagrees, it is named).
func TestVoteDissent(t *testing.T) {
	good := RAMDump{1, 2, 3}
	bad := RAMDump{1, 2, 9} // hardware-grade dissent at offset 2
	oracles := []Oracle{
		fakeOracle{"gopher2600", good},
		fakeOracle{"stella", good},
		fakeOracle{"silicon", bad},
	}
	maj, dissenters, ok, err := Vote(oracles, "x.bin", 3)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatalf("expected a strict majority")
	}
	if maj != good {
		t.Fatalf("majority dump wrong")
	}
	if len(dissenters) != 1 || dissenters[0] != "silicon" {
		t.Fatalf("expected the lone dissenter 'silicon', got %v", dissenters)
	}

	// all-agree: no dissenters.
	if _, d, ok, _ := Vote([]Oracle{fakeOracle{"a", good}, fakeOracle{"b", good}}, "x", 3); !ok || len(d) != 0 {
		t.Fatalf("unanimous vote must have no dissenters (ok=%v d=%v)", ok, d)
	}
}
