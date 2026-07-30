package cyclebound

import (
	"strings"
	"testing"

	"github.com/jetsetilly/gopher2600/hardware/cpu/instructions"
)

// The switch oracle has to see mechanisms a data-access test cannot:
//
//   - control transferred INTO a hotspot: `jmp $1FF9` never reads $1FF9 as data,
//     it sets PC there, and the instruction fetch at that address is the
//     cartridge read that selects the bank. Gopher2600 classifies jmp/jsr as
//     Subroutine/Flow rather than Read, so accessOf answers "touches no memory"
//     for 33 of the three-byte opcodes, jmp and jsr among them;
//   - an instruction whose OWN bytes span a hotspot, so the fetch switches
//     mid-instruction and the opcode that executes comes from the NEW bank. That
//     one is a measured bug in this repo already — banked_game.asm records a
//     reboot loop caused by putting `rts` on $FFF9.
//
// Missing either is the unsound direction: the region is not refused,
// UnmodelledSwitches never counts it, and a cartridge that leaves for a bank the
// analysis never entered can come back CERTIFIED.
//
// Neither has a ROM witness in the corpus, which is stated rather than hidden: the
// Atari mappers put their hotspots directly beneath the vectors, so there is no room
// for a landing site, and a ROM built to span one would be deliberately broken. The
// predicate is therefore tested directly, on constructed input.
//
// The table is split by what the flow model now DOES: a data access that reaches a
// hotspot is MODELLED (it gets a cross-bank edge and no refusal), while the fetch-side
// mechanisms above stay REFUSED. A case in the wrong column is a real defect either
// way round — a refusal for something modelled is a permanent false negative, and a
// modelled edge for something unmodellable is unsound.
func TestSwitchOracleSeesControlFlowAndFetch(t *testing.T) {
	sw := switchModel{
		banked:   true,
		hotspots: map[uint16]string{0x1FF8: "BANK0", 0x1FF9: "BANK1"},
		banks:    map[int]bool{0: true, 1: true},
	}

	// A WSYNC opens the region; the instruction under test follows; a second WSYNC
	// closes it. Addresses are in the $F000 mirror, exactly as a decode produces.
	// The opening WSYNC sits immediately before the instruction under test, so the
	// region genuinely contains it however high in the address space it lives.
	build := func(mid Instr) (map[site]Instr, site) {
		openAddr := mid.Addr - 3
		open := Instr{Addr: openAddr, Op: 0x8D, Def: instructions.Definitions[0x8D], Operand: 0x0002}
		shut := Instr{Addr: mid.Addr + mid.size(), Op: 0x8D, Def: instructions.Definitions[0x8D], Operand: 0x0002}
		return map[site]Instr{open.site(): open, mid.site(): mid, shut.site(): shut}, open.site()
	}

	cases := []struct {
		name string
		mid  Instr
		want string // substring the refusal must contain, or "" for "modelled, no refusal"
	}{
		// --- still refused: the fetch itself selects the bank ---
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
			name: "indirect store under a hotspot-bearing mapper",
			// (ind),Y: the pointer lives in RAM, so no address, no symbol, no bank.
			mid:  Instr{Addr: 0xF003, Op: 0x91, Def: instructions.Definitions[0x91], Operand: 0x0084},
			want: "target cannot be resolved",
		},
		// --- modelled now: a data access that reaches a hotspot gets an edge ---
		{
			name: "lda from a hotspot (the canonical switch)",
			mid:  Instr{Addr: 0xF003, Op: 0xAD, Def: instructions.Definitions[0xAD], Operand: 0xFFF9},
			want: "",
		},
		{
			name: "sta to a hotspot",
			mid:  Instr{Addr: 0xF003, Op: 0x8D, Def: instructions.Definitions[0x8D], Operand: 0x1FF8},
			want: "",
		},
		// --- neither: nothing about this instruction touches a hotspot ---
		{
			name: "jmp to an ordinary address",
			mid:  Instr{Addr: 0xF003, Op: 0x4C, Def: instructions.Definitions[0x4C], Operand: 0xF100},
			want: "",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			instrs, start := build(c.mid)
			got, _ := residualSwitchRefusal(instrs, start, sw, map[site]State{})
			switch {
			case c.want == "" && got != "":
				t.Errorf("refused a switch the flow model claims to handle: %q — a permanent false "+
					"negative, since no better decoder can lift it", got)
			case c.want != "" && got == "":
				t.Errorf("did NOT refuse a region that switches banks in an unmodellable way — this is " +
					"the unsound direction: the region would be bounded, UnmodelledSwitches would not " +
					"count it, and the cartridge could be certified while leaving for a bank whose " +
					"bytes were never even decoded")
			case c.want != "" && !strings.Contains(got, c.want):
				t.Errorf("refusal does not name the mechanism; want a mention of %q, got %q", c.want, got)
			}
		})
	}
}

