package scenario

import (
	"strings"
	"testing"
)

// TestRAMBudgetGradesTheROMAndNotTheClaim covers all four paths of `ram_budget`.
//
// ★The check exists because `design.ScrollBackgroundFitsRAM` had **no non-test caller anywhere**
// — the second function found in that state in one day, after `ScrollScanlinesConstant`. A rule
// with an implementation and a unit test that no pipeline reaches is a rule the corpus cannot
// break. Found by the mailing-list distillation (helper-3).
//
// ★★But wiring it up alone would have been worth little. The four numbers are things an AUTHOR
// writes into a scenario, so arithmetic on them grades the claim, not the ROM. The second half of
// the check is the static write set from `cyclebound.DefUse` — **every address any reachable
// instruction might write, over all paths** — which is the set a real execution's writes must be
// contained in. If the program touches more RAM than the layout declares, the layout is fiction.
//
// ★★★Four paths, four witnesses here:
//
//	honest declaration, program fits inside it .................. passes
//	declaration SMALLER than the program's write set ............ fails on the second half
//	declaration over 128 bytes ................................. fails on the arithmetic
//	rom is a .bin, so no source to analyse ..................... SKIPS, and says so
//
// The third and fourth are the ones that would otherwise rot quietly: an arithmetic-only failure
// looks the same as a real one, and a skip prints the same OK as a pass.
func TestRAMBudgetGradesTheROMAndNotTheClaim(t *testing.T) {
	t.Chdir("../..")

	run := func(rom string, b RAMBudgetCheck) (*Result, string) {
		t.Helper()
		res, err := Run(&Scenario{Rom: rom, Checks: &Checks{RAMBudget: &b}}, false)
		if err != nil {
			t.Fatal(err)
		}
		var descs []string
		for _, a := range res.Asserts {
			descs = append(descs, a.Desc)
		}
		return res, strings.Join(descs, " | ")
	}

	// `framelines_clean.asm` writes a single RAM byte, which makes it the right fixture for a
	// declaration that is honest at a small number — and for one that is smaller still.
	const small = "roms/litmus/framelines_clean.asm"

	if res, d := run(small, RAMBudgetCheck{Board: 8, Stack: 4}); !res.Pass {
		t.Errorf("an honest declaration failed: %s", d)
	}

	res, d := run(small, RAMBudgetCheck{Board: 0})
	if res.Pass {
		t.Errorf("a declaration of zero bytes passed for a ROM that writes RAM — the write-set "+
			"half of the check is not running: %s", d)
	}
	if !strings.Contains(d, "grading a claim rather than the ROM") {
		t.Errorf("the under-declaration failed, but not on the write-set comparison: %s", d)
	}

	res, d = run(small, RAMBudgetCheck{Board: 130})
	if res.Pass {
		t.Errorf("130 bytes of declared RAM passed on a 128-byte machine: %s", d)
	}
	if !strings.Contains(d, "130 of 128 bytes") {
		t.Errorf("the arithmetic half did not report the overflow: %s", d)
	}

	// ★A .bin has no source, so the write set cannot be computed. The check must say it skipped
	// rather than print the same OK as a pass — the shape this repository has been bitten by.
	res, d = run("roms/litmus/framelines_clean.bin", RAMBudgetCheck{Board: 8})
	if !res.Pass {
		t.Errorf("a .bin with a fitting declaration should still pass the arithmetic: %s", d)
	}
	if !strings.Contains(d, "SKIPPED") {
		t.Errorf("the write-set half was skipped for a .bin without saying so: %s", d)
	}
}
