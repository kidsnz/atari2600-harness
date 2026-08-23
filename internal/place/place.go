// Package place answers the question a 2600 picture always ends up asking, and answers it by
// search rather than by arithmetic in someone's head: given a row of shapes at fixed screen
// positions, can the TIA's movable objects be strobed so that every one of them is drawn -- and
// if not, what is in the way.
//
// WHY IT EXISTS. A shape 12 colour clocks wide is an 8-clock part and a 4-clock part, and where
// each part can go is decided by grids that do not line up:
//
//	a player   lands at x = 3c - 60 and stops at x = 3   (multiples of 3)
//	a missile  lands at x = 3c - 61 and stops at x = 2   (one clock to the LEFT of that grid)
//	the ball   is a missile in this respect, exactly
//
// So a row that must begin at x=2 cannot put a PLAYER there, by a strobe or by any copy of one --
// and yet it can be drawn, by turning the leftmost shape round so its missile takes the left four
// clocks and its player the right eight. Working that out by hand went wrong twice before this
// package existed: the first answer was "impossible", and it was wrong because the search was
// missing one measured fact (that each clamp spans a WINDOW of write cycles, not a single one).
// Every constant below is measured by roms/litmus/litmus_sprite_place.asm and graded by
// internal/emu/spriteplace_test.go, which also asserts these values against the fixture, so a
// drift in the model cannot pass CI.
package place

import (
	"fmt"
	"sort"
)

// The measured placement grid. See docs/techniques/sprite-placement.md.
const (
	PlayerSlope     = 3
	PlayerIntercept = -60
	PlayerFloor     = 3 // a player cannot be strobed left of here
	SolidSlope      = 3
	SolidIntercept  = -61 // missiles AND the ball
	SolidFloor      = 2   // and they stop one clock left of a player
	ClampFirst      = 16  // the earliest write cycle measured to still land on a floor
	LineClocks      = 160 // a copy past this wraps and draws at the left edge, same line
	MinStrobeGap    = 3   // two stores cannot be closer than this
	HeadWidth       = 8   // the part a player draws
	TailWidth       = 4   // the part a missile, the ball, or a second player draws
	ShapeWidth      = HeadWidth + TailWidth
)

// nusizCopies maps a NUSIZ copy code to the offsets it repeats an object at. A missile repeats
// with its OWN player's code, which is why a pair shares one entry here.
var nusizCopies = map[int][]int{
	0x00: {0},
	0x01: {0, 16},
	0x02: {0, 32},
	0x03: {0, 16, 32},
	0x04: {0, 64},
	0x06: {0, 32, 64},
}

// A Shape is one 12-clock thing to be drawn, at X, with what is known about its two halves.
// SolidLeft/SolidRight say whether that half is uniform enough for a SOLID object (a missile or
// the ball) to stand in for it -- a caller with a bitmap knows this; the placer cannot.
type Shape struct {
	X          int
	SolidLeft  bool // source pixels 0,1 identical -> the left 4 clocks can be a missile
	SolidRight bool // source pixels 4,5 identical -> the right 4 clocks can be a missile
}

// Split says which way a shape was cut.
type Split string

const (
	HeadFirst    Split = "head-first"    // player at X, solid at X+8
	MissileFirst Split = "missile-first" // solid at X, player at X+4
	TwoPlayers   Split = "two-players"   // player at X and another at X+8; no solid needed
)

// An Object is one strobe: what to reset, where it lands, and the NUSIZ code that repeats it.
type Object struct {
	Reg    string `json:"reg"`    // RESP0 / RESM0 / RESP1 / RESM1 / RESBL
	Base   int    `json:"base"`   // where the strobe puts it
	Nusiz  int    `json:"nusiz"`  // the copy code (0 for the ball, which has none)
	Cycle  int    `json:"cycle"`  // the write cycle to strobe it on
	Copies []int  `json:"copies"` // every position it draws at, wraps included
}

// A Plan is a placement that works.
type Plan struct {
	Objects []Object `json:"objects"`
	Splits  []Split  `json:"splits"` // one per shape, in the order given
}

// cyclesFor returns every write cycle that puts this kind of object at x. It is a LIST because at
// the floor the strobe stops moving: several consecutive cycles land on the same pixel, and that
// window is what lets two objects four pixels apart be strobed three cycles apart.
func cyclesFor(x int, solid bool, first, last int) []int {
	slope, icept, floor := PlayerSlope, PlayerIntercept, PlayerFloor
	if solid {
		slope, icept, floor = SolidSlope, SolidIntercept, SolidFloor
	}
	if x < floor {
		return nil
	}
	if x == floor {
		var out []int
		for c := first; c*slope+icept <= floor; c++ {
			if c >= first && c <= last {
				out = append(out, c)
			}
		}
		return out
	}
	if (x-icept)%slope != 0 {
		return nil
	}
	c := (x - icept) / slope
	if c < first || c > last {
		return nil
	}
	return []int{c}
}

type cand struct {
	base   int
	nusiz  int
	covers map[int]bool
	cycles []int
}

