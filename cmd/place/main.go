// place — can a row of shapes be drawn on one scanline, and if so how are the objects strobed.
//
//	place -at 3,35,67,99,131            # one pass of a ten-shape row starting at x=3
//	place -at 2,34,66,98,130            # the same row one pixel left, where no player reaches
//	place -at 2 -nosolid 0              # and why that one cannot be drawn at all
//
// The shapes are the ones drawn on ONE scanline: in a staggered row that is every other letter,
// so they are twice the pitch apart. -nosolid marks a shape whose halves are not uniform enough
// for a missile to stand in for either of them.
//
// This answers PLACEMENT only -- where the objects go. Whether the line then has the cycles to
// write every shape's bytes is the kernel's own question; prove_line_budget answers that one.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/kidsnz/atari2600-harness/internal/place"
)

func ints(s string) []int {
	var out []int
	for _, f := range strings.Split(s, ",") {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		v, err := strconv.Atoi(f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "not a number: %q\n", f)
			os.Exit(2)
		}
		out = append(out, v)
	}
	return out
}

func main() {
	at := flag.String("at", "", "comma-separated left edges of the shapes on ONE scanline")
	noSolid := flag.String("nosolid", "", "indexes of shapes whose halves are not solid-safe")
	noLeft := flag.String("nosolid-left", "", "indexes whose LEFT half is not solid-safe")
	noRight := flag.String("nosolid-right", "", "indexes whose RIGHT half is not solid-safe")
	first := flag.Int("first-cycle", 0, "earliest write cycle the caller's blank line can strobe on")
	last := flag.Int("last-cycle", 0, "latest write cycle it can strobe on")
	asJSON := flag.Bool("json", false, "emit the plan as JSON")
	flag.Parse()

	xs := ints(*at)
	if len(xs) == 0 {
		flag.Usage()
		os.Exit(2)
	}
	shapes := make([]place.Shape, len(xs))
	for i, x := range xs {
		shapes[i] = place.Shape{X: x, SolidLeft: true, SolidRight: true}
	}
	for _, i := range append(ints(*noSolid), ints(*noLeft)...) {
		if i >= 0 && i < len(shapes) {
			shapes[i].SolidLeft = false
		}
	}
	for _, i := range append(ints(*noSolid), ints(*noRight)...) {
		if i >= 0 && i < len(shapes) {
			shapes[i].SolidRight = false
		}
	}

	p, err := place.Solve(shapes, *first, *last)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if *asJSON {
		b, _ := json.MarshalIndent(p, "", "  ")
		fmt.Println(string(b))
		return
	}
	fmt.Printf("%d shapes placed\n", len(shapes))
	for i, sh := range shapes {
		fmt.Printf("  x=%-4d %s\n", sh.X, p.Splits[i])
	}
	for _, o := range p.Objects {
		fmt.Printf("  %-6s base %-4d NUSIZ $%02X  strobe on write cycle %-3d draws at %v\n",
			o.Reg, o.Base, o.Nusiz, o.Cycle, o.Copies)
	}
}
