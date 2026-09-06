package cyclebound

import "sort"

// RAMFootprint is what a program's RAM price looks like when it is measured rather than declared.
//
// ★Why it is not simply MayWrite. Every 2600 ROM opens by clearing RAM with an unknown index —
// `ldx #$FF / sta $00,x` — and the may-set correctly reports that as touching all 128 bytes. Run
// against eight technique ROMs, `MayWrite ∩ RAM` is **128 for every one of them**, which is true
// and answers nothing. The report already carries the distinction needed to fix it: `Access.Wide`
// marks an enumerated set that came from an unknown index, "sound, but it carries no information
// about WHICH cell". Excluding those leaves the stores that name their target.
//
// ★★And a byte that is only ever written is not carrying state, so the footprint is the union of
// **precisely written** and **precisely read** RAM addresses. Measured 2026-09-06 across the
// technique corpus: tia_pcm 4, two_line_kernel 5, rpgmap 5, maze 9, sound_driver 9,
// music_driver 10, flicker_multiplex 14, bitmap48 17, score6 17, text12 23.
//
// ★★★EVERY NUMBER HERE IS A LOWER BOUND, and the first version of this comment called them
// prices. Measured across eleven technique ROMs: **all eleven report imprecise accesses**, because
// the RAM-clear loop every 2600 program opens with is itself a wide access. So `Imprecise > 0` is
// universal and does not discriminate — it is not a warning flag, it is the normal state. What a
// caller must not do is read `len(Bytes)` as "this technique uses exactly N bytes".
//
// ★★★★CALIBRATED against a known answer, which is what makes the bound trustworthy rather than
// merely conservative. `roms/260816_transistor/src/rom/meterv-dual.asm` states its own RAM map in
// source: `MUSZP = $80`, `LVL = $8A` (ten bytes), `TMP = $94`, `BAR = $96` with the comment
// "$96..$F9", `BARD = $C8`. That is $80–$F9 = **122 bytes** of the 128, leaving $FA–$FF. This
// function reports **119** — a lower bound three bytes under the author's own map, on a real work
// with a hundred-byte array in it. A bound that lands three short of a known 122 is worth quoting;
// one that landed at 5 would not be.
//
// ★★★★★The signal that does discriminate is **`len(Bytes) == 0` with `Imprecise > 0`**: nothing
// could be pinned at all. `zone_multiplex` is that case — every one of its RAM accesses goes
// through an unknown index, so it prices at 0 with 15 imprecise accesses, for a ROM that plainly
// uses RAM. A confident zero there would read as "this technique uses no RAM", which is false.
type RAMFootprint struct {
	// Bytes is the union of precisely-written and precisely-read RAM addresses, ascending.
	Bytes []uint16

	// Written and Read are the two halves, for a caller that wants to see a write-only cell
	// (a variable nothing consumes) or a read-only one (something else initialises it).
	Written, Read []uint16

	// Imprecise counts accesses whose target could not be pinned to particular cells — wide
	// (unknown index) or unbounded (pointer in RAM). It is non-zero for every ROM in this corpus,
	// because the opening RAM-clear loop is itself one, so Bytes is ALWAYS a lower bound. The
	// case to act on is Bytes == 0 while this is non-zero: nothing could be pinned at all.
	Imprecise int
}

// RAMFootprintOf measures the RAM price of the program at asmPath.
func RAMFootprintOf(asmPath string) (*RAMFootprint, error) {
	rep, err := DefUse(asmPath, 0)
	if err != nil {
		return nil, err
	}
	isRAM := func(a uint16) bool { return a >= 0x80 && a <= 0xFF }
	out := &RAMFootprint{}
	gather := func(m map[string]Access) []uint16 {
		set := map[uint16]bool{}
		for _, acc := range m {
			if acc.Wide || acc.Unbounded {
				out.Imprecise++
				continue
			}
			for _, a := range acc.Addrs {
				if isRAM(a) {
					set[a] = true
				}
			}
		}
		return sortedSet(set)
	}
	out.Written = gather(rep.Writes)
	out.Read = gather(rep.Reads)
	all := map[uint16]bool{}
	for _, a := range out.Written {
		all[a] = true
	}
	for _, a := range out.Read {
		all[a] = true
	}
	out.Bytes = sortedSet(all)
	return out, nil
}

func sortedSet(m map[uint16]bool) []uint16 {
	out := make([]uint16, 0, len(m))
	for a := range m {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
