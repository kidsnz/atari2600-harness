package cyclebound

import (
	"os"
	"path/filepath"
	"strings"
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
	return mustLintDetail(t, asm).Warnings
}

func mustLintDetail(t *testing.T, asm string) LintResult {
	t.Helper()
	r, err := LintDetail(asm)
	if err != nil {
		t.Fatalf("LintDetail(%s): %v", asm, err)
	}
	return r
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
	n, declined, banked, instrs := 0, 0, 0, 0
	for _, e := range ents {
		if e.IsDir() || filepath.Ext(e.Name()) != ".asm" {
			continue
		}
		n++
		asm := filepath.Join(dir, e.Name())
		res := mustLintDetail(t, asm)
		// A refusal is not a timing claim. "not-analysed" says the ROM was never
		// decoded, which is the opposite of asserting something wrong about it —
		// counting it as a false positive would push the linter back towards
		// staying silent about programs it cannot read.
		var timing []LintWarning
		for _, w := range res.Warnings {
			if w.Rule == "not-analysed" {
				continue
			}
			timing = append(timing, w)
		}
		if len(timing) != 0 {
			t.Errorf("FALSE POSITIVE on known-good %s: %+v", e.Name(), timing)
		}
		if res.Declined != "" {
			declined++
			continue
		}
		// Claiming a ROM was analysed while decoding nothing is the exact shape of a
		// silent all-clear, so it is an error rather than a quiet zero.
		if res.Instructions == 0 {
			t.Errorf("%s: reported as analysed but decoded 0 instructions", e.Name())
		}
		instrs += res.Instructions
		if res.Banks > 1 {
			banked++
			for b, c := range res.PerBank {
				if c == 0 {
					t.Errorf("%s: bank %d decoded 0 instructions — that bank is unlinted", e.Name(), b)
				}
			}
		}
	}
	if n == 0 {
		t.Skip("no technique kernels found")
	}
	// Report the denominator. A clean sweep over a corpus that was silently mostly
	// skipped is how a check passes while proving nothing.
	t.Logf("linted %d technique kernels / %d instructions with zero timing false positives; "+
		"%d were bank-switched (read per bank), %d DECLINED", n-declined, instrs, banked, declined)
	if banked == 0 {
		t.Error("no ROM in the corpus was linted as more than one bank; banked_game.asm is an 8K F8 " +
			"cartridge, so the linter has either gone back to declining it outright or is reading it " +
			"as a flat image — a program that does not exist")
	}
}

// TestLintReadsBothBanks is the bank-coverage pair. Both fixtures are 8K F8
// cartridges built from banked_game.asm, and on the previous linter BOTH produced
// the same single "not-analysed" warning with zero instructions decoded — the trap
// in one and the correctness of the other were equally invisible.
//
// lint_bank_hazard plants an HMOVE hazard in BANK 1 (positive: must be found there).
// lint_bank_split puts a CORRECT motion's two halves in different banks — HMP0 staged
// in bank 0, HMOVE strobed in bank 1 (negative: R1 and R2 both ask "is it used
// ANYWHERE", so a per-bank survey would report two warnings and both would be false).
func TestLintReadsBothBanks(t *testing.T) {
	const (
		hazard = "../../roms/litmus/lint_bank_hazard.asm"
		split  = "../../roms/litmus/lint_bank_split.asm"
	)
	for _, asm := range []string{hazard, split} {
		if _, err := os.Stat(asm); err != nil {
			t.Skipf("fixture %s not present (%v)", asm, err)
		}
	}

	haz := mustLintDetail(t, hazard)
	if haz.Declined != "" {
		t.Fatalf("hazard fixture declined: %s", haz.Declined)
	}
	if haz.Banks != 2 || haz.PerBank[0] == 0 || haz.PerBank[1] == 0 {
		t.Fatalf("hazard fixture: expected 2 banks both decoded, got banks=%d per-bank=%v",
			haz.Banks, haz.PerBank)
	}
	if !hasRule(haz.Warnings, "hmove-hazard") {
		t.Errorf("planted bank-1 hazard not found: %+v", haz.Warnings)
	}
	for _, w := range haz.Warnings {
		if w.Rule != "hmove-hazard" {
			t.Errorf("unexpected extra rule %q on the hazard fixture (only hmove-hazard intended)", w.Rule)
		}
		// The location must name the bank it is in. Pointing at a bank-0 label would
		// send the author to a line that does not contain the fault.
		if w.Rule == "hmove-hazard" && !strings.Contains(w.Loc, "bank 1") {
			t.Errorf("hazard reported at %q — a bank-1 fault must say which bank", w.Loc)
		}
	}

	spl := mustLintDetail(t, split)
	if spl.Declined != "" {
		t.Fatalf("split fixture declined: %s", spl.Declined)
	}
	if spl.Banks != 2 || spl.PerBank[0] == 0 || spl.PerBank[1] == 0 {
		t.Fatalf("split fixture: expected 2 banks both decoded, got banks=%d per-bank=%v",
			spl.Banks, spl.PerBank)
	}
	if len(spl.Warnings) != 0 {
		t.Errorf("motion split across banks is correct and must be silent, got %+v", spl.Warnings)
	}
}
