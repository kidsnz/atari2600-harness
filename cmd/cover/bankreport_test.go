package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/kidsnz/atari2600-harness/internal/cyclebound"
	"github.com/kidsnz/atari2600-harness/internal/emu"
)

// run drives a ROM exactly as main() does at the defaults and hands back the
// finished report together with the coverage it was built from, so a test can
// compare the report against the raw record rather than against itself.
func run(t *testing.T, rom string, frames int) (report, *emu.Coverage) {
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
	if err := driveROM(e, "hold", frames, nil); err != nil {
		t.Fatal(err)
	}
	return buildReport(rom, frames, 2, "hold", e.Coverage()), e.Coverage()
}

// A branch that ran only in the OTHER bank must be reported unreached.
//
// This is the whole point of the report on a bank-switched cartridge. Both banks of
// an 8K image decode $F000-$FFFF, so bank 0's $F123 and bank 1's $F123 are two
// different instructions sharing one number; the report used to ask emu.Coverage
// "was this ADDRESS executed anywhere", which answers yes for the twin that never
// ran. Measured at -frames 120 -warmup 2 against the bank-aware static branch set,
// that was 80 branches across the corpus called covered which a (bank, address)
// comparison calls unreached — exerciser 16, and 35/19/9 on Pressure Cooker,
// Vanguard and Aquaventure, which live outside this repo.
//
// The truth this checks against is built from SeenSites, NOT from SeenIn, so
// reverting SeenIn to an address-only lookup makes the test fail on the assertion
// rather than quietly agreeing with itself. Confirmed by planting exactly that.
func TestABranchThatRanOnlyInTheOtherBankIsReportedUnreached(t *testing.T) {
	const rom = "../../roms/exerciser/exerciser.bin"
	rep, cov := run(t, rom, 120)

	static, banked, err := cyclebound.StaticBranchSites(rom)
	if err != nil {
		t.Fatalf("static decode: %v", err)
	}
	if !banked {
		t.Fatalf("premise broken: %s is not a bank-switched image, so it cannot exercise the collision", rom)
	}

	ranIn := map[cyclebound.CodeSite]bool{}
	anyBank := map[uint16]bool{}
	for _, s := range cov.SeenSites() {
		ranIn[cyclebound.CodeSite{Bank: s[0], Addr: uint16(s[1])}] = true
		anyBank[uint16(s[1])] = true
	}

	var falseCovered []cyclebound.CodeSite
	for _, s := range static {
		if !ranIn[s] && anyBank[s.Addr] {
			falseCovered = append(falseCovered, s)
		}
	}
	if len(falseCovered) == 0 {
		t.Fatal("premise broken: no static branch site went unexecuted while its twin in the other " +
			"bank ran, so this ROM no longer exercises the collision and the test proves nothing")
	}

	listed := map[string]bool{}
	for _, s := range rep.UnreachedBranches {
		listed[s] = true
	}
	missed := 0
	for _, s := range falseCovered {
		want := fmt.Sprintf("bank %d 0x%04X", s.Bank, s.Addr)
		if !listed[want] {
			missed++
			if missed <= 5 {
				t.Errorf("bank %d $%04X was never executed, but the report does not list it as "+
					"unreached — $%04X ran in the other bank and a bank-blind lookup calls that covered",
					s.Bank, s.Addr, s.Addr)
			}
		}
	}
	if missed > 5 {
		t.Errorf("... and %d more of the same", missed-5)
	}

	// The opposite direction, so the fix cannot be "report everything": no site the
	// machine actually executed may appear in the unreached list.
	for _, s := range static {
		if ranIn[s] && listed[fmt.Sprintf("bank %d 0x%04X", s.Bank, s.Addr)] {
			t.Errorf("bank %d $%04X executed, yet the report calls it unreached", s.Bank, s.Addr)
		}
	}
	t.Logf("%d static branch sites, %d unreached, %d of those would have read as covered "+
		"without the bank", len(static), len(rep.UnreachedBranches), len(falseCovered))
}

