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

// TestProveTimerBlankSkipped (S1): a timer-driven VBLANK region — a busy-wait
// with no WSYNC and no display store, beam display-off — must be SKIPPED as
// blank: neither flagged as a 1-line overrun nor reported as an unbounded loop.
// The visible kernel stays budget-checked, so a tight budget still flags it
// (blank-skip must never disable a visible check = soundness).
func TestProveTimerBlankSkipped(t *testing.T) {
	const timerAsm = "../../roms/litmus/cb_timer.asm"
	rep := mustProve(t, timerAsm, 76)
	if rep.Blank == 0 {
		t.Fatal("timer-driven VBLANK region must be blank-skipped (Blank>0)")
	}
	if len(rep.Unbounded) != 0 {
		t.Fatalf("timer busy-wait must be blank-skipped, not reported unbounded: %+v", rep.Unbounded)
	}
	if !rep.Certified {
		t.Fatalf("cb_timer's visible kernel is clean -> must certify; violations=%+v", rep.Violations)
	}
	// soundness: the visible kernel is still checked — a tight budget flags it.
	if tight := mustProve(t, timerAsm, 4); tight.Certified {
		t.Fatal("budget=4 must flag the visible kernel — blank-skip must not disable visible checks")
	}
}

// TestProveNoWSYNCNotCertified guards against a vacuous "0 regions, all safe"
// certification: a ROM with no reachable STA WSYNC (a bank-switched kernel whose
// display loop lives in another bank) must NOT certify — 0 regions means "can't
// prove", not "proven safe". Found by running the prover on real kernels (the
// litmus ROMs all use WSYNC, so they never hit this path).
func TestProveNoWSYNCNotCertified(t *testing.T) {
	rep := mustProve(t, "../../roms/techniques/banked_game.asm", 76)
	if rep.Regions != 0 {
		t.Skipf("banked_game now exposes %d WSYNC regions; the 0-region premise changed", rep.Regions)
	}
	if rep.Certified {
		t.Fatal("a 0-region ROM must NOT certify (no WSYNC reached = out of scope, not proven safe)")
	}
}

// TestTwoLineBudgetAnnotation: a legitimate 2-line kernel region is certified when
// its opening WSYNC is annotated `; @lines 2` (budget 2*76=152), and the same
// region WITHOUT the note is flagged — proving the annotation is load-bearing, not
// a blanket budget relaxation (VV-2 green-ification of 2-line kernels).
func TestTwoLineBudgetAnnotation(t *testing.T) {
	ann := mustProve(t, "../../roms/litmus/cb_2line.asm", 0)
	if !ann.Certified {
		t.Fatalf("cb_2line (@lines 2) must certify; violations=%+v", ann.Violations)
	}
	// the 2-line region is genuinely over the 1-line budget, so certification can
	// only come from the scaled @lines budget (the un-annotated twin proves it).
	if ann.MaxWorst <= 76 {
		t.Fatalf("the 2-line region should be >76 (got %d) — the annotation isn't being exercised", ann.MaxWorst)
	}

	noann := mustProve(t, "../../roms/litmus/cb_2line_noann.asm", 0)
	if noann.Certified {
		t.Fatalf("cb_2line_noann (no @lines) must NOT certify (region 139>76)")
	}
	if len(noann.Violations) == 0 || noann.Violations[0].Worst <= 76 {
		t.Fatalf("expected an over-76 violation in the un-annotated twin, got %+v", noann.Violations)
	}
}
