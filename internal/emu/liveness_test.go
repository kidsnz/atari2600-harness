package emu

import (
	"os"
	"strings"
	"testing"
)

// TestRespondsToInputSeparatesAttractFromPlay is the witness for the liveness check,
// and it uses the exact ROM that fooled three separate measurements today.
//
// Outlaw sits in attract mode after power-on. Held input changes nothing, yet every
// position accessor keeps returning a stable, plausible number: y_top 101, x 7, for
// as many frames as you care to run. Nothing errors. That is the shape of the
// failure — a confident constant — and it is why the check compares BEHAVIOUR under
// two inputs rather than looking for a magic byte.
//
// Skips if the ROM is absent; it lives outside this repo.
func TestRespondsToInputSeparatesAttractFromPlay(t *testing.T) {
	const rom = "../../../sandbox/studies/outlaw/Outlaw.bin"
	if _, err := os.Stat(rom); err != nil {
		t.Skipf("Outlaw.bin not present (%v) — it lives outside this repo", err)
	}
	e, err := New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM(rom); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 30; i++ {
		if _, err := e.StepFrame(); err != nil {
			t.Fatal(err)
		}
	}

	// BEFORE RESET: attract mode. The check must say no.
	live, why, err := e.RespondsToInput(0, "up", 30)
	if err != nil {
		t.Fatal(err)
	}
	if live {
		t.Errorf("reported responsive in attract mode: %s", why)
	}
	if !strings.Contains(why, "not reacting to input") {
		t.Errorf("the refusal does not say what is wrong: %s", why)
	}

	// And prove the trap is real rather than asserted: the position accessor is
	// perfectly happy to hand back a constant in this state.
	e.SetInput(0, "up", true)
	first, _, _, _ := e.ObjectYExtent(0)
	for i := 0; i < 30; i++ {
		e.StepFrame()
	}
	last, _, _, _ := e.ObjectYExtent(0)
	e.SetInput(0, "up", false)
	if first != last {
		t.Errorf("the gunman moved in attract mode (%d -> %d) — this ROM no longer demonstrates the "+
			"trap and the comment on RespondsToInput needs re-measuring", first, last)
	}

	// AFTER RESET: the game runs. The check must say yes.
	if err := e.SetPanel("reset", true); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4; i++ {
		e.StepFrame()
	}
	e.SetPanel("reset", false)
	for i := 0; i < 30; i++ {
		e.StepFrame()
	}
	live, why, err = e.RespondsToInput(0, "up", 30)
	if err != nil {
		t.Fatal(err)
	}
	if !live {
		t.Errorf("reported unresponsive after RESET, when the gunman does move: %s", why)
	}
	t.Logf("attract -> refused; after RESET -> %s", why)
}

// TestRespondsToInputRestoresTheMachine: the check runs the emulator twice, so it
// must leave the caller exactly where it found them. A measurement helper that
// advances the machine is a trap of its own.
func TestRespondsToInputRestoresTheMachine(t *testing.T) {
	e, err := New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM("../../roms/litmus/smoke.bin"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		e.StepFrame()
	}
	beforeRAM, _ := e.CurrentRAM()
	beforeFrame := e.Coords().Frame

	if _, _, err := e.RespondsToInput(0, "fire", 10); err != nil {
		t.Fatal(err)
	}

	afterRAM, _ := e.CurrentRAM()
	if afterRAM != beforeRAM {
		t.Error("RAM differs after the liveness check — it did not restore the machine")
	}
	if got := e.Coords().Frame; got != beforeFrame {
		t.Errorf("frame counter moved %d -> %d; the check advanced the emulator it was asked about",
			beforeFrame, got)
	}
}
