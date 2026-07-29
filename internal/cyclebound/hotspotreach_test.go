package cyclebound

import (
	"strings"
	"testing"

	"github.com/jetsetilly/gopher2600/hardware/cpu/instructions"
)

// Hotspot detection used to run entirely through accessOf, which answers "this
// instruction touches no memory" for 33 of the three-byte opcodes — jmp and jsr
// among them. Missing a switch is the unsound direction: the region is not
// refused, UnmodelledSwitches never counts it, and a cartridge that leaves for a
// bank the analysis never entered can come back CERTIFIED.
//
// Two mechanisms are checked here that a data-access test cannot see:
//
//   - control transferred INTO a hotspot: `jmp $1FF9` never reads $1FF9 as data,
//     it sets PC there, and the instruction fetch at that address is the
//     cartridge read that selects the bank;
//   - an instruction whose OWN bytes span a hotspot, so the fetch switches
//     mid-instruction. That one is a measured bug in this repo already —
//     banked_game.asm records a reboot loop caused by putting `rts` on $FFF9.
//
// Neither has a ROM witness in the corpus, which is stated rather than hidden:
// the Atari mappers put their hotspots directly beneath the vectors, so there is
// no room for a landing site, and a ROM built to span one would be deliberately
// broken. The predicate is therefore tested directly, on constructed input.
func TestHotspotRefusalSeesControlFlowAndFetch(t *testing.T) {
	hotspots := map[uint16]string{0x1FF8: "BANK0", 0x1FF9: "BANK1"}

	// A WSYNC opens the region; the instruction under test follows; a second WSYNC
	// closes it. Addresses are in the $F000 mirror, exactly as a decode produces.
	// The opening WSYNC sits immediately before the instruction under test, so the
	// region genuinely contains it however high in the address space it lives.
	build := func(mid Instr) (map[uint16]Instr, uint16) {
		openAddr := mid.Addr - 3
		open := Instr{Addr: openAddr, Op: 0x8D, Def: instructions.Definitions[0x8D], Operand: 0x0002}
		shut := Instr{Addr: mid.Addr + mid.size(), Op: 0x8D, Def: instructions.Definitions[0x8D], Operand: 0x0002}
		return map[uint16]Instr{open.Addr: open, mid.Addr: mid, shut.Addr: shut}, openAddr
	}

	cases := []struct {
		name string
		mid  Instr
		want string // substring the refusal must contain, or "" for no refusal
	}{
		{
			name: "jmp into a hotspot",
			mid:  Instr{Addr: 0xF003, Op: 0x4C, Def: instructions.Definitions[0x4C], Operand: 0xFFF9},
			want: "transfers control to BANK1",
		},
		{
			name: "jsr into a hotspot",
			mid:  Instr{Addr: 0xF003, Op: 0x20, Def: instructions.Definitions[0x20], Operand: 0x1FF9},
			want: "transfers control to BANK1",
		},
		{
			name: "instruction fetched across a hotspot",
			// A three-byte instruction at $FFF7 has its operand bytes on $FFF8/$FFF9.
			mid:  Instr{Addr: 0xFFF7, Op: 0xAD, Def: instructions.Definitions[0xAD], Operand: 0x0080},
			want: "fetched across BANK0",
		},
		{
			name: "jmp to an ordinary address",
			mid:  Instr{Addr: 0xF003, Op: 0x4C, Def: instructions.Definitions[0x4C], Operand: 0xF100},
			want: "",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			instrs, start := build(c.mid)
			got := hotspotRefusal(instrs, start, hotspots, map[uint16]State{})
			switch {
			case c.want == "" && got != "":
				t.Errorf("refused a region that switches no bank: %q", got)
			case c.want != "" && got == "":
				t.Errorf("did NOT refuse a region that switches banks — this is the unsound direction: " +
					"the region would be bounded, UnmodelledSwitches would not count it, and the " +
					"cartridge could be certified while leaving for a bank never analysed")
			case c.want != "" && !strings.Contains(got, c.want):
				t.Errorf("refusal does not name the mechanism; want a mention of %q, got %q", c.want, got)
			}
		})
	}
}

// A cartridge with no hotspots at all must never be refused for switching banks,
// or the check would refuse everything and be sound but useless.
func TestHotspotRefusalIsSilentWithoutHotspots(t *testing.T) {
	mid := Instr{Addr: 0xF003, Op: 0x4C, Def: instructions.Definitions[0x4C], Operand: 0xFFF9}
	open := Instr{Addr: 0xF000, Op: 0x8D, Def: instructions.Definitions[0x8D], Operand: 0x0002}
	shut := Instr{Addr: 0xF006, Op: 0x8D, Def: instructions.Definitions[0x8D], Operand: 0x0002}
	instrs := map[uint16]Instr{open.Addr: open, mid.Addr: mid, shut.Addr: shut}
	if got := hotspotRefusal(instrs, 0xF000, nil, map[uint16]State{}); got != "" {
		t.Errorf("refused on a cartridge with no bank-switch hotspots: %q", got)
	}
}
