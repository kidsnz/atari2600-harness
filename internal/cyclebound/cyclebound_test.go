package cyclebound

import (
	"testing"

	"github.com/kidsnz/atari2600-harness/internal/emu"
)

const (
	branchAsm  = "../../roms/litmus/cyclebound_branch.asm"
	branchBin  = "../../roms/litmus/cyclebound_branch.bin"
	overrunAsm = "../../roms/litmus/litmus_overrun.asm"
	overrunBin = "../../roms/litmus/litmus_overrun.bin"
	smokeAsm   = "../../roms/litmus/smoke.asm"
)

func mustProve(t *testing.T, asm string, budget int) *Report {
	t.Helper()
	rep, err := Prove(asm, budget)
	if err != nil {
		t.Fatalf("Prove(%s, %d): %v", asm, budget, err)
	}
	return rep
}

func runtimeOverruns(t *testing.T, bin string, budget int) bool {
	t.Helper()
	e, err := emu.New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM(bin); err != nil {
		t.Fatalf("load %s: %v", bin, err)
	}
	over, _, _, err := e.RunUntilBudget(3, budget)
	if err != nil {
		t.Fatal(err)
	}
	return over
}

// TestProveSmokeCertified: a well-behaved kernel proves clean — every
// WSYNC-to-WSYNC region is bounded and within budget.
func TestProveSmokeCertified(t *testing.T) {
	rep := mustProve(t, smokeAsm, 76)
	if !rep.Certified {
		t.Fatalf("smoke must certify; violations=%d unbounded=%d maxWorst=%d\nunbounded=%+v\nviolations=%+v",
			len(rep.Violations), len(rep.Unbounded), rep.MaxWorst, rep.Unbounded, rep.Violations)
	}
	if rep.MaxWorst > 76 {
		t.Fatalf("certified but maxWorst=%d > 76", rep.MaxWorst)
	}
	if rep.Regions < 4 {
		t.Fatalf("expected several WSYNC regions, got %d", rep.Regions)
	}
}

// TestProveBranchFlaggedButRuntimeLucky is the planted-discrepancy core: the
// static prover must flag the branch-dependent overrun over ALL paths, while
// the runtime guard (assert_line_budget), seeing only the flag==0 path it
// actually takes, passes. ∀ catches what ∃ misses.
func TestProveBranchFlaggedButRuntimeLucky(t *testing.T) {
	rep := mustProve(t, branchAsm, 76)
	if rep.Certified {
		t.Fatal("branch litmus must NOT certify — one path overruns")
	}
	if len(rep.Violations) == 0 {
		t.Fatalf("expected >=1 violation; unbounded=%+v", rep.Unbounded)
	}
	worst := rep.Violations[0].Worst
	if worst < 90 || worst > 120 {
		t.Fatalf("branch worst-case = %d cy, want ~101", worst)
	}
	if len(rep.Violations[0].Path) == 0 {
		t.Fatal("a violation must carry its worst-case path breakdown")
	}
	// The whole point: the LIVE run is lucky (flag==0 -> light path), so the
	// runtime guard does NOT fire. Only the static ∀ proof catches it.
	if runtimeOverruns(t, branchBin, 76) {
		t.Fatal("runtime guard fired; the litmus must be a lucky pass at runtime (static must be the catcher)")
	}
}

// TestProveOverrunLoopFlagged exercises counted-loop bounding: litmus_overrun's
// heavy line spins a ~20-iteration delay loop with no WSYNC (~100cy). The prover
// must bound that loop and flag the region (here runtime agrees — it always
// overruns — which sanity-checks the bound).
func TestProveOverrunLoopFlagged(t *testing.T) {
	rep := mustProve(t, overrunAsm, 76)
	if rep.Certified {
		t.Fatalf("litmus_overrun must NOT certify; unbounded=%+v violations=%+v", rep.Unbounded, rep.Violations)
	}
	if len(rep.Violations) == 0 {
		t.Fatalf("heavy line must be a VIOLATION (counted loop bounded), not unbounded: %+v", rep.Unbounded)
	}
	if !runtimeOverruns(t, overrunBin, 76) {
		t.Fatal("sanity: runtime should also catch the always-overrun line")
	}
}

// TestProveNonVacuous proves the prover isn't rubber-stamping: a tight budget
// must turn even smoke's small regions into violations, and a generous budget
// must re-certify the branch litmus (the budget is actually honored).
func TestProveNonVacuous(t *testing.T) {
	if tight := mustProve(t, smokeAsm, 6); tight.Certified {
		t.Fatalf("budget=6 must flag smoke's regions (maxWorst=%d) — vacuous otherwise", tight.MaxWorst)
	}
	if loose := mustProve(t, branchAsm, 200); !loose.Certified {
		t.Fatalf("budget=200 must certify the branch litmus (worst ~101); violations=%+v", loose.Violations)
	}
}

// TestObservedWithinProven is the observed-vs-proven dual: for the certified
// kernel, running the runtime guard with the budget set to the PROVEN bound
// must not fire — i.e. no observed WSYNC interval exceeds what the proof said is
// the worst case. The static upper bound holds dynamically.
func TestObservedWithinProven(t *testing.T) {
	rep := mustProve(t, smokeAsm, 76) // also (re)assembles smoke.bin
	if !rep.Certified || rep.MaxWorst <= 0 {
		t.Fatalf("need a certified smoke with a positive bound; got certified=%v maxWorst=%d", rep.Certified, rep.MaxWorst)
	}
	if runtimeOverruns(t, "../../roms/litmus/smoke.bin", 76) {
		t.Fatal("certified kernel overran at runtime under a 76cy budget — proof and observation disagree")
	}
}
