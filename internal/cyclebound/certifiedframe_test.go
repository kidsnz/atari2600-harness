package cyclebound

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/kidsnz/atari2600-harness/internal/emu"
)

// frameLines runs a ROM past its warmup and returns the scanline count, or 0 if it never settles.
func frameLines(t *testing.T, bin string) int {
	t.Helper()
	e, err := emu.New("NTSC")
	if err != nil {
		return 0
	}
	if err := e.LoadROM(bin); err != nil {
		return 0
	}
	n := 0
	for i := 0; i < 12; i++ {
		m, err := e.StepFrame()
		if err != nil {
			return 0
		}
		if i < 4 {
			continue
		}
		if n == 0 {
			n = m
		} else if m != n {
			return 0 // not stable; a moving line count is a different question
		}
	}
	return n
}

// TestNoTechniqueIsCertifiedAndBroken bounds the hole that `twolineregion_test.go` documents.
//
// `prove_line_budget` gives a region beginning with `sta WSYNC` a two-line budget of 152, so a loop
// that overruns its *inner* line by a few cycles is certified anyway — demonstrated by
// `roms/litmus/litmus_store7_overrun.asm`, which certifies and renders **269** scanlines.
//
// That is a property of the instrument. The question this test answers is whether anything in the
// technique catalogue has actually fallen into it. Measured 2026-09-07: **it has not.** Of 31
// technique ROMs, 15 certify and every one of those renders exactly 262 lines; the other 16 do not
// certify, which is a separate matter and not this test's business.
//
// The zero is guarded by the overrun litmus: swept alongside the catalogue it must be counted, or
// this test is a sweep that cannot find anything. A clean sweep and a broken sweep look identical
// without that.
func TestNoTechniqueIsCertifiedAndBroken(t *testing.T) {
	sweep := func(patterns ...string) (certifiedBroken []string, certified int) {
		var asms []string
		for _, g := range patterns {
			m, _ := filepath.Glob(g)
			asms = append(asms, m...)
		}
		for _, a := range asms {
			r, err := Prove(a, 76)
			if err != nil || !r.Certified {
				continue
			}
			n := frameLines(t, strings.TrimSuffix(a, ".asm")+".bin")
			if n == 0 {
				continue // never settled; not a claim about the budget
			}
			certified++
			if n != 262 {
				certifiedBroken = append(certifiedBroken, filepath.Base(a))
			}
		}
		return
	}

	broken, certified := sweep("../../roms/techniques/*.asm")
	if certified < 10 {
		t.Fatalf("only %d technique ROMs certified with a settled frame — the walk found too "+
			"little for the zero below to mean anything", certified)
	}
	if len(broken) > 0 {
		t.Errorf("these technique ROMs pass prove_line_budget and do NOT render 262 scanlines: %v.\n"+
			"That is the shape litmus_store7_overrun.asm exists to illustrate: a WSYNC-started "+
			"region gets a two-line budget and hides a per-line overrun. Check the frame length, "+
			"not just the budget.", broken)
	}

	// The control: the same sweep, with the known-broken ROM in it, must find exactly that one.
	withBait, _ := sweep("../../roms/techniques/*.asm", "../../roms/litmus/litmus_store7_overrun.asm")
	if len(withBait) != 1 || withBait[0] != "litmus_store7_overrun.asm" {
		t.Errorf("the sweep found %v when the deliberately broken ROM was added, want exactly "+
			"[litmus_store7_overrun.asm]. Without this the clean result above could mean the "+
			"sweep looks at nothing", withBait)
	}
}
