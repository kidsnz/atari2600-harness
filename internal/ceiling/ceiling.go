package ceiling

import (
	"fmt"
	"image"
	"math"
	"runtime"
	"sort"
	"sync"
)

// Rung names one constraint set. A ceiling is a property of (image, constraint
// set), so a rung is never optional context — it IS the denominator.
type Rung string

const (
	// C1 — PLAYFIELD ONLY, two colours per line, on the 40-column x 4-clock grid.
	// COLUBK + COLUPF, playfield written asymmetrically (PF0/PF1/PF2 twice per
	// scanline). This is exactly the budget of a hand-written asymmetric-PF kernel
	// such as the practice tree's horizon.asm.
	C1 Rung = "C1"

	// C2 — C1 plus ONE object on the line: a third colour inside a window 8 colour
	// clocks wide (one player at NUSIZ single-copy, one clock per GRP bit), with
	// per-clock control inside that window. See the SpriteColumns note below for
	// the one restriction and its direction.
	C2 Rung = "C2"

	// C3 — NO column grid, two colours per line. Not achievable on a 2600 by the
	// playfield; it is the diagnostic bound that isolates what the 4-clock column
	// grid itself costs. C1-C3 was measured at 8.95 rmse on Barnstorming (content
	// = fine sprite detail) against 3.25 on Chopper Command (content = landscape).
	C3 Rung = "C3"
)

// DefaultColumns is the playfield's column count: 40 columns of 4 colour clocks
// across the 160-clock visible line (CLAUDE.md, "playfield (bit order)").
const DefaultColumns = 40

// SpriteColumns is the width of C2's object in playfield columns. A player at
// NUSIZ single-copy is 8 colour clocks = exactly 2 columns.
//
// STATED RESTRICTION: the window is aligned to the column grid (39 positions on a
// 40-column line) rather than free to start at any of the 153 clock positions.
// Restricting the search can only make the computed error LARGER, so this can
// only ever understate the machine, never overstate it — the safe direction for a
// bound whose job is to say "the hardware cannot do this". The cost of the
// restriction is measured, not assumed: TestColumnAlignedSpriteWindowCostIsMeasured
// compares against a free-start reference implementation and reports the gap.
const SpriteColumns = 2

// Options configures a ladder computation. The zero value is usable.
type Options struct {
	Columns int    // playfield columns (default DefaultColumns)
	Rungs   []Rung // rungs to compute (default C1, C2, C3)
	Workers int    // parallel line workers (default GOMAXPROCS)
}

func (o Options) columns() int {
	if o.Columns > 0 {
		return o.Columns
	}
	return DefaultColumns
}

func (o Options) rungs() []Rung {
	if len(o.Rungs) > 0 {
		return o.Rungs
	}
	return []Rung{C1, C2, C3}
}

// RungResult is one rung's achievable error over the whole frame.
type RungResult struct {
	Rung  Rung    `json:"rung"`
	Model string  `json:"model"` // the constraint set, spelled out
	SumSq float64 `json:"sum_sq_err"`
	RMSE  float64 `json:"rmse"` // sqrt(SumSq / (pixels*3)), RGB units
}

// Delta is a difference between two rungs — the actual deliverable. A rung on its
// own is a number against constraints the kernel may not be working under; the
// difference between two rungs is an answer to a question an author asks.
type Delta struct {
	From     Rung    `json:"from"`
	To       Rung    `json:"to"`
	RMSEDrop float64 `json:"rmse_drop"`
	Question string  `json:"question"`
}

// Result is the ladder.
type Result struct {
	Spec        string       `json:"tv_spec"`
	Width       int          `json:"width"`
	Height      int          `json:"height"`
	Pixels      int          `json:"pixels"`
	Columns     int          `json:"columns"`
	UniqueLines int          `json:"unique_lines"` // distinct scanlines actually solved
	Flat        RungResult   `json:"flat"`         // reference: best single colour per line
	Rungs       []RungResult `json:"rungs"`
	Deltas      []Delta      `json:"deltas"`
}

