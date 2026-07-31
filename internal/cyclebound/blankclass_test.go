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
	// The whole corpus, not a subset: this grading was implicitly limited to the
	// technique kernels, and the two defects that limit exposed (a bank-blind key and
	// a sampling point taken before a delayed register write resolves) were both
	// invisible while it ran on 32 ROMs.
	for _, pat := range []string{"../../roms/litmus/*.asm", "../../roms/exerciser/*.asm"} {
		more, _ := filepath.Glob(pat)
		files = append(files, more...)
	}

	romsChecked, regionsChecked, hits, skippedNeverReached := 0, 0, 0, 0
	for _, asm := range files {
		rep, err := Prove(asm, DefaultBudget)
		if err != nil || len(rep.BlankLines) == 0 {
			continue
		}
		// Keyed on (bank, address), not address alone. exerciser and banked_game are
		// 8K images, and two banks decode the SAME addresses: a bank-blind map matches
		// whatever happens to sit at $Fxxx in the OTHER bank and checks the display
		// state of unrelated code. Measured 2026-07-30 on exerciser — the probe landed
		// at scanline 36 with the picture on, nowhere near the frame-top region the
		// prover had classified. banked_game is in this corpus, so the old number was
		// partly aimed at the wrong instructions.
		type key struct {
			bank int
			addr uint16
		}
		blank := map[key]bool{}
		bankedImage := false
		for _, r := range rep.BlankLines {
			bk := 0
			if r.BankValid {
				bk, bankedImage = r.Bank, true
			}
			blank[key{bk, r.Start}] = true
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
		// 120k rather than 400k instructions per ROM. Measured 2026-07-30 while taking
		// the corpus from 32 ROMs to 128: at 400k the run costs 180s, at 120k it costs
		// 57s, and the number that matters — blank regions never reached, i.e. the ones
		// this grading does NOT cover — is 1 either way. The extra 280k instructions
		// per ROM buy repeat executions of entry points already covered, not coverage.
		for step := 0; step < 120000; step++ {
			pc := uint16(e.VCS.CPU.PC.Address())
			bk := 0
			if bankedImage {
				bk, _ = e.Bank()
			}
			opensBlankRegion := blank[key{bk, pc}]
			// Sample AFTER the opening WSYNC has been executed, not before it. A TIA
			// register write is delayed (futureVblank), so at the strobe instruction
			// itself a `sta VBLANK` issued one instruction earlier has not reached the
			// signal yet: measured on litmus_bound_proxy, DisplayOff() is false at the
			// strobe and true one instruction later, for a region that is genuinely
			// blanked. The question this test asks is about the LINE the region opens,
			// so the strobe is the wrong instant to ask it at.
			if err := e.StepInstruction(); err != nil {
				break
			}
			if opensBlankRegion {
				reached[pc] = true
				regionsChecked++
				if !e.DisplayOff() {
					bad[pc]++
					hits++
				}
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
