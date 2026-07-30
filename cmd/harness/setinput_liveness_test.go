package main

import (
	"context"
	"os"
	"testing"

	"github.com/kidsnz/atari2600-harness/internal/emu"
)

// TestSetInputReportsLiveness pins that a HELD direction comes back with a liveness
// verdict, and that a release or a panel switch does not (the question is meaningless
// there). cmd/harness had no test of its own before this one.
//
// The behaviour matters because of how the failure looks. A cartridge in attract mode
// does not error: it returns a stable, plausible constant for as long as you read it.
// Three measurements of Outlaw's gunman were taken in that state in one day. The
// verdict is attached to the call that opens the trap rather than offered as an option
// nobody remembers to use.
func TestSetInputReportsLiveness(t *testing.T) {
	const rom = "../../../sandbox/studies/outlaw/Outlaw.bin"
	if _, err := os.Stat(rom); err != nil {
		t.Skipf("Outlaw.bin not present (%v) — it lives outside this repo", err)
	}
	mu.Lock()
	e, err := emu.New("NTSC")
	if err != nil {
		mu.Unlock()
		t.Fatal(err)
	}
	if err := e.LoadROM(rom); err != nil {
		mu.Unlock()
		t.Fatal(err)
	}
	for i := 0; i < 30; i++ {
		e.StepFrame()
	}
	current = e // the package-level machine `get()` hands out
	mu.Unlock()

	ctx := context.Background()

	// Held direction in attract mode: must report NOT responding.
	_, out, err := handleSetInput(ctx, nil, SetInputIn{Action: "up", Pressed: true})
	if err != nil {
		t.Fatal(err)
	}
	if out.Responds == nil {
		t.Fatal("a held direction came back with no liveness verdict — the trap is open again")
	}
	if *out.Responds {
		t.Errorf("attract mode reported as responding: %s", out.Liveness)
	}

	// Release: no verdict, because there is nothing to probe.
	_, out, err = handleSetInput(ctx, nil, SetInputIn{Action: "up", Pressed: false})
	if err != nil {
		t.Fatal(err)
	}
	if out.Responds != nil {
		t.Errorf("a RELEASE was given a liveness verdict (%v) — the probe should only run for a held "+
			"direction", *out.Responds)
	}

	// Panel switch: no verdict either.
	_, out, err = handleSetInput(ctx, nil, SetInputIn{Action: "reset", Pressed: true})
	if err != nil {
		t.Fatal(err)
	}
	if out.Responds != nil {
		t.Errorf("a panel switch was given a liveness verdict — the question is meaningless there")
	}
	// RESET has to be HELD across frames to register — pressing and releasing with no
	// frames between does nothing, which is how the first version of this test
	// "proved" the game was still dead after reset.
	mu.Lock()
	for i := 0; i < 4; i++ {
		e.StepFrame()
	}
	mu.Unlock()
	handleSetInput(ctx, nil, SetInputIn{Action: "reset", Pressed: false})
	mu.Lock()
	for i := 0; i < 30; i++ {
		e.StepFrame()
	}
	mu.Unlock()

	// After RESET the game runs: the same held direction must now report responding.
	_, out, err = handleSetInput(ctx, nil, SetInputIn{Action: "up", Pressed: true})
	if err != nil {
		t.Fatal(err)
	}
	if out.Responds == nil || !*out.Responds {
		t.Errorf("after RESET the gunman moves, but liveness said no: %v", out.Liveness)
	}
	t.Logf("attract -> responds=false; after RESET -> %s", out.Liveness)
}
