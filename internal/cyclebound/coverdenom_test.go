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
func TestStaticBranchSetContainsObserved(t *testing.T) {
	files, err := filepath.Glob("../../roms/techniques/*.asm")
	if err != nil || len(files) == 0 {
		t.Skip("technique corpus unavailable")
	}
	roms, gaps := 0, 0
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
		if !banked {
			// Not bank-switched, so the decoder should have reached this code. Report
			// it rather than letting it quietly shrink the denominator.
			addrs := make([]uint16, len(missing))
			for i, s := range missing {
				addrs[i] = s.Addr
			}
			t.Logf("%s: %d executed branches were never decoded (%v) — the denominator is "+
				"too small here and coverage is an over-estimate",
				filepath.Base(asm), len(missing), hexAddrs(addrs))
		}
	}
	if roms == 0 {
		t.Fatal("no ROM was measured — the test proves nothing")
	}
	t.Logf("%d ROMs measured, %d with executed-but-undecoded branches", roms, gaps)
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
