package cyclebound

import (
	"fmt"
	"os"
	"sort"
	"strings"
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
//
// Since cross-bank seeding the claim is the whole cartridge, not just bank 0:
// measured before seeding, 5 executed pairs across this corpus were absent from
// the decode (litmus_bank 4 — bank 1 $FF03/$FF05/$FF07/$FF09; banked_game 1 —
// bank 1 $FF83), all of them worker-bank code entered by a trampoline. After
// seeding: 0 on all four ROMs.
func TestBankedDecodeContainsWhatTheMachineExecutes(t *testing.T) {
	everSeeded := 0 // executed pairs the vector-only decode missed, summed over the corpus
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

		// The static side: every (bank, addr) the per-bank decodes reached, WITH the
		// cross-bank seeding that closes a worker bank entered by a trampoline.
		static := map[[2]int]bool{}
		decodes, seeds := decodeUnits(units)
		for _, u := range decodes {
			for a := range u.instrs {
				static[[2]int{u.bank, int(a)}] = true
			}
		}
		if seeds.capped {
			t.Errorf("%s: the cross-bank seeding fixpoint hit its %d-round cap, so the decode is "+
				"incomplete by an unknown amount and any containment result below is meaningless",
				asm, seedRoundCap)
		}

		// Negative control for the seeding itself. Vector-only is what the decode was
		// before seeding existed; if it already covered everything, this ROM cannot
		// tell a working seeder from a no-op and the check below proves nothing about
		// seeding. Every pair the vector-only decode had must survive — seeding only
		// ever ADDS entry points — so a shrink is a bug, not a trade-off.
		vectorOnly := map[[2]int]bool{}
		for _, u := range units {
			instrs, _ := u.prog.decodeFromVectors()
			for a := range instrs {
				vectorOnly[[2]int{u.bank, int(a)}] = true
			}
		}
		for k := range vectorOnly {
			if !static[k] {
				t.Fatalf("%s: seeding LOST (bank %d, $%04X) that the vector-only decode had; "+
					"seeding must be strictly additive", asm, k[0], k[1])
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

		// Stage 2 seeds each bank at the address following every hotspot access in
		// every other bank, so a worker bank entered only by the trampoline is no
		// longer excused: the claim under test is now "NOTHING the machine executes is
		// absent from the decode, in any bank".
		missingByBank := map[int]int{}
		var missing [][2]int
		unseeded := 0
		for k := range observed {
			if !static[k] {
				missing = append(missing, k)
				missingByBank[k[0]]++
			}
			if !vectorOnly[k] {
				unseeded++
			}
		}
		total := len(missing)
		sort.Slice(missing, func(i, j int) bool {
			if missing[i][0] != missing[j][0] {
				return missing[i][0] < missing[j][0]
			}
			return missing[i][1] < missing[j][1]
		})
		if total > 0 {
			var where []string
			for _, m := range missing {
				where = append(where, fmt.Sprintf("bank %d $%04X", m[0], m[1]))
			}
			t.Errorf("%s: %d executed instruction(s) absent from the decode: %s — the prover is "+
				"reasoning about a different program from the one that runs",
				asm, total, strings.Join(where, ", "))
		}
		// Teeth check: on this corpus the vector-only decode MUST leave something out
		// on at least one ROM, otherwise the containment result above would pass with
		// the seeding deleted. Reported per ROM; asserted across the set below.
		everSeeded += unseeded

		// Whatever residue is left must be NAMED, not left to look like coverage. If a
		// bank contributes executed instructions the decode never saw, the report has
		// to show that bank as barely decoded. (Measured: 0 such banks on all four
		// ROMs since seeding — this survives so that a regression cannot quietly turn
		// a decoded bank back into an assumed one.)
		rep := mustProve(t, asm, 76)
		if len(rep.CrossBankSeeds) != len(seeds.seeds) {
			t.Errorf("%s: the report lists %d cross-bank seed(s) but the decode used %d — the report "+
				"is not describing the analysis that ran", asm, len(rep.CrossBankSeeds), len(seeds.seeds))
		}
		// Every seed must be an entry point the machine really reaches, or it is an
		// invented control-flow edge. Over-approximation is allowed for the DECODE
		// (extra bytes decoded), so this only checks the ones the run touched.
		for _, s := range rep.CrossBankSeeds {
			if observed[[2]int{s.FromBank, int(s.FromAddr)}] && !observed[[2]int{s.ToBank, int(s.ToAddr)}] {
				t.Errorf("%s: seed says (bank %d, $%04X) reaching %s lands at (bank %d, $%04X), but the "+
					"machine executed the source and never that landing address",
					asm, s.FromBank, s.FromAddr, s.Symbol, s.ToBank, s.ToAddr)
			}
		}
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
			"decode across %d banks (bank 0: %d); %d cross-bank seed(s) in %d fixpoint round(s) rescued "+
			"%d executed pair(s) the vector-only decode missed — %s",
			asm, len(observed), len(pcOnly), total, len(static), len(units), missingByBank[0],
			len(seeds.seeds), seeds.rounds, unseeded,
			map[bool]string{
				true:  "banks share an executed address, so this ROM WOULD catch a flat-keyed instrument",
				false: "banks never execute the same address, so this ROM CANNOT catch a flat-keyed instrument",
			}[sharesAddr])
	}
	// A containment check that the seeding cannot affect would keep passing if the
	// seeding were deleted, which is how a test stops testing. Measured: the
	// vector-only decode misses 5 executed pairs across this corpus (litmus_bank 4,
	// banked_game 1), so this is a real constraint on the corpus, not a formality.
	if everSeeded == 0 {
		t.Errorf("no ROM in this set has code reachable only across a bank switch, so the containment "+
			"check above would pass with the cross-bank seeding removed and proves nothing about it "+
			"(expected %d such executed pairs)", 5)
	}
}

// Seeding names the target bank from the mapper's OWN symbol, and a symbol that
// does not name a bank must be refused rather than scraped for digits. Two shapes
// in the vendored mappers make this concrete: Parker Bros publishes "B0S0" (bank 0
// of SEGMENT 0 — only a 1K slice moves, so "the same address in the other bank" is
// not where execution lands) and M-Network publishes "RAM0" (cartridge RAM, which
// is not in the image at all). A loose parse would read those as banks 0 and 0/3
// and seed a decode entry the hardware never reaches.
func TestHotspotSymbolIsParsedNotGuessed(t *testing.T) {
	for _, c := range []struct {
		sym  string
		bank int
		ok   bool
	}{
		{"BANK0", 0, true},   // F8/F6/F4/EF/BF/CBS/JANE
		{"BANK1", 1, true},   // F8 $1FF9, measured
		{"BANK7", 7, true},   // F4 $1FFB, measured
		{"BANK15", 15, true}, // EF
		{"BANK63", 63, true}, // BF
		{"B0S0", 0, false},   // Parker Bros E0: bank-in-segment
		{"B7S2", 0, false},   // Parker Bros E0
		{"RAM0", 0, false},   // M-Network: selects cartridge RAM, not an image bank
		{"RAM3", 0, false},   // M-Network
		{"BANK", 0, false},   // prefix with no number
		{"BANKX", 0, false},  // not a number
		{"BANK+1", 0, false}, // strconv.Atoi would take this; a hotspot symbol never looks like it
		{"MYBANK1", 0, false},
		{"", 0, false},
	} {
		got, ok := hotspotTargetBank(c.sym)
		if ok != c.ok || (ok && got != c.bank) {
			t.Errorf("hotspotTargetBank(%q) = (%d, %v), want (%d, %v)", c.sym, got, ok, c.bank, c.ok)
		}
	}
}

// A flat ROM must go down a path indistinguishable from the one before seeding
// existed — same instruction set, same entry list, no seeds — which is the reason
// its JSON stays byte-identical.
func TestFlatRomIsNotSeeded(t *testing.T) {
	asm := "../../roms/litmus/cb_clean.asm"
	bin := build.BinPathFor(asm)
	if out, err := build.Assemble(asm, bin); err != nil {
		t.Fatalf("assemble %s: %s", asm, out)
	}
	rom, err := os.ReadFile(bin)
	if err != nil {
		t.Fatal(err)
	}
	units, decline := analysisUnits(rom, bin)
	if decline != "" || len(units) != 1 {
		t.Fatalf("%s: premise broken, want 1 analysis unit and no decline, got %d unit(s) %q",
			asm, len(units), decline)
	}
	want, wantEntries := units[0].prog.decodeFromVectors()
	decodes, seeds := decodeUnits(units)
	if len(seeds.seeds) != 0 || seeds.rounds != 0 || seeds.capped ||
		len(seeds.unresolved) != 0 || seeds.unresolvable != 0 {
		t.Errorf("%s: a flat ROM was touched by the seeding pass: %+v", asm, seeds)
	}
	if len(decodes) != 1 || len(decodes[0].instrs) != len(want) || decodes[0].seeded != 0 ||
		len(decodes[0].entries) != len(wantEntries) {
		t.Fatalf("%s: flat decode changed: %d instrs / %d entries / %d seeded, want %d / %d / 0",
			asm, len(decodes[0].instrs), len(decodes[0].entries), decodes[0].seeded,
			len(want), len(wantEntries))
	}
	for a := range want {
		if _, ok := decodes[0].instrs[a]; !ok {
			t.Fatalf("%s: flat decode lost $%04X", asm, a)
		}
	}
	rep := mustProve(t, asm, 76)
	if rep.Banks != 0 || len(rep.CrossBankSeeds) != 0 || rep.CrossBankSeedRounds != 0 ||
		rep.CrossBankSeedCapped || len(rep.UnresolvedHotspots) != 0 || rep.UnresolvableSwitchAccesses != 0 {
		t.Errorf("%s: a flat ROM's report carries bank-seeding fields, so its JSON is no longer "+
			"byte-identical to the pre-seeding output", asm)
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