// candidates lists every (base, NUSIZ) an object of this kind can be given, with the positions it
// then draws at. A copy past the end of the line wraps to the left edge and draws there, so bases
// on the RIGHT of the screen reach positions on the LEFT that no strobe can.
func candidates(solid bool, first, last int, patterns []int) []cand {
	var out []cand
	for c := first; c <= last; c++ {
		slope, icept, floor := PlayerSlope, PlayerIntercept, PlayerFloor
		if solid {
			slope, icept, floor = SolidSlope, SolidIntercept, SolidFloor
		}
		base := c*slope + icept
		if base < floor {
			base = floor
		}
		if base >= LineClocks {
			continue
		}
		for _, n := range patterns {
			cov := map[int]bool{}
			var cps []int
			for _, o := range nusizCopies[n] {
				p := (base + o) % LineClocks
				cov[p] = true
				cps = append(cps, p)
			}
			sort.Ints(cps)
			out = append(out, cand{base: base, nusiz: n, covers: cov, cycles: []int{c}})
		}
	}
	// Merge candidates that are the same object reached from different cycles (the clamp window).
	byKey := map[[2]int]*cand{}
	var merged []cand
	for _, c := range out {
		k := [2]int{c.base, c.nusiz}
		if p, ok := byKey[k]; ok {
			p.cycles = append(p.cycles, c.cycles...)
			continue
		}
		cc := c
		byKey[k] = &cc
		merged = append(merged, cc)
	}
	res := make([]cand, 0, len(merged))
	for i := range merged {
		res = append(res, *byKey[[2]int{merged[i].base, merged[i].nusiz}])
	}
	return res
}

func copiesOf(base, nusiz int) []int {
	var out []int
	for _, o := range nusizCopies[nusiz] {
		out = append(out, (base+o)%LineClocks)
	}
	sort.Ints(out)
	return out
}

// Solve places every shape, or returns why it cannot.
//
// firstCycle/lastCycle bound the write cycles a caller's blank line can actually strobe on; pass
// 0,0 for the whole usable line.
func Solve(shapes []Shape, firstCycle, lastCycle int) (*Plan, error) {
	if len(shapes) == 0 {
		return nil, fmt.Errorf("no shapes to place")
	}
	if firstCycle == 0 && lastCycle == 0 {
		firstCycle, lastCycle = ClampFirst, 72
	}
	patterns := []int{0x00, 0x01, 0x02, 0x03, 0x04, 0x06}
	players := candidates(false, firstCycle, lastCycle, patterns)
	solids := candidates(true, firstCycle, lastCycle, patterns)
	balls := candidates(true, firstCycle, lastCycle, []int{0x00})

	n := len(shapes)
	modes := make([]Split, n)
	var best *Plan
	var try func(i int)

	fits := func() bool {
		needP := map[int]bool{}
		needS := map[int]bool{}
		for i, sh := range shapes {
			switch modes[i] {
			case HeadFirst:
				needP[sh.X] = true
				needS[(sh.X+HeadWidth)%LineClocks] = true
			case MissileFirst:
				needS[sh.X] = true
				needP[(sh.X+TailWidth)%LineClocks] = true
			default:
				needP[sh.X] = true
				needP[(sh.X+HeadWidth)%LineClocks] = true
			}
		}
		if len(needP) > 6 || len(needS) > 7 {
			return false
		}
		// Two player objects cover the player positions; their NUSIZ codes then bind the missiles,
		// because a missile repeats with its own player's code.
		for _, p0 := range players {
			rest := left(needP, p0.covers)
			if len(rest) > 3 {
				continue
			}
			for _, p1 := range players {
				if len(left(setOf(rest), p1.covers)) > 0 {
					continue
				}
				for _, m0 := range solids {
					if m0.nusiz != p0.nusiz {
						continue // a missile repeats with its player's copies, not its own
					}
					r1 := left(needS, m0.covers)
					if len(r1) > 4 {
						continue
					}
					for _, m1 := range solids {
						if m1.nusiz != p1.nusiz {
							continue
						}
						r2 := left(setOf(r1), m1.covers)
						if len(r2) > 1 {
							continue
						}
						if len(r2) == 0 {
							if pl := assemble(p0, p1, m0, m1, nil, modes); pl != nil {
								best = pl
								return true
							}
							continue
						}
						for _, bl := range balls { // the ball is the spare solid
							if !bl.covers[r2[0]] {
								continue
							}
							if pl := assemble(p0, p1, m0, m1, &bl, modes); pl != nil {
								best = pl
								return true
							}
						}
					}
				}
			}
		}
		return false
	}

	try = func(i int) {
		if best != nil {
			return
		}
		if i == n {
			fits()
			return
		}
		for _, m := range []Split{HeadFirst, MissileFirst, TwoPlayers} {
			if m == HeadFirst && !shapes[i].SolidRight {
				continue
			}
			if m == MissileFirst && !shapes[i].SolidLeft {
				continue
			}
			modes[i] = m
			try(i + 1)
			if best != nil {
				return
			}
		}
	}
	try(0)
	if best != nil {
		// A wrong plan is worse than no plan, so the solver checks its own answer before handing
		// it back: every shape's two halves must actually be covered by something the plan strobes.
		if err := Validate(shapes, best); err != nil {
			return nil, fmt.Errorf("internal: the placer produced a plan that does not check out: %w", err)
		}
	}
	if best == nil {
		return nil, fmt.Errorf("no placement exists for %d shapes at %v within write cycles %d..%d",
			n, xs(shapes), firstCycle, lastCycle)
	}
	return best, nil
}

