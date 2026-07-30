package cyclebound

import (
	"testing"
	"time"

	"github.com/kidsnz/atari2600-harness/internal/build"
	"github.com/kidsnz/atari2600-harness/internal/emu"
)

// The whole point of modelling cross-bank flow, graded against the machine rather than
// against the prover's own arithmetic.
//
// Before this existed, every one of these regions was REFUSED ("region can switch
// banks: the access at $FF00 reaches BANK1 ($1FF9)"). A refusal is sound but useless:
// the region is where all the cross-bank work happens, so the cartridge could never be
// certified and the author was told nothing about the cost of the crossing.
//
// The gate is proven >= measured, which is the direction that must never be violated.
// Three of these kernels are deterministic — the machine walks the single path the
// prover costs — so the stronger EQUALITY is asserted for them and a gap in either
// direction is reported. banked_game is excluded from the equality check because its
// level loader runs once every 120 frames, so a 6-frame run does not take the worst
// path; only proven >= measured is meaningful there.
func TestCrossBankRegionsBeatTheMachine(t *testing.T) {
	for _, c := range []struct {
		asm      string
		exact    bool // deterministic kernel: proven must EQUAL measured
		wantOver bool // the crossing legitimately spends more than one scanline
	}{
		{asm: "../../roms/litmus/litmus_bank.asm", exact: true},
		{asm: "../../roms/litmus/litmus_bank_f6.asm", exact: true},
		{asm: "../../roms/litmus/litmus_bank_f4.asm", exact: true, wantOver: true},
		{asm: "../../roms/litmus/litmus_bank_shared_addr.asm", exact: true},
		{asm: "../../roms/litmus/litmus_bank_bound.asm", exact: true},
		{asm: "../../roms/techniques/banked_game.asm"},
	} {
		rep := mustProve(t, c.asm, 76)
		bin := build.BinPathFor(c.asm)
		e, err := emu.New("NTSC")
		if err != nil {
			t.Skip("emulator unavailable")
		}
		if err := e.LoadROM(bin); err != nil {
			t.Fatal(err)
		}
		rows, _, err := e.ProfileLineWorst(6, nil)
		if err != nil {
			t.Fatal(err)
		}
		measured := map[site]int{}
		for _, r := range rows {
			measured[site{r.Bank, r.StrobePC}] = r.WorstCycles
		}

		crossings, checked := 0, 0
		for _, r := range append(append([]Region(nil), rep.Lines...), rep.BlankLines...) {
			if r.SwitchEdges > 0 {
				crossings++
			}
			m, ok := measured[site{r.Bank, r.Start}]
			if !ok || !r.Bounded {
				continue
			}
			checked++
			if r.Worst < m {
				t.Errorf("%s: bank %d $%04X proven %dcy, MEASURED %dcy — proven below measured is the "+
					"one direction this package forbids", c.asm, r.Bank, r.Start, r.Worst, m)
			}
			if c.exact && r.Worst != m {
				t.Errorf("%s: bank %d $%04X proven %dcy, measured %dcy — this kernel is deterministic, "+
					"so a gap means the path costed is not the path run",
					c.asm, r.Bank, r.Start, r.Worst, m)
			}
		}
		if crossings == 0 {
			t.Errorf("%s: no region recorded a cross-bank edge, so nothing here tests the crossing",
				c.asm)
		}
		if checked == 0 {
			t.Errorf("%s: no bounded region had a measurement to be graded against", c.asm)
		}
		// litmus_bank_f4's crossing chain genuinely spends more than one scanline (the
		// source compensates with `ldx #29` instead of 30). It must be reported as a
		// stated over-budget number, not as a refusal — and not silently greened by an
		// `@lines 2` that srcmap cannot read on a banked image.
		if c.wantOver {
			over := false
			for _, v := range rep.Violations {
				if v.SwitchEdges > 0 {
					over = true
				}
			}
			if !over {
				t.Errorf("%s: its crossing chain spends more than one scanline on the machine, so it must "+
					"appear as a stated budget VIOLATION at 76 rather than pass or be refused", c.asm)
			}
			if rep.SourceAnnotations == "" {
				t.Errorf("%s: it is over budget only because @lines cannot be read on a banked image; "+
					"the report must say the annotations were unavailable rather than leaving that to "+
					"be inferred", c.asm)
			}
		}
		t.Logf("%s: %d region(s) graded against the machine, %d with cross-bank edges, %d modelled edge(s) "+
			"total, certified=%v", c.asm, checked, crossings, rep.ModelledSwitchEdges, rep.Certified)
	}
}

// The merged program is several times the size of any single bank, and the value-domain
// fixpoint runs over it TWICE. A capped run leaves under-approximated states, which
// Converged=false must keep blocking — but a capped run also buys nothing, so the
// iteration cap and the wall time are measured here rather than assumed.
func TestMergedFixpointConvergesAndIsAffordable(t *testing.T) {
	for _, asm := range []string{
		"../../roms/litmus/litmus_bank.asm",
		"../../roms/litmus/litmus_bank_f6.asm",
		"../../roms/litmus/litmus_bank_f4.asm",
		"../../roms/techniques/banked_game.asm",
	} {
		t0 := time.Now()
		rep := mustProve(t, asm, 76)
		el := time.Since(t0)
		if !rep.Converged {
			t.Errorf("%s: the merged fixpoint hit its iteration cap. Nothing derived from it may be "+
				"certified, and a capped run answers the caller's question with a refusal for an "+
				"arithmetic reason nobody asked about", asm)
		}
		sites := 0
		for _, c := range rep.BankCoverage {
			sites += c.Instructions
		}
		if el > 30*time.Second {
			t.Errorf("%s: Prove took %v over %d merged sites — too slow for an MCP tool call", asm, el, sites)
		}
		t.Logf("%s: %d merged sites across %d banks, Prove in %v, converged=%v (cap %d)",
			asm, sites, rep.Banks, el.Round(time.Millisecond), rep.Converged, iterCap(sites))
	}
}

// determineBound's `lda #imm` address proxy on the BCS/BCC divide path is the same
// shape of heuristic that under-approximated by 40x on the dex/dey path (SD-9). It is
// kept, gated to one bank — and counted, so "nothing reaches it" stays a measurement
// instead of a belief. If a kernel ever starts depending on it, this test says so.
func TestDivideLoopAddressProxyIsUnused(t *testing.T) {
	before := proxyFallbackHits
	for _, asm := range []string{
		"../../roms/litmus/cb_divloop.asm",
		"../../roms/litmus/cb_blank_amax.asm",
		"../../roms/litmus/cb_blank_noamax.asm",
		"../../roms/litmus/litmus_bound_proxy.asm",
		"../../roms/techniques/shared_setxpos.asm",
		"../../roms/techniques/bitmap48.asm",
		"../../roms/litmus/litmus_bank_f4.asm",
		"../../roms/litmus/litmus_bank_bound.asm",
	} {
		mustProve(t, asm, 76)
	}
	if n := proxyFallbackHits - before; n != 0 {
		t.Errorf("the divide-loop address proxy fired %d time(s) across this set. A bound taken from "+
			"'the closest immediate below the header' is only valid while address order matches "+
			"execution order; if a corpus kernel now needs it, give that kernel an `@amax` and delete "+
			"the proxy rather than leaving an SD-9-shaped heuristic live", n)
	}
}
