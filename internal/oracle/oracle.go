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