// A coverage fraction above 1 is not a coverage fraction.
//
// banked_game reported edge_coverage 216.7% — 13 exercised edges over a denominator
// of 3 branches — because the denominator came from a FLAT fold of an 8K file at
// $E000-$FFFF, an address space the console does not have, while the numerator was
// counted per (bank, address) by the emulator. The two sides described different
// machines. Now both come from the same per-bank decode.
func TestEdgeCoverageOnABankedImageIsAFraction(t *testing.T) {
	for _, rom := range []string{
		"../../roms/litmus/litmus_bank.bin",
		"../../roms/litmus/litmus_bank_f6.bin",
		"../../roms/litmus/litmus_bank_f4.bin",
		"../../roms/techniques/banked_game.bin",
		"../../roms/exerciser/exerciser.bin",
	} {
		rep, _ := run(t, rom, 120)
		if rep.BranchesStatic == nil || rep.EdgeCoverage == nil {
			t.Errorf("%s: no static denominator (%s)", rom, rep.StaticRefused)
			continue
		}
		if len(rep.ExecutedButUndecoded) > 0 {
			// A real, reported condition: the decoder did not reach code the machine
			// ran, so the denominator is genuinely too small. Not this test's subject.
			t.Logf("%s: %d executed-but-undecoded branches, skipping the bound",
				rom, len(rep.ExecutedButUndecoded))
			continue
		}
		if *rep.EdgeCoverage > 1.0 {
			t.Errorf("%s: edge_coverage %.4f — %d edges over %d branches; a numerator counted per "+
				"(bank, address) cannot exceed a denominator that contains it",
				rom, *rep.EdgeCoverage, rep.EdgesExercised, *rep.BranchesStatic)
		}
		t.Logf("%s: static=%d edges=%d edge_coverage=%.4f",
			rom, *rep.BranchesStatic, rep.EdgesExercised, *rep.EdgeCoverage)
	}
}

// A cartridge this harness refuses to decode gets a null, not a zero.
//
// A superchip overlays 128 bytes of RAM on $F000-$F0FF, so the image is not what the
// CPU reads there and internal/cyclebound declines it by name. The report used to
// carry that through as branches_static: 0 and edge_coverage: 0.0 — numbers, and a
// number is something a reader averages, plots and compares. This is the witness
// that the refusal path is reachable and says what it refused.
func TestSuperchipCartridgeIsRefusedRatherThanScoredZero(t *testing.T) {
	const rom = "../../roms/litmus/litmus_superchip.bin"
	rep, _ := run(t, rom, 20)

	if rep.StaticRefused == "" {
		t.Fatal("a superchip cartridge was given a static denominator: its bottom page is RAM, " +
			"so the bytes decoded there are not the instructions the CPU executes")
	}
	if !strings.Contains(rep.StaticRefused, "F8SC") {
		t.Errorf("the refusal must name the mapper the ENGINE fingerprinted; got %q", rep.StaticRefused)
	}
	if !strings.Contains(rep.StaticRefused, "a superchip overlays 128 bytes of RAM") {
		t.Errorf("the refusal must say WHY, not just that it refused; got %q", rep.StaticRefused)
	}
	if rep.BranchesStatic != nil || rep.EdgeCoverage != nil || rep.UnreachedBranches != nil {
		t.Errorf("a refused decode must leave the static fields unset, got branches_static=%v "+
			"edge_coverage=%v unreached=%v", rep.BranchesStatic, rep.EdgeCoverage, rep.UnreachedBranches)
	}
	if !rep.DecoderIncomplete {
		t.Error("a refused decode is the strongest form of an incomplete one")
	}
	// The JSON is the artifact, so assert on the JSON: 0 and null are the same Go
	// zero value away from each other and only one of them is honest here.
	b, err := json.Marshal(rep)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"branches_static":null`, `"edge_coverage":null`} {
		if !strings.Contains(string(b), want) {
			t.Errorf("marshalled report is missing %s: %s", want, b)
		}
	}
	// The observed half is still real and still reported — refusing the denominator
	// is not refusing the run.
	if rep.PCExecuted == 0 {
		t.Error("no instructions recorded; the refusal must not suppress the observed side")
	}
}

// On a flat 4K cartridge there is exactly one bank, so printing it would be noise —
// and would change the text of every report on the 4K images that are most of the
// corpus. This pins the label format both ways.
func TestBranchLabelsCarryTheBankOnlyWhenThereIsMoreThanOne(t *testing.T) {
	flatLabel := regexp.MustCompile(`^0x[0-9A-F]{4}$`)
	bankLabel := regexp.MustCompile(`^bank \d+ 0x[0-9A-F]{4}$`)

	repFlat, _ := run(t, "../../roms/techniques/divtable.bin", 60)
	n := 0
	for _, group := range [][]string{repFlat.UnreachedBranches, repFlat.OneSided, repFlat.ExecutedButUndecoded} {
		for _, s := range group {
			n++
			if !flatLabel.MatchString(s) {
				t.Errorf("flat 4K image: label %q must be a bare address", s)
			}
		}
	}
	if n == 0 {
		t.Fatal("the flat ROM produced no labels at all, so the format was not checked")
	}

	repBank, _ := run(t, "../../roms/exerciser/exerciser.bin", 120)
	n = 0
	for _, group := range [][]string{repBank.UnreachedBranches, repBank.OneSided, repBank.ExecutedButUndecoded} {
		for _, s := range group {
			n++
			if !bankLabel.MatchString(s) {
				t.Errorf("bank-switched image: label %q must name its bank", s)
			}
		}
	}
	if n == 0 {
		t.Fatal("the banked ROM produced no labels at all, so the format was not checked")
	}
}
