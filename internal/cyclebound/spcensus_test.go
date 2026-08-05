package cyclebound

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/jetsetilly/gopher2600/hardware/cpu/instructions"
	"github.com/kidsnz/atari2600-harness/internal/build"
)

// spCensus counts, over one image, how well SP is known at each decoded site.
type spCensus struct {
	rom                          string
	sites, exact, ranged, top    int
	pushSites, pushTop, pushRng  int
	pullSites, pullTop, pullRng  int
	jsr, rts, rti, brk, txs, tsx int
	converged                    bool
}

func (c spCensus) String() string {
	return fmt.Sprintf("%-28s sites=%5d exact=%5d range=%4d top=%5d | push=%3d(top %d,rng %d) pull=%3d(top %d,rng %d) | jsr=%3d rts=%3d rti=%d brk=%d txs=%d tsx=%d conv=%v",
		c.rom, c.sites, c.exact, c.ranged, c.top, c.pushSites, c.pushTop, c.pushRng,
		c.pullSites, c.pullTop, c.pullRng, c.jsr, c.rts, c.rti, c.brk, c.txs, c.tsx, c.converged)
}

// statesFor reproduces the exact decode+fixpoint pipeline Prove runs, so a census
// taken here describes the states the prover actually consumes rather than a
// separate model of them.
func statesFor(path string) (map[site]Instr, map[site]State, bool, error) {
	bin := path
	if !strings.EqualFold(filepath.Ext(path), ".bin") {
		bin = build.BinPathFor(path)
		if out, err := build.Assemble(path, bin); err != nil {
			return nil, nil, false, fmt.Errorf("assemble: %s", out)
		}
	}
	rom, err := os.ReadFile(bin)
	if err != nil {
		return nil, nil, false, err
	}
	units, unitErr := analysisUnits(rom, bin)
	if unitErr != "" {
		return nil, nil, false, fmt.Errorf("declined: %s", unitErr)
	}
	decodes, instrs, entries, _ := decodeUnits(units)
	sw := switchModel{banked: len(units) > 1, banks: map[int]bool{}}
	if sw.banked {
		sw.hotspots = units[0].hotspots
		for _, u := range units {
			sw.banks[u.bank] = true
		}
	}
	widen, _ := unmodelledLandings(instrs, sw)
	states, conv := computeStates(instrs, entries, romByBank(decodes), sw, widen)
	return instrs, states, conv, nil
}

func censusOf(path string) (spCensus, error) {
	instrs, states, conv, err := statesFor(path)
	if err != nil {
		return spCensus{}, err
	}
	c := spCensus{rom: filepath.Base(path), converged: conv}
	for at, in := range instrs {
		st := states[at]
		c.sites++
		switch {
		case !st.valid || st.SP.Top:
			c.top++
		default:
			if _, ok := st.SP.konst(); ok {
				c.exact++
			} else {
				c.ranged++
			}
		}
		bucket := func(n, tp, rg *int) {
			*n++
			if !st.valid || st.SP.Top {
				*tp++
			} else if _, ok := st.SP.konst(); !ok {
				*rg++
			}
		}
		switch in.Def.Operator {
		case instructions.PHA, instructions.PHP:
			bucket(&c.pushSites, &c.pushTop, &c.pushRng)
		case instructions.PLA, instructions.PLP:
			bucket(&c.pullSites, &c.pullTop, &c.pullRng)
		case instructions.JSR:
			c.jsr++
		case instructions.RTS:
			c.rts++
		case instructions.RTI:
			c.rti++
		case instructions.BRK:
			c.brk++
		case instructions.TXS:
			c.txs++
		case instructions.TSX:
			c.tsx++
		}
	}
	return c, nil
}

func spCorpus(t *testing.T) []string {
	t.Helper()
	var files []string
	for _, pat := range []string{
		"../../roms/litmus/*.asm",
		"../../roms/techniques/*.asm",
		"../../roms/exerciser/*.asm",
		"../../roms/carts/*.asm",
		"../../reference/roms-study/*.bin",
		"../../../reference/roms-study/*.bin",
		"../../../reference/pizza-boy/Samples for Pizza Boy/*.bin",
	} {
		more, _ := filepath.Glob(pat)
		files = append(files, more...)
	}
	sort.Strings(files)
	return files
}

