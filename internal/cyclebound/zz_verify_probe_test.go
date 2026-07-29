package cyclebound

import (
	"fmt"
	"os"
	"sort"
	"testing"

	"github.com/jetsetilly/gopher2600/hardware/cpu/instructions"
)

// TEMPORARY verification probe — delete after use.
func proveBinLike(t *testing.T, path string, budget int) {
	rom, err := os.ReadFile(path)
	if err != nil {
		t.Logf("%s: %v", path, err)
		return
	}
	if len(rom) < 6 || len(rom) > 0x10000 {
		fmt.Printf("%-28s size=%d REJECTED by Prove size guard\n", path, len(rom))
		return
	}
	p := &program{rom: rom, base: uint16(0x10000 - len(rom))}
	instrs := map[uint16]Instr{}
	var entries []uint16
	var vecs []string
	for _, va := range []uint16{0xFFFC, 0xFFFA, 0xFFFE} {
		lo, _ := p.byteAt(va)
		hi, _ := p.byteAt(va + 1)
		tt := uint16(lo) | uint16(hi)<<8
		vecs = append(vecs, fmt.Sprintf("%04X->%04X", va, tt))
		if tt >= p.base {
			p.decodeInto(instrs, tt)
			entries = append(entries, tt)
		}
	}
	states := computeStates(instrs, entries, p.byteAt)
	var starts []uint16
	for a, in := range instrs {
		if in.isWSYNC() {
			starts = append(starts, a)
		}
	}
	sort.Slice(starts, func(i, j int) bool { return starts[i] < starts[j] })
	rep := &Report{Budget: budget}
	for _, sa := range starts {
		reg := analyzeRegion(instrs, instrs[sa], budget, 0, nil, states)
		rep.Regions++
		if reg.Kind == "blank" {
			rep.Blank++
			continue
		}
		if !reg.Bounded {
			rep.Unbounded = append(rep.Unbounded, reg)
			continue
		}
		if reg.Worst > rep.MaxWorst {
			rep.MaxWorst = reg.Worst
		}
		if reg.Over {
			rep.Violations = append(rep.Violations, reg)
		}
	}
	cert := rep.Regions > 0 && len(rep.Violations) == 0 && len(rep.Unbounded) == 0
	fmt.Printf("%-52s size=%5d base=$%04X vec=%v instrs=%4d regions=%2d blank=%2d unb=%2d viol=%2d maxworst=%3d certified=%v\n",
		path[len(path)-minz(len(path), 40):], len(rom), p.base, vecs, len(instrs), rep.Regions, rep.Blank, len(rep.Unbounded), len(rep.Violations), rep.MaxWorst, cert)
}

func minz(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func TestZZProbe2(t *testing.T) {
	roms := []string{
		"../../roms/litmus/smoke.bin",
		"../../roms/litmus/cb_clean.bin",
		"/Users/shinji/Documents/2D/260609_atari2600-dev/reference/roms-study/VideoOlympics.bin",
		"/Users/shinji/Documents/2D/260609_atari2600-dev/reference/pizza-boy/Samples for Pizza Boy/Adventure.bin",
		"/Users/shinji/Documents/2D/260609_atari2600-dev/reference/pizza-boy/Samples for Pizza Boy/Seaquest.bin",
		"/Users/shinji/Documents/2D/260609_atari2600-dev/reference/pizza-boy/Samples for Pizza Boy/Star Wars - The Empire Strikes Back.bin",
		"/Users/shinji/Documents/2D/260609_atari2600-dev/reference/pizza-boy/Samples for Pizza Boy/Chopper Command.bin",
		"/Users/shinji/Documents/2D/260609_atari2600-dev/reference/pizza-boy/Samples for Pizza Boy/Stampede.bin",
		"/Users/shinji/Documents/2D/260609_atari2600-dev/reference/pizza-boy/Samples for Pizza Boy/Barnstorming.bin",
		"/Users/shinji/Documents/2D/260609_atari2600-dev/reference/2600-technique-sources/sidescroll/scrolldemo_20150210.bin",
	}
	for _, r := range roms {
		proveBinLike(t, r, 76)
	}
}

// Does the mirror fix actually recover VideoOlympics?
func TestZZMirrorFix(t *testing.T) {
	path := "/Users/shinji/Documents/2D/260609_atari2600-dev/reference/roms-study/VideoOlympics.bin"
	rom, err := os.ReadFile(path)
	if err != nil {
		t.Skip(err)
	}
	// folded byteAt
	folded := func(addr uint16) (byte, bool) {
		if addr&0x1000 == 0 {
			return 0, false
		}
		return rom[int(addr)&(len(rom)-1)], true
	}
	instrs := map[uint16]Instr{}
	p := &program{rom: rom, base: uint16(0x10000 - len(rom))}
	_ = p
	// re-implement decodeInto with folded byteAt
	var decodeAt func(uint16) (Instr, bool)
	decodeAt = func(addr uint16) (Instr, bool) {
		op, ok := folded(addr)
		if !ok {
			return Instr{}, false
		}
		in := Instr{Addr: addr, Op: op}
		in.Def = instructions.Definitions[op]
		switch in.Def.Bytes {
		case 2:
			b1, _ := folded(addr + 1)
			in.Operand = uint16(b1)
		case 3:
			lo, _ := folded(addr + 1)
			hi, _ := folded(addr + 2)
			in.Operand = uint16(lo) | uint16(hi)<<8
		}
		return in, true
	}
	var walk func(uint16)
	walk = func(entry uint16) {
		work := []uint16{entry}
		for len(work) > 0 {
			a := work[len(work)-1]
			work = work[:len(work)-1]
			if _, seen := instrs[a]; seen {
				continue
			}
			in, ok := decodeAt(a)
			if !ok {
				continue
			}
			instrs[a] = in
			work = append(work, decodeSuccessors(in)...)
		}
	}
	lo, _ := folded(0xFFFC)
	hi, _ := folded(0xFFFD)
	reset := uint16(lo) | uint16(hi)<<8
	walk(reset)
	nw := 0
	for _, in := range instrs {
		if in.isWSYNC() {
			nw++
		}
	}
	fmt.Printf("VideoOlympics mirror-folded: reset=$%04X instrs=%d wsync_regions=%d\n", reset, len(instrs), nw)
}