// RMSEOf returns a rung's rmse, and whether it was computed.
func (r *Result) RMSEOf(g Rung) (float64, bool) {
	for _, x := range r.Rungs {
		if x.Rung == g {
			return x.RMSE, true
		}
	}
	return 0, false
}

// Analysis is the computed ladder plus the per-line solutions, which is what
// Render needs to draw a rung as a picture.
type Analysis struct {
	Result Result

	pal   Palette
	cols  int
	cellW int
	w, h  int
	rows  []*lineSolution // one per image row (shared for identical rows)
	want  map[Rung]bool

	src       *image.RGBA // the target frame, kept so Render can reproduce the optimum exactly
	srcOrigin image.Point
}

type lineSolution struct {
	flatV   int
	flatErr int64

	c1a, c1b int
	c1Err    int64

	c2a, c2b, c2t, c2k int
	c2Err              int64
	c2Cand             int // pairs that survived the C2 bound (diagnostic)

	c3a, c3b int
	c3Err    int64
}

const infErr = int64(math.MaxInt64 / 4)

// Compute solves the ladder for one frame.
//
// WHAT THE NUMBERS MEAN. Each rung's rmse is the error of the BEST picture
// obtainable under that rung's constraint set, measured against the target at
// full pixel resolution in RGB units (0..255 per channel). 0 = the constraint set
// can reproduce the target exactly. It is NOT a score for any kernel: it is the
// denominator a kernel's own error should be read against.
func Compute(img *image.RGBA, pal Palette, opts Options) (*Analysis, error) {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	cols := opts.columns()
	if w <= 0 || h <= 0 {
		return nil, fmt.Errorf("ceiling: empty image")
	}
	if cols < 2 || w%cols != 0 {
		return nil, fmt.Errorf("ceiling: image width %d is not a whole multiple of %d playfield columns — a ceiling on the column grid needs an integer cell width", w, cols)
	}
	requested := map[Rung]bool{}
	for _, r := range opts.rungs() {
		switch r {
		case C1, C2, C3:
			requested[r] = true
		default:
			return nil, fmt.Errorf("ceiling: unknown rung %q (want C1, C2 or C3)", r)
		}
	}
	want := map[Rung]bool{}
	for r := range requested {
		want[r] = true
	}
	// C2 is defined as C1 plus one object, and its search is seeded from C1's
	// optimal pair, so C1 is always SOLVED when C2 is asked for — but it is only
	// REPORTED when it was asked for.
	if want[C2] {
		want[C1] = true
	}

	// Deduplicate identical scanlines. A frame is mostly repeated lines (a 20-band
	// litmus fixture has 20 distinct lines in 192), and the per-line solve is the
	// expensive part.
	type job struct {
		key string
		px  [][3]int
	}
	index := map[string]int{}
	var jobs []job
	rowJob := make([]int, h)
	buf := make([]byte, w*3)
	for y := 0; y < h; y++ {
		px := make([][3]int, w)
		for x := 0; x < w; x++ {
			i := img.PixOffset(b.Min.X+x, b.Min.Y+y)
			px[x] = [3]int{int(img.Pix[i]), int(img.Pix[i+1]), int(img.Pix[i+2])}
			buf[x*3], buf[x*3+1], buf[x*3+2] = img.Pix[i], img.Pix[i+1], img.Pix[i+2]
		}
		key := string(buf)
		if j, ok := index[key]; ok {
			rowJob[y] = j
			continue
		}
		index[key] = len(jobs)
		rowJob[y] = len(jobs)
		jobs = append(jobs, job{key: key, px: px})
	}

	sols := make([]*lineSolution, len(jobs))
	workers := opts.Workers
	if workers <= 0 {
		workers = runtime.GOMAXPROCS(0)
	}
	if workers > len(jobs) {
		workers = len(jobs)
	}
	var wg sync.WaitGroup
	next := make(chan int)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range next {
				sols[j] = solveLine(jobs[j].px, cols, &pal, want)
			}
		}()
	}
	for j := range jobs {
		next <- j
	}
	close(next)
	wg.Wait()

	a := &Analysis{pal: pal, cols: cols, cellW: w / cols, w: w, h: h, want: want, src: img, srcOrigin: b.Min}
	a.rows = make([]*lineSolution, h)
	for y := 0; y < h; y++ {
		a.rows[y] = sols[rowJob[y]]
	}

	npix := w * h
	var flat, c1, c2, c3 int64
	for _, s := range a.rows {
		flat += s.flatErr
		c1 += s.c1Err
		c2 += s.c2Err
		c3 += s.c3Err
	}
	rmse := func(sum int64) float64 { return math.Sqrt(float64(sum) / float64(npix*3)) }

	res := Result{
		Spec: pal.Spec, Width: w, Height: h, Pixels: npix, Columns: cols,
		UniqueLines: len(jobs),
		Flat: RungResult{Rung: "flat", Model: "one flat colour per line — the weakest picture the machine can draw; the normalising reference",
			SumSq: float64(flat), RMSE: rmse(flat)},
	}
	add := func(r Rung, model string, sum int64) {
		if requested[r] {
			res.Rungs = append(res.Rungs, RungResult{Rung: r, Model: model, SumSq: float64(sum), RMSE: rmse(sum)})
		}
	}
	add(C1, fmt.Sprintf("playfield only, 2 colours per line (COLUBK+COLUPF), %d columns x %d colour clocks", cols, 160/cols), c1)
	add(C2, fmt.Sprintf("C1 plus one %d-clock object (a third colour, per-clock inside the window, window aligned to the column grid)", SpriteColumns*160/cols), c2)
	add(C3, "no column grid, 2 colours per line (not 2600-achievable; isolates the grid's own cost)", c3)

	if requested[C1] && requested[C2] {
		res.Deltas = append(res.Deltas, Delta{From: C1, To: C2, RMSEDrop: rmse(c1) - rmse(c2),
			Question: "what would ONE sprite buy on this picture?"})
	}
	if requested[C1] && requested[C3] {
		res.Deltas = append(res.Deltas, Delta{From: C1, To: C3, RMSEDrop: rmse(c1) - rmse(c3),
			Question: "how much is the 4-clock column grid costing on this picture?"})
	}
	a.Result = res
	return a, nil
}

