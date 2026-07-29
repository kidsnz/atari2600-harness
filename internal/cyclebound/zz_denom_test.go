package cyclebound

import (
	"fmt"
	"os"
	"testing"

	"github.com/kidsnz/atari2600-harness/internal/emu"
)

// Measures the quality of the STATIC denominator option A proposes:
// keys(decodeInto) MINUS executed PCs = "statically reachable but never executed".
func TestZZDenominator(t *testing.T) {
	roms := []string{
		"/Users/shinji/Documents/2D/260609_atari2600-dev/reference/pizza-boy/Samples for Pizza Boy/Adventure.bin",
		"/Users/shinji/Documents/2D/260609_atari2600-dev/reference/pizza-boy/Samples for Pizza Boy/Seaquest.bin",
		"/Users/shinji/Documents/2D/260609_atari2600-dev/reference/pizza-boy/Samples for Pizza Boy/Chopper Command.bin",
		"/Users/shinji/Documents/2D/260609_atari2600-dev/reference/pizza-boy/Samples for Pizza Boy/Barnstorming.bin",
	}
	for _, r := range roms {
		rom, err := os.ReadFile(r)
		if err != nil {
			continue
		}
		p := &program{rom: rom, base: uint16(0x10000 - len(rom))}
		instrs := map[uint16]Instr{}
		for _, va := range []uint16{0xFFFC, 0xFFFA, 0xFFFE} {
			lo, _ := p.byteAt(va)
			hi, _ := p.byteAt(va + 1)
			tt := uint16(lo) | uint16(hi)<<8
			if tt >= p.base {
				p.decodeInto(instrs, tt)
			}
		}
		e, err := emu.New("NTSC")
		if err != nil {
			t.Fatal(err)
		}
		if err := e.LoadROM(r); err != nil {
			t.Fatal(err)
		}
		e.EnableCoverage()
		if err := e.RunFrames(300); err != nil {
			t.Fatal(err)
		}
		seen := map[uint16]bool{}
		for _, a := range e.Coverage().SeenPCs() {
			seen[a] = true
		}
		// phantom detector: a decoded start that lies strictly INSIDE an EXECUTED
		// instruction cannot be real code (the CPU proved that byte is an operand).
		execSpan := map[uint16]bool{} // operand bytes of executed instructions
		for a := range seen {
			in, ok := instrs[a]
			if !ok {
				continue
			}
			for i := 1; i < in.Def.Bytes; i++ {
				execSpan[a+uint16(i)] = true
			}
		}
		notExec, phantom := 0, 0
		for a := range instrs {
			if seen[a] {
				continue
			}
			notExec++
			if execSpan[a] {
				phantom++
			}
		}
		execNotDecoded := 0
		for a := range seen {
			if _, ok := instrs[a]; !ok {
				execNotDecoded++
			}
		}
		fmt.Printf("%-30s decoded=%4d executed=%4d  never-exec=%4d  of which PROVABLY-phantom=%3d (%.0f%%)  executed-but-NOT-decoded=%d\n",
			r[len(r)-28:], len(instrs), len(seen), notExec, phantom,
			100*float64(phantom)/float64(max1(notExec)), execNotDecoded)
	}
}

func max1(n int) int {
	if n == 0 {
		return 1
	}
	return n
}
