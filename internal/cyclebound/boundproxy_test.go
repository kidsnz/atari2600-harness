package cyclebound

import (
	"path/filepath"
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

// The cross-bank sibling. Merging two banks' instructions into ONE node set is
// precisely the condition that breaks any heuristic keyed on address order, so the
// predecessor-based bound has to keep working when the counter's initialiser and the
// loop it feeds are in DIFFERENT banks.
//
// litmus_bank_bound plants that: `ldx #5` in bank 0 at $FF02's predecessor, the
// `dex`/`bne` loop at bank 1 $FF05, and no other initialiser anywhere. The header's
// only non-back-edge predecessor is reachable ONLY across the switch, so:
//
//   - an intra-bank-only predecessor scan finds nothing but the back edge, leaves the
//     predecessor set incomplete, and either under-approximates the entry value or
//     returns 0 and loses the bound;
//   - an address-order filter applied across banks compares bank 0's $FF02 with bank
//     1's $FF05 as if they were one program.
//
// Both regressions are caught here: the region must be BOUNDED, and proven must be at
// least measured. This kernel is deterministic, so the two must agree exactly.
func TestCrossBankLoopBoundComesFromThePredecessorsState(t *testing.T) {
	const asm = "../../roms/litmus/litmus_bank_bound.asm"
	rep := mustProve(t, asm, 76)
	if rep.Banks != 2 {
		t.Fatalf("premise broken: want a 2-bank cartridge, got banks=%d", rep.Banks)
	}
	if rep.UnmodelledSwitches != 0 {
		t.Fatalf("its switches are the modelled shape; %d were refused: %+v",
			rep.UnmodelledSwitches, rep.Unbounded)
	}

	bin := build.BinPathFor(asm)
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
	measured := map[site]int{}
	for _, r := range rows {
		measured[site{r.Bank, r.StrobePC}] = r.WorstCycles
	}

	// The crossing region is the one carrying cross-bank edges.
	var crossing *Region
	for i, r := range rep.Lines {
		if r.SwitchEdges > 0 {
			crossing = &rep.Lines[i]
		}
	}
	if crossing == nil {
		t.Fatal("no visible region recorded a cross-bank edge, so the loop's bank boundary is not " +
			"being crossed and this ROM is no longer testing what it was built for")
	}
	m, ok := measured[site{crossing.Bank, crossing.Start}]
	if !ok {
		t.Fatalf("the machine never executed the crossing region at bank %d $%04X",
			crossing.Bank, crossing.Start)
	}
	if !crossing.Bounded {
		t.Fatalf("the crossing region is UNBOUNDED (%s) while the machine bounds it at %dcy — the "+
			"loop's only initialiser is in the other bank, so a predecessor scan that does not cross "+
			"banks loses it", crossing.Reason, m)
	}
	if crossing.Worst < m {
		t.Fatalf("PROVEN %dcy is BELOW the measured %dcy at bank %d $%04X — an incomplete predecessor "+
			"set under-approximates the counter's entry value, hence the trip count, hence the worst "+
			"case. That is the one direction this package forbids.",
			crossing.Worst, m, crossing.Bank, crossing.Start)
	}
	if crossing.Worst != m {
		t.Errorf("proven %dcy vs measured %dcy at bank %d $%04X — this kernel is deterministic, so a "+
			"gap means the path costed is not the path run", crossing.Worst, m, crossing.Bank, crossing.Start)
	}
	t.Logf("crossing region bank %d $%04X: proven %dcy, machine measured %dcy, %d cross-bank edge(s)",
		crossing.Bank, crossing.Start, crossing.Worst, m, crossing.SwitchEdges)
}

// The divide-loop path of determineBound takes A's loop-entry ceiling as the
// MAXIMUM over the header's predecessors. Its own comment promised "Unknown => 0
// (stay unbounded)" and the code did not do it: a predecessor with no abstract
// state, or one whose A is Top, was SKIPPED and the maximum taken over whichever
// predecessors happened to be known.
//
// Skipping is not neutral. A maximum computed over a subset is a LOWER bound on
// the real maximum, so the trip count comes out too small and the proven worst
// case with it — an under-approximation, in the same function whose dex/dey path
// produced SD-9's fortyfold one.
//
// The corpus cannot witness it (every divide loop here has fully-known
// predecessors, and the fix left all 43 golden reports byte-identical), so the
// guarantee is asserted where it lives: nothing the prover BOUNDS may sit below
// what the machine measures, across every ROM that has a divide loop at all.
func TestDivideLoopBoundsHoldAgainstTheMachine(t *testing.T) {
	files, err := filepath.Glob("../../roms/techniques/*.asm")
	if err != nil || len(files) == 0 {
		t.Skip("technique corpus unavailable")
	}
	graded, regions := 0, 0
	for _, asm := range files {
		rep, err := Prove(asm, 76)
		if err != nil || rep.BankedDeclined != "" {
			continue
		}
		bin := build.BinPathFor(asm)
		e, err := emu.New("NTSC")
		if err != nil {
			t.Fatalf("emulator unavailable: %v", err) // not a skip: the gate would cover nothing
		}
		if err := e.LoadROM(bin); err != nil {
			continue
		}
		rows, _, err := e.ProfileLineWorst(4, nil)
		if err != nil {
			continue
		}
		meas := map[uint16]int{}
		for _, r := range rows {
			meas[r.StrobePC] = r.WorstCycles
		}
		graded++
		for _, rg := range append(append([]Region{}, rep.Lines...), rep.BlankLines...) {
			if !rg.Bounded {
				continue
			}
			m, ok := meas[uint16(rg.Start)]
			if !ok {
				continue
			}
			regions++
			if m > rg.Worst {
				t.Errorf("%s %s: proven %d cycles, machine measured %d — an under-approximation on a "+
					"BOUNDED region", filepath.Base(asm), rg.StartLoc, rg.Worst, m)
			}
		}
	}
	if graded == 0 || regions == 0 {
		t.Fatalf("nothing graded (%d ROMs, %d regions) — the gate proves nothing", graded, regions)
	}
	t.Logf("%d bounded regions across %d ROMs, none below what the machine measured", regions, graded)
}
