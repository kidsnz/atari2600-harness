package cyclebound

import (
	"os"
	"testing"

	"github.com/kidsnz/atari2600-harness/internal/build"
	"github.com/kidsnz/atari2600-harness/internal/emu"
)

// Analysing a bank-switched cartridge one bank at a time is only worth anything
// if the union of those per-bank decodes actually covers the instructions the
// machine executes. Checking that against the analysis itself would agree with
// itself, so the oracle is the machine: run the ROM and record every
// (bank, PC) pair it really reaches.
//
// The direction that matters: an executed pair MISSING from the static decode
// means the prover is reasoning about a different program from the one that runs.
// A decoded pair never executed is only over-approximation, which is allowed.
//
// It also guards the instrument. If the dynamic side were keyed on PC alone, two
// banks' identical addresses would collide and the containment check would pass
// while testing half of what it claims — so the test asserts the (bank,pc) set is
// strictly larger than the pc set on a ROM whose banks genuinely share addresses.
func TestBankedDecodeContainsWhatTheMachineExecutes(t *testing.T) {
	for _, asm := range []string{
		"../../roms/techniques/banked_game.asm",
		"../../roms/litmus/litmus_bank.asm",
		"../../roms/litmus/litmus_bank_f6.asm",
		"../../roms/litmus/litmus_bank_f4.asm",
	} {
		bin := build.BinPathFor(asm)
		if out, err := build.Assemble(asm, bin); err != nil {
			t.Fatalf("assemble %s: %s", asm, out)
		}
		rom, err := os.ReadFile(bin)
		if err != nil {
			t.Fatal(err)
		}
		units, decline := analysisUnits(rom, bin)
		if decline != "" {
			t.Fatalf("%s declined: %s", asm, decline)
		}
		if len(units) < 2 {
			t.Fatalf("%s: premise broken, expected a multi-bank cartridge, got %d unit(s)", asm, len(units))
		}

		// The static side: every (bank, addr) the per-bank decodes reached.
		static := map[[2]int]bool{}
		for _, u := range units {
			instrs, _ := u.prog.decodeFromVectors()
			for a := range instrs {
				static[[2]int{u.bank, int(a)}] = true
			}
		}

		// The machine.
		e, err := emu.New("NTSC")
		if err != nil {
			t.Skip("emulator unavailable")
		}
		if err := e.LoadROM(bin); err != nil {
			t.Fatal(err)
		}
		if err := e.RunFrames(4); err != nil { // settle past power-on
			t.Fatal(err)
		}
		observed := map[[2]int]bool{}
		pcOnly := map[int]bool{}
		for i := 0; i < 120000; i++ {
			bank, isRAM := e.Bank()
			pc := int(e.PC())
			if !isRAM {
				observed[[2]int{bank, pc}] = true
				pcOnly[pc] = true
			}
			if err := e.StepInstruction(); err != nil {
				break
			}
		}
		if len(observed) == 0 {
			t.Fatalf("%s: nothing observed, the test proves nothing", asm)
		}

		// Stage 1 decodes each bank from ITS OWN vectors and does not follow flow
		// across a switch, so a worker bank entered only by the trampoline is
		// expected to be missing. The claim under test is therefore not "nothing is
		// missing" but "nothing is missing FROM BANK 0, the bank reached from the
		// power-on vectors, and every other absence is in a bank the analysis openly
		// says it did not enter".
		missingByBank := map[int]int{}
		var example [2]int
		total := 0
		for k := range observed {
			if !static[k] {
				if total == 0 {
					example = k
				}
				missingByBank[k[0]]++
				total++
			}
		}
		if n := missingByBank[0]; n > 0 {
			t.Errorf("%s: %d executed instructions in BANK 0 are absent from its own decode "+
				"(e.g. $%04X) — bank 0 is entered from the reset vector, so nothing there has the "+
				"cross-bank excuse; the prover is reasoning about a different program",
				asm, n, example[1])
		}

		// The residue must be NAMED, not left to look like coverage. If a bank
		// contributes executed instructions the decode never saw, the report has to
		// show that bank as barely decoded.
		rep := mustProve(t, asm, 76)
		cov := map[int]BankCoverage{}
		for _, c := range rep.BankCoverage {
			cov[c.Bank] = c
		}
		for bank, n := range missingByBank {
			if bank == 0 {
				continue
			}
			c, ok := cov[bank]
			if !ok {
				t.Errorf("%s: bank %d executed %d instructions the decode never saw and the report "+
					"does not mention that bank at all", asm, bank, n)
				continue
			}
			if c.Regions > 0 {
				t.Errorf("%s: bank %d is reported with %d proven regions while %d of its executed "+
					"instructions were never decoded — a partially-decoded bank must not carry "+
					"region verdicts", asm, bank, c.Regions, n)
			}
		}
		if rep.UnmodelledSwitches == 0 && total > 0 {
			t.Errorf("%s: %d executed instructions are outside the decode, yet no region was refused "+
				"for switching banks — the residue has no stated cause", asm, total)
		}

		// Instrument check. If the banks never execute the same address, the
		// (bank,pc) keying cannot be distinguished from plain pc keying by this ROM,
		// and saying so is the point: it is a gap in the CORPUS, not a pass.
		sharesAddr := len(observed) > len(pcOnly)
		t.Logf("%s: %d executed (bank,pc) pairs over %d distinct PCs; %d absent from a %d-instruction "+
			"decode across %d banks (bank 0: %d) — %s",
			asm, len(observed), len(pcOnly), total, len(static), len(units), missingByBank[0],
			map[bool]string{
				true:  "banks share an executed address, so this ROM WOULD catch a flat-keyed instrument",
				false: "banks never execute the same address, so this ROM CANNOT catch a flat-keyed instrument",
			}[sharesAddr])
	}
}

// Certification must not survive a bank switch the analysis refused to follow.
// litmus_bank came back certified:true before this was gated — a true statement
// about bank 0 of 2, presented as a verdict on the cartridge.
func TestBankSwitchBlocksCertification(t *testing.T) {
	for _, asm := range []string{
		"../../roms/litmus/litmus_bank.asm",
		"../../roms/litmus/litmus_bank_f6.asm",
		"../../roms/litmus/litmus_bank_f4.asm",
		"../../roms/techniques/banked_game.asm",
	} {
		rep := mustProve(t, asm, 76)
		if rep.UnmodelledSwitches == 0 {
			t.Errorf("%s switches banks every frame; no region was refused for it, so either the "+
				"hotspot is not being seen or the ROM changed", asm)
		}
		if rep.Certified {
			t.Errorf("%s certified with %d unmodelled bank switch(es): the program leaves for a bank "+
				"this analysis never followed, so 'every region I looked at passed' is not a proof",
				asm, rep.UnmodelledSwitches)
		}
	}
}