// C2Candidates reports how many colour pairs survived C2's branch-and-bound on
// the worst line — a diagnostic that says how close the exact search came to
// degenerating into the full 8128-pair scan.
func (a *Analysis) C2Candidates() int {
	worst := 0
	seen := map[*lineSolution]bool{}
	for _, s := range a.rows {
		if seen[s] {
			continue
		}
		seen[s] = true
		if s.c2Cand > worst {
			worst = s.c2Cand
		}
	}
	return worst
}

func minI32(a, b int32) int32 {
	if b < a {
		return b
	}
	return a
}

func dist2(a, b [3]int) int32 {
	dr := int32(a[0] - b[0])
	dg := int32(a[1] - b[1])
	db := int32(a[2] - b[2])
	return dr*dr + dg*dg + db*db
}

// solveLine computes the exact optimum of each requested rung for one scanline.
//
// EXHAUSTIVE BY CONSTRUCTION for C1 and C3: all 128*129/2 = 8256 ordered colour
// pairs (8128 distinct pairs + 128 single-colour cases) are evaluated, so the
// number is a true optimum and not a heuristic that could understate the machine.
// C2 is exact too, via a branch-and-bound whose bound is valid rather than
// heuristic: an 8-clock object can only reduce the error inside the two columns
// it covers, and at best to zero.
func solveLine(px [][3]int, cols int, pal *Palette, want map[Rung]bool) *lineSolution {
	w := len(px)
	cellW := w / cols
	sol := &lineSolution{c1Err: 0, c2Err: 0, c3Err: 0}

	// Bucket the line's distinct colours: a 160-pixel TIA line holds far fewer
	// than 160 distinct values, and identical pixels cost the same.
	uniqIdx := map[[3]int]int{}
	var uniq [][3]int
	idx := make([]int, w)
	for x, p := range px {
		u, ok := uniqIdx[p]
		if !ok {
			u = len(uniq)
			uniqIdx[p] = u
			uniq = append(uniq, p)
		}
		idx[x] = u
	}
	cnt := make([]int64, len(uniq))
	for _, u := range idx {
		cnt[u]++
	}

	// du[v][u] = squared RGB error of painting a pixel of colour uniq[u] with
	// palette entry v.
	du := make([][]int32, PaletteSize)
	flatDU := make([]int32, PaletteSize*len(uniq))
	for v := 0; v < PaletteSize; v++ {
		row := flatDU[v*len(uniq) : (v+1)*len(uniq) : (v+1)*len(uniq)]
		for u := range uniq {
			row[u] = dist2(uniq[u], pal.Colors[v])
		}
		du[v] = row
	}

	// cc[v][c] = cost of painting playfield column c entirely with palette entry
	// v, charged at PIXEL resolution against the original line. Charging at pixel
	// resolution is what makes the C1 bound honest about the grid: detail finer
	// than 4 clocks is lost, and that loss belongs to the hardware.
	cc := make([][]int64, PaletteSize)
	flatCC := make([]int64, PaletteSize*cols)
	for v := 0; v < PaletteSize; v++ {
		row := flatCC[v*cols : (v+1)*cols : (v+1)*cols]
		dv := du[v]
		for c := 0; c < cols; c++ {
			var s int64
			for x := c * cellW; x < (c+1)*cellW; x++ {
				s += int64(dv[idx[x]])
			}
			row[c] = s
		}
		cc[v] = row
	}

	// flat reference: the single best colour for the whole line.
	sol.flatErr = infErr
	for v := 0; v < PaletteSize; v++ {
		var s int64
		for c := 0; c < cols; c++ {
			s += cc[v][c]
		}
		if s < sol.flatErr {
			sol.flatErr, sol.flatV = s, v
		}
	}

	// --- C1: exhaustive over all colour pairs on the column grid ---
	needPairInfo := want[C2]
	type pairRec struct {
		a, b int32
		lb   int64 // valid lower bound on this pair's C2 error
	}
	var recs []pairRec
	if needPairInfo {
		recs = make([]pairRec, 0, PaletteSize*(PaletteSize+1)/2)
	}
	if want[C1] {
		sol.c1Err = infErr
		pc := make([]int64, cols)
		for a := 0; a < PaletteSize; a++ {
			ca := cc[a]
			for b := a; b < PaletteSize; b++ {
				cb := cc[b]
				var s int64
				if needPairInfo {
					for c := 0; c < cols; c++ {
						m := ca[c]
						if cb[c] < m {
							m = cb[c]
						}
						pc[c] = m
						s += m
					}
					// The object covers SpriteColumns adjacent columns and can at
					// best erase their error entirely, so this is a valid bound.
					var maxAdj int64
					for k := 0; k+SpriteColumns <= cols; k++ {
						var t int64
						for j := 0; j < SpriteColumns; j++ {
							t += pc[k+j]
						}
						if t > maxAdj {
							maxAdj = t
						}
					}
					recs = append(recs, pairRec{a: int32(a), b: int32(b), lb: s - maxAdj})
				} else {
					for c := 0; c < cols; c++ {
						m := ca[c]
						if cb[c] < m {
							m = cb[c]
						}
						s += m
						if s >= sol.c1Err {
							break
						}
					}
				}
				if s < sol.c1Err {
					sol.c1Err, sol.c1a, sol.c1b = s, a, b
				}
			}
		}
	}

	// --- C3: exhaustive over all colour pairs, no column grid ---
	if want[C3] {
		sol.c3Err = infErr
		duw := make([][]int64, PaletteSize)
		flatW := make([]int64, PaletteSize*len(uniq))
		for v := 0; v < PaletteSize; v++ {
			row := flatW[v*len(uniq) : (v+1)*len(uniq) : (v+1)*len(uniq)]
			for u := range uniq {
				row[u] = int64(du[v][u]) * cnt[u]
			}
			duw[v] = row
		}
		for a := 0; a < PaletteSize; a++ {
			wa := duw[a]
			for b := a; b < PaletteSize; b++ {
				wb := duw[b]
				var s int64
				for u := range uniq {
					m := wa[u]
					if wb[u] < m {
						m = wb[u]
					}
					s += m
					if s >= sol.c3Err {
						break
					}
				}
				if s < sol.c3Err {
					sol.c3Err, sol.c3a, sol.c3b = s, a, b
				}
			}
		}
	}

	// --- C2: C1 plus one 8-clock object, exact via branch-and-bound ---
	if want[C2] {
		// Seed with C1's optimal pair, which gives a real achievable value to
		// prune against.
		err, t, k := c2ForPair(sol.c1a, sol.c1b, cc, du, idx, cols, cellW)
		sol.c2Err, sol.c2a, sol.c2b, sol.c2t, sol.c2k = err, sol.c1a, sol.c1b, t, k
		sol.c2Cand = 1
		sort.Slice(recs, func(i, j int) bool { return recs[i].lb < recs[j].lb })
		for _, r := range recs {
			if r.lb >= sol.c2Err {
				break // lb is ascending: no later pair can beat the incumbent
			}
			a, b := int(r.a), int(r.b)
			if a == sol.c1a && b == sol.c1b {
				continue
			}
			sol.c2Cand++
			e, tt, kk := c2ForPair(a, b, cc, du, idx, cols, cellW)
			if e < sol.c2Err {
				sol.c2Err, sol.c2a, sol.c2b, sol.c2t, sol.c2k = e, a, b, tt, kk
			}
		}
	}
	return sol
}

