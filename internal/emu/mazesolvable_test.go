package emu

import (
	"testing"
)

// TestEveryMazeSeedIsTraversable answers a question `maze.md` raises and does not settle.
//
// The page lists, under **Notes / scaling**: *"Carve guaranteed-solvable passages by forcing one open
// column per row (mask a passage bit before storing), the way **Entombed** guarantees a path."* That
// is written as something to do, not something done — `roms/techniques/maze.asm` has no such mask,
// and so the demo has no guarantee **by construction**.
//
// It has one by exhaustion. The generator is an 8-bit Galois LFSR of period 255 stepped per row from
// a fixed seed, so the seed picks the entire maze and there are exactly 255 of them. All 255 were
// generated and flood-filled, 2026-09-07: **every one is traversable from the top edge to the bottom
// by an 8-pixel-wide sprite.**
//
// ★**The width matters and the first version of this measurement left it out.** A 4-connected fill on
// pixels asks whether a 1-px path exists, which no player is. Requiring eight contiguous open columns
// cuts the open space at the top edge roughly in half — from 48–112 px to 22–91 — and the answer
// still holds, which is the useful form of it.
//
// ★★**Exhaustion is not construction.** Nothing in the ROM prevents a blocked maze; it happens that
// no seed produces one. That is a property of this generator at this height with this step rate, and
// any of the three changing puts it back to being a question. The forced-open-column carve is what
// would make it structural, and it is still not implemented.
//
// Raised by the mailing-list distillation (helper-1), who noted the generation can fail in principle
// and so a design decision is involved. The decision turns out to be affordable to defer: at this
// size, it never does.
func TestEveryMazeSeedIsTraversable(t *testing.T) {
	const (
		playerWidth = 8
		visible     = 160
	)

	// traversable generates the maze for one LFSR seed and reports whether a playerWidth-wide sprite
	// can reach the bottom row from the top, along with the open space at each end.
	traversable := func(seed uint8) (ok bool, rows, openTop, openBottom int) {
		e, err := New("NTSC")
		if err != nil {
			t.Fatal(err)
		}
		if err := e.LoadROM("../../roms/techniques/maze.bin"); err != nil {
			t.Fatal(err)
		}
		if err := e.RunFrames(2); err != nil {
			t.Fatal(err)
		}
		if err := e.Poke(0x80, seed); err != nil { // `lfsr` in maze.asm
			t.Fatal(err)
		}
		if err := e.RunFrames(6); err != nil {
			t.Fatal(err)
		}

		var wall [][]bool
		for ln := 0; ln < 262; ln++ {
			runs, _, err := e.DecomposeRow(ln)
			if err != nil || len(runs) == 0 {
				continue
			}
			row := make([]bool, visible)
			any := false
			for _, r := range runs {
				if r.Element != "PF" {
					continue
				}
				for x := r.Clock; x < r.Clock+r.Len && x < visible; x++ {
					row[x] = true
					any = true
				}
			}
			if any {
				wall = append(wall, row)
			}
		}
		if len(wall) == 0 {
			return false, 0, 0, 0
		}

		// A cell is passable only if the sprite fits with its left edge there.
		h := len(wall)
		blocked := make([][]bool, h)
		for y := range wall {
			blocked[y] = make([]bool, visible)
			for x := 0; x < visible; x++ {
				fits := x+playerWidth <= visible
				for k := 0; fits && k < playerWidth; k++ {
					if wall[y][x+k] {
						fits = false
					}
				}
				blocked[y][x] = !fits
			}
		}

		seen := make([][]bool, h)
		for i := range seen {
			seen[i] = make([]bool, visible)
		}
		var stack [][2]int
		for x := 0; x < visible; x++ {
			if !blocked[0][x] {
				stack = append(stack, [2]int{0, x})
				seen[0][x] = true
				openTop++
			}
			if !blocked[h-1][x] {
				openBottom++
			}
		}
		for len(stack) > 0 {
			p := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if p[0] == h-1 {
				ok = true
			}
			for _, d := range [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
				y, x := p[0]+d[0], p[1]+d[1]
				if y < 0 || y >= h || x < 0 || x >= visible || seen[y][x] || blocked[y][x] {
					continue
				}
				seen[y][x] = true
				stack = append(stack, [2]int{y, x})
			}
		}
		return ok, h, openTop, openBottom
	}

	// The control first: a seed must actually produce a maze, or 255 passes would mean 255 blank
	// screens.
	ok, rows, top, bottom := traversable(0x5A)
	if rows < 100 {
		t.Fatalf("the default seed drew %d playfield rows; a maze this small is not what the sweep "+
			"below is about", rows)
	}
	if top == 0 || bottom == 0 || top >= 160 || bottom >= 160 {
		t.Fatalf("the default seed leaves %d open at the top and %d at the bottom of 160. Neither 0 "+
			"nor everything is a maze, and both would make the fill meaningless", top, bottom)
	}
	if !ok {
		t.Fatal("the default seed is not traversable")
	}

	var blockedSeeds []uint8
	for s := 1; s <= 255; s++ {
		if pass, _, _, _ := traversable(uint8(s)); !pass {
			blockedSeeds = append(blockedSeeds, uint8(s))
		}
	}
	if len(blockedSeeds) > 0 {
		t.Errorf("%d of the 255 LFSR seeds produce a maze an %d-px sprite cannot cross: %v.\n"+
			"maze.asm has no forced-open-column carve, so this was only ever true by exhaustion — "+
			"if it has stopped being true, maze.md's 'Notes / scaling' entry has become a "+
			"requirement rather than a suggestion.", len(blockedSeeds), playerWidth, blockedSeeds)
	}
}
