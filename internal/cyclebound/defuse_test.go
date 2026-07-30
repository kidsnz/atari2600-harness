package cyclebound

import (
	"fmt"
	"strings"
	"testing"

	"github.com/kidsnz/atari2600-harness/internal/build"
	"github.com/kidsnz/atari2600-harness/internal/emu"
)

func defUseOf(t *testing.T, asm string) *DefUseReport {
	t.Helper()
	r, err := DefUse(asm, DefaultBudget)
	if err != nil {
		t.Skipf("DefUse(%s): %v", asm, err)
	}
	return r
}

func addrFacts(r *DefUseReport, addr string) []AddrFacts {
	var out []AddrFacts
	for _, reg := range r.Regions {
		for _, f := range reg.Addrs {
			if f.Addr == addr {
				out = append(out, f)
			}
		}
	}
	return out
}

// The load-bearing claim, and the reason this analysis is worth trusting: a
// may-write set that does not CONTAIN what the machine actually writes is not
// an over-approximation, it is wrong. Every write a real execution performs must
// appear in it.
//
// Checking it this way — static claim versus the emulator's own behaviour — is
// what stops the analysis from being graded by the same reasoning that produced
// it. The check is only meaningful when nothing was left unbounded, since an
// unbounded writer trivially covers everything; the report says so, and so does
// this test.
func TestDefUseMayWriteContainsObservedWrites(t *testing.T) {
	realChecks := 0
	for _, asm := range []string{
		"../../roms/litmus/motion_glide.asm",
		"../../roms/litmus/litmus_indexed_tia.asm",
		"../../roms/techniques/vertical_pos.asm",
	} {
		t.Run(shortName(asm), func(t *testing.T) {
			r := defUseOf(t, asm)
			if !r.Converged {
				t.Fatalf("%s: the fixpoint did not converge, so the may-set below is not a claim — "+
					"reported as a failure rather than skipped, because a soundness sweep that "+
					"silently covers nothing is worse than one that is absent", asm)
			}
			may, hasUnbounded := r.mayWriteAddrs()

			observedByPC, err := observedWritesByPC(asm, 8)
			observed := map[uint16]bool{}
			for _, addrs := range observedByPC {
				for a := range addrs {
					observed[a] = true
				}
			}
			if err != nil {
				t.Skipf("could not run %s: %v", asm, err)
			}
			if len(observed) == 0 {
				t.Fatal("premise broken: the ROM wrote nothing in 8 frames")
			}

			// Per-INSTRUCTION containment. Checking the whole-program may-set is
			// nearly vacuous on any 2600 ROM, because the RAM clear at reset makes it
			// "all of zero page" — true, and an answer to no question. The claim worth
			// grading is the sharp one: the address THIS instruction reached has to be
			// in the set THIS instruction was predicted to reach.
			sharp, sharpOK := 0, 0
			for pc, addrs := range observedByPC {
				acc, known := r.Writes[hexAddr(pc)]
				if !known {
					// A DIFFERENT failure from an unsound may-set, and worth keeping apart:
					// the analysis did not mis-predict this instruction, it never saw it.
					// The CFG is recovered by following flow from the vectors, so code
					// reached by bank switching or by an RTS-computed dispatch is invisible
					// to it. Reported as a coverage gap, not as unsoundness.
					t.Errorf("PC $%04X wrote memory but was never decoded — the CFG did not "+
						"reach it (bank switch or computed dispatch)", pc)
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
					sharp++
					if in[a] {
						sharpOK++
						continue
					}
					t.Errorf("PC $%04X wrote $%04X, which is NOT in its predicted target set "+
						"(%d addrs, wide=%v) — a may-set that misses reality is unsound",
						pc, a, len(acc.Addrs), acc.Wide)
				}
			}
			if sharp > 0 {
				t.Logf("per-instruction containment: %d/%d observed (pc,addr) pairs inside their "+
					"predicted sets", sharpOK, sharp)
			}

			var missing []uint16
			for a := range observed {
				if !may[a] {
					missing = append(missing, a)
				}
			}
			if !hasUnbounded {
				realChecks++
				t.Logf("checked %d observed writes against %d may-write addresses",
					len(observed), len(may))
			}
			if len(missing) > 0 {
				if hasUnbounded {
					t.Logf("%d observed writes are outside the bounded may-set, but the report "+
						"declares %d unbounded writer(s), so the set was never complete: %v",
						len(missing), len(r.UnboundedWriters), hexAddrs(missing))
					return
				}
				t.Errorf("the static may-write set MISSES addresses the machine actually wrote: %v\n"+
					"a may-set that does not contain reality is unsound, not imprecise",
					hexAddrs(missing))
			}
		})
	}
	// A pass in which every ROM took the "unbounded writer, cannot judge" escape
	// would be a pass that tested nothing — the same vacuous green this project
	// refuses elsewhere. At least one containment check has to have been real.
	if realChecks == 0 {
		t.Error("every ROM escaped via an unbounded writer; nothing was actually checked")
	}
}

