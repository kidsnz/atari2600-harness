package cyclebound

// How much of the commercial corpus this prover can actually answer, stated as one
// number that cannot drift silently.
//
// THE DENOMINATOR MATTERS AND WAS WRONG FOR A WHILE. `Prove` reports one Region per
// (address, call context), so a scanline entered from two call sites appears twice.
// Counting those rows gives "626 of 958 = 65.3%", which is not a fact about the ROM:
// an address is only USEFULLY proven when EVERY context proves it, because a builder
// asking "does this line fit in 76 cycles?" gets a refusal if any context refuses.
// By that measure the same corpus reads 295 of 626 = 47.1%.
//
// Both numbers are true about what they count. Only the second is true about the
// question. This test pins the second.
//
// WHAT THE CEILING IS, measured by forcing each obstacle to pass (unsound, done once
// by hand, recorded in docs/capability-gap-audit.md):
//
//	47.1%   as shipped
//	54.1%   if every trip count were established
//	47.1%   if WSYNC-in-body were ignored          <- ZERO on its own
//	47.1%   if call-or-jump-in-body were ignored   <- ZERO on its own
//	60.2%   if all three were
//
// The two that are worth nothing alone are worth 6.1 points together with the first.
// THE OBSTACLES ARE NOT INDEPENDENT: a loop blocked by two of them shows up in
// neither single measurement, so measuring one at a time systematically understates
// the pair. This is the same trap as the refusal histogram — a census of first
// obstacles — one level up, and it is why the three repairs made on 2026-08-03/04
// each measured as ~zero while the fourth, if it lands, should move 7 points.

import (
	"testing"
)

// coverageFloor is the fraction of addresses proven in EVERY call context. It is a
// floor, not a target: a change that raises it should raise this constant, and a
// change that lowers it has taken something away from a builder and must say so.
const coverageFloor = 0.47

func TestProverCoverageOnTheCommercialCorpus(t *testing.T) {
	paths := commercialROMPaths()
	if len(paths) < 16 {
		t.Fatalf("only %d cartridges discovered; this number is a fact about the corpus "+
			"and means nothing if the corpus shrank", len(paths))
	}

	type key struct {
		rom   string
		start uint16
		bank  int
	}
	// allProven[k] stays true only while every context for k is bounded.
	allProven := map[key]bool{}
	var byRom = map[string][2]int{} // rom -> {proven, total}

	for _, p := range paths {
		rep, err := Prove(p, 76)
		if err != nil {
			t.Fatalf("prove %s: %v", p, err)
		}
		for _, r := range rep.Lines {
			k := key{p, r.Start, r.Bank}
			if _, seen := allProven[k]; !seen {
				allProven[k] = true
			}
		}
		for _, r := range rep.Unbounded {
			allProven[key{p, r.Start, r.Bank}] = false
		}
	}
	if len(allProven) == 0 {
		t.Fatal("no regions at all; the corpus produced nothing to measure")
	}

	proven := 0
	for k, ok := range allProven {
		e := byRom[k.rom]
		e[1]++
		if ok {
			proven++
			e[0]++
		}
		byRom[k.rom] = e
	}
	got := float64(proven) / float64(len(allProven))
	t.Logf("prover coverage: %d/%d addresses proven in every call context = %.1f%%",
		proven, len(allProven), 100*got)

	// A per-cartridge line, so a regression says WHERE rather than only how much.
	for rom, e := range byRom {
		if e[1] == 0 {
			continue
		}
		t.Logf("  %-44s %3d/%3d  %5.1f%%", trimRom(rom), e[0], e[1], 100*float64(e[0])/float64(e[1]))
	}

	if got < coverageFloor {
		t.Errorf("coverage %.1f%% is below the floor of %.1f%% — a builder asking whether a "+
			"scanline fits now gets a refusal on addresses that used to answer",
			100*got, 100*coverageFloor)
	}
	// A floor that the code has drifted far above is a floor that stopped measuring
	// anything. This is not a failure of the prover, it is a reminder to re-pin.
	if got > coverageFloor+0.05 {
		t.Errorf("coverage %.1f%% is more than 5 points above the floor of %.1f%% — raise "+
			"coverageFloor so it keeps catching regressions", 100*got, 100*coverageFloor)
	}
}

func trimRom(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' {
			return p[i+1:]
		}
	}
	return p
}