// TestSPCensus is the SD-3 measurement: how often is SP known at all, and how often
// is it unknown at exactly the instruction whose memory footprint depends on it.
// It is a measurement, not a gate — but it fails if the corpus vanishes, because a
// census over nothing is the failure mode this package keeps finding.
func TestSPCensus(t *testing.T) {
	files := spCorpus(t)
	if len(files) == 0 {
		t.Skip("no corpus: needs roms/ (and optionally the umbrella reference/ trees)")
	}
	var tot spCensus
	tot.rom = "TOTAL"
	analysed, declined := 0, 0
	for _, f := range files {
		c, err := censusOf(f)
		if err != nil {
			declined++
			t.Logf("skip %s: %v", filepath.Base(f), err)
			continue
		}
		analysed++
		t.Log(c.String())
		tot.sites += c.sites
		tot.exact += c.exact
		tot.ranged += c.ranged
		tot.top += c.top
		tot.pushSites, tot.pushTop, tot.pushRng = tot.pushSites+c.pushSites, tot.pushTop+c.pushTop, tot.pushRng+c.pushRng
		tot.pullSites, tot.pullTop, tot.pullRng = tot.pullSites+c.pullSites, tot.pullTop+c.pullTop, tot.pullRng+c.pullRng
		tot.jsr, tot.rts, tot.rti, tot.brk = tot.jsr+c.jsr, tot.rts+c.rts, tot.rti+c.rti, tot.brk+c.brk
		tot.txs, tot.tsx = tot.txs+c.txs, tot.tsx+c.tsx
	}
	t.Logf("analysed %d images, declined %d", analysed, declined)
	t.Log(tot.String())
	if tot.sites == 0 {
		t.Fatal("no sites measured — the census proved nothing")
	}
}

// TestSPCauseCensus splits the "SP unknown" population by CAUSE, because
// "86% Top" is a symptom and the fix has to be aimed at whatever produces it.
func TestSPCauseCensus(t *testing.T) {
	files := spCorpus(t)
	if len(files) == 0 {
		t.Skip("no corpus")
	}
	groups := map[string]*[8]int{} // sites, valid, exact, ranged, validTop, invalid, retPoints, retReach
	order := []string{}
	get := func(k string) *[8]int {
		if g, ok := groups[k]; ok {
			return g
		}
		g := &[8]int{}
		groups[k] = g
		order = append(order, k)
		return g
	}
	// JSR modelling facts, counted over every JSR in the corpus.
	jsrTotal, jsrAccessed, calleeSPMatchesCaller, calleeSPIsCallerMinus2 := 0, 0, 0, 0
	retTop := 0
	for _, f := range files {
		instrs, states, _, err := statesFor(f)
		if err != nil {
			continue
		}
		key := "litmus"
		switch {
		case strings.Contains(f, "/techniques/"):
			key = "techniques"
		case strings.Contains(f, "/carts/"):
			key = "carts"
		case strings.Contains(f, "/exerciser/"):
			key = "exerciser"
		case strings.Contains(f, "reference/"):
			key = "commercial"
		}
		g := get(key)
		// Sites whose ONLY way in is a JSR's return edge.
		retPoint := map[site]bool{}
		for _, in := range instrs {
			if in.Def.Operator == instructions.JSR {
				retPoint[in.nextSite()] = true
			}
		}
		for at, in := range instrs {
			st := states[at]
			g[0]++
			if !st.valid {
				g[5]++
			} else {
				g[1]++
				switch {
				case st.SP.Top:
					g[4]++
				default:
					if _, ok := st.SP.konst(); ok {
						g[2]++
					} else {
						g[3]++
					}
				}
			}
			if retPoint[at] {
				g[6]++
				if st.valid && st.SP.Top {
					retTop++
				}
			}
			if in.Def.Operator != instructions.JSR {
				continue
			}
			jsrTotal++
			if _, ok := accessOf(in, st); ok {
				jsrAccessed++
			}
			sp, ok := st.SP.konst()
			if !ok {
				continue
			}
			cst := states[site{in.Bank, in.Operand}]
			if !cst.valid {
				continue
			}
			if csp, ok := cst.SP.konst(); ok {
				if csp == sp {
					calleeSPMatchesCaller++
				}
				if csp == ((sp - 2) & 0xFF) {
					calleeSPIsCallerMinus2++
				}
			}
		}
	}
	sort.Strings(order)
	var all [8]int
	for _, k := range order {
		g := groups[k]
		t.Logf("%-11s sites=%6d  reached=%6d (exact %5d / range %4d / TOP %5d)  unreached=%5d  jsr-return-points=%4d",
			k, g[0], g[1], g[2], g[3], g[4], g[5], g[6])
		for i := range all {
			all[i] += g[i]
		}
	}
	t.Logf("%-11s sites=%6d  reached=%6d (exact %5d / range %4d / TOP %5d)  unreached=%5d  jsr-return-points=%4d",
		"ALL", all[0], all[1], all[2], all[3], all[4], all[5], all[6])
	t.Logf("JSR: %d total; %d have a modelled memory access (accessOf ok) — the return address it pushes",
		jsrTotal, jsrAccessed)
	t.Logf("JSR with caller SP known-exact and callee entry SP known-exact: %d equal to the caller's SP, "+
		"%d equal to caller SP-2 (the hardware's value)", calleeSPMatchesCaller, calleeSPIsCallerMinus2)
	t.Logf("JSR return points whose entry SP is TOP: %d of %d", retTop, all[6])
	if all[0] == 0 {
		t.Fatal("nothing measured")
	}
}
