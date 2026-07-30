// cpucheck — VV-7 silicon CPU differential (docs/capability-gap-audit.md).
//
// Runs generated single-instruction vectors on BOTH the embedded Gopher2600 CPU
// core and the perfect6502 transistor netlist (bin/p6502step) and reports any
// divergence in registers, cycle count, or memory writes. This is a CPU-LAYER
// oracle: it is NOT part of the full-system RAM vote (cmd/oraclevote), because
// perfect6502 has no TIA/RIOT and cannot run a 2600 ROM. Its value is catching a
// CPU bug shared by the software emulators, and covering undocumented opcodes /
// decimal edges that the fixed Tom Harte corpus (VV-1) excludes.
//
// Requires bin/p6502step (scripts/install_perfect6502.sh). Exit codes:
//
//	0  agreement (modulo expected divergences), or harness not built (note)
//	1  at least one UNEXPECTED divergence (a Gopher bug or a harness artifact)
//	2  usage / runtime error
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"

	"github.com/kidsnz/atari2600-harness/internal/cpudiff"
)

// mustClass names the allow-list class of an opcode known to be on it.
func mustClass(op byte) string {
	c, _ := cpudiff.ExpectedDivergence(op)
	return c
}

func main() {
	seed := flag.Int64("seed", 1, "PRNG seed (deterministic)")
	n := flag.Int("n", 5000, "number of random vectors")
	set := flag.String("opcodes", "all", "opcode set: all | smoke")
	verbose := flag.Bool("v", false, "print every unexpected divergence")
	flag.Parse()

	exe := cpudiff.FindP6502()
	if exe == "" {
		fmt.Fprintln(os.Stderr, "p6502step not built — run scripts/install_perfect6502.sh for the silicon cross-check")
		fmt.Println(`{"skipped":"p6502step not built"}`)
		return // exit 0: nothing to check, not a failure
	}

	var ops []byte
	switch *set {
	case "all":
		ops = cpudiff.AllOpcodes()
	case "smoke":
		ops = cpudiff.DocumentedSmoke
	default:
		fmt.Fprintf(os.Stderr, "unknown -opcodes %q (want all|smoke)\n", *set)
		os.Exit(2)
	}

	vs := cpudiff.GenVectors(*seed, *n, ops)
	sil, err := cpudiff.RunP6502Batch(exe, vs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: p6502step: %v\n", err)
		os.Exit(2)
	}

	agreed := 0
	expected := map[string]int{} // class -> count
	expectedByOp := map[byte]int{}
	testedByOp := map[byte]int{}
	unexpectedByOp := map[byte]int{}
	unexpected := 0
	for i := range vs {
		op := vs[i].Opcode()
		testedByOp[op]++
		gop, err := cpudiff.RunGopher(vs[i])
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: gopher vector %d: %v\n", i, err)
			os.Exit(2)
		}
		if cpudiff.HaltEquivalent(gop.Status, sil[i].Status) {
			expected["halt"]++
			continue
		}
		d := cpudiff.Compare(gop, sil[i])
		if len(d) == 0 {
			agreed++
			continue
		}
		if class, ok := cpudiff.ExpectedDivergence(op); ok {
			expected[class]++
			expectedByOp[op]++
			continue
		}
		unexpected++
		unexpectedByOp[op]++
		if *verbose {
			fmt.Fprintf(os.Stderr, "DIVERGE op %02X regs[A=%02X X=%02X Y=%02X S=%02X P=%02X] ops[%02X %02X]: %v\n",
				op, vs[i].A, vs[i].X, vs[i].Y, vs[i].S, vs[i].P, vs[i].Mem[0xF801], vs[i].Mem[0xF802], d)
		}
	}

	var unexpOps []string
	for op := range unexpectedByOp {
		unexpOps = append(unexpOps, fmt.Sprintf("%02X(%d)", op, unexpectedByOp[op]))
	}
	sort.Strings(unexpOps)

	// Report the allow-list's own denominator. An entry that is permitted to diverge
	// and never does is a hole with no witness: it silences a whole opcode, so a real
	// engine bug there would be waved through under the label "known unstable". The
	// counts say which entries earned their place in this run, and unexercised says
	// which did not.
	allowed, unexercised := cpudiff.ExpectedDivergenceOpcodes(), []string(nil)
	var allowOps []string
	for _, op := range allowed {
		allowOps = append(allowOps, fmt.Sprintf("%02X:%s(%d/%d)", op,
			mustClass(op), expectedByOp[op], testedByOp[op]))
		if testedByOp[op] > 0 && expectedByOp[op] == 0 {
			unexercised = append(unexercised, fmt.Sprintf("%02X:%s(0/%d)", op, mustClass(op), testedByOp[op]))
		}
	}

	out := struct {
		Seed         int64          `json:"seed"`
		Vectors      int            `json:"vectors"`
		Opcodes      string         `json:"opcodes"`
		Agreed       int            `json:"agreed"`
		Expected     map[string]int `json:"expected_divergences"`
		Unexpected   int            `json:"unexpected_divergences"`
		UnexpectedBy []string       `json:"unexpected_opcodes,omitempty"`
		AllowList    []string       `json:"allow_list_diverged_over_tested"`
		Unexercised  []string       `json:"allow_list_never_diverged,omitempty"`
	}{*seed, *n, *set, agreed, expected, unexpected, unexpOps, allowOps, unexercised}
	b, _ := json.MarshalIndent(out, "", "  ")
	fmt.Println(string(b))

	if unexpected > 0 {
		fmt.Fprintf(os.Stderr, "FAIL: %d unexpected silicon divergences on opcodes %v\n", unexpected, unexpOps)
		os.Exit(1)
	}
	if len(unexercised) > 0 {
		fmt.Fprintf(os.Stderr, "NOTE: %d allow-list entries never diverged in this run and are silencing "+
			"an opcode for nothing: %v\n", len(unexercised), unexercised)
	}
	fmt.Fprintf(os.Stderr, "OK: %d agreed, %d expected divergences (allow-list %d entries), 0 unexpected\n",
		agreed, sum(expected), len(allowed))
}

func sum(m map[string]int) int {
	t := 0
	for _, v := range m {
		t += v
	}
	return t
}
