package emu

import (
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// TestHScrollTableIsAPeriodicStripeNotARingRotation settles what `hscroll`'s eight-phase table
// actually is, before anyone trades it for the runtime rotation that looks equivalent.
//
// Manuel Polik posted a runtime version in 2002 — alternating `ROR`/`ROL` across PF2/PF1/PF0 with a
// wrap at the end 〔`200209/msg00126`〕 — and the obvious reading is that harness spends 24 bytes of
// ROM on a table to avoid spending cycles on that rotation. **It is not the same operation.**
//
// Read out of `roms/techniques/hscroll.asm` and reassembled into the twenty bits as the screen shows
// them (PF0 `D4..D7`, PF1 `D7..D0`, PF2 `D0..D7`), measured 2026-09-07:
//
//	each phase IS the previous shifted left by one
//	but the bit shifted IN alternates: 0,0,0,0,1,1,1 across the seven steps
//	rotating the twenty-bit ring by eight does NOT return phase 0
//	the pattern's period is 8, and 8 does not divide 20
//
// ★**So the table holds eight phases of an eight-periodic stripe, not eight rotations of a
// playfield.** A true ring rotation repeats after twenty steps; this repeats after eight, which is
// why `and #7` is correct here and would be wrong for an arbitrary picture.
//
// ★★**The trade is therefore not the one it looks like.** 24 bytes buys eight phases of *one
// periodic* pattern. An arbitrary twenty-bit playfield tabled the same way needs twenty phases —
// **60 bytes** — or a runtime rotation and its cycles. Anyone "simplifying" this table into a
// rotation gets a different picture, silently, for every pattern whose period does not divide 20.
//
// Raised by the mailing-list distillation (helper-1) as a cycles-versus-ROM question; the answer is
// that the two sides are not doing the same thing.
func TestHScrollTableIsAPeriodicStripeNotARingRotation(t *testing.T) {
	src, err := os.ReadFile("../../roms/techniques/hscroll.asm")
	if err != nil {
		t.Fatal(err)
	}
	table := func(label string) []int {
		re := regexp.MustCompile(`(?m)^` + label + `:\s*\.?byte\s+(.*)$`)
		m := re.FindSubmatch(src)
		if m == nil {
			t.Fatalf("no %s table in hscroll.asm — the ROM was restructured and this test is "+
				"reading nothing", label)
		}
		var out []int
		for _, f := range strings.Split(string(m[1]), ",") {
			f = strings.TrimSpace(strings.SplitN(f, ";", 2)[0])
			f = strings.TrimPrefix(f, "$")
			v, err := strconv.ParseUint(f, 16, 8)
			if err != nil {
				t.Fatalf("%s: cannot read %q as a byte: %v", label, f, err)
			}
			out = append(out, int(v))
		}
		return out
	}
	pf0, pf1, pf2 := table("ScrPF0"), table("ScrPF1"), table("ScrPF2")
	if len(pf0) != 8 || len(pf1) != 8 || len(pf2) != 8 {
		t.Fatalf("the tables hold %d/%d/%d entries, want 8 each", len(pf0), len(pf1), len(pf2))
	}

	// The twenty bits in the order the beam paints them.
	ring := func(p int) []int {
		var b []int
		for i := 4; i < 8; i++ {
			b = append(b, (pf0[p]>>i)&1)
		}
		for i := 7; i >= 0; i-- {
			b = append(b, (pf1[p]>>i)&1)
		}
		for i := 0; i < 8; i++ {
			b = append(b, (pf2[p]>>i)&1)
		}
		return b
	}
	var phases [8][]int
	for p := 0; p < 8; p++ {
		phases[p] = ring(p)
		if len(phases[p]) != 20 {
			t.Fatalf("phase %d assembled to %d bits, want 20", p, len(phases[p]))
		}
	}

	// 1. Each phase is the previous shifted left by one.
	incoming := map[int]int{}
	for p := 0; p < 7; p++ {
		for j := 0; j < 19; j++ {
			if phases[p][j+1] != phases[p+1][j] {
				t.Fatalf("phase %d is not phase %d shifted left by one (bit %d)", p+1, p, j)
			}
		}
		incoming[phases[p+1][19]]++
	}
	// 2. And the bit shifted in is NOT the one that fell off, which is what a rotation would do.
	if incoming[0] == 0 || incoming[1] == 0 {
		t.Errorf("the incoming bit was always %v across the seven steps. A ring rotation feeds back "+
			"the bit that left; this table does not, and if it now does then it IS a rotation and "+
			"the note on this test is wrong", incoming)
	}

	// 3. Rotating the twenty-bit ring by eight must NOT return phase 0.
	same := true
	for j := 0; j < 20; j++ {
		if phases[0][(j+8)%20] != phases[0][j] {
			same = false
			break
		}
	}
	if same {
		t.Error("rotating the ring by eight returns phase 0, so the table and a ring rotation agree " +
			"after all — which would only happen if the pattern's period divided 20")
	}

	// 4. The period, stated rather than implied: 8, which does not divide 20.
	period := 0
	for p := 1; p <= 20 && period == 0; p++ {
		ok := true
		for j := 0; j+p < 20; j++ {
			if phases[0][j] != phases[0][j+p] {
				ok = false
				break
			}
		}
		if ok {
			period = p
		}
	}
	if period != 8 {
		t.Errorf("the stripe's period is %d, was 8 — the whole point is that it does not divide 20, "+
			"so if this changed the conclusion has to be re-derived", period)
	}
	if 20%period == 0 {
		t.Errorf("the period %d now divides 20, which would make the table and a ring rotation "+
			"equivalent and this test's warning unnecessary", period)
	}
}
