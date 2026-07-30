// Package oracle abstracts an independent witness of a ROM's behavior: run it
// from power-on for N frames and report its RIOT RAM ($80-$FF). Different oracles
// — the embedded Gopher2600, Stella, MAME (VV-6), perfect6502 (VV-7) — can then be
// cross-checked or majority-voted, which surfaces "all software agrees but the
// hardware-grade member dissents" = the verification suite's reason to exist.
// Extracted from cmd/stellacheck so MAME / vote can reuse one interface.
package oracle

import "sort"

// RAMDump is the 128-byte RIOT RAM ($80-$FF) after a power-on run.
type RAMDump [128]byte

// Oracle is one independent emulator/model that can run a ROM and report its RAM.
type Oracle interface {
	Name() string
	DumpRAM(romPath string, frames int) (RAMDump, error)
}

// Diff returns the RAM offsets (0..127, i.e. addresses $80+i) where a and b differ.
func Diff(a, b RAMDump) []int {
	var d []int
	for i := 0; i < 128; i++ {
		if a[i] != b[i] {
			d = append(d, i)
		}
	}
	return d
}

// ClassifyDiff separates a real disagreement between two oracles from an artefact
// of WHEN each one reads RAM.
//
// "Run N frames and dump RAM" does not name a moment. Measured on
// roms/litmus/litmus_framephase.bin, which bumps a different counter at three points
// in the frame: at frames=10 the embedded Gopher2600 reports $80=10 $81=9 $82=9 —
// it stops just after VSYNC, at the program's own frame boundary — while MAME
// reports $80=10 $81=10 $82=9, i.e. its frame notifier fires after the midpoint of
// the visible field. So for any byte a game updates between those two moments the
// two oracles disagree by one, every time, on a ROM where nothing is wrong.
//
// refN and refNext are the REFERENCE oracle sampled at N and N+1 frames; they bound
// what a sub-frame phase difference can account for. A byte of `other` that differs
// from refN but equals refNext is reported as phase, not as a divergence.
//
// Phase bytes are RETURNED, not swallowed. A one-off in a frame counter is exactly
// what a real engine bug looks like too, so the caller has to see the count and
// decide; hiding them would turn this into a check that cannot fail.
func ClassifyDiff(refN, refNext, other RAMDump) (real, phase []int) {
	for i := 0; i < 128; i++ {
		if other[i] == refN[i] {
			continue
		}
		if other[i] == refNext[i] {
			phase = append(phase, i)
			continue
		}
		real = append(real, i)
	}
	return real, phase
}

// Vote runs every oracle and returns the majority RAM dump plus the names of any
// oracles that dissent from it. A tie (no strict majority) is reported via ok=false.
// This is the fusion point for VV-6/7: a lone hardware-grade dissent is visible.
func Vote(oracles []Oracle, romPath string, frames int) (majority RAMDump, dissenters []string, ok bool, err error) {
	type entry struct {
		dump RAMDump
		name string
	}
	var entries []entry
	counts := map[RAMDump]int{}
	for _, o := range oracles {
		d, e := o.DumpRAM(romPath, frames)
		if e != nil {
			return RAMDump{}, nil, false, e
		}
		entries = append(entries, entry{d, o.Name()})
		counts[d]++
	}
	best, bestN := RAMDump{}, 0
	for d, n := range counts {
		if n > bestN {
			best, bestN = d, n
		}
	}
	if bestN*2 <= len(oracles) { // no strict majority
		return best, nil, false, nil
	}
	for _, e := range entries {
		if e.dump != best {
			dissenters = append(dissenters, e.name)
		}
	}
	sort.Strings(dissenters)
	return best, dissenters, true, nil
}
