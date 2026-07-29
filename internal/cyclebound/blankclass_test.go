package cyclebound

import (
	"path/filepath"
	"testing"

	"github.com/kidsnz/atari2600-harness/internal/build"
	"github.com/kidsnz/atari2600-harness/internal/emu"
)

// Classifying a region as "blank" excuses it from the visible-line budget proof,
// so it is the one verdict in this package that can hide a real scanline tear.
// Checking it against the analysis that produced it would agree with itself, so
// the oracle is the machine: run the ROM and, every time execution reaches a
// region's opening WSYNC, ask the television whether the beam is actually
// blanked. A region the prover called blank must never be reached with the
// display on.
//
// The direction matters. A region the prover calls VISIBLE while the display
// happens to be off is only imprecision — it gets checked against the budget for
// nothing. A region called BLANK while the display is on is the prover skipping
// code that draws, which is the failure this exists to prevent.
func TestBlankClassificationAgreesWithTheMachine(t *testing.T) {
	files, err := filepath.Glob("../../roms/techniques/*.asm")
	if err != nil || len(files) == 0 {
		t.Skip("technique corpus unavailable")
	}
	files = append(files, "../../roms/litmus/litmus_jsr_display.asm")

	romsChecked, regionsChecked, hits, skippedNeverReached := 0, 0, 0, 0
	for _, asm := range files {
		rep, err := Prove(asm, DefaultBudget)
		if err != nil || len(rep.BlankLines) == 0 {
			continue
		}
		blank := map[uint16]bool{}
		for _, r := range rep.BlankLines {
			blank[r.Start] = true
		}

		bin := build.BinPathFor(asm)
		if out, err := build.Assemble(asm, bin); err != nil {
			t.Logf("assemble %s: %s", asm, out)
			continue
		}
		e, err := emu.New("NTSC")
		if err != nil {
			continue
		}
		if err := e.LoadROM(bin); err != nil {
			continue
		}
		if err := e.RunFrames(6); err != nil { // settle past power-on
			continue
		}

		reached := map[uint16]bool{}
		bad := map[uint16]int{}
		// Long enough for a state machine to move through more than its idle state;
		// game_states swaps at frame 60, so a short run would prove less than it
		// looks like it does.
		for step := 0; step < 400000; step++ {
			pc := uint16(e.VCS.CPU.PC.Address())
			if blank[pc] {
				reached[pc] = true
				regionsChecked++
				if !e.DisplayOff() {
					bad[pc]++
					hits++
				}
			}
			if err := e.StepInstruction(); err != nil {
				break
			}
		}
		for pc, n := range bad {
			var loc string
			for _, r := range rep.BlankLines {
				if r.Start == pc {
					loc = r.StartLoc
				}
			}
			t.Errorf("%s: region %s ($%04X) is classified blank, but the machine reached its opening "+
				"WSYNC with the display ON %d times — the prover would skip the budget check on code "+
				"that draws", filepath.Base(asm), loc, pc, n)
		}
		skippedNeverReached += len(blank) - len(reached)
		romsChecked++
	}

	if regionsChecked == 0 {
		t.Fatal("no blank region was ever reached at run time — the test proves nothing")
	}
	// An unreached region is not evidence either way, and saying so is the point:
	// a silent skip is how a check reports a pass while covering nothing.
	t.Logf("blank classification held on %d executions of blank-region entry points across %d ROMs "+
		"(%d disagreements); %d blank regions were never reached in the run and are NOT covered",
		regionsChecked, romsChecked, hits, skippedNeverReached)
}