// Validate re-derives what a Plan draws and checks it against the shapes it claims to place.
// Solve calls this on every answer; a caller that builds a plan by hand can call it too.
func Validate(shapes []Shape, p *Plan) error {
	if len(p.Splits) != len(shapes) {
		return fmt.Errorf("%d splits for %d shapes", len(p.Splits), len(shapes))
	}
	drawnP, drawnS := map[int]bool{}, map[int]bool{}
	for _, o := range p.Objects {
		for _, c := range o.Copies {
			if o.Reg == "RESP0" || o.Reg == "RESP1" {
				drawnP[c] = true
			} else {
				drawnS[c] = true
			}
		}
	}
	for i, sh := range shapes {
		var wantP, wantS []int
		switch p.Splits[i] {
		case HeadFirst:
			wantP, wantS = []int{sh.X}, []int{(sh.X + HeadWidth) % LineClocks}
			if !sh.SolidRight {
				return fmt.Errorf("shape %d at %d is head-first but its right half is not solid", i, sh.X)
			}
		case MissileFirst:
			wantS, wantP = []int{sh.X}, []int{(sh.X + TailWidth) % LineClocks}
			if !sh.SolidLeft {
				return fmt.Errorf("shape %d at %d is missile-first but its left half is not solid", i, sh.X)
			}
		default:
			wantP = []int{sh.X, (sh.X + HeadWidth) % LineClocks}
		}
		for _, x := range wantP {
			if !drawnP[x] {
				return fmt.Errorf("shape %d at %d: no player draws at %d", i, sh.X, x)
			}
		}
		for _, x := range wantS {
			if !drawnS[x] {
				return fmt.Errorf("shape %d at %d: no missile or ball draws at %d", i, sh.X, x)
			}
		}
	}
	for i, a := range p.Objects {
		for _, b := range p.Objects[i+1:] {
			if abs(a.Cycle-b.Cycle) < MinStrobeGap {
				return fmt.Errorf("%s and %s are strobed %d cycles apart; two stores need %d",
					a.Reg, b.Reg, abs(a.Cycle-b.Cycle), MinStrobeGap)
			}
		}
	}
	return nil
}

func xs(shapes []Shape) []int {
	out := make([]int, len(shapes))
	for i, s := range shapes {
		out[i] = s.X
	}
	return out
}

func setOf(v []int) map[int]bool {
	m := map[int]bool{}
	for _, x := range v {
		m[x] = true
	}
	return m
}

func left(need map[int]bool, covered map[int]bool) []int {
	var out []int
	for x := range need {
		if !covered[x] {
			out = append(out, x)
		}
	}
	sort.Ints(out)
	return out
}

// assemble picks one write cycle per object such that no two are closer than three, which is the
// shortest gap between two stores. At a floor an object has several cycles to choose from.
func assemble(p0, p1, m0, m1 cand, bl *cand, modes []Split) *Plan {
	type slot struct {
		reg  string
		c    cand
		opts []int
	}
	slots := []slot{{"RESP0", p0, p0.cycles}, {"RESM0", m0, m0.cycles},
		{"RESP1", p1, p1.cycles}, {"RESM1", m1, m1.cycles}}
	if bl != nil {
		slots = append(slots, slot{"RESBL", *bl, bl.cycles})
	}
	// Objects that came out identical are one object, strobed once.
	pick := make([]int, len(slots))
	var walk func(i int) bool
	walk = func(i int) bool {
		if i == len(slots) {
			return true
		}
		for _, c := range slots[i].opts {
			ok := true
			for j := 0; j < i; j++ { // every strobe is its own store, so none may be closer than three
				if abs(c-pick[j]) < MinStrobeGap {
					ok = false
					break
				}
			}
			if !ok {
				continue
			}
			pick[i] = c
			if walk(i + 1) {
				return true
			}
		}
		return false
	}
	if !walk(0) {
		return nil
	}
	pl := &Plan{Splits: append([]Split(nil), modes...)}
	for i, s := range slots {
		pl.Objects = append(pl.Objects, Object{Reg: s.reg, Base: s.c.base, Nusiz: s.c.nusiz,
			Cycle: pick[i], Copies: copiesOf(s.c.base, s.c.nusiz)})
	}
	return pl
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// CyclesForTest exposes cyclesFor to the cross-check in internal/emu, which is the one place the
// constants above are compared against the fixture that measured them.
func CyclesForTest(x int, solid bool) []int { return cyclesFor(x, solid, ClampFirst, 72) }
