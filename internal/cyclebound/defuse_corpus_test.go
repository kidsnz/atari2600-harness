package cyclebound

import (
	"path/filepath"
	"testing"
)

// The soundness claim, swept over the whole technique corpus rather than a
// hand-picked ROM or two. Two distinct things are counted, because conflating
// them would hide both:
//
//   - CONTAINMENT: every address an instruction actually reached must be inside
//     the set that instruction was predicted to reach. A miss here is unsoundness
//     and fails the build.
//   - REACH: an instruction that wrote memory but was never decoded at all. That
//     is not a wrong prediction, it is a hole in the control-flow graph — the CFG
//     is recovered by following flow from the reset/NMI/IRQ vectors, so code
//     entered through a bank switch or an RTS-computed dispatch is invisible to
//     it. Measured, not assumed: the sweep names which ROMs have it.
func TestDefUseCorpusSoundness(t *testing.T) {
	files, err := filepath.Glob("../../roms/techniques/*.asm")
	if err != nil || len(files) == 0 {
		t.Skip("technique corpus unavailable")
	}
	pairs, contained, undecoded := 0, 0, 0
	var gaps []string

	for _, asm := range files {
		r, err := DefUse(asm, DefaultBudget)
		if err != nil || !r.Converged {
			continue
		}
		byPC, err := observedWritesByPC(asm, 6)
		if err != nil {
			continue
		}
		romGap := 0
		for pc, addrs := range byPC {
			acc, known := r.Writes[hexAddr(pc)]
			if !known {
				undecoded += len(addrs)
				romGap += len(addrs)
				continue
			}
			if acc.Unbounded {
				continue
			}
			in := map[uint16]bool{}
			for _, x := range acc.Addrs {
				in[x] = true
			}
			for a := range addrs {
				pairs++
				if in[a] {
					contained++
					continue
				}
				t.Errorf("%s: PC $%04X wrote $%04X, outside its predicted set (%d addrs, wide=%v)",
					filepath.Base(asm), pc, a, len(acc.Addrs), acc.Wide)
			}
		}
		if romGap > 0 {
			gaps = append(gaps, filepath.Base(asm))
		}
	}

	if pairs == 0 {
		t.Fatal("no (pc,addr) pairs were checked — the sweep proved nothing")
	}
	t.Logf("containment: %d/%d observed (pc,addr) pairs inside their predicted sets", contained, pairs)
	if undecoded > 0 {
		t.Logf("CFG reach gap: %d writes came from instructions the decoder never reached, in %v "+
			"— flow from the vectors cannot follow a bank switch or an RTS-computed dispatch",
			undecoded, gaps)
	}
}
