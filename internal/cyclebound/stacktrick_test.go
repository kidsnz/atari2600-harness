package cyclebound

import (
	"testing"

	"github.com/kidsnz/atari2600-harness/internal/beamtrace"
	"github.com/kidsnz/atari2600-harness/internal/emu"
)

const stackTrickASM = "../../roms/litmus/litmus_stack_trick.asm"
const stackTrickBIN = "../../roms/litmus/litmus_stack_trick.bin"

// A push is a store. On the 2600 page 1 mirrors the addresses the console
// decodes, so aiming SP at a TIA register and pushing writes that register —
// which is how Combat enables its missile in fewer cycles than a `sta` costs.
//
// The instruction is a single implied-addressing byte with no operand, so
// anything reading the opcode alone concludes it touches no memory. Before SP
// was tracked, neither the static analysis nor beamtrace could see this write,
// while the screen showed the background turning green.
//
// The two halves must agree, and they must agree with the picture.
func TestStackTrickWriteIsSeenStaticallyAndDynamically(t *testing.T) {
	const colubk = 0x09

	// Static: the push resolves to COLUBK, at one exact beam position.
	b, err := BeamIntervals(stackTrickASM)
	if err != nil {
		t.Skipf("BeamIntervals: %v", err)
	}
	var staticPC uint16
	staticClock := -1000
	for _, reg := range b.Regions {
		if !reg.Bounded {
			continue
		}
		for _, w := range reg.Writes {
			if w.Reg != "COLUBK" {
				continue
			}
			staticPC, staticClock = parseHexAddr(w.PC), w.MinClock
			if !w.Exact {
				t.Errorf("the push has no branch before it, so its position should be exact, "+
					"got clk %d..%d", w.MinClock, w.MaxClock)
			}
		}
	}
	if staticClock == -1000 {
		t.Fatal("static analysis found no COLUBK write; the PHA is invisible to it")
	}

	// Dynamic: the machine writes COLUBK with the green, at the same clock.
	e, err := emu.New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM(stackTrickBIN); err != nil {
		t.Skipf("ROM unavailable: %v", err)
	}
	if err := e.RunFrames(3); err != nil {
		t.Fatal(err)
	}
	ws, err := beamtrace.Trace(e, 1)
	if err != nil {
		t.Fatal(err)
	}
	seen := 0
	for _, w := range ws {
		if w.Reg != colubk {
			continue
		}
		seen++
		if w.PC != staticPC {
			t.Errorf("COLUBK written by PC $%04X, static analysis attributed it to $%04X", w.PC, staticPC)
		}
		if w.Clock != staticClock {
			t.Errorf("COLUBK write observed at clk %d, proven at clk %d", w.Clock, staticClock)
		}
		if !w.HasValue || w.Value != 0xC4 {
			t.Errorf("COLUBK written with $%02X (has=%v), the ROM pushes $C4", w.Value, w.HasValue)
		}
	}
	if seen != 1 {
		t.Errorf("observed %d COLUBK writes, want exactly 1 — the ROM pushes once per frame", seen)
	}
}

// And the picture is the final arbiter: whatever the tools say, the background
// really is green. A ROM whose control twin stays dark makes that comparison
// mean something.
func TestStackTrickChangesThePicture(t *testing.T) {
	bg := func(rom string) string {
		e, err := emu.New("NTSC")
		if err != nil {
			t.Fatal(err)
		}
		if err := e.LoadROM(rom); err != nil {
			t.Skipf("ROM unavailable (%s): %v", rom, err)
		}
		if err := e.RunFrames(4); err != nil {
			t.Fatal(err)
		}
		runs, _, err := e.ReadRow(100)
		if err != nil || len(runs) == 0 {
			t.Fatalf("ReadRow(%s): %v", rom, err)
		}
		return runs[0].Hex
	}
	trick := bg(stackTrickBIN)
	control := bg("../../roms/litmus/motion_glide.bin")
	if trick == control {
		t.Errorf("the stack-trick ROM's background (%s) matches the control ROM's (%s) — "+
			"the push did not reach COLUBK, so this litmus tests nothing", trick, control)
	}
}

// SP has to be tracked through the whole init sequence for any of this to work:
// the ROM sets SP to $FF, clears RAM, then aims SP at COLUBK. If the analysis
// lost SP anywhere along the way the push would resolve to nothing.
func TestSPSurvivesInitSequence(t *testing.T) {
	r, err := DefUse(stackTrickASM, DefaultBudget)
	if err != nil {
		t.Skipf("DefUse: %v", err)
	}
	found := false
	for _, acc := range r.Writes {
		if acc.Exact && len(acc.Addrs) == 1 && acc.Addrs[0] == 0x0109 {
			found = true
		}
	}
	if !found {
		t.Error("no write resolved to $0109 (page-1 mirror of COLUBK); SP was lost before the push")
	}
}
