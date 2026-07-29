package behavmatch

import (
	"os"
	"path/filepath"
	"testing"
)

// A scenario is an input script and a list of objects to watch — data, not code —
// so it has to be loadable from a file. The check that matters is not that the
// JSON parses but that a scenario driven from disk moves the machine ITSELF
// identically: same inputs, same trace, byte for byte across all 128 RAM
// addresses. Anything less and a game described in a file would be measured
// against subtly different conditions from one described in Go.
func TestLoadedScenarioDrivesTheMachineIdentically(t *testing.T) {
	b, err := ExportBuiltins()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "builtins.json")
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
	lib, order, err := LoadScenarios(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(order) != len(ScenarioNames()) {
		t.Fatalf("round-trip lost scenarios: %d in, %d out", len(ScenarioNames()), len(order))
	}

	// Driving is expensive, so compare a representative few in full rather than
	// all 24 shallowly — a partial trace comparison would not catch a dropped
	// input edge, which is the failure this is for.
	for _, name := range []string{"p0-right", "p1-fire-tap", "both-fire", "select-then-reset"} {
		built, ok := Library[name]
		if !ok {
			t.Fatalf("built-in %q vanished; the test's premise is stale", name)
		}
		loaded, ok := lib[name]
		if !ok {
			t.Fatalf("%q did not survive the round-trip", name)
		}
		a, err := Record(romAnim, "NTSC", built, 0)
		if err != nil {
			t.Skipf("ROM unavailable: %v", err)
		}
		c, err := Record(romAnim, "NTSC", loaded, 0)
		if err != nil {
			t.Fatal(err)
		}
		g := GateRAM(a, c, FullMask())
		if !g.Pass() {
			t.Errorf("%s: the loaded scenario drove the machine to a different state: %v", name, g.First)
		}
		if len(a.Samples) != len(c.Samples) {
			t.Errorf("%s: %d frames from the built-in, %d from the file", name, len(a.Samples), len(c.Samples))
		}
		for i := range a.Samples {
			if a.Samples[i].Inputs.Key() != c.Samples[i].Inputs.Key() {
				t.Errorf("%s frame %d: inputs %q vs %q", name, i,
					a.Samples[i].Inputs.Key(), c.Samples[i].Inputs.Key())
				break
			}
		}
	}
}

// A malformed scenario has to be an error, not a skip. One quietly missing from
// a suite is a hole in coverage that nothing else would report — the same reason
// the coverage denominator and the RAM-gate mask print what they left out.
func TestLoadScenariosRejectsRatherThanSkips(t *testing.T) {
	cases := []struct {
		name, body string
	}{
		{"no name", `{"scenarios":[{"frames":10}]}`},
		{"no frames", `{"scenarios":[{"name":"x"}]}`},
		{"frame past the end", `{"scenarios":[{"name":"x","frames":5,"at":{"9":[{"action":"fire","press":true}]}}]}`},
		{"neither panel nor action", `{"scenarios":[{"name":"x","frames":5,"at":{"1":[{"press":true}]}}]}`},
		{"both panel and action", `{"scenarios":[{"name":"x","frames":5,"at":{"1":[{"panel":"reset","action":"fire","press":true}]}}]}`},
		{"duplicate name", `{"scenarios":[{"name":"x","frames":5},{"name":"x","frames":5}]}`},
		{"empty", `{"scenarios":[]}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			p := filepath.Join(dir, "s.json")
			if err := os.WriteFile(p, []byte(tc.body), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, _, err := LoadScenarios(p); err == nil {
				t.Errorf("accepted a scenario file that should have been rejected: %s", tc.body)
			}
		})
	}
}

// A directory of files is how a game carries its scenarios next to its own
// source, so that path has to work and has to keep a stable order.
func TestLoadScenariosFromDirectory(t *testing.T) {
	dir := t.TempDir()
	one := `{"scenarios":[{"name":"a","frames":4,"objects":[0]}]}`
	two := `{"scenarios":[{"name":"b","frames":4,"at":{"1":[{"action":"fire","player":1,"press":true}]}}]}`
	if err := os.WriteFile(filepath.Join(dir, "1.json"), []byte(one), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "2.json"), []byte(two), 0o644); err != nil {
		t.Fatal(err)
	}
	lib, order, err := LoadScenarios(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(lib) != 2 || len(order) != 2 || order[0] != "a" || order[1] != "b" {
		t.Fatalf("got %v (%d scenarios), want [a b]", order, len(lib))
	}
	if got := lib["b"].At[1]; len(got) != 1 || got[0].Player != 1 || got[0].Action != "fire" || !got[0].Press {
		t.Errorf("input edge did not survive: %+v", got)
	}
}