// A modelled switch must produce an EDGE, not merely the absence of a refusal.
// Without this, deleting the edge-building code would still leave the table above
// green while the longest-path walk quietly followed the wrong bytes.
func TestModelledSwitchProducesTheEdgeToTheTargetBank(t *testing.T) {
	sw := switchModel{
		banked:   true,
		hotspots: map[uint16]string{0x1FF8: "BANK0", 0x1FF9: "BANK1"},
		banks:    map[int]bool{0: true, 1: true},
	}
	// bank 0 $FF00: lda $FFF9 — 3 bytes, so the next fetch is $FF03, in bank 1.
	in := Instr{Bank: 0, Addr: 0xFF00, Op: 0xAD, Def: instructions.Definitions[0xAD], Operand: 0xFFF9}
	edges, keep, refusal := sw.switchEdges(in, topState())
	if refusal != "" {
		t.Fatalf("the canonical switch was refused: %s", refusal)
	}
	if len(edges) != 1 || edges[0] != (site{1, 0xFF03}) {
		t.Fatalf("edges = %v, want exactly [bank 1 $FF03] — the same address in the bank the mapper's "+
			"own symbol names", edges)
	}
	if keep {
		t.Error("the intra-bank fall-through was kept for an EXACT hotspot access: the mapper switches on " +
			"that access, so bank 0 $FF03 is not executed and costing it would inflate or misroute the path")
	}
	// A bank the analysis does not hold must be refused rather than fanned out to.
	narrow := switchModel{banked: true, hotspots: sw.hotspots, banks: map[int]bool{0: true}}
	if _, _, why := narrow.switchEdges(in, topState()); why == "" {
		t.Error("a hotspot naming a bank outside the analysed set was not refused; modelling the " +
			"resolvable candidates and dropping the rest deletes a successor, and deleting a successor " +
			"shortens the longest path")
	}
}

// A cartridge with no hotspots at all must never be refused for switching banks,
// or the check would refuse everything and be sound but useless.
func TestSwitchOracleIsSilentWithoutHotspots(t *testing.T) {
	mid := Instr{Addr: 0xF003, Op: 0x4C, Def: instructions.Definitions[0x4C], Operand: 0xFFF9}
	open := Instr{Addr: 0xF000, Op: 0x8D, Def: instructions.Definitions[0x8D], Operand: 0x0002}
	shut := Instr{Addr: 0xF006, Op: 0x8D, Def: instructions.Definitions[0x8D], Operand: 0x0002}
	instrs := map[site]Instr{open.site(): open, mid.site(): mid, shut.site(): shut}
	for _, sw := range []switchModel{
		{},             // flat image
		{banked: true}, // banked, mapper publishes nothing
		{banked: false, hotspots: map[uint16]string{0x1FF9: "BANK1"}}, // hotspots but one bank
	} {
		if got, _ := residualSwitchRefusal(instrs, open.site(), sw, map[site]State{}); got != "" {
			t.Errorf("refused with switchModel %+v (active=%v): %q", sw, sw.active(), got)
		}
	}
}