// A must-analysis has to be timid in the right direction. Every address it
// claims is definitely written must in fact be written on every path, so the
// report may never mark a read of it as read-before-write when a write really
// does precede it.
func TestDefUseNoFalseReadBeforeWriteOnInitialisedCell(t *testing.T) {
	r := defUseOf(t, "../../roms/litmus/motion_glide.asm")
	if !r.Converged {
		t.Fatal("the fixpoint did not converge; every assertion below rests on the states it " +
			"produces, so this is a broken premise, not a reason to pass quietly")
	}
	// The ROM clears RAM at reset and then reads $80 (posY) after storing to it.
	// Whatever the analysis says about $80, it must not be nonsense: an address
	// with no reader cannot have a read-before-write.
	for _, reg := range r.Regions {
		for _, f := range reg.Addrs {
			if len(f.ReadBeforeWrite) > 0 && len(f.Readers) == 0 {
				t.Errorf("region %s: %s is flagged read-before-write at %v but has no readers",
					reg.Start, f.Addr, f.ReadBeforeWrite)
			}
			for _, rbw := range f.ReadBeforeWrite {
				if !containsStr(f.Readers, rbw) {
					t.Errorf("region %s: %s read-before-write at %s is not among its readers %v",
						reg.Start, f.Addr, rbw, f.Readers)
				}
			}
		}
	}
}

// An indexed store with a KNOWN index must resolve to the one register it
// reaches, not to its base — the same fact the emulator-side litmus pins down.
// If the static and dynamic sides disagreed here, every cross-check between them
// would be meaningless.
func TestDefUseResolvesKnownIndexedStore(t *testing.T) {
	r := defUseOf(t, "../../roms/litmus/litmus_indexed_tia.asm")
	if !r.Converged {
		t.Fatal("the fixpoint did not converge; every assertion below rests on the states it " +
			"produces, so this is a broken premise, not a reason to pass quietly")
	}
	may, _ := r.mayWriteAddrs()
	if !may[0x0009] {
		t.Errorf("COLUBK ($0009) is not in the may-write set; `ldx #3 / sta COLUP0,x` reaches it.\n"+
			"may-write = %v", r.MayWrite)
	}
}

// Every region the prover analyses must also appear here, because both walk the
// same decoded program. A drift between them would mean two decoders, and the
// disagreement would be invisible.
func TestDefUseRegionsMatchProver(t *testing.T) {
	const asm = "../../roms/techniques/vertical_pos.asm"
	r := defUseOf(t, asm)
	p, err := Prove(asm, DefaultBudget)
	if err != nil {
		t.Skipf("Prove: %v", err)
	}
	if len(r.Regions) != p.Regions {
		t.Errorf("def-use found %d regions, the prover found %d — they must decode the same program",
			len(r.Regions), p.Regions)
	}
}

func containsStr(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

func shortName(p string) string {
	i := strings.LastIndex(p, "/")
	return p[i+1:]
}

// observedWritesByPC runs the ROM and records, per writing instruction, every
// effective address it reached. This is the ground truth the static claim is
// graded against — the emulator's behaviour, not another analysis. Keeping the
// PC is what makes the check sharp: without it the comparison collapses to
// "the program wrote somewhere in memory", which nothing can fail.
func observedWritesByPC(asmPath string, frames int) (map[uint16]map[uint16]bool, error) {
	bin := build.BinPathFor(asmPath)
	if out, err := build.Assemble(asmPath, bin); err != nil {
		return nil, fmt.Errorf("assemble: %s", out)
	}
	e, err := emu.New("NTSC")
	if err != nil {
		return nil, err
	}
	if err := e.LoadROM(bin); err != nil {
		return nil, err
	}
	out := map[uint16]map[uint16]bool{}
	start := e.Coords().Frame
	for i := 0; i < 4_000_000 && e.Coords().Frame-start < frames; i++ {
		pc := e.VCS.CPU.PC.Value()
		if err := e.StepInstruction(); err != nil {
			return nil, err
		}
		if a, ok := e.LastMemWrite(); ok {
			if out[pc] == nil {
				out[pc] = map[uint16]bool{}
			}
			out[pc][a] = true
		}
	}
	return out, nil
}
