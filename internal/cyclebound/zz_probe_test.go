package cyclebound

import (
	"fmt"
	"os"
	"testing"

	"github.com/jetsetilly/gopher2600/hardware/cpu/instructions"
)

type prog2 struct {
	rom  []byte
	mask uint16
}

// naive fold exactly as the catalog's option A proposes: addr & (len(rom)-1)
func (p *prog2) at(addr uint16) (byte, bool) {
	return p.rom[addr&p.mask], true
}

func (p *prog2) dec(addr uint16) (Instr, bool) {
	op, _ := p.at(addr)
	in := Instr{Addr: addr, Op: op, Def: instructions.Definitions[op]}
	switch in.Def.Bytes {
	case 2:
		b1, _ := p.at(addr + 1)
		in.Operand = uint16(b1)
	case 3:
		lo, _ := p.at(addr + 1)
		hi, _ := p.at(addr + 2)
		in.Operand = uint16(lo) | uint16(hi)<<8
	}
	return in, true
}

func (p *prog2) walk(m map[uint16]Instr, entry uint16) {
	work := []uint16{entry}
	for len(work) > 0 {
		a := work[len(work)-1]
		work = work[:len(work)-1]
		if _, seen := m[a]; seen {
			continue
		}
		in, _ := p.dec(a)
		m[a] = in
		work = append(work, decodeSuccessors(in)...)
	}
}

func TestZZProbe(t *testing.T) {
	roms := []string{
		"/Users/shinji/Documents/2D/260609_atari2600-dev/reference/roms-study/VideoOlympics.bin",
		"/Users/shinji/Documents/2D/260609_atari2600-dev/reference/pizza-boy/Samples for Pizza Boy/Adventure.bin",
		"/Users/shinji/Documents/2D/260609_atari2600-dev/reference/pizza-boy/Samples for Pizza Boy/Seaquest.bin",
		"/Users/shinji/Documents/2D/260609_atari2600-dev/reference/pizza-boy/Samples for Pizza Boy/Star Wars - The Empire Strikes Back.bin",
		"/Users/shinji/Documents/2D/260609_atari2600-dev/reference/pizza-boy/Samples for Pizza Boy/Chopper Command.bin",
	}
	for _, r := range roms {
		rom, err := os.ReadFile(r)
		if err != nil {
			t.Logf("SKIP %s: %v", r, err)
			continue
		}
		// current behaviour
		p := &program{rom: rom, base: uint16(0x10000 - len(rom))}
		cur := map[uint16]Instr{}
		var vecs []uint16
		for _, va := range []uint16{0xFFFC, 0xFFFA, 0xFFFE} {
			lo, _ := p.byteAt(va)
			hi, _ := p.byteAt(va + 1)
			tg := uint16(lo) | uint16(hi)<<8
			vecs = append(vecs, tg)
			if tg >= p.base {
				p.decodeInto(cur, tg)
			}
		}
		// proposed naive-mask behaviour
		q := &prog2{rom: rom, mask: uint16(len(rom) - 1)}
		fix := map[uint16]Instr{}
		for _, va := range []uint16{0xFFFC, 0xFFFA, 0xFFFE} {
			lo, _ := q.at(va)
			hi, _ := q.at(va + 1)
			q.walk(fix, uint16(lo)|uint16(hi)<<8)
		}
		outside := 0
		for a := range fix {
			if a < 0x1000 || (a&0x1000) == 0 {
				outside++
			}
		}
		fmt.Printf("%-30s size=%5d vectors=%v  current=%4d  naive-mask=%4d  decoded-outside-cart=%d\n",
			r[len(r)-28:], len(rom), fmtv(vecs), len(cur), len(fix), outside)
	}
}

func fmtv(v []uint16) string {
	s := ""
	for _, x := range v {
		s += fmt.Sprintf("$%04X ", x)
	}
	return s
}