// c2ForPair returns the exact best C2 error for a fixed playfield pair (a,b),
// searching all object colours t and all column-aligned window positions k.
// Inside the window each colour clock independently takes the object colour or
// falls through to the playfield beneath it — which is what a player's 8 GRP bits
// actually do.
func c2ForPair(a, b int, cc [][]int64, du [][]int32, idx []int, cols, cellW int) (best int64, bestT, bestK int) {
	ca, cb := cc[a], cc[b]
	pc := make([]int64, cols)
	var pairSum int64
	for c := 0; c < cols; c++ {
		m := ca[c]
		if cb[c] < m {
			m = cb[c]
		}
		pc[c] = m
		pairSum += m
	}
	best = infErr
	m := make([]int64, cols)
	da, db := du[a], du[b]
	for t := 0; t < PaletteSize; t++ {
		dt := du[t]
		for c := 0; c < cols; c++ {
			var sa, sb int64
			for x := c * cellW; x < (c+1)*cellW; x++ {
				u := idx[x]
				v := dt[u]
				xa, xb := da[u], db[u]
				if v < xa {
					xa = v
				}
				if v < xb {
					xb = v
				}
				sa += int64(xa)
				sb += int64(xb)
			}
			if sb < sa {
				sa = sb
			}
			m[c] = sa
		}
		for k := 0; k+SpriteColumns <= cols; k++ {
			s := pairSum
			for j := 0; j < SpriteColumns; j++ {
				s += m[k+j] - pc[k+j]
			}
			if s < best {
				best, bestT, bestK = s, t, k
			}
		}
	}
	return best, bestT, bestK
}
