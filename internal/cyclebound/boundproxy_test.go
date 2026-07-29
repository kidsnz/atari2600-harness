package cyclebound

import (
	"testing"

	"github.com/kidsnz/atari2600-harness/internal/build"
	"github.com/kidsnz/atari2600-harness/internal/emu"
)

// A counted loop's trip count used to come from "the immediate LDX/LDY at the
// greatest address below the loop header" — a proxy for "the initialiser that ran
// most recently before it", valid only while address order matches execution
// order. A backward jump breaks that, and needs no bank switching to do so.
//
// litmus_bound_proxy plants exactly that: the `ldx #200` the loop really runs with
// sits ABOVE the header, reached by a forward jump, while a `ldx #2` that executes
// and is then discarded sits below it. Measured before the fix, the prover
// answered certified:true, roll_free:true, max_worst 25 — while the machine ran
// that interval at 1015 cycles over 14 scanlines in a 273-line frame. A fortyfold
// under-approximation carried on the roll_free verdict.
//
// The gate is the machine, not the prover's own arithmetic: the proven worst case
// must be at least what the emulator measures, and the emulator is asked here.
func TestLoopBoundIsNotAnAddressProxy(t *testing.T) {
	const asm = "../../roms/litmus/litmus_bound_proxy.asm"
	rep := mustProve(t, asm, 76)

	// Find the region the planted loop lives in: the one whose worst case is far
	// beyond a single scanline.
	worst := 0
	bounded := false
	for _, r := range append(append([]Region{}, rep.Lines...), rep.BlankLines...) {
		if r.Worst > worst {
			worst, bounded = r.Worst, r.Bounded
		}
	}

	bin := build.BinPathFor(asm)
	if out, err := build.Assemble(asm, bin); err != nil {
		t.Fatalf("assemble: %s", out)
	}
	e, err := emu.New("NTSC")
	if err != nil {
		t.Skip("emulator unavailable")
	}
	if err := e.LoadROM(bin); err != nil {
		t.Fatal(err)
	}
	rows, _, err := e.ProfileLineWorst(6, nil)
	if err != nil {
		t.Fatal(err)
	}
	measured := 0
	for _, r := range rows {
		if r.WorstCycles > measured {
			measured = r.WorstCycles
		}
	}
	if measured < 500 {
		t.Fatalf("premise broken: the planted loop should run for hundreds of cycles, the machine "+
			"measured %d", measured)
	}

	if !bounded {
		t.Logf("the region is reported unbounded; that is sound (a refusal, not a wrong number) "+
			"but the machine can bound it at %d", measured)
		return
	}
	if worst < measured {
		t.Errorf("PROVEN worst case %d is BELOW the measured %d — an under-approximation, which is the "+
			"one direction this package forbids. The address proxy for the loop's initialiser is back.",
			worst, measured)
	}
	// The roll-free verdict must not survive a region that spans 14 scanlines.
	if rep.RollFree {
		t.Errorf("roll_free is true on a ROM the machine runs at %d cycles in one interval and whose "+
			"frame is 273 scanlines", measured)
	}
	t.Logf("proven %d cycles, machine measured %d (roll_free=%v)", worst, measured, rep.RollFree)
}
