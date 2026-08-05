package cyclebound

import (
	"path/filepath"
	"testing"

	"github.com/kidsnz/atari2600-harness/internal/build"
	"github.com/kidsnz/atari2600-harness/internal/emu"
)

// A coverage denominator has to be the branches the program HAS, not the ones a
// run happened to execute. With the observed denominator, a branch that is never
// reached leaves the arithmetic entirely, so the percentage rises as the test
// gets worse — measured on this repo's own kernels, divtable reported 100% edge
// coverage with 12 of its 17 branches never executed.
//
// The static set must CONTAIN the observed set. A branch the machine executed
// that the decoder never reached means the denominator is too small and the
// figure is an over-estimate; that is a real condition (bank switching, computed
// dispatch) and must be reported as such rather than hidden inside a percentage.
//
// THIS TEST USED TO PRINT ITS OWN VERDICT. It found the ROMs whose denominator is
// too small, t.Logf'd their names and addresses, and then passed — the only
// t.Fatal in the function was "no ROM was measured". Every technique kernel could
// have drifted into computed dispatch and the tick would still be green, which is
// the same defect this file exists to describe one level down: a figure that
// silently stops covering what it claims to cover.
//
// The offenders are now NAMED rather than counted. Measured over the 31-kernel
// technique corpus, 2026-08-04: exactly one, `rts_dispatch`, with 6 executed
// branches never decoded ($F081 $F0A2 $F0E3 $F0E8 $F108 $F118) — a kernel that
// pushes a target and RTSes to it, so a decoder that follows flow cannot reach the
// code and the gap is a property of the technique, not a defect. A NEW name means
// a kernel started doing something the decoder cannot see. `rts_dispatch`
// disappearing from the list means the decoder got better and this comment is
// stale; both must be a failing test rather than a line in a log.
func TestStaticBranchSetContainsObserved(t *testing.T) {
	// Kernels whose control flow is computed at run time, so a flow-following
	// decoder provably cannot reach every branch. The name is the evidence, not
	// an exemption: each one has to earn its place with a stated reason.
	knownComputedDispatch := map[string]string{
		"rts_dispatch.asm": "pushes a return address and RTSes to it (docs/techniques/rts-dispatch.md)",
	}
	files, err := filepath.Glob("../../roms/techniques/*.asm")
	if err != nil || len(files) == 0 {
		t.Skip("technique corpus unavailable")
	}
	roms, gaps := 0, 0
	seenGap := map[string]bool{}
	for _, asm := range files {
		bin := build.BinPathFor(asm)
		if out, err := build.Assemble(asm, bin); err != nil {
			t.Logf("assemble %s: %s", asm, out)
			continue
		}
		static, banked, err := StaticBranchSites(bin)
		if err != nil {
			continue
		}
		set := map[CodeSite]bool{}
		for _, s := range static {
			set[s] = true
		}

		e, err := emu.New("NTSC")
		if err != nil {
			continue
		}
		if err := e.LoadROM(bin); err != nil {
			continue
		}
		if err := e.RunFrames(2); err != nil {
			continue
		}
		e.EnableCoverage()
		if err := e.RunFrames(30); err != nil {
			continue
		}
		roms++
		var missing []CodeSite
		for _, s := range e.Coverage().BranchSites() {
			if !set[(CodeSite{Bank: s[0], Addr: uint16(s[1])})] {
				missing = append(missing, CodeSite{Bank: s[0], Addr: uint16(s[1])})
			}
		}
		if len(missing) == 0 {
			continue
		}
		gaps++
		if banked {
			continue // flow between banks is not modelled; that refusal is stated elsewhere
		}
		// Not bank-switched, so the decoder should have reached this code.
		base := filepath.Base(asm)
		seenGap[base] = true
		addrs := make([]uint16, len(missing))
		for i, s := range missing {
			addrs[i] = s.Addr
		}
		if why, ok := knownComputedDispatch[base]; ok {
			t.Logf("%s: %d executed branches never decoded (%v) — expected: %s",
				base, len(missing), hexAddrs(addrs), why)
			continue
		}
		t.Errorf("%s: %d executed branches were never decoded (%v) — the denominator is too "+
			"small here and its coverage figure is an over-estimate. Either the decoder "+
			"stopped reaching code it used to reach, or this kernel started computing its "+
			"control flow and belongs in knownComputedDispatch WITH the reason",
			base, len(missing), hexAddrs(addrs))
	}
	if roms == 0 {
		t.Fatal("no ROM was measured — the test proves nothing")
	}
	// The list is a ratchet in both directions: an entry that no longer has a gap is
	// a decoder that improved, and leaving the name behind would exempt a future
	// regression on that ROM.
	for name := range knownComputedDispatch {
		if !seenGap[name] {
			t.Errorf("%s is listed as computed-dispatch but its executed branches are now ALL "+
				"decoded — the decoder reaches it, so the exemption is stale and must go",
				name)
		}
	}
	t.Logf("%d ROMs measured, %d with executed-but-undecoded branches (%d expected by name)",
		roms, gaps, len(knownComputedDispatch))
}

// The honest figure can never exceed the flattering one on a ROM the decoder
// fully covers: the denominator only grows. Where it does exceed, the decoder is
// incomplete, and that has to be the stated reason rather than an accident.
func TestStaticCoverageIsNeverHigherThanObserved(t *testing.T) {
	files, _ := filepath.Glob("../../roms/techniques/*.asm")
	checked := 0
	for _, asm := range files {
		bin := build.BinPathFor(asm)
		if _, err := build.Assemble(asm, bin); err != nil {
			continue
		}
		static, banked, err := StaticBranchSites(bin)
		if err != nil || banked || len(static) == 0 {
			continue
		}
		set := map[CodeSite]bool{}
		for _, s := range static {
			set[s] = true
		}
		e, err := emu.New("NTSC")
		if err != nil {
			continue
		}
		if err := e.LoadROM(bin); err != nil {
			continue
		}
		if err := e.RunFrames(2); err != nil {
			continue
		}
		e.EnableCoverage()
		if err := e.RunFrames(30); err != nil {
			continue
		}
		cov := e.Coverage()
		complete := true
		for _, s := range cov.BranchSites() {
			if !set[(CodeSite{Bank: s[0], Addr: uint16(s[1])})] {
				complete = false
			}
		}
		if !complete || cov.BranchCount() == 0 {
			continue
		}
		checked++
		obs := float64(cov.EdgeCount()) / float64(cov.BranchCount()*2)
		stat := float64(cov.EdgeCount()) / float64(len(static)*2)
		if stat > obs+1e-9 {
			t.Errorf("%s: static-denominator coverage %.3f exceeds observed-denominator %.3f on a "+
				"fully decoded ROM — the denominator cannot have shrunk",
				filepath.Base(asm), stat, obs)
		}
	}
	if checked == 0 {
		t.Fatal("no fully decoded ROM was compared")
	}
	t.Logf("static <= observed coverage on %d fully decoded ROMs", checked)
}
