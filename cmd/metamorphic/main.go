// Command metamorphic checks a metamorphic relation between two scenario runs:
// A.field <rel> B.field. Oracle-free verification of a relation (e.g. a longer
// run never scores less than its shorter prefix). docs/testing-playbook.md.
//
//	go run ./cmd/metamorphic -a short.json -b long.json -field ram.0x85 -rel "<="
//
// exit 0 if the relation holds, 1 if it is violated.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/kidsnz/atari2600-harness/internal/metamorphic"
)

func main() {
	a := flag.String("a", "", "scenario A (json)")
	b := flag.String("b", "", "scenario B (json)")
	field := flag.String("field", "", "field to compare at end of each run (e.g. ram.0x85)")
	rel := flag.String("rel", "<=", "relation A.field <rel> B.field (== != < <= > >=)")
	flag.Parse()
	if *a == "" || *b == "" || *field == "" {
		fmt.Fprintln(os.Stderr, "usage: metamorphic -a A.json -b B.json -field ram.0xNN -rel '<='")
		os.Exit(2)
	}
	out, err := metamorphic.Eval(*a, *b, *field, *rel)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(2)
	}
	status := "HOLDS"
	if !out.Pass {
		status = "VIOLATED"
	}
	fmt.Printf("%s  %s\n", status, out.Desc)
	if !out.Pass {
		os.Exit(1)
	}
}
