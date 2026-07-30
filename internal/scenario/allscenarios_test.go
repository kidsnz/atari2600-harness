package scenario

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestEveryScenarioRuns runs EVERY scenario in the repository, not just the ones a
// hand-written CI command happens to name.
//
// MEASURED 2026-07-30: there are 95 scenario files in three directories — 57 under
// roms/litmus, 31 under roms/techniques, 7 under roms/exerciser — and the CI mirror
// ran `cmd/scenario roms/litmus/scenarios/*.json` and nothing else. 38 of 95, 40% of
// the suite, were written and then never executed by any gate. All 38 pass today, so
// nothing was hiding behind them; the point is that nothing would have noticed if
// they stopped.
//
// Discovering the directories rather than listing them is deliberate: a fourth
// scenario directory added tomorrow is picked up here without anyone remembering to
// extend a command line, which is the failure this test exists to end.
//
// Runtime measured: litmus 3.3s, techniques + exerciser 15.5s.
func TestEveryScenarioRuns(t *testing.T) {
	// Scenarios name their ROM relative to the harness root, and Run assembles it
	// through that path, so the test has to stand where cmd/scenario stands.
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir("../.."); err != nil {
		t.Skipf("cannot reach the harness root: %v", err)
	}
	t.Cleanup(func() { os.Chdir(wd) })

	root := "roms"
	var files []string
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Base(filepath.Dir(path)) == "scenarios" && strings.HasSuffix(path, ".json") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Skipf("roms tree not present (%v)", err)
	}
	if len(files) == 0 {
		t.Skip("no scenarios found")
	}
	sort.Strings(files)

	byDir := map[string]int{}
	failed := 0
	for _, path := range files {
		byDir[filepath.Dir(path)]++
		s, err := Load(path)
		if err != nil {
			t.Errorf("%s: load: %v", path, err)
			failed++
			continue
		}
		// Scenarios name their ROM relative to the harness root; Load does not chdir.
		res, err := Run(s, false)
		if err != nil {
			t.Errorf("%s: run: %v", path, err)
			failed++
			continue
		}
		if !res.Pass {
			failed++
			for _, a := range res.Asserts {
				if !a.Pass {
					t.Errorf("%s: %s", path, a.Desc)
				}
			}
		}
	}

	// Report the denominator per directory. A sweep that silently covered one
	// directory is exactly the state this replaces.
	dirs := make([]string, 0, len(byDir))
	for d := range byDir {
		dirs = append(dirs, d)
	}
	sort.Strings(dirs)
	var parts []string
	for _, d := range dirs {
		parts = append(parts, filepath.Base(filepath.Dir(d))+":"+itoa(byDir[d]))
	}
	if len(byDir) < 2 {
		t.Errorf("scenarios found in only %d directory (%v) — the walk is not reaching the whole tree",
			len(byDir), parts)
	}
	t.Logf("ran %d scenarios across %d directories (%s); %d failed",
		len(files), len(byDir), strings.Join(parts, " "), failed)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
