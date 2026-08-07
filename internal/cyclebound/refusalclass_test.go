package cyclebound

import (
	"sort"
	"strings"
	"testing"
)

// TestRefusalClassesAccountForEveryUnboundedAddress measures what each refusal class
// would be WORTH if it vanished:
// the addresses whose every remaining refusal is that class. An address counts as
// proven only when EVERY call context proves it (the denominator coverage_test uses),
// so this is an upper bound on the axis and it does not require the unsound forcing
// the recorded ceiling table needed.
func TestRefusalClassesAccountForEveryUnboundedAddress(t *testing.T) {
	paths := commercialROMPaths()
	if len(paths) == 0 {
		t.Skip("no commercial corpus")
	}
	type key struct {
		rom   string
		start uint16
		bank  int
	}
	proven := map[key]bool{}
	blockers := map[key]map[string]bool{}
	all := map[key]bool{}

	class := func(reason string) string {
		switch {
		case strings.HasPrefix(reason, "multiple back-edges"):
			return "multiple back-edges"
		case strings.HasPrefix(reason, "WSYNC inside loop body"):
			return "WSYNC in body"
		case strings.HasPrefix(reason, "loop bound unknown"):
			return "trip count"
		case strings.HasPrefix(reason, "no WSYNC reached"):
			return "no WSYNC reached"
		case strings.HasPrefix(reason, "branch inside loop body"):
			return "branch in body"
		case strings.HasPrefix(reason, "call or jump inside loop body"):
			return "call/jump in body"
		case strings.Contains(reason, "bank-switch hotspots"):
			return "unresolved bank switch"
		case strings.HasPrefix(reason, "timer wait"):
			return "RIOT timer wait"
		case strings.HasPrefix(reason, "BRK in region"):
			return "BRK"
		case strings.HasPrefix(reason, "indirect JMP"):
			return "indirect JMP"
		}
		return "other"
	}

	for _, p := range paths {
		rep, err := Prove(p, 76)
		if err != nil {
			continue
		}
		for _, r := range rep.Lines {
			k := key{p, r.Start, r.Bank}
			all[k] = true
			if _, ok := proven[k]; !ok {
				proven[k] = true
			}
		}
		for _, list := range [][]Region{rep.Unbounded, rep.BlankUnbounded} {
			for _, r := range list {
				k := key{p, r.Start, r.Bank}
				all[k] = true
				proven[k] = false
				if blockers[k] == nil {
					blockers[k] = map[string]bool{}
				}
				blockers[k][class(r.Reason)] = true
			}
		}
	}

	base := 0
	for k := range all {
		if proven[k] {
			base++
		}
	}
	worth := map[string]int{}
	for k, bs := range blockers {
		if proven[k] || len(bs) != 1 {
			continue // already proven, or blocked by more than this one class
		}
		for c := range bs {
			worth[c]++
		}
	}
	var out []struct {
		c string
		n int
	}
	for c, n := range worth {
		out = append(out, struct {
			c string
			n int
		}{c, n})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].n > out[j].n })
	t.Logf("addresses %d, proven in every context %d = %.1f%%", len(all), base, 100*float64(base)/float64(len(all)))
	t.Logf("ADDRESSES WHOSE ONLY REMAINING BLOCKER IS THIS CLASS — AN UPPER BOUND, AND THE GAP TO")
	t.Logf("REALITY CAN BE TOTAL. Removing a class does not free its addresses if a SECOND obstacle")
	t.Logf("waits behind it. Measured on WSYNC-in-body: this table said +5.7 pt, disabling the refusal")
	t.Logf("moved coverage by 0.0 and pushed 25 of its 36 addresses onto the trip-count analysis instead.")
	t.Logf("Treat every row below as \"at most\", and force the obstacle before planning around it:")
	for _, e := range out {
		t.Logf("%5d  (+%.1f pt)  %s", e.n, 100*float64(e.n)/float64(len(all)), e.c)
	}
	multi := 0
	for k, bs := range blockers {
		if !proven[k] && len(bs) > 1 {
			multi++
		}
	}
	per := map[string]map[string]int{}
	for k, bs := range blockers {
		if proven[k] || len(bs) != 1 {
			continue
		}
		for c := range bs {
			if per[c] == nil {
				per[c] = map[string]int{}
			}
			short := k.rom
			if i := strings.LastIndex(short, "/"); i >= 0 {
				short = short[i+1:]
			}
			per[c][short]++
		}
	}
	for _, c := range []string{"unresolved bank switch", "multiple back-edges", "trip count"} {
		var rs []string
		for r, n := range per[c] {
			rs = append(rs, r+":"+itoaN(n))
		}
		sort.Strings(rs)
		t.Logf("  %s -> %s", c, strings.Join(rs, " "))
	}
	t.Logf("%5d addresses are blocked by MORE THAN ONE class — none of the single figures above can reach them, "+
		"which is the non-independence the ceiling table already warned about", multi)

	// THE GATE. The table above is only as good as the classifier, and the classifier
	// matches on the prover's own message prefixes. Add a refusal reason and forget
	// this file and the new class silently lands in "other", where it looks small and
	// nobody re-plans around it — which is exactly how "unresolved bank switch", worth
	// 145 addresses, stayed off the recorded ceiling table for as long as it did.
	//
	// Measured 2026-08-07: other = 6 of 320. The allowance is deliberately tight.
	const maxOther = 12
	if worth["other"] > maxOther {
		t.Errorf("%d unbounded addresses fall into \"other\" (allowance %d) — the prover has grown a "+
			"refusal reason this census cannot name, so every figure above understates some axis. "+
			"Add the class to class() and re-measure the table in docs/capability-gap-audit.md",
			worth["other"], maxOther)
	}
	if base == 0 || len(all) == 0 {
		t.Fatal("measured nothing — the corpus resolved but produced no regions")
	}
}

func itoaN(n int) string {
	if n == 0 {
		return "0"
	}
	var b [8]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
