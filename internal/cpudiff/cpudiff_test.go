package cpudiff

import (
	"fmt"
	"testing"
)

// --- always-on, no external binary: lock the differ logic itself ---

// TestDiffLogic is the falsifiable core (CI-locked, no perfect6502 needed): the
// comparator must report NO diff for identical results and MUST report the one
// planted field for a one-field mutation, in both directions.
func TestDiffLogic(t *testing.T) {
	base := Result{Status: "ok", A: 0x42, X: 0x10, Y: 0x20, S: 0xFD, P: 0xA4, PC: 0xF802, Cycles: 2,
		Writes: map[uint16]byte{0x1234: 0xAB}}

	if d := Compare(base, base); len(d) != 0 {
		t.Fatalf("identical results reported diffs: %v", d)
	}

	// each mutation must be caught (planted-discrepancy, both directions)
	muts := []struct {
		name string
		mod  func(r *Result)
	}{
		{"A", func(r *Result) { r.A ^= 0xFF }},
		{"X", func(r *Result) { r.X ^= 0xFF }},
		{"Y", func(r *Result) { r.Y ^= 0xFF }},
		{"S", func(r *Result) { r.S ^= 0xFF }},
		{"P", func(r *Result) { r.P ^= 0x01 }}, // a real flag bit, not masked 4/5
		{"PC", func(r *Result) { r.PC ^= 0x0001 }},
		{"cycles", func(r *Result) { r.Cycles++ }},
		{"writes", func(r *Result) { r.Writes = map[uint16]byte{0x1234: 0x00} }},
		{"status", func(r *Result) { r.Status = "jam" }},
	}
	for _, m := range muts {
		mut := base
		mut.Writes = map[uint16]byte{0x1234: 0xAB}
		m.mod(&mut)
		d := Compare(base, mut)
		if len(d) == 0 {
			t.Errorf("mutation of %s not detected", m.name)
			continue
		}
		found := false
		for _, f := range d {
			if f.Name == m.name {
				found = true
			}
		}
		if !found {
			t.Errorf("mutation of %s: diffs %v did not name it", m.name, d)
		}
		// symmetric
		if d2 := Compare(mut, base); len(d2) == 0 {
			t.Errorf("mutation of %s not detected (reversed)", m.name)
		}
	}
}

// TestPMaskIgnored confirms bits 4/5 of P are excluded (convention difference,
// not real register state).
func TestPMaskIgnored(t *testing.T) {
	a := Result{Status: "ok", P: 0x00}
	b := Result{Status: "ok", P: 0x30} // bits 4,5 only
	if d := Compare(a, b); len(d) != 0 {
		t.Fatalf("bits 4/5 of P should be ignored, got diffs: %v", d)
	}
	c := Result{Status: "ok", P: 0x02} // bit 1 differs => must be caught
	if d := Compare(a, c); len(d) == 0 {
		t.Fatal("a real P flag difference must be detected")
	}
}

// TestGopherKnownAnswers validates RunGopher + buildImage against hand-computed
// results (no perfect6502 needed).
func TestGopherKnownAnswers(t *testing.T) {
	cases := []struct {
		name   string
		v      Vector
		a      byte
		pc     uint16
		cycles int
		writes map[uint16]byte
	}{
		{"LDA #$42", Vector{A: 0x11, X: 0x22, Y: 0x33, S: 0xFD, P: 0x00,
			Mem: map[uint16]byte{0xF800: 0xA9, 0xF801: 0x42}}, 0x42, 0xF802, 2, nil},
		{"STA $1234", Vector{A: 0xAB, X: 0x22, Y: 0x33, S: 0xFD, P: 0x00,
			Mem: map[uint16]byte{0xF800: 0x8D, 0xF801: 0x34, 0xF802: 0x12}}, 0xAB, 0xF803, 4,
			map[uint16]byte{0x1234: 0xAB}},
	}
	for _, c := range cases {
		r, err := RunGopher(c.v)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if r.Status != "ok" {
			t.Fatalf("%s: status %s", c.name, r.Status)
		}
		if r.A != c.a || r.PC != c.pc || r.Cycles != c.cycles {
			t.Errorf("%s: A=%02X PC=%04X cyc=%d, want A=%02X PC=%04X cyc=%d",
				c.name, r.A, r.PC, r.Cycles, c.a, c.pc, c.cycles)
		}
		for addr, val := range c.writes {
			if r.Writes[addr] != val {
				t.Errorf("%s: write %04X=%02X, got %02X", c.name, addr, val, r.Writes[addr])
			}
		}
	}
}

// --- gated on the perfect6502 harness binary ---

func skipNoP6502(t *testing.T) string {
	t.Helper()
	exe := FindP6502()
	if exe == "" {
		t.Skip("p6502step not built (run scripts/install_perfect6502.sh) — skipping silicon differential")
	}
	return exe
}

// runBoth runs the same vectors on both engines.
func runBoth(t *testing.T, exe string, vs []Vector) ([]Result, []Result) {
	t.Helper()
	sil, err := RunP6502Batch(exe, vs)
	if err != nil {
		t.Fatalf("p6502step: %v", err)
	}
	gop := make([]Result, len(vs))
	for i, v := range vs {
		r, err := RunGopher(v)
		if err != nil {
			t.Fatalf("gopher vector %d: %v", i, err)
		}
		gop[i] = r
	}
	return gop, sil
}

