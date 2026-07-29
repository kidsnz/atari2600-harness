package cyclebound

import (
	"os"
	"path/filepath"
	"testing"
)

// hasRule reports whether the lint output contains a warning with the given rule.
func hasRule(ws []LintWarning, rule string) bool {
	for _, w := range ws {
		if w.Rule == rule {
			return true
		}
	}
	return false
}

func mustLint(t *testing.T, asm string) []LintWarning {
	t.Helper()
	ws, err := Lint(asm)
	if err != nil {
		t.Fatalf("Lint(%s): %v", asm, err)
	}
	return ws
}

// TestLintTrapsFire locks the POSITIVE direction: each planted-trap fixture emits
// exactly its intended rule and nothing else (the rules don't bleed into one
// another).
func TestLintTrapsFire(t *testing.T) {
	cases := []struct {
		asm, rule string
	}{
		{"../../roms/litmus/lint_r1_hmove_nohmxx.asm", "hmove-without-hmxx"},
		{"../../roms/litmus/lint_r2_hmxx_nohmove.asm", "hmxx-without-hmove"},
		{"../../roms/litmus/lint_r3_hazard.asm", "hmove-hazard"},
	}
	for _, c := range cases {
		ws := mustLint(t, c.asm)
		if !hasRule(ws, c.rule) {
			t.Errorf("%s: expected rule %q, got %+v", filepath.Base(c.asm), c.rule, ws)
		}
		for _, w := range ws {
			if w.Rule != c.rule {
				t.Errorf("%s: unexpected extra rule %q (only %q intended)", filepath.Base(c.asm), w.Rule, c.rule)
			}
		}
	}
}

// TestLintCleanSilent locks the NEGATIVE direction: the canonical correct idiom
// (stage HMxx, WSYNC, HMOVE, 24cy gap, HMCLR) emits ZERO warnings.
func TestLintCleanSilent(t *testing.T) {
	if ws := mustLint(t, "../../roms/litmus/lint_clean.asm"); len(ws) != 0 {
		t.Fatalf("lint_clean must be silent, got %+v", ws)
	}
}

// TestLintNoFalsePositivesOnTechniques is the corpus guard: every known-good
// technique kernel (renders a stable 262, so any HMOVE use is correct) must lint
// clean. A single false positive on real kernels makes the linter untrustworthy,
// so this is the central quality bar. Skips if the techniques dir isn't present
// (keeps the unit test hermetic if the corpus ever relocates).
func TestLintNoFalsePositivesOnTechniques(t *testing.T) {
	dir := "../../roms/techniques"
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Skipf("techniques corpus not present (%v) — covered by the manual sweep", err)
	}
	n, skipped := 0, 0
	for _, e := range ents {
		if e.IsDir() || filepath.Ext(e.Name()) != ".asm" {
			continue
		}
		n++
		asm := filepath.Join(dir, e.Name())
		ws := mustLint(t, asm)
		// A refusal is not a timing claim. "not-analysed" says the ROM was never
		// decoded, which is the opposite of asserting something wrong about it —
		// counting it as a false positive would push the linter back towards
		// staying silent about programs it cannot read.
		var timing []LintWarning
		declined := false
		for _, w := range ws {
			if w.Rule == "not-analysed" {
				declined = true
				continue
			}
			timing = append(timing, w)
		}
		if len(timing) != 0 {
			t.Errorf("FALSE POSITIVE on known-good %s: %+v", e.Name(), timing)
		}
		if declined {
			skipped++
		}
	}
	if n == 0 {
		t.Skip("no technique kernels found")
	}
	// Report the denominator. A clean sweep over a corpus that was silently mostly
	// skipped is how a check passes while proving nothing.
	t.Logf("linted %d technique kernels with zero timing false positives; %d were DECLINED "+
		"(bank-switched) and are not covered by that claim", n-skipped, skipped)
	if skipped == 0 {
		t.Error("no ROM in the corpus was declined; banked_game.asm is an 8K F8 cartridge, so a " +
			"linter that analysed all 31 is reading a program that does not exist in at least one")
	}
}
