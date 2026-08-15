package scenario_test

// The authored ROMs are not covered by anything, and that is how one of them broke
// without being noticed.
//
// `sandbox/` holds the work this harness exists to serve — 92 authored `.asm` files,
// eleven scenario files, the eight-step PONG, the Pizza Boy reproduction. It lives in
// the UMBRELLA directory above `harness/`, is a separate git repository, and **has no
// remote**, so GitHub Actions cannot see it and no CI anywhere runs those scenarios.
//
// On 2026-08-04 `pizza-boy-tokyo/build/scenarios/phase0.json` was found to be RED, and
// nothing had reported it. It was found by hand, while looking for something else.
// Eleven scenarios with no runner is eleven chances to discover a regression by
// accident.
//
// This test closes that on the machine where the work happens. It cannot close it in
// CI — the tree is not there — so it SKIPS with the reason printed rather than passing
// silently, which is the same rule `check_provenance.py` and the coverage test follow
// after all three turned real CI red today.

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// knownFailing lists umbrella scenarios that are red RIGHT NOW, each with the reason
// and what would fix it. It is not an exemption list: every entry is printed on every
// run, an entry that starts passing FAILS the test so it cannot rot into permission,
// and anything red that is NOT listed fails immediately.
// EMPTY, and it got there by the entries being fixed rather than deleted.
// pizza-boy's phase0.json was the last one. Its `prove_line_budget` half survived two
// wrong diagnoses — first "the region continues in the caller and no WSYNC is found
// ahead of it", then "the blank classification does not cross a JSR" — before the
// measurement said what it actually was: A's range at a shared routine's divide loop is
// the join over every caller, so the callers passing a constant inherited the ones
// passing a RAM byte. Closed 2026-08-05 by K6 (docs/capability-gap-audit.md), witnessed
// by roms/litmus/litmus_divctx.asm.
var knownFailing = map[string]string{}

func umbrellaScenarios(t *testing.T) (root string, found []string) {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	// internal/scenario -> harness -> umbrella
	root = filepath.Dir(filepath.Dir(filepath.Dir(wd)))
	sb := filepath.Join(root, "sandbox")
	if st, err := os.Stat(sb); err != nil || !st.IsDir() {
		return root, nil
	}
	filepath.Walk(sb, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if strings.HasSuffix(p, ".json") && strings.Contains(p, string(os.PathSeparator)+"scenarios"+string(os.PathSeparator)) {
			found = append(found, p)
		}
		return nil
	})
	sort.Strings(found)
	return root, found
}

func TestTheAuthoredROMsStillPass(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the whole authored corpus; skipped under -short")
	}
	root, scen := umbrellaScenarios(t)
	if len(scen) == 0 {
		t.Skip("no umbrella sandbox/ tree — the authored ROMs live in a separate repository " +
			"one level above this one, which a CI checkout does not have. This check runs on " +
			"the machine where the ROMs are written, which is the machine that breaks them.")
	}
	// A tree that shrank is a different failure from a tree that is absent.
	// Floor re-measured 2026-08-15: the tree holds 17, and this said 12 — a floor five below
	// reality cannot notice five scenarios disappearing, which is the one thing it is for.
	if len(scen) < 17 {
		t.Fatalf("only %d umbrella scenarios found; there were 17 on 2026-08-15 (13 on 2026-08-04, "+
			"grown since). A partial "+
			"corpus is worse than none — the missing one is where the next regression hides",
			len(scen))
	}

	bin := filepath.Join(t.TempDir(), "scenario")
	build := exec.Command("go", "build", "-o", bin, "./cmd/scenario")
	build.Dir = filepath.Dir(filepath.Dir(mustWD(t))) // harness root
	build.Env = append(os.Environ(), "GOWORK=off", "CGO_ENABLED=0")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building cmd/scenario: %v\n%s", err, out)
	}

	var unexpected, fixed []string
	pass := 0
	for _, s := range scen {
		rel, _ := filepath.Rel(filepath.Join(root, "sandbox"), s)
		cmd := exec.Command(bin, s)
		// THE SCENARIOS ARE WRITTEN RELATIVE TO `sandbox/`, not to the umbrella root:
		// `"rom": "practice/starshot/starshot.asm"`. Running them from one directory up
		// makes every one of them fail to find its ROM, which reads exactly like eight
		// regressions. (Pizza Boy's two use absolute paths and passed either way, which
		// is what made the mistake look like a real finding.)
		cmd.Dir = filepath.Join(root, "sandbox")
		out, err := cmd.CombinedOutput()
		_, known := knownFailing[rel]
		switch {
		case err == nil && known:
			fixed = append(fixed, rel)
		case err == nil:
			pass++
		case known:
			// still red, as recorded
		default:
			line := strings.TrimSpace(string(out))
			if i := strings.Index(line, "FAIL"); i >= 0 {
				line = line[i:]
			}
			if len(line) > 220 {
				line = line[:220]
			}
			unexpected = append(unexpected, rel+"\n        "+line)
		}
	}

	t.Logf("umbrella scenarios: %d total, %d pass, %d known-red", len(scen), pass, len(knownFailing))
	for k, why := range knownFailing {
		t.Logf("  known-red  %s\n             %s", k, why)
	}

	for _, u := range unexpected {
		t.Errorf("an authored ROM regressed and nothing else would have told you:\n      %s", u)
	}
	// An entry that starts passing must be deleted, or the list stops meaning anything.
	for _, f := range fixed {
		t.Errorf("%s now PASSES but is still listed in knownFailing — delete the entry so the "+
			"list keeps its meaning", f)
	}
}

func mustWD(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return wd
}
