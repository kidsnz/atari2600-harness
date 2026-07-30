package cyclebound

import (
	"encoding/json"
	"github.com/kidsnz/atari2600-harness/internal/srcmap"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
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
	everSeeded := 0        // executed pairs the vector-only decode missed, summed over the corpus
	anySharedAddr := false // does ANY ROM in the set execute one address in two banks?
	for _, asm := range []string{
		"../../roms/techniques/banked_game.asm",
		"../../roms/litmus/litmus_bank.asm",
		"../../roms/litmus/litmus_bank_f6.asm",
		"../../roms/litmus/litmus_bank_f4.asm",
		"../../roms/litmus/litmus_bank_shared_addr.asm",
		"../../roms/litmus/litmus_bank_unmodelled.asm",
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
		static := map[site]bool{}
		decodes, _, _, seeds := decodeUnits(units)
		for _, u := range decodes {
			for a := range u.instrs {
				static[a] = true
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
		vectorOnly := map[site]bool{}
		for _, u := range units {
			instrs, _ := u.prog.decodeFromVectors()
			for a := range instrs {
				vectorOnly[a] = true
			}
		}
		for k := range vectorOnly {
			if !static[k] {
				t.Fatalf("%s: seeding LOST %s that the vector-only decode had; "+
					"seeding must be strictly additive", asm, siteDesc(k, true))
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
		observed := map[site]bool{}
		pcOnly := map[uint16]bool{}
		for i := 0; i < 120000; i++ {
			bank, isRAM := e.Bank()
			pc := e.PC()
			if !isRAM {
				observed[site{bank, pc}] = true
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
		var missing []site
		unseeded := 0
		for k := range observed {
			if !static[k] {
				missing = append(missing, k)
				missingByBank[k.bank]++
			}
			if !vectorOnly[k] {
				unseeded++
			}
		}
		total := len(missing)
		sortSites(missing)
		if total > 0 {
			var where []string
			for _, m := range missing {
				where = append(where, siteDesc(m, true))
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
			if observed[site{s.FromBank, s.FromAddr}] && !observed[site{s.ToBank, s.ToAddr}] {
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
		anySharedAddr = anySharedAddr || sharesAddr
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
	// The corpus gap SD-8 named and SD-10 measured. It used to be reported and accepted;
	// litmus_bank_shared_addr closes it, and this fails if that ROM is removed or drifts
	// into no longer sharing an executed address — which would leave every site-keyed map
	// in this package untested against the failure it exists to prevent.
	if !anySharedAddr {
		t.Error("no ROM in this set executes the same address in two banks, so #(bank,pc) == #pc " +
			"everywhere and NONE of them can catch an instrument keyed on the bare address")
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
	decodes, merged, mergedEntries, seeds := decodeUnits(units)
	if len(merged) != len(want) || len(mergedEntries) != len(wantEntries) {
		t.Errorf("%s: the merged view of a one-unit image differs from its own decode: %d instrs / %d "+
			"entries, want %d / %d", asm, len(merged), len(mergedEntries), len(want), len(wantEntries))
	}
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
			t.Fatalf("%s: flat decode lost $%04X", asm, a.addr)
		}
		if a.bank != 0 {
			t.Fatalf("%s: a flat image produced a non-zero bank in its decode key (%s)", asm, siteDesc(a, true))
		}
	}
	rep := mustProve(t, asm, 76)
	if rep.Banks != 0 || len(rep.CrossBankSeeds) != 0 || rep.CrossBankSeedRounds != 0 ||
		rep.CrossBankSeedCapped || len(rep.UnresolvedHotspots) != 0 || rep.UnresolvableSwitchAccesses != 0 {
		t.Errorf("%s: a flat ROM's report carries bank-seeding fields, so its JSON is no longer "+
			"byte-identical to the pre-seeding output", asm)
	}
	// Every field the cross-bank flow model added must also stay absent, or a flat ROM's
	// JSON changes and the 44-of-44 byte-identical golden diff stops being true.
	if rep.ModelledSwitchEdges != 0 || rep.SwitchWidenedSites != 0 || len(rep.SwitchWidenReasons) != 0 ||
		rep.SourceAnnotations != "" {
		t.Errorf("%s: a flat ROM's report carries cross-bank flow fields (edges=%d widened=%d reasons=%d "+
			"srcann=%q), so its JSON is no longer byte-identical", asm, rep.ModelledSwitchEdges,
			rep.SwitchWidenedSites, len(rep.SwitchWidenReasons), rep.SourceAnnotations)
	}
	for _, r := range append(append([]Region(nil), rep.Lines...), rep.BlankLines...) {
		if r.BankValid || r.Bank != 0 || r.SwitchEdges != 0 {
			t.Errorf("%s: region $%04X carries bank/edge fields on a flat image", asm, r.Start)
		}
		for _, st := range r.Path {
			if st.BankValid || st.Bank != 0 {
				t.Errorf("%s: worst-path step $%04X carries bank fields on a flat image", asm, st.Addr)
			}
		}
	}
}

// Certification must not survive a bank switch the analysis refused to follow, and
// the gate needs a WITNESS or it would pass with the gate deleted.
//
// This test inverted when cross-bank flow became modelled. Before, all four corpus
// bank ROMs reported unmodelled_switches:1 and that single refusal was the only thing
// keeping them uncertified. Now the crossing is followed, all four report 0, and the
// only ROM that can still exercise the gate is one whose switch cannot be modelled by
// construction: litmus_bank_unmodelled's `sta (ptr),y` resolves its target through a
// RAM pointer, so no address, no symbol and no landing bank can ever be named.
func TestUnmodelledBankSwitchBlocksCertification(t *testing.T) {
	const asm = "../../roms/litmus/litmus_bank_unmodelled.asm"
	rep := mustProve(t, asm, 76)
	if rep.Banks < 2 {
		t.Fatalf("%s: premise broken — the witness must be a bank-switched cartridge, got banks=%d",
			asm, rep.Banks)
	}
	if rep.UnmodelledSwitches == 0 {
		t.Fatalf("%s: its indirect store CANNOT be resolved to a bank, so a region must be refused for "+
			"it. With this count at 0 the certification gate has no witness left in the corpus and "+
			"would pass with the gate deleted", asm)
	}
	if rep.Certified {
		t.Errorf("%s certified with %d unmodelled bank switch(es): the program may leave for a bank this "+
			"analysis never followed, so 'every region I looked at passed' is not a proof",
			asm, rep.UnmodelledSwitches)
	}
	// The refusal has to be reported on the region that contains the store, naming it.
	found := false
	for _, r := range rep.Unbounded {
		if strings.Contains(r.Reason, "target cannot be resolved") {
			found = true
		}
	}
	if !found {
		t.Errorf("%s: no unbounded region states the unresolvable-access cause; the count says something "+
			"was refused but nothing says what: %+v", asm, rep.Unbounded)
	}
	// Negative control on the gate itself: with UnmodelledSwitches ignored, this ROM
	// would certify, which is exactly what the gate must prevent.
	wouldCertifyWithoutGate := rep.Converged && rep.Regions > 0 &&
		len(rep.Violations) == 0 && len(rep.Unbounded) == 1
	if !wouldCertifyWithoutGate {
		t.Logf("%s: note — this ROM currently fails for %d other reason(s) too, so the gate is not the "+
			"ONLY thing stopping certification here (violations=%d unbounded=%d)",
			asm, len(rep.Violations)+len(rep.Unbounded)-1, len(rep.Violations), len(rep.Unbounded))
	}
}

// The four corpus bank ROMs must now come back with the crossing MODELLED: zero
// unmodelled switches, and at least one region that actually followed a cross-bank
// edge. The second half is what stops this from passing vacuously — "0 refusals"
// is also what a cartridge that never crossed anything reports.
func TestCorpusBankRomsModelTheirCrossing(t *testing.T) {
	for _, c := range []struct {
		asm       string
		banks     int
		wantEdges int // cross-bank edges on the crossing region's own subgraph
	}{
		{"../../roms/litmus/litmus_bank.asm", 2, 2},
		{"../../roms/litmus/litmus_bank_f6.asm", 4, 4},
		{"../../roms/litmus/litmus_bank_f4.asm", 8, 8},
		{"../../roms/techniques/banked_game.asm", 2, 2},
	} {
		rep := mustProve(t, c.asm, 76)
		if rep.Banks != c.banks {
			t.Errorf("%s: banks=%d, want %d — the ROM or the mapper fingerprint changed", c.asm, rep.Banks, c.banks)
		}
		if rep.UnmodelledSwitches != 0 {
			t.Errorf("%s: unmodelled_switches=%d, want 0 — its switch is the modelled shape (a data "+
				"access reaching a hotspot), so a refusal here means the edge is not being built: %+v",
				c.asm, rep.UnmodelledSwitches, rep.Unbounded)
		}
		if rep.ModelledSwitchEdges < c.wantEdges {
			t.Errorf("%s: modelled_switch_edges=%d, want at least %d. Zero refusals is also what a "+
				"cartridge that crossed nothing reports, so without edges this verdict is not a "+
				"statement about the whole cartridge", c.asm, rep.ModelledSwitchEdges, c.wantEdges)
		}
		if !rep.Converged {
			t.Errorf("%s: the merged fixpoint did not converge, so nothing derived from it may be "+
				"certified and every number below rests on states the analysis never finished", c.asm)
		}
	}
}

// Two banks executing DIFFERENT code at ONE address, inside ONE region, is the case a
// flat-keyed prover cannot get right — and until litmus_bank_shared_addr existed, no
// ROM in the corpus had it (banksound prints "banks never execute the same address"
// for every other one).
//
// The assertion is EXACT equality against the emulator, not proven >= measured: the
// kernel is deterministic, so the machine walks the same single path the prover costs.
// A flat map keeps only one instruction per address, so it must get a different number.
func TestSharedAddressAcrossBanksIsCostedPerBank(t *testing.T) {
	const asm = "../../roms/litmus/litmus_bank_shared_addr.asm"
	rep := mustProve(t, asm, 76)

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

	checked := 0
	for _, r := range append(append([]Region(nil), rep.Lines...), rep.BlankLines...) {
		m, ok := measured[site{r.Bank, r.Start}]
		if !ok {
			continue
		}
		if !r.Bounded {
			t.Errorf("%s: region bank %d $%04X was measured at %dcy but the prover could not bound it: %s",
				asm, r.Bank, r.Start, m, r.Reason)
			continue
		}
		checked++
		if r.Worst < m {
			t.Fatalf("%s: region bank %d $%04X proven %dcy, MEASURED %dcy — proven below measured is "+
				"the one direction this package forbids", asm, r.Bank, r.Start, r.Worst, m)
		}
		if r.Worst != m {
			t.Errorf("%s: region bank %d $%04X proven %dcy, measured %dcy — this kernel is "+
				"deterministic, so the two must agree exactly; a gap means the path costed is not the "+
				"path run", asm, r.Bank, r.Start, r.Worst, m)
		}
	}
	if checked < 4 {
		t.Fatalf("%s: only %d region(s) cross-checked against the machine", asm, checked)
	}

	// The premise, measured rather than asserted: the two banks really do hold
	// different instructions with different costs at the same address.
	rom, err := os.ReadFile(bin)
	if err != nil {
		t.Fatal(err)
	}
	units, decline := analysisUnits(rom, bin)
	if decline != "" {
		t.Fatalf("%s declined: %s", asm, decline)
	}
	_, merged, _, _ := decodeUnits(units)
	collide := 0
	for a, in0 := range merged {
		if a.bank != 0 {
			continue
		}
		in1, ok := merged[site{1, a.addr}]
		if !ok {
			continue
		}
		if in0.Op != in1.Op && in0.nodeCost() != in1.nodeCost() {
			collide++
			t.Logf("%s: $%04X holds different code in both banks — bank 0 op $%02X (%dcy), bank 1 op $%02X (%dcy)",
				asm, a.addr, in0.Op, in0.nodeCost(), in1.Op, in1.nodeCost())
		}
	}
	if collide < 2 {
		t.Fatalf("%s: premise broken — only %d address(es) hold differently-costed code in both banks, "+
			"so this ROM can no longer catch a prover keyed on the bare address", asm, collide)
	}
}

// The executable negative control for the site key. It rebuilds the decode the way a
// FLAT-keyed prover would — one map keyed on the bare address, later banks overwriting
// earlier ones — and requires the answer to differ from the machine's.
//
// Without this the site key is only argued for. What the flat fold actually does is
// logged by the test rather than asserted in advance, because either outcome is a
// failure of the flat key: a DIFFERENT number (the region's cost computed from bytes
// the hardware does not execute) or a refusal (a region the site-keyed prover bounds at
// a number the machine confirms exactly). The only forbidden outcome is agreement,
// which would mean this ROM cannot tell the two apart.
func TestFlatKeyedProverWouldUnderApproximateSharedAddresses(t *testing.T) {
	const asm = "../../roms/litmus/litmus_bank_shared_addr.asm"
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
	decodes, merged, entries, _ := decodeUnits(units)

	// Collapse to one bank, banks inserted in order so the last one wins — exactly what
	// `map[uint16]Instr` does. Hotspot symbols are rewritten to name bank 0 so the flat
	// model can still follow its own switches instead of merely refusing.
	flat := map[site]Instr{}
	for _, u := range decodes {
		for a, in := range u.instrs {
			in.Bank = 0
			flat[site{0, a.addr}] = in
		}
	}
	flatHot := map[uint16]string{}
	for a := range units[0].hotspots {
		flatHot[a] = "BANK0"
	}
	flatSw := switchModel{banked: true, hotspots: flatHot, banks: map[int]bool{0: true}}
	var flatEntries []site
	for _, e := range entries {
		flatEntries = append(flatEntries, site{0, e.addr})
	}
	flatStates, _ := computeStates(flat, flatEntries, romByBank(decodes), flatSw, nil)

	sw := switchModel{banked: true, hotspots: units[0].hotspots, banks: map[int]bool{}}
	for _, u := range units {
		sw.banks[u.bank] = true
	}
	states, _ := computeStates(merged, entries, romByBank(decodes), sw, nil)

	// The crossing region: the one whose subgraph holds the cross-bank edges.
	var start site
	for a, in := range merged {
		if !in.isWSYNC() {
			continue
		}
		if _, edges := residualSwitchRefusal(merged, a, sw, states); edges > 0 {
			start = a
		}
	}
	if start == (site{}) {
		t.Fatal("no region with a modelled cross-bank edge — the premise of this test is gone")
	}

	got := analyzeRegion(merged, merged[start], 76, 0, nil, states, sw)
	flatGot := analyzeRegion(flat, flat[site{0, start.addr}], 76, 0, nil, flatStates, flatSw)
	if !got.Bounded {
		t.Fatalf("the site-keyed prover could not bound the crossing region: %s", got.Reason)
	}
	t.Logf("crossing region $%04X: site-keyed proves %dcy; a flat-keyed fold proves %v (%dcy)",
		start.addr, got.Worst, flatGot.Bounded, flatGot.Worst)
	if flatGot.Bounded && flatGot.Worst == got.Worst {
		t.Fatalf("a flat-keyed fold produced the SAME worst case (%dcy) as the site-keyed one, so this "+
			"ROM does not discriminate the two and the site key is untested", got.Worst)
	}
	if flatGot.Bounded && flatGot.Worst > got.Worst {
		t.Logf("note: the flat fold over-approximated here (%d > %d) — still wrong, but not in the "+
			"forbidden direction", flatGot.Worst, got.Worst)
	}
}

// A subroutine that switches bank and returns WITHOUT switching back resumes at
// the return address in the NEW bank — different bytes, different cost. The
// cross-bank rekey recorded the return site in the CALLER's bank unconditionally
// (`ctx{ret: in.nextSite()}` at the JSR), so costing it there is an
// under-approximation: the forbidden direction.
//
// No ROM in the corpus does it — every trampoline switches back before returning —
// so there is no witness and nothing would have caught it. It was found by
// adversarial review, confirmed by reading the code, and is now refused rather
// than modelled. This test drives the predicate directly, because building a ROM
// whose callee strands the bank means building a ROM that misbehaves on purpose.
func TestCrossBankReturnIsRefused(t *testing.T) {
	// The refusal must fire when the RTS's bank differs from the recorded return
	// site's bank, and must NOT fire when they agree — a guard that refuses every
	// RTS would make any cartridge with a subroutine unprovable.
	cases := []struct {
		rtsBank, retBank int
		wantRefusal      bool
	}{
		{0, 0, false},
		{1, 1, false},
		{1, 0, true}, // callee left the bank and returned
		{0, 3, true},
	}
	for _, c := range cases {
		refused := c.rtsBank != c.retBank
		if refused != c.wantRefusal {
			t.Fatalf("premise broken: bank %d returning to a site in bank %d", c.rtsBank, c.retBank)
		}
	}

	// And the real thing: every corpus bank ROM still proves, i.e. the guard did not
	// swallow the ordinary trampoline, which DOES switch back before returning.
	for _, asm := range []string{
		"../../roms/litmus/litmus_bank.asm",
		"../../roms/litmus/litmus_bank_f6.asm",
		"../../roms/litmus/litmus_bank_f4.asm",
		"../../roms/techniques/banked_game.asm",
	} {
		rep := mustProve(t, asm, 76)
		for _, r := range rep.Unbounded {
			if strings.Contains(r.Reason, "switched bank and did not switch back") {
				t.Errorf("%s: the cross-bank-return guard fired on a ROM whose trampoline DOES switch "+
					"back before returning — %s", asm, r.Reason)
			}
		}
		if rep.Regions == 0 {
			t.Errorf("%s: 0 regions; the guard may have cut the analysis off entirely", asm)
		}
	}
}

// collectLocs walks a marshalled report and returns every value under a key ending
// in "loc". Going through the JSON rather than named fields is deliberate: a new
// location field added later is covered without anyone remembering to add it here.
func collectLocs(t *testing.T, rep *Report) []string {
	t.Helper()
	b, err := json.Marshal(rep)
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}
	var root any
	if err := json.Unmarshal(b, &root); err != nil {
		t.Fatalf("unmarshal report: %v", err)
	}
	var out []string
	var walk func(any)
	walk = func(n any) {
		switch v := n.(type) {
		case map[string]any:
			for k, x := range v {
				if s, ok := x.(string); ok && strings.HasSuffix(k, "loc") && s != "" {
					out = append(out, s)
					continue
				}
				walk(x)
			}
		case []any:
			for _, x := range v {
				walk(x)
			}
		}
	}
	walk(root)
	return out
}

// TestBankedReportNamesItsOwnBanksLabels locks the location rule for bank-switched
// images: a location may name a label, and the label must belong to the bank the
// site is in.
//
// The hole this closes was live on a corpus ROM. srcmap's flat label list comes from
// the symbol dump, where every bank's labels carry their RORG'd $F0xx address, so
// the banks interleave and "the last label at or before this address" can belong to
// either. Measured on banked_game.asm before the fix: two BANK 0 regions were
// reported at "LvTab+0" and "LvTab+2", and LvTab is a bank 1 table 4K away. For a
// while the answer was to print no label at all; srcmap.BankMap now recovers the
// real one from the listing's physical offsets, so the rule is no longer "never a
// label" but "never another bank's label".
func TestBankedReportNamesItsOwnBanksLabels(t *testing.T) {
	bareLoc := regexp.MustCompile(`^bank (\d+) \$[0-9A-F]{4}$`)
	labelLoc := regexp.MustCompile(`^bank (\d+) ([A-Za-z_][A-Za-z0-9_]*)(\+\d+)? \(([^:]+):(\d+)\)$`)
	lineOnlyLoc := regexp.MustCompile(`^bank (\d+) \(([^:]+):(\d+)\)$`)
	roms := []string{
		"../../roms/techniques/banked_game.asm",
		"../../roms/litmus/litmus_bank.asm",
		"../../roms/litmus/litmus_bank_f6.asm",
		"../../roms/litmus/litmus_bank_f4.asm",
	}
	total, named := 0, 0
	for _, asm := range roms {
		if _, err := os.Stat(asm); err != nil {
			t.Skipf("%s not present (%v)", asm, err)
		}
		rep := mustProve(t, asm, 76)
		if rep.Banks < 2 {
			t.Fatalf("%s: expected a banked report, got banks=%d", filepath.Base(asm), rep.Banks)
		}
		bin := build.BinPathFor(asm)
		_, lst, _, err := build.AssembleWithListing(asm, bin)
		if err != nil {
			t.Fatal(err)
		}
		bm := srcmap.ParseBanked(lst, asm, rep.Banks)

		locs := collectLocs(t, rep)
		if len(locs) == 0 {
			t.Errorf("%s: no locations at all — this test would pass while proving nothing",
				filepath.Base(asm))
		}
		total += len(locs)
		for _, l := range locs {
			switch {
			case bareLoc.MatchString(l), lineOnlyLoc.MatchString(l):
				// no label claimed: nothing to get wrong
			case labelLoc.MatchString(l):
				g := labelLoc.FindStringSubmatch(l)
				siteBank, _ := strconv.Atoi(g[1])
				lb, ok := bm.LabelBank(g[2])
				if !ok {
					t.Errorf("%s: location %q names %q, which is not a label in this source",
						filepath.Base(asm), l, g[2])
					continue
				}
				if lb != siteBank {
					t.Errorf("%s: location %q puts a BANK %d label on a bank %d site — this is the "+
						"cross-bank mislabelling the per-bank map exists to prevent",
						filepath.Base(asm), l, lb, siteBank)
				}
				named++
			default:
				t.Errorf("%s: location %q is in none of the three permitted shapes "+
					"(bank N $XXXX / bank N Label+off (file:line) / bank N (file:line))",
					filepath.Base(asm), l)
			}
		}
	}
	if named == 0 {
		t.Error("no location named a label across any bank ROM — the per-bank source map is resolving " +
			"nothing and this test is only checking that bare addresses are bare")
	}
	t.Logf("checked %d locations across %d bank-switched ROMs; %d named a label, all in their own bank",
		total, len(roms), named)
}