// TestSiliconAgreesDocumented is the cross-validation: on legal opcodes the
// silicon netlist and the embedded CPU must agree on every field. Any diff here
// is either a real Gopher bug (a find) or a harness artifact (must fix) — never
// silently tolerated.
func TestSiliconAgreesDocumented(t *testing.T) {
	exe := skipNoP6502(t)
	vs := GenVectors(1, 400, DocumentedSmoke)
	gop, sil := runBoth(t, exe, vs)
	bad := 0
	for i := range vs {
		if HaltEquivalent(gop[i].Status, sil[i].Status) {
			continue
		}
		d := Compare(gop[i], sil[i])
		if len(d) > 0 {
			bad++
			if bad <= 12 {
				t.Errorf("op %02X regs[A=%02X X=%02X Y=%02X S=%02X P=%02X] ops[%02X %02X]: %v",
					vs[i].Opcode(), vs[i].A, vs[i].X, vs[i].Y, vs[i].S, vs[i].P,
					vs[i].Mem[0xF801], vs[i].Mem[0xF802], d)
			}
		}
	}
	if bad > 0 {
		t.Fatalf("%d/%d documented-opcode vectors diverged silicon vs gopher", bad, len(vs))
	}
}

// TestSiliconAllOpcodesClassified runs the full 256-opcode space and requires
// that every silicon-vs-Gopher divergence is either a halt or one of the
// classified illegal opcodes (ExpectedDivergence). A divergence on any
// documented opcode fails — that would be a real CPU bug or a harness artifact.
func TestSiliconAllOpcodesClassified(t *testing.T) {
	exe := skipNoP6502(t)
	if testing.Short() {
		t.Skip("skipping full 256-opcode silicon sweep in -short mode")
	}
	vs := GenVectors(1, 4000, AllOpcodes())
	gop, sil := runBoth(t, exe, vs)
	unexpected := map[byte]int{}
	for i := range vs {
		if HaltEquivalent(gop[i].Status, sil[i].Status) {
			continue
		}
		if len(Compare(gop[i], sil[i])) == 0 {
			continue
		}
		if _, ok := ExpectedDivergence(vs[i].Opcode()); ok {
			continue
		}
		unexpected[vs[i].Opcode()]++
	}
	if len(unexpected) > 0 {
		t.Fatalf("unexpected silicon divergences on non-allow-listed opcodes: %v", unexpected)
	}
}

// TestSiliconDeterministic: identical vectors must yield identical silicon
// results across runs.
func TestSiliconDeterministic(t *testing.T) {
	exe := skipNoP6502(t)
	vs := GenVectors(7, 64, DocumentedSmoke)
	a, err := RunP6502Batch(exe, vs)
	if err != nil {
		t.Fatal(err)
	}
	b, err := RunP6502Batch(exe, vs)
	if err != nil {
		t.Fatal(err)
	}
	for i := range vs {
		if d := Compare(a[i], b[i]); len(d) > 0 {
			t.Fatalf("nondeterministic at vector %d (op %02X): %v", i, vs[i].Opcode(), d)
		}
	}
}

// helper to keep fmt imported for ad-hoc debugging
var _ = fmt.Sprintf

// TestAllowListEntriesEarnTheirPlace is the other direction of
// TestSiliconAllOpcodesClassified. That test asks "does anything diverge that is not
// allowed"; this one asks "is anything allowed that never diverges".
//
// An allow-list entry that is exercised and never fires silences a whole opcode for
// nothing: a genuine engine bug there would be waved through under the label "known
// unstable", and no test in the suite could see it. That was live —
// LXA/LAX #imm ($AB) was on the list, and across seeds 1-4 it was exercised 110 times
// and diverged ZERO times. It was removed rather than kept as a comfortable
// exception; if the netlist and Gopher2600 ever do disagree on $AB, that is now a
// failure a human reads.
//
// The sweep is deterministic (seeded generator, deterministic engines), so this pins
// a fact and not a probability.
func TestAllowListEntriesEarnTheirPlace(t *testing.T) {
	exe := skipNoP6502(t)
	if testing.Short() {
		t.Skip("skipping allow-list sweep in -short mode")
	}
	vs := GenVectors(1, 4000, AllOpcodes())
	gop, sil := runBoth(t, exe, vs)

	diverged, tested := map[byte]int{}, map[byte]int{}
	for i := range vs {
		op := vs[i].Opcode()
		if _, ok := ExpectedDivergence(op); !ok {
			continue
		}
		tested[op]++
		if HaltEquivalent(gop[i].Status, sil[i].Status) {
			continue
		}
		if len(Compare(gop[i], sil[i])) > 0 {
			diverged[op]++
		}
	}

	allowed := ExpectedDivergenceOpcodes()
	if len(allowed) == 0 {
		t.Fatal("allow-list is empty — this test would pass while checking nothing")
	}
	for _, op := range allowed {
		if tested[op] == 0 {
			t.Errorf("allow-listed opcode $%02X was never generated in %d vectors, so its entry is "+
				"unverifiable here", op, len(vs))
			continue
		}
		if diverged[op] == 0 {
			class, _ := ExpectedDivergence(op)
			t.Errorf("allow-listed opcode $%02X (%s) was exercised %d times and never diverged — the "+
				"entry silences the opcode for nothing; remove it so a real divergence there fails",
				op, class, tested[op])
		}
	}
	t.Logf("allow-list: %d entries, all exercised and all diverging", len(allowed))
}
