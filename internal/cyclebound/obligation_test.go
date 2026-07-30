package cyclebound

import (
	"path/filepath"
	"testing"
)

// A conditional bound is only useful if its number is exact. "At most 7
// iterations" has to mean that 7 fits and 8 does not — a bound that is merely
// safe would send an author trimming work that was never over budget, and one
// that is merely optimistic is worse than saying nothing.
//
// Both edges are checked, so an off-by-one in the search cannot pass.
func TestObligationBoundIsTight(t *testing.T) {
	files, _ := filepath.Glob("../../roms/techniques/*.asm")
	checked := 0
	for _, asm := range files {
		rep, err := Prove(asm, DefaultBudget)
		if err != nil {
			continue
		}
		for _, reg := range rep.Unbounded {
			ob := reg.Conditional
			if ob == nil {
				continue
			}
			checked++
			if ob.MaxIterations == 0 {
				// The region overruns even at one iteration. Its own worst is then
				// meaningless, but the claim must still not contradict itself.
				if ob.WorstAtMax != 0 {
					t.Errorf("%s region %s: MaxIterations 0 but WorstAtMax %d",
						filepath.Base(asm), reg.StartLoc, ob.WorstAtMax)
				}
				continue
			}
			if ob.WorstAtMax > ob.Budget {
				t.Errorf("%s region %s: claims %d iterations fit a %d-cycle budget, but the worst "+
					"case there is %d", filepath.Base(asm), reg.StartLoc, ob.MaxIterations,
					ob.Budget, ob.WorstAtMax)
			}
			// Tightness: one more iteration must NOT fit. Re-derive rather than
			// trusting the search that produced the number.
			if got, ok := worstAtTrip(asm, reg.Start, ob.MaxIterations+1); ok && got <= ob.Budget {
				t.Errorf("%s region %s: claims a maximum of %d iterations, but %d also fits "+
					"(%d <= %d) — the bound is not tight",
					filepath.Base(asm), reg.StartLoc, ob.MaxIterations, ob.MaxIterations+1, got, ob.Budget)
			}
			// And the claimed count must itself fit, re-derived the same way.
			if got, ok := worstAtTrip(asm, reg.Start, ob.MaxIterations); ok && got != ob.WorstAtMax {
				t.Errorf("%s region %s: re-deriving at %d iterations gives %d, report says %d",
					filepath.Base(asm), reg.StartLoc, ob.MaxIterations, got, ob.WorstAtMax)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no conditional bound was checked — the test proves nothing")
	}
	t.Logf("checked %d conditional bounds for tightness", checked)
}

// worstAtTrip re-solves one region with the pending loop pinned to exactly n
// iterations, independently of the search that produced the reported bound.
func worstAtTrip(asmPath string, regionStart uint16, n int) (int, bool) {
	p, instrs, entries, sm, err := loadProgram(asmPath)
	if err != nil {
		return 0, false
	}
	states, converged := computeStates(instrs, entries, p.byteAtBank, switchModel{}, nil)
	if !converged {
		return 0, false
	}
	start, ok := instrs[site{0, regionStart}]
	if !ok {
		return 0, false
	}
	s := &solver{
		nodes: map[site]Instr{}, sinks: map[site]bool{}, folds: map[site]loopInfo{},
		memo: map[lkey]result{}, state: map[lkey]int{}, absStates: states, sm: sm,
	}
	if msg := s.collectRegion(instrs, start); msg != "" {
		return 0, false
	}
	if msg := s.foldLoops(); msg == "" || s.pending == nil {
		return 0, false // this region's loop was bounded, or it failed for another reason
	}
	pl := s.pending
	s.folds = map[site]loopInfo{pl.header: {cost: pl.costAt(n), minCost: pl.costAt(1), exit: pl.exit, n: n}}
	s.memo = map[lkey]result{}
	s.state = map[lkey]int{}
	r := s.longest(start.nextSite(), ctx{})
	if s.cyclic || s.unbounded {
		return 0, false
	}
	return r.cyc, true
}

// A conditional bound is an obligation, not a proof. Reporting one must never
// make a region count as proven — the whole value of the harness rests on that
// line staying where it is.
func TestObligationDoesNotCertify(t *testing.T) {
	files, _ := filepath.Glob("../../roms/techniques/*.asm")
	withObligation := 0
	for _, asm := range files {
		rep, err := Prove(asm, DefaultBudget)
		if err != nil {
			continue
		}
		has := false
		for _, reg := range rep.Unbounded {
			if reg.Conditional != nil {
				has = true
				if reg.Bounded {
					t.Errorf("%s region %s: carries a conditional bound but is marked Bounded",
						filepath.Base(asm), reg.StartLoc)
				}
			}
		}
		if has {
			withObligation++
			if rep.Certified {
				t.Errorf("%s: certified while a region only has a CONDITIONAL bound", filepath.Base(asm))
			}
		}
	}
	if withObligation == 0 {
		t.Fatal("no ROM produced a conditional bound — the test proves nothing")
	}
	t.Logf("%d ROMs carry at least one conditional bound; none of them certify", withObligation)
}
