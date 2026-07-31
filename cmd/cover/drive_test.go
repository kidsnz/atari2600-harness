package main

import (
	"os"
	"testing"

	"github.com/kidsnz/atari2600-harness/internal/emu"
)

func covered(t *testing.T, rom, mode string, frames int) int {
	t.Helper()
	e, err := emu.New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM(rom); err != nil {
		t.Skipf("%s: %v", rom, err)
	}
	if err := e.RunFrames(2); err != nil {
		t.Fatal(err)
	}
	e.EnableCoverage()
	if err := driveROM(e, mode, frames, nil); err != nil {
		t.Fatalf("drive %s: %v", mode, err)
	}
	return e.Coverage().PCCount()
}

// TestExploreReachesCodeHoldingInputsDoesNot pins that the two drivings are not the same
// thing, on a real cartridge.
//
// A 2600 attract mode runs the GAME LOOP with synthetic input, so a cartridge left alone
// already covers most of what playing it covers — measured on Chopper Command, RESET plus
// a dozen rounds of stick moved the count by 4 instructions out of 2358. What sitting
// there does not cover are the other game VARIATIONS, and those are behind SELECT:
// Seaquest goes 51% of its decoded instructions to 60%.
//
// If this ever stops holding, the interesting possibility is not that the test is stale
// but that SetPanel has stopped pressing anything — which has happened once already, when
// SetInput was used for a console switch and its error ignored.
func TestExploreReachesCodeHoldingInputsDoesNot(t *testing.T) {
	const rom = "/Users/shinji/Documents/2D/260609_atari2600-dev/reference/pizza-boy/Samples for Pizza Boy/Seaquest.bin"
	if _, err := os.Stat(rom); err != nil {
		t.Skip("commercial cartridge not present — it lives outside this repo")
	}
	hold := covered(t, rom, "hold", 600)
	explore := covered(t, rom, "explore", 600)
	if hold == 0 || explore == 0 {
		t.Fatalf("no coverage recorded (hold=%d explore=%d)", hold, explore)
	}
	if explore <= hold {
		t.Errorf("explore covered %d instructions, holding covered %d — cycling SELECT is supposed to "+
			"reach the game variations that sitting on the attract loop never enters; equal numbers "+
			"more likely mean the panel switch stopped being pressed than that the game changed",
			explore, hold)
	}
	t.Logf("Seaquest: hold=%d explore=%d (+%d instructions)", hold, explore, explore-hold)
}
