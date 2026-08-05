// Package cyclebound is a STATIC per-scanline cycle-budget prover (VV-2). Where
// assert_line_budget OBSERVES one execution and reports whether that run's
// WSYNC interval overran 76 cycles (∃ — an existential, one-path claim), this
// package PROVES the worst case over ALL reachable paths (∀ — the only
// universal-claim member of the verification suite). It directly attacks gap B
// (timing): a kernel that overruns only on one branch can pass a lucky run yet
// roll the screen on hardware; the static proof flags it regardless.
//
// Method: assemble the .asm (internal/build), recursive-descent decode the ROM
// from its reset/IRQ/NMI vectors (so inline data tables aren't misdecoded),
// cost each instruction from the in-tree exact cycle table
// (instructions.Definitions: Cycles + page/branch penalties), cut the CFG at
// every `STA WSYNC` ($02 store) into WSYNC-to-WSYNC regions, and prove each
// region's longest reachable path <= budget via a DAG longest-path search.
// Counted loops inside a region (ldx/ldy #N … dex/dey … bne/bpl) are folded by
// their iteration bound; anything we cannot bound (unbounded loop, JSR,
// indirect JMP) is reported honestly as "unbounded — out of scope", never
// silently passed.
//
// Scope, stated as what the code does. A flat 2K/4K kernel is analysed as one
// unit. A bank-switched cartridge is analysed as ONE MERGED program whose code
// identity is (bank, address) — every bank occupies the same $F000-$FFFF window,
// so a bare address cannot tell two banks apart (measured: litmus_bank_f4 has 1399
// of 1427 decoded addresses claimed by 2+ banks). Cross-bank flow IS modelled for
// the shape the hardware actually uses: an instruction whose DATA access reaches a
// bank-switch hotspot continues at the same address in the bank that hotspot's own
// mapper symbol names. Still refused, counted in UnmodelledSwitches and blocking
// certification: an instruction whose OWN BYTES span a hotspot (the opcode comes
// from the new bank, so the decoded instruction is not the one that executes), a
// `jmp`/`jsr` INTO a hotspot, an access whose target cannot be resolved at all
// under a hotspot-bearing mapper, a hotspot symbol that does not name a bank
// ("B0S0", "RAM0"), a target bank outside the analysed set, and any mapper whose
// banks are not the whole 4K window at $F000 (declined before analysis). Indirect
// JMP and nested/multiple intra-line loops are reported as unbounded rather than
// guessed.
//
// Provenance: implicit-path-enumeration / longest-path WCET — Li & Malik, "Performance
// analysis of embedded software using implicit path enumeration", DAC 1995;
// Ballabriga & Cassé, WCET 2008. Exact cycle table: Gopher2600
// hardware/cpu/instructions. The 76-cycle line budget: 228 color clocks / 3.
package cyclebound

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/jetsetilly/gopher2600/hardware/cpu/instructions"
	"github.com/jetsetilly/gopher2600/hardware/memory/memorymap"
	"github.com/kidsnz/atari2600-harness/internal/build"
	"github.com/kidsnz/atari2600-harness/internal/emu"
	"github.com/kidsnz/atari2600-harness/internal/srcmap"
)

// DefaultBudget is one NTSC scanline = 228 color clocks / 3 = 76 CPU cycles
// (mirrors pkg/design.LineCycles; the runtime sibling assert_line_budget uses
// the same default).
const DefaultBudget = 76

// Instr is one statically decoded 6502 instruction.
//
// Bank is the bank whose bytes this instruction was decoded FROM. It is part of the
// instruction's identity, not decoration: every bank of a bank-switched cartridge
// is mapped into the same $F000-$FFFF window, so two banks routinely hold different
// code at one address (measured on litmus_bank_f4: 1399 of 1427 decoded addresses
// are claimed by 2 or more banks). Instr is never JSON-marshalled — only Region,
// Step, Access and BeamWrite are — so the field does not appear in any output.
type Instr struct {
	Bank    int
	Addr    uint16
	Op      byte
	Def     instructions.Definition
	Operand uint16 // 1- or 2-byte operand, by Def.Bytes (little-endian for 3-byte)
}

// site is the CODE identity used as the map key everywhere in this package: which
// bank, and where in it. It is comparable, so it is a legal map key.
//
// DATA addresses stay bare uint16 on purpose and the asymmetry is the whole point:
// RAM $80-$FF, TIA/RIOT $00-$3F and the page-1 stack mirrors are NOT banked, so a
// bank in a data key would split one physical cell into N keys and break the store
// kill-set, the value domain and the def-use may-sets.
type site struct {
	bank int
	addr uint16
}

func (in Instr) site() site     { return site{in.Bank, in.Addr} }
func (in Instr) nextSite() site { return site{in.Bank, in.next()} }

// branchSite is the taken destination of a relative branch. A branch cannot change
// bank: the only cartridge access it performs is its own fetch, and an instruction
// whose own bytes touch a hotspot is refused outright (see switchModel.switchEdges).
func (in Instr) branchSite() site { return site{in.Bank, in.branchTarget()} }

// lineAt resolves a code site to a source line, using the per-bank map when the
// image is banked and the flat one otherwise. Written once so the two annotation
// readers cannot drift apart.
func lineAt(sm *srcmap.Map, at site) (int, bool) {
	if ln, ok := sm.LineBank(at.bank, at.addr); ok {
		return ln, true
	}
	return sm.Line(at.addr)
}

// siteDesc renders a code site for a human. On a banked image the bank is part of
// the identity and must be printed; on a flat image there is only one, so printing
// "bank 0" would be noise — and would change the text of every existing report.
func siteDesc(s site, banked bool) string {
	if banked {
		return fmt.Sprintf("bank %d $%04X", s.bank, s.addr)
	}
	return fmt.Sprintf("$%04X", s.addr)
}

func sortSites(s []site) {
	sort.Slice(s, func(i, j int) bool {
		if s[i].bank != s[j].bank {
			return s[i].bank < s[j].bank
		}
		return s[i].addr < s[j].addr
	})
}

func (in Instr) size() uint16 {
	if in.Def.Bytes < 1 {
		return 1
	}
	return uint16(in.Def.Bytes)
}

func (in Instr) next() uint16 { return in.Addr + in.size() }

// branchTarget resolves a relative branch's destination (Def.Bytes == 2).
func (in Instr) branchTarget() uint16 {
	return in.next() + uint16(int8(byte(in.Operand)))
}

// isWSYNC reports a store to WSYNC ($02): STA/STX/STY, non-indexed (zero-page or
// absolute — Gopher2600 tags both as AddressingMode==Absolute, distinguished by
// Bytes), operand == $0002. Indexed (AbsoluteX/Y) is excluded: `sta WSYNC` is
// never indexed, and an indexed effective address can't be resolved statically.
func (in Instr) isWSYNC() bool {
	switch in.Def.Operator {
	case instructions.STA, instructions.STX, instructions.STY:
	default:
		return false
	}
	if in.Def.AddressingMode != instructions.Absolute {
		return false
	}
	return in.Operand == 0x0002
}

// nodeCost is the worst-case cycle cost of executing this instruction, EXCLUDING
// branch taken/page penalties (those are applied on the CFG edges). A
// page-sensitive non-branch (an indexed read like LDA abs,X) is charged its +1
// worst case conservatively, since the index is generally unknown statically.
// Stores are never page-sensitive (6502 rule), so WSYNC stores cost exactly 3.
func (in Instr) nodeCost() int {
	c := in.Def.Cycles
	if in.Def.PageSensitive && !in.Def.IsBranch() {
		c++
	}
	return c
}

// --- ROM image + recursive-descent decode ---

type program struct {
	rom  []byte
	base uint16 // first ROM address = 0x10000 - len(rom) (0xF000 for 4K, 0xF800 for 2K)

	// bank is the bank number this image IS. Every Instr decoded from it is stamped
	// with it, which is the single point where bank identity enters the analysis.
	bank int

	// banked marks an image larger than the 4K the console can address at once,
	// i.e. one that switches banks at run time. Everything in this package keys
	// on a flat address, so on such an image the vectors, the decode and every
	// analysis built on them describe a program that does not exist. Callers must
	// decline rather than report — measured on banked_game.asm, the flat model
	// decoded 66 instructions from entry points $FFE0/$FFFF and produced a
	// confident finding about an address it had never decoded.
	banked bool

	// declined is the reason this image must not be analysed at all, or "" when it
	// is safe. Set once by the loader so every entry point shares one verdict.
	declined string
}

// newProgram wraps a flat cartridge image (bank 0 — the only bank there is).
func newProgram(rom []byte) *program {
	return &program{rom: rom, base: uint16(0x10000 - len(rom)), banked: len(rom) > 4096}
}

// newBankProgram wraps ONE bank of a bank-switched cartridge as the self-contained
// 4K image it is: its own length gives the mask, the address space gives the base,
// and every instruction decoded from it is stamped with this bank number.
func newBankProgram(rom []byte, bank int) *program {
	return &program{rom: rom, base: uint16(0x10000 - len(rom)), bank: bank}
}

// cartInfo asks the engine what cartridge this image actually is: the mapper it
// fingerprints as, and how many banks that mapper has.
func cartInfo(binPath string) (id string, banks int, err error) {
	e, err := emu.New("NTSC")
	if err != nil {
		return "", 0, err
	}
	if err := e.LoadROM(binPath); err != nil {
		return "", 0, err
	}
	id, banks = e.CartInfo()
	return id, banks, nil
}

// analysisUnit is one bank presented as a self-contained 4K image, together with
// the addresses that switch away from it.
type analysisUnit struct {
	bank     int
	prog     *program
	hotspots map[uint16]string
}

// analysisUnits splits a cartridge into the units this package can analyse: one
// per bank for a bank-switched image, or the single flat image otherwise. The
// second return is a decline reason; non-empty means analyse nothing.
//
// Each bank is a 4K image in its own right — the mask falls out of its own length
// and the base out of the address space — so `newProgram` over the bank's bytes
// needs no special case, and the flat ROM is literally the one-unit version of
// the same thing.
func analysisUnits(rom []byte, binPath string) ([]analysisUnit, string) {
	flat := []analysisUnit{{bank: 0, prog: newProgram(rom)}}

	e, err := emu.New("NTSC")
	if err == nil {
		err = e.LoadROM(binPath)
	}
	if err != nil {
		// Cannot ask the engine. Being unable to identify the cartridge is a reason
		// to be MORE careful, not less: fall back to the size test and decline.
		if len(rom) > 4096 {
			return nil, fmt.Sprintf("image is %d bytes and the cartridge could not be identified (%v); "+
				"the console addresses 4K at a time, so a flat-address analysis would describe a "+
				"program that does not exist", len(rom), err)
		}
		return flat, ""
	}

	id, banks := e.CartInfo()
	// A superchip overlays 128 bytes of RAM on $F000-$F0FF, so the image is not what
	// the CPU reads there. romTableRange constant-folds real bytes into an EXACT
	// value range, that range sets a loop's trip count, and the trip count sets the
	// worst case — so folding a RAM address produces a narrow, confident, wrong
	// number in the forbidden direction. Measured hole: this function accepted any
	// mapper with banks > 1 that published hotspots and never asked.
	// Ask "does this cartridge map RAM into the window", not "is it a superchip".
	// Measured: HasSuperchip is implemented only by mapper_atari.go, so every other
	// mapper answers false through the type assertion — while 3E+ and M-Network both
	// overlay cartridge RAM and set banking.Information.IsRAM. Asking the narrower
	// question left those unguarded.
	// Ask the full question and REPEAT ITS ANSWER. MapsCartridgeRAM delegates to
	// CartridgeWindowNotImage and throws the reason away, and the flattened message
	// called everything "RAM" — which is wrong about DPC, whose $1000-$107F is the
	// data-fetcher/RNG/music REGISTER FILE. A builder told "this cartridge maps RAM"
	// goes looking for a RAM overlay that does not exist; the bus interface knew the
	// real reason all along, one call deeper. (G1's report filed this as the one
	// line described but not made.)
	notImage, why, ramErr := e.CartridgeWindowNotImage()
	if ramErr != nil {
		return nil, fmt.Sprintf("cartridge banks could not be enumerated to check for mapped RAM (%v); "+
			"declining rather than folding bytes that may not be image", ramErr)
	}
	if notImage {
		return nil, fmt.Sprintf("cartridge is mapper %s and %s; "+
			"the image is not what the CPU reads there, so folding those bytes into a value range "+
			"would bound a loop on data the hardware never holds", id, why)
	}
	if banks <= 1 {
		return flat, ""
	}
	// The cross-bank edge this package models — "the access selects the bank its
	// hotspot SYMBOL names, and the next fetch happens at the following address in
	// that bank" — is Atari's, read out of mapper_atari.go. Every gate up to here is
	// GEOMETRIC (bank count, 4K, one origin, no mapped RAM), and geometry does not
	// tell you how a mapper decides its target bank. A mapper can pass all of them
	// and still switch by a different rule.
	//
	// Measured, and it is not hypothetical: WF8 is 8K, two 4K banks at $F000, no
	// RAM, and publishes $1FF8:BANK0 / $1FF9:BANK1 — it clears every gate. Its actual
	// bankswitch (mapper_atari_wf8.go) responds ONLY to $0FF8 and takes the target
	// from DATA BUS BIT 2. So this analysis would (a) invent an edge for $1FF9, which
	// does nothing at all on that cartridge, and (b) send $1FF8 to bank 0 on the
	// strength of a symbol, when the hardware goes to bank 0 or 1 depending on the
	// value being written. Both are edges the machine does not take, and a wrong edge
	// can SHORTEN the longest path.
	//
	// So the mapper is named explicitly. An ID that is not in the table is declined
	// as unverified rather than assumed to behave like Atari's — the point being that
	// when the engine gains a mapper, this package says so instead of guessing.
	if why := unverifiedEdgeSemantics(id); why != "" {
		return nil, why
	}
	hotspots, published := e.BankSwitchHotspots()
	if !published || len(hotspots) == 0 {
		// The mapper's switch is not an address in cartridge space — it is driven by
		// a data value (3E, 3F), by bus behaviour outside the cartridge (UA, SB), by
		// bus timing (FE, WD), or by a coprocessor. An address model does not lose
		// precision there, it misses the switch entirely.
		return nil, fmt.Sprintf("cartridge is mapper %s with %d banks and publishes no bank-switch "+
			"hotspot addresses; its switch is driven by data, bus behaviour or a coprocessor, which "+
			"an address-based analysis cannot see at all", id, banks)
	}
	contents, err := e.CopyBanks()
	if err != nil {
		return nil, fmt.Sprintf("cartridge is mapper %s with %d banks and its banks could not be "+
			"read for analysis: %v", id, banks, err)
	}
	return unitsFromBanks(id, banks, contents, hotspots)
}

// unitsFromBanks turns the engine's bank images into analysis units, or declines by
// name. Split out of analysisUnits so its guards can be exercised directly: every
// one of them fires only on a cartridge shape no ROM in this repo has, so without a
// test they are code that has never run and cannot be shown to work.
func unitsFromBanks(id string, banks int, contents []emu.CartBank, hotspots map[uint16]string) ([]analysisUnit, string) {
	var units []analysisUnit
	for _, c := range contents {
		if len(c.Data) == 0 {
			return nil, fmt.Sprintf("cartridge is mapper %s: bank %d came back empty", id, c.Number)
		}
		// The cross-bank control-flow edge this package models is "the SAME ADDRESS in
		// the target bank". That sentence is only true when every bank is the whole 4K
		// window at $F000, and the geometry has to be CHECKED rather than assumed,
		// because a mapper exists that passes every other test and fails this one:
		// M-Network publishes parseable BANK0..BANK6 hotspots (mapper_mnetwork.go
		// HotspotBankSwitch) while its banks are 2K segments mapped at TWO different
		// origins — $F000 for banks 0..n-2 and $F000+2048 for the last. "The same
		// address in the target bank" is simply not where execution lands there. Under
		// the decode-only stage a wrong seed merely over-decoded; now it would be a CFG
		// edge the longest-path walk follows, and a wrong edge can SHORTEN the longest
		// path, which is the one direction this package forbids.
		if len(c.Data) != 4096 {
			return nil, fmt.Sprintf("cartridge is mapper %s with %d banks, and bank %d is %d bytes rather "+
				"than the whole 4K window; the cross-bank edge this analysis models is \"the same address "+
				"in the target bank\", which is not true of a segmented mapper", id, banks, c.Number, len(c.Data))
		}
		if len(c.Origins) != 1 || c.Origins[0] != memorymap.OriginCartFxxx {
			return nil, fmt.Sprintf("cartridge is mapper %s with %d banks, and bank %d maps at origin(s) %v "+
				"rather than $%04X alone; the cross-bank edge this analysis models is \"the same address in "+
				"the target bank\", which is not true when banks sit at different origins",
				id, banks, c.Number, c.Origins, memorymap.OriginCartFxxx)
		}
		units = append(units, analysisUnit{bank: c.Number, prog: newBankProgram(c.Data, c.Number), hotspots: hotspots})
	}
	// An empty unit list with no reason is the worst possible answer: every caller
	// reads "no decline" as permission to proceed, then finds nothing to analyse and
	// reports zero regions — which the 0-region backstop turns into "not certified"
	// rather than into "I was handed nothing". Reachable if CopyBanks returns an
	// empty slice while CartInfo reports banks > 1.
	if len(units) == 0 {
		return nil, fmt.Sprintf("cartridge is mapper %s reporting %d banks, but no bank could be turned "+
			"into an analysable image; declining rather than returning an empty analysis that would "+
			"read as a clean one", id, banks)
	}
	if len(units) != banks {
		return nil, fmt.Sprintf("cartridge is mapper %s reports %d banks but %d were readable; "+
			"analysing a subset would certify on the part that happened to be available",
			id, banks, len(units))
	}
	return units, ""
}

// verifiedEdgeSemantics lists the mapper IDs whose bank-switch rule was READ in the
// engine's source and found to match the edge this package models: the hotspot
// address alone selects the bank, the symbol names which one, and PC is untouched
// so the next fetch is the following address in the new bank.
//
// The value is the evidence, not a label. Anything absent is declined.
var verifiedEdgeSemantics = map[string]string{
	"F8": "mapper_atari.go atari8k.bankswitch: $0FF8/$0FF9 -> bank 0/1, address only",
	"F6": "mapper_atari.go atari16k.bankswitch: $0FF6-$0FF9 -> bank 0-3, address only",
	"F4": "mapper_atari.go atari32k.bankswitch: $0FF4-$0FFB -> bank 0-7, address only",
	"EF": "mapper_atari_ef.go ef.bankswitch: $0FE0-$0FEF -> addr & 0x0F, address only",
	"BF": "mapper_atari_bf.go bf.bankswitch: $0F80-$0FBF -> addr - $0F80, address only",
	"DF": "mapper_df.go df.bankswitch: $0FC0-$0FDF -> one bank per address, address only",
	"JANE": "mapper_atari_jane.go jane.bankswitch: $0FF0/$0FF1/$0FF8/$0FF9 -> bank 0/1/2/3; " +
		"it takes a data argument and does not read it",
}

// unverifiedEdgeSemantics returns a decline reason for any mapper whose switching
// rule this package has not checked, and "" for the ones it has.
func unverifiedEdgeSemantics(id string) string {
	if _, ok := verifiedEdgeSemantics[id]; ok {
		return ""
	}
	if why, ok := knownDifferentEdgeSemantics[id]; ok {
		return fmt.Sprintf("cartridge is mapper %s, whose bank-switch rule is NOT the one this analysis "+
			"models: %s. A cross-bank edge derived from the hotspot symbol would be an edge the "+
			"hardware does not take", id, why)
	}
	return fmt.Sprintf("cartridge is mapper %s, which is not among the mappers whose bank-switch rule has "+
		"been checked against the engine's source (%s). Passing the geometric tests says the banks are "+
		"shaped like Atari's, not that the switch behaves like Atari's, so the edge model is not applied "+
		"to it", id, verifiedEdgeSemanticsList())
}

// knownDifferentEdgeSemantics records mappers measured to break the model, so the
// refusal can say what is actually wrong instead of "unverified".
//
// REACHABILITY, measured 2026-07-31 — most of these messages never print, and the
// reason is structural rather than accidental. `bankedUnits` refuses a cartridge that
// maps RAM into the window BEFORE it consults this table, and FA, FA2 and E7 all
// carry cartridge RAM by construction (CBS RAM Plus, its NVRAM successor, and
// M-Network's 1K+256B). So those three are always answered by the coarser refusal and
// their text below is documentation, not output. E0 has `IsRAM: false` on every
// segment and does reach here: witnessed on three real cartridges (Montezuma's
// Revenge Trainer, Super Cobra, Swtagrc).
//
// They are still worth stating. The table's job is to stop a future reader from
// pattern-matching a mapper's published hotspots onto the Atari rule and promoting it
// to `verifiedEdgeSemantics` — where it WOULD be consulted, and where being wrong
// invents edges the hardware never takes. A shadowed refusal is a live guard against
// the wrong promotion even when it is not a live message.
//
// This is the same shape as the `foldLoops` finding: a fine-grained refusal sitting
// behind a coarser one that always fires first. Recorded rather than reordered — RAM
// in the window is the more fundamental objection, and it should keep winning.
var knownDifferentEdgeSemantics = map[string]string{
	"WF8": "mapper_atari_wf8.go wf8.bankswitch responds only to $0FF8 and takes the target bank from " +
		"DATA BUS BIT 2, while it publishes $1FF8:BANK0 and $1FF9:BANK1 — so $1FF9 does nothing at all " +
		"and $1FF8 goes to bank 0 or 1 depending on the value written",
	"WFSC": "the superchip WF8; same data-bus target selection",
	"FA": "mapper_cbs.go cbs.bankswitch gates the whole switch on DATA BUS BIT 0 — it selects the bank from " +
		"the address ($0FF8/$0FF9/$0FFA -> bank 0/1/2) but only inside `if data&0x01 == 0x01`, quoting the " +
		"CBS patent's requirement that D0 be high. So `lda $1FF9` switches or does nothing depending on what " +
		"happens to be on the bus, and an edge taken from the symbol alone would be taken unconditionally",
	"FA2": "mapper_fa2.go fa2.bankswitch is address-selected ($0FF5-$0FFB -> bank 0-6) but two of its " +
		"hotspots are not plain bank switches: $0FFB is guarded by `len(cart.banks) > 6`, so on a 6-bank " +
		"image the published $1FFB:BANK6 edge is one the hardware does not take, and $0FF4 drives NVRAM " +
		"save/load (file I/O) while changing no bank at all",
	"E0": "mapper_parkerbros.go parkerBros.bankswitch does not switch a bank at all — it assigns one of " +
		"THREE 1K SEGMENTS ($0FE0-$0FE7 -> segment 0, $0FE8-$0FEF -> segment 1, $0FF0-$0FF7 -> segment 2), " +
		"and Access reads each quarter of the window through its own segment. The fourth quarter " +
		"($0C00-$0FFF), which holds the hotspots and the vectors, is never assigned and cannot move. A " +
		"landing site of (targetBank, same address) is therefore wrong twice over: three quarters of the " +
		"window are unaffected, and the quarter the switching instruction lives in is fixed",
	"E7": "mapper_mnetwork.go mnetwork.bankswitch spends most of its hotspot range on things that are not " +
		"banks: $0FE7 maps 1K of RAM in place of ROM, $0FE8-$0FEB select which 256-byte RAM block appears " +
		"in a window and leave the bank untouched, and the result is reduced by `bank %= NumBanks()` so the " +
		"address-to-bank map depends on the image size (an 8K E7 folds the 16K addresses). Only $0FE0-$0FE6 " +
		"behave like a bank select",
}

func verifiedEdgeSemanticsList() string {
	ids := make([]string, 0, len(verifiedEdgeSemantics))
	for id := range verifiedEdgeSemantics {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return strings.Join(ids, " ")
}

// switchModel is the single oracle for "can this instruction change the bank, and
// if so where does the next fetch come from?".
//
// One function decides BOTH the control-flow edge and the refusal, for the same
// reason collectRegion and collectFrom are one walker: two predicates would
// eventually disagree, and a successor function that models a switch the refusal
// does not guard is silently unsound in the direction that lies.
type switchModel struct {
	hotspots map[uint16]string // folded to the primary mirror by cartHotspotKey
	banks    map[int]bool      // the bank numbers actually analysed
	banked   bool
}

func (sw switchModel) active() bool { return sw.banked && len(sw.hotspots) > 0 }

// switchEdges reports the cross-bank successors of in, whether the intra-bank
// fall-through is still possible, and — when the switch it can perform is one this
// analysis does not model — the refusal that must propagate to the whole region.
//
// THE EDGE. For an instruction whose DATA access reaches a hotspot the successor is
// the SAME ADDRESS IN THE TARGET BANK: site{targetBank, in.next()}. That comes from
// the engine, not from folklore: on a hotspot access the Atari mapper switches and
// does not touch PC (Gopher2600 hardware/memory/cartridge/mapper_atari.go Read/Write
// call bankswitch(addr) and then return cart.banks[cart.state.bank][addr]), and the
// hotspot is matched on the 12-bit-folded address, which is what cartHotspotKey
// reproduces. A READ switches exactly as a write does — `lda $FFF9` is the canonical
// form. Measured on litmus_bank: bank0 $FF00 `ad f9 ff` (3 bytes) is followed by a
// fetch at bank1 $FF03, and bank1 $FF09 `ad f8 ff` by bank0 $FF0C; the emulator
// executes both landings.
//
// COST. The switching instruction is charged its ordinary cost and the edge is
// charged nothing: the latch takes effect at the next fetch, so no cycle belongs to
// the crossing.
//
// CERTAIN vs POSSIBLE. If every address the access can reach is a hotspot, the
// switch is certain and the intra-bank fall-through is REPLACED (keep=false). If the
// footprint also contains non-hotspot addresses (an unknown index reaches all 256),
// the successors are every hotspot's landing site PLUS the fall-through.
// Over-approximating the successor set raises the longest path and lowers the
// beam-interval minimum, which is the safe direction for both consumers.
//
// ALL-OR-NOTHING. If ANY candidate cannot be resolved — an unparseable symbol
// (Parker Bros "B0S0", M-Network "RAM0"), a bank outside the analysed set, or a
// landing address that wraps past $FFFF — the WHOLE instruction is refused.
// Modelling the resolvable candidates and dropping the rest would delete a
// successor, and deleting a successor shortens the longest path.
func (sw switchModel) switchEdges(in Instr, st State) (edges []site, keepFallThrough bool, refusal string) {
	if !sw.active() {
		return nil, true, ""
	}
	where := siteDesc(in.site(), sw.banked)
	// The instruction's OWN bytes are fetched from the cartridge, so an opcode or
	// operand byte sitting on a hotspot switches the bank mid-instruction: the opcode
	// at the hotspot comes from the NEW bank, so the instruction decoded here is not
	// the one that executes, and each further operand byte can select yet another
	// bank. That is a measured authoring bug in this repo already (banked_game.asm
	// records a reboot loop from putting `rts` on $FFF9); it is refused, never modelled.
	for off := uint16(0); off < in.size(); off++ {
		if ma, ok := cartHotspotKey(in.Addr + off); ok {
			if sym, isHot := sw.hotspots[ma]; isHot {
				return nil, true, fmt.Sprintf("region can switch banks: the instruction at %s is fetched "+
					"across %s ($%04X), so the fetch itself selects another bank", where, sym, ma)
			}
		}
	}
	// A control transfer INTO a hotspot switches banks by a different mechanism:
	// `jmp $1FF9` does not read $1FF9 as data, it sets PC there, and the instruction
	// FETCH at that address is a cartridge read. Its successor would be
	// site{targetBank, hotspotAddr} — an instruction that is itself fetched across a
	// hotspot and so refuses under the rule above. Gopher2600 classifies jmp/jsr as
	// Subroutine/Flow rather than Read, so a check driven off the data access alone
	// never looks at them.
	switch in.Def.Operator {
	case instructions.JMP, instructions.JSR:
		if in.Def.AddressingMode == instructions.Absolute {
			if ma, ok := cartHotspotKey(in.Operand); ok {
				if sym, isHot := sw.hotspots[ma]; isHot {
					return nil, true, fmt.Sprintf("region can switch banks: the %v at %s transfers control to "+
						"%s ($%04X), whose instruction fetch selects another bank — flow between banks "+
						"is not modelled", in.Def.Operator, where, sym, ma)
				}
			}
		}
	}
	acc, ok := accessOf(in, st)
	if !ok {
		return nil, true, "" // instruction touches no memory
	}
	if acc.Unbounded {
		// Indirect ((ind),Y / (ind,X)): no address, therefore no symbol, therefore no
		// bank. Measured on all four corpus bank ROMs: 0 such accesses, so refusing
		// costs nothing there today — but it is still counted rather than assumed away.
		return nil, true, fmt.Sprintf("region contains an access at %s whose target cannot be resolved, "+
			"and this cartridge has bank-switch hotspots — it may therefore switch banks", where)
	}
	hot := 0
	seen := map[site]bool{}
	for _, a := range acc.Addrs {
		ma, ok := cartHotspotKey(a)
		if !ok {
			continue
		}
		sym, isHot := sw.hotspots[ma]
		if !isHot {
			continue
		}
		hot++
		tb, ok := hotspotTargetBank(sym)
		if !ok {
			return nil, true, fmt.Sprintf("region can switch banks: the access at %s reaches hotspot %s "+
				"($%04X), whose symbol does not name a bank number, so the landing bank cannot be "+
				"resolved", where, sym, ma)
		}
		if !sw.banks[tb] {
			return nil, true, fmt.Sprintf("region can switch banks: the access at %s reaches %s ($%04X), "+
				"which names bank %d — not among the %d banks analysed", where, sym, ma, tb, len(sw.banks))
		}
		if int(in.Addr)+int(in.size()) > 0xFFFF {
			return nil, true, fmt.Sprintf("region can switch banks: the access at %s reaches %s ($%04X), "+
				"but the following fetch address wraps past $FFFF and is not in cartridge space",
				where, sym, ma)
		}
		land := site{tb, in.next()}
		if !seen[land] {
			seen[land] = true
			edges = append(edges, land)
		}
	}
	if hot == 0 {
		return nil, true, ""
	}
	// Every address this access can reach is a hotspot => a switch definitely happens
	// and the intra-bank fall-through cannot be executed.
	sortSites(edges)
	return edges, hot < len(acc.Addrs), ""
}

// cartHotspotKey folds a CPU address to the form the mapper keys its hotspot
// table by: the primary cartridge mirror, $1000-$1FFF.
//
// memorymap.MapAddress is NOT enough here and the difference is silent.
// Measured: it returns $FFF9 unchanged for $FFF9 and $3FF9 for $3FF9 — it
// identifies cartridge space but does not fold the mirrors — so comparing its
// output against a table keyed at $1FF9 matches nothing at all, and a region full
// of `lda $FFF9` reports no bank switch whatsoever. The cartridge itself masks
// with its cartridgeBits and adds the origin, so this does the same.
func cartHotspotKey(addr uint16) (uint16, bool) {
	if _, area := memorymap.MapAddress(addr, false); area != memorymap.Cartridge {
		return 0, false
	}
	return memorymap.OriginCart | (addr & memorymap.CartridgeBits), true
}

// hotspotTargetBank parses the bank a bank-switch hotspot selects out of the
// mapper's OWN symbol for it, or reports that the symbol does not name one.
//
// The match is deliberately strict — the whole symbol must be "BANK" followed by
// decimal digits — because the vendored mappers publish two other shapes for
// which "the same address in another 4K image" is simply not where execution
// lands, and a loose digit-scrape would invent a control-flow edge the hardware
// does not have. Measured over Gopher2600/hardware/memory/cartridge:
//
//	BANK0..BANK1    F8, WF8              (mapper_atari.go, mapper_atari_wf8.go)
//	BANK0..BANK3    F6                   (mapper_atari.go)
//	BANK0..BANK7    F4                   (mapper_atari.go)
//	BANK0..BANK15   EF                   (mapper_atari_ef.go)
//	BANK0..BANK63   BF                   (mapper_atari_bf.go)
//	BANK0..BANK2    CBS/FA               (mapper_cbs.go)
//	BANK0..BANK3    JANE                 (mapper_atari_jane.go)
//	B0S0..B7S2      Parker Bros E0       (mapper_parkerbros.go) — bank-in-SEGMENT:
//	                                     only a 1K slice moves, so the landing
//	                                     address may not move at all
//	RAM0..RAM3      M-Network            (mapper_mnetwork.go) — selects cartridge
//	                                     RAM into the window, which is not in the
//	                                     image and cannot be decoded
//
// "B0S0" would scrape to bank 0 and "RAM3" to bank 3; both are wrong, so both are
// reported unresolvable rather than guessed.
func hotspotTargetBank(symbol string) (int, bool) {
	if !strings.HasPrefix(symbol, "BANK") {
		return 0, false
	}
	digits := symbol[len("BANK"):]
	if digits == "" {
		return 0, false
	}
	for _, r := range digits {
		if r < '0' || r > '9' {
			return 0, false // strconv.Atoi accepts "+1"/"-1"; a hotspot symbol is neither
		}
	}
	n, err := strconv.Atoi(digits)
	if err != nil || n < 0 {
		return 0, false
	}
	return n, true
}

// unitDecode is one analysis unit after decoding: the bank's own vector-reachable
// code plus whatever cross-bank seeding added to it.
type unitDecode struct {
	bank     int
	prog     *program
	hotspots map[uint16]string
	instrs   map[site]Instr
	entries  []site
	seeded   int // landing addresses seeded into this bank from another bank
}

// CrossBankSeed is one decode entry point contributed by a bank switch: the
// instruction that reaches the hotspot, the hotspot it reaches, and the address
// the next fetch comes from in the bank the hotspot names.
type CrossBankSeed struct {
	FromBank int    `json:"from_bank"`
	FromAddr uint16 `json:"from_addr"`
	Hotspot  uint16 `json:"hotspot"`
	Symbol   string `json:"symbol"`
	ToBank   int    `json:"to_bank"`
	ToAddr   uint16 `json:"to_addr"`
	// Desc is the same edge in the hex a 6502 author reads. The numeric fields keep
	// the package's existing decimal-address convention (see Region.Start); an
	// address nobody can read at a glance is a fact that does not get checked.
	Desc string `json:"desc"`
}

// seedRoundCap bounds the seeding fixpoint. Each round can only add landing
// addresses that were not already seeded, and the set of (bank, address) pairs is
// finite, so the loop terminates on its own; the cap exists so that a mapper
// shape nobody has measured cannot spin here. Measured on the four bank ROMs in
// the corpus, the fixpoint closes in 2 rounds (1 productive + 1 that adds
// nothing), so 8 is not a limit any of them approaches — and when it IS hit the
// report says so rather than presenting a capped decode as a closed one.
const seedRoundCap = 8

// seedOutcome is everything the seeding pass wants reported: it must never be
// possible to read a decode as complete when part of it was skipped.
type seedOutcome struct {
	seeds      []CrossBankSeed
	rounds     int
	capped     bool
	unresolved []string // hotspot symbols that do not name a bank number
	// unresolvable counts instructions whose memory target could not be resolved at
	// all under a hotspot-bearing mapper. Such an access MIGHT be a switch, but no
	// symbol is reached, so no bank can be named and nothing can be seeded. The
	// region containing it is still refused by residualSwitchRefusal.
	unresolvable int
}

// decodeUnits decodes every analysis unit, then closes the decode over bank
// switches: an instruction whose memory access reaches a bank-switch hotspot
// continues at the FOLLOWING address in the bank that hotspot names, so that
// address is a decode entry point for the target bank.
//
// This is what a worker bank is missing. A bank's own reset/NMI/IRQ vectors reach
// only whatever that bank's stub does; the code that matters is entered by the
// trampoline that switched to it. Measured before seeding: litmus_bank executed 4
// (bank,pc) pairs the decode never saw — bank 1 $FF03/$FF05/$FF07/$FF09, all of
// them the body reached from bank 0's `lda $FFF9` at $FF00 — and banked_game 1,
// bank 1 $FF83.
//
// The landing address is in the OTHER bank at the SAME address, and each bank is
// decoded as its own self-contained 4K image; the merged map that the flow analysis
// runs on is keyed on (bank, address), so nothing collides.
//
// Seeding is a fixpoint: seeding bank B can reveal a hotspot access in B that
// seeds bank C. It is also STRICTLY ADDITIVE — it only ever hands more entry
// points to a decode that was already running — so it cannot remove an
// instruction the old decode had.
//
// Address resolution uses topState() rather than the abstract-interpretation
// result, because the states are computed FROM the decode and would therefore be
// circular here. Top over-approximates: an indexed access with an unknown index
// contributes its whole 256-address footprint, so a hotspot inside that footprint
// is seen. The DECODE also keeps the intra-bank fall-through of a switching
// instruction, which the FLOW model drops as unexecutable: extra decoded bytes cost
// only precision, while a missing decoded instruction would make the flow walk
// refuse a region it could otherwise bound.
//
// The second and third returns are the MERGED decode and the merged VECTOR entry
// list. Cross-bank landing sites are deliberately NOT in that entry list: seeding
// them as fresh Top entries (which is what the per-bank stage did) would join Top
// into the state at exactly the site the cross-bank edge is there to give a real
// state to, and Top wins every join.
func decodeUnits(units []analysisUnit) ([]unitDecode, map[site]Instr, []site, seedOutcome) {
	decodes := make([]unitDecode, len(units))
	byBank := map[int]int{}
	merged := map[site]Instr{}
	var vectorEntries []site
	for i, u := range units {
		instrs, entries := u.prog.decodeFromVectors()
		decodes[i] = unitDecode{bank: u.bank, prog: u.prog, hotspots: u.hotspots, instrs: instrs, entries: entries}
		byBank[u.bank] = i
		vectorEntries = append(vectorEntries, entries...)
	}
	mergeInto := func() {
		for i := range decodes {
			for k, v := range decodes[i].instrs {
				merged[k] = v
			}
		}
	}
	// A flat ROM has exactly one unit and no hotspots, so there is nothing to seed
	// and it goes down a path identical to the one before this existed. That is why
	// its JSON is byte-for-byte unchanged.
	if len(units) < 2 {
		mergeInto()
		return decodes, merged, vectorEntries, seedOutcome{}
	}

	var out seedOutcome
	seen := map[site]bool{}     // (target bank, landing address) already seeded
	badSym := map[string]bool{} // hotspot symbols already reported unresolvable
	badSite := map[site]bool{}  // (bank, pc) already counted as unresolvable
	for {
		if out.rounds >= seedRoundCap {
			out.capped = true
			break
		}
		out.rounds++
		added := 0
		for i := range decodes {
			d := &decodes[i]
			if len(d.hotspots) == 0 {
				continue
			}
			// Snapshot the addresses before walking them: this loop decodes INTO
			// d.instrs when i is also the target bank, and ranging over a map being
			// written to visits new keys unpredictably. The outer round loop is what
			// picks anything up that this pass could not see.
			addrs := make([]site, 0, len(d.instrs))
			for a := range d.instrs {
				addrs = append(addrs, a)
			}
			sortSites(addrs)
			for _, a := range addrs {
				in := d.instrs[a]
				acc, ok := accessOf(in, topState())
				if !ok {
					continue // instruction touches no memory
				}
				for _, ea := range acc.Addrs {
					ma, ok := cartHotspotKey(ea)
					if !ok {
						continue
					}
					sym, ok := d.hotspots[ma]
					if !ok {
						continue
					}
					tb, ok := hotspotTargetBank(sym)
					if !ok {
						if !badSym[sym] {
							badSym[sym] = true
							out.unresolved = append(out.unresolved,
								fmt.Sprintf("%s ($%04X): the symbol does not name a bank number, so the "+
									"landing bank cannot be resolved and nothing was seeded", sym, ma))
						}
						continue
					}
					j, ok := byBank[tb]
					if !ok {
						if !badSym[sym] {
							badSym[sym] = true
							out.unresolved = append(out.unresolved,
								fmt.Sprintf("%s ($%04X): names bank %d, which this cartridge's %d analysed "+
									"banks do not include", sym, ma, tb, len(units)))
						}
						continue
					}
					land := in.next()
					if _, ok := decodes[j].prog.canon(land); !ok {
						// The landing address is not in cartridge space (in.next() wrapped past
						// $FFFF). Nothing can be decoded there — and switchEdges refuses the
						// region for the same reason, so this skip cannot silently delete a
						// successor from the flow model.
						continue
					}
					key := site{tb, land}
					if seen[key] {
						continue
					}
					seen[key] = true
					out.seeds = append(out.seeds, CrossBankSeed{
						FromBank: d.bank, FromAddr: in.Addr, Hotspot: ma, Symbol: sym,
						ToBank: tb, ToAddr: land,
						Desc: fmt.Sprintf("bank %d $%04X reaches %s ($%04X), so the next fetch is "+
							"bank %d $%04X", d.bank, in.Addr, sym, ma, tb, land),
					})
					decodes[j].prog.decodeInto(decodes[j].instrs, land)
					decodes[j].entries = append(decodes[j].entries, key)
					decodes[j].seeded++
					added++
				}
				if acc.Unbounded {
					// No address, therefore no symbol, therefore no bank to name. Counted,
					// not guessed; residualSwitchRefusal refuses the region it sits in.
					if k := in.site(); !badSite[k] {
						badSite[k] = true
						out.unresolvable++
					}
				}
			}
		}
		if added == 0 {
			break
		}
	}
	sort.Strings(out.unresolved)
	mergeInto()
	return decodes, merged, vectorEntries, out
}

// residualSwitchRefusal reports why a region cannot be bounded because it can
// change bank in a way this analysis does NOT model, or "" when every switch it can
// perform has a modelled edge.
//
// It refuses only the residue — the instruction's own bytes spanning a hotspot, a
// jmp/jsr INTO one, an unresolvable access target under a hotspot-bearing mapper, an
// unparseable hotspot symbol, a target bank outside the analysed set, a landing
// address outside cartridge space. Each of those verdicts comes from the SAME
// switchEdges call the successor function uses, never from a second scan: a
// successor function modelling a switch the refusal does not guard would be silently
// unsound, and a refusal for a switch the successor function does model would be a
// permanent false negative.
//
// The scan order is the region's sites in (bank, address) order so the reported
// reason is deterministic.
func residualSwitchRefusal(instrs map[site]Instr, start site, sw switchModel, states map[site]State) (string, int) {
	if !sw.active() {
		return "", 0
	}
	s := &solver{
		nodes: map[site]Instr{}, sinks: map[site]bool{}, folds: map[site]loopInfo{},
		memo: map[lkey]result{}, state: map[lkey]int{}, absStates: states, sw: sw, banked: sw.banked,
	}
	// Collect, but scan whatever was collected even when collection FAILS. A
	// collection that fails because flow left for an address the map does not hold
	// is very often the bank switch itself, and returning early there swallowed
	// every one of the control-transfer cases.
	_ = s.collectRegion(instrs, instrs[start])
	if len(s.nodes) == 0 {
		s.nodes = map[site]Instr{start: instrs[start]}
	}
	var at []site
	for k := range s.nodes {
		at = append(at, k)
	}
	sortSites(at)
	edges := 0
	for _, k := range at {
		es, _, refusal := sw.switchEdges(s.nodes[k], states[k])
		if refusal != "" {
			return refusal, edges
		}
		edges += len(es)
	}
	return "", edges
}

// unmodelledLandings returns the code sites whose abstract state must be forced to
// Top, plus the distinct reasons, because they are a possible landing point of a bank
// switch switchEdges does NOT model.
//
// It exists because the abstract interpretation is a WHOLE-PROGRAM fixpoint while a
// refusal is per-REGION. An unmodelled switch leaves its real landing site with an
// incomplete set of incoming edges, and a join over an incomplete predecessor set is an
// UNDER-approximation — a narrow, confident, wrong value range that could then bound a
// loop in a DIFFERENT region that was never refused. Refusing the region that CONTAINS
// the switch does not protect the region that contains its landing.
//
// The landing set is over-approximated deliberately, and per class:
//   - an instruction whose own bytes span a hotspot: the opcode may come from the new
//     bank, so neither the instruction nor its length is known. Every address in
//     [addr, addr+3] of EVERY analysed bank is a possible next fetch.
//   - a jmp/jsr INTO a hotspot: the fetch at the hotspot address itself, in any bank.
//   - an unresolvable target, an unparseable symbol, a bank outside the set: the
//     landing address is in.next() but the bank is unknown, so every analysed bank.
//
// The scan uses topState() rather than the computed states, for the same reason the
// decode does: the states are computed from this decision, so using them would be
// circular. topState over-approximates only the classes that depend on state at all
// (an unresolvable indirect access), so it cannot MISS a refusal.
func unmodelledLandings(instrs map[site]Instr, sw switchModel) ([]site, []string) {
	if !sw.active() {
		return nil, nil
	}
	var banks []int
	for b := range sw.banks {
		banks = append(banks, b)
	}
	sort.Ints(banks)
	out := map[site]bool{}
	reasons := map[string]bool{}
	var at []site
	for k := range instrs {
		at = append(at, k)
	}
	sortSites(at)
	for _, k := range at {
		in := instrs[k]
		_, _, refusal := sw.switchEdges(in, topState())
		if refusal == "" {
			continue
		}
		reasons[refusal] = true
		var addrs []uint16
		switch {
		case in.Def.Operator == instructions.JMP || in.Def.Operator == instructions.JSR:
			addrs = []uint16{in.Operand, in.next()}
		default:
			for off := 0; off <= 3; off++ {
				addrs = append(addrs, in.Addr+uint16(off))
			}
		}
		for _, b := range banks {
			for _, a := range addrs {
				if _, decoded := instrs[site{b, a}]; decoded {
					out[site{b, a}] = true
				}
			}
		}
	}
	var sites []site
	for s := range out {
		sites = append(sites, s)
	}
	sortSites(sites)
	var rs []string
	for r := range reasons {
		rs = append(rs, r)
	}
	sort.Strings(rs)
	return sites, rs
}

// DecodedAddrsPerBank returns, per bank, the set of addresses the decode reached.
//
// Exposed so the composite (bank, address) key's justification stays a MEASUREMENT
// rather than an argument: if two banks decode code at the same address, one map
// keyed on the bare address cannot hold both, and everything built on that map — the
// region set, the abstract states, the source locations — silently describes
// whichever bank won. This is the measurement the site key rests on, and it must
// keep being re-taken: if a corpus ever stops overlapping, it is this premise that
// broke rather than the code.
func DecodedAddrsPerBank(binPath string) (map[int]map[uint16]bool, error) {
	rom, err := os.ReadFile(binPath)
	if err != nil {
		return nil, err
	}
	units, decline := analysisUnits(rom, binPath)
	if decline != "" {
		return nil, fmt.Errorf("%s", decline)
	}
	decodes, _, _, _ := decodeUnits(units)
	out := map[int]map[uint16]bool{}
	for _, d := range decodes {
		set := map[uint16]bool{}
		for a := range d.instrs {
			set[a.addr] = true
		}
		out[d.bank] = set
	}
	return out, nil
}

// declineBanked returns a reason string when the image must not be analysed by
// this flat-address package, or "" when it is safe to proceed.
//
// The verdict comes from the ENGINE — it loads the cartridge and fingerprints the
// mapper — rather than from len(rom). Size is not the machine: an 8192-byte image
// is F8 only unless it fingerprints as WF8/3F/E0/E7/WD/FE/UA, and a superchip
// variant overlays RAM on part of the window whatever the size says. Guessing
// from the length means the analysis and the emulator the acceptance tests run on
// can disagree about which machine is being described.
//
// A single bank is not automatically fine either: a mapper can map cartridge RAM
// into the window, and bytes executed from there are not in the image at all, so
// any decode of them is fiction.
func (p *program) declineBanked(binPath string) string {
	id, banks, err := cartInfo(binPath)
	if err != nil {
		// Fall back to the size test rather than proceeding blind. Being unable to
		// ask the engine is a reason to be MORE careful, not less.
		if p.banked {
			return fmt.Sprintf("image is %d bytes and the cartridge could not be identified (%v); "+
				"the console addresses 4K at a time, so a flat-address analysis would describe "+
				"a program that does not exist", len(p.rom), err)
		}
		return ""
	}
	if banks > 1 {
		return fmt.Sprintf("cartridge is mapper %s with %d banks; every address in this package is a "+
			"flat 16-bit number, so the vectors, the decode and everything built on them would "+
			"describe a program that does not exist", id, banks)
	}
	return ""
}

// canon folds a CPU address to the cartridge offset it addresses, or reports
// that the address is not in cartridge space at all.
//
// The 6507 drives 13 address lines, so the cartridge answers at $1000-$1FFF and
// at every mirror of it up to $F000-$FFFF; a 2K image answers TWICE inside that
// window. Keying anything on the raw address therefore splits one byte of ROM
// across several keys, and comparing a statically-decoded address set against
// the PCs a real execution reported then subtracts sets that do not overlap —
// producing a number with no meaning rather than an answer.
//
// Cartridge space is decided by Gopher2600's own memory map rather than by a
// bit test here, and only THEN is the offset taken. Masking first would fold
// RAM, TIA and RIOT addresses into the ROM and decode whatever bytes happened to
// be there.
func (p *program) canon(addr uint16) (uint16, bool) {
	_, area := memorymap.MapAddress(addr, true)
	if area != memorymap.Cartridge || len(p.rom) == 0 {
		return 0, false
	}
	return uint16(int(addr) & (len(p.rom) - 1)), true
}

// byteAtBank is byteAt in the shape the value domain wants (see State.romAt): a
// single-bank program answers only for its own bank, so a stray request for another
// one comes back as unreadable rather than as this bank's byte.
func (p *program) byteAtBank(bank int, addr uint16) (byte, bool) {
	if bank != p.bank {
		return 0, false
	}
	return p.byteAt(addr)
}

func (p *program) byteAt(addr uint16) (byte, bool) {
	off, ok := p.canon(addr)
	if !ok || int(off) >= len(p.rom) {
		return 0, false
	}
	return p.rom[off], true
}

// decodeFromVectors decodes the program reachable from the reset, NMI and
// IRQ/BRK vectors. Duplicate targets dedupe.
func (p *program) decodeFromVectors() (map[site]Instr, []site) {
	instrs := map[site]Instr{}
	var entries []site
	seen := map[uint16]bool{}
	for _, va := range []uint16{0xFFFC, 0xFFFA, 0xFFFE} {
		lo, _ := p.byteAt(va)
		hi, _ := p.byteAt(va + 1)
		t := uint16(lo) | uint16(hi)<<8
		if _, ok := p.canon(t); !ok || seen[t] {
			continue
		}
		seen[t] = true
		p.decodeInto(instrs, t)
		entries = append(entries, site{p.bank, t})
	}
	return instrs, entries
}

func (p *program) decodeAt(addr uint16) (Instr, bool) {
	op, ok := p.byteAt(addr)
	if !ok {
		return Instr{}, false
	}
	// This is the single point where bank identity enters the analysis: every
	// instruction is stamped with the bank whose bytes it was read from.
	in := Instr{Bank: p.bank, Addr: addr, Op: op, Def: instructions.Definitions[op]}
	switch in.Def.Bytes {
	case 2:
		b1, _ := p.byteAt(addr + 1)
		in.Operand = uint16(b1)
	case 3:
		lo, _ := p.byteAt(addr + 1)
		hi, _ := p.byteAt(addr + 2)
		in.Operand = uint16(lo) | uint16(hi)<<8
	}
	return in, true
}

// decodeInto walks the CFG from entry, decoding only reachable instructions
// into instrs (recursive descent avoids misdecoding inline data tables).
//
// entry stays a bare address because one program holds exactly ONE bank; routing a
// cross-bank landing to the right program is decodeUnits' job.
func (p *program) decodeInto(instrs map[site]Instr, entry uint16) {
	work := []site{{p.bank, entry}}
	for len(work) > 0 {
		at := work[len(work)-1]
		work = work[:len(work)-1]
		if _, seen := instrs[at]; seen {
			continue
		}
		in, ok := p.decodeAt(at.addr)
		if !ok {
			continue
		}
		instrs[at] = in
		work = append(work, decodeSuccessors(in)...)
	}
}

// decodeSuccessors lists the INTRA-BANK sites reachable for DECODING (JSR decodes
// both its callee and its return point; an indirect JMP's target is unknown).
// Cross-bank edges are switchModel's business — see successors.
func decodeSuccessors(in Instr) []site {
	d := in.Def
	switch d.Operator {
	case instructions.JMP:
		if d.AddressingMode == instructions.Indirect {
			return nil
		}
		return []site{{in.Bank, in.Operand}}
	case instructions.JSR:
		return []site{{in.Bank, in.Operand}, in.nextSite()}
	case instructions.RTS, instructions.RTI, instructions.BRK, instructions.JAM:
		return nil
	}
	if d.IsBranch() {
		return []site{in.nextSite(), in.branchSite()}
	}
	return []site{in.nextSite()}
}

// successors is the ONE successor function every flow walker in this package uses:
// collectFrom, longest, absSuccessors, beamSuccessors, topoOrder, regionInstrs,
// mustWrittenAt, reachableWithinCallee, displayPreserver and determineBound's
// predecessor scan. A second walker that modelled the CFG slightly differently would
// eventually disagree with the refusal, and the disagreement would be invisible.
//
// A non-empty second return means the flow model cannot follow this instruction and
// the CALLER MUST REFUSE. It must never skip: silently dropping a successor shortens
// the longest path, which is the one direction this package forbids.
// invalidStateAsTop counts how often successors was handed a state it could not
// use. It is deliberately NOT surfaced in Report: adding a field would change every
// flat ROM's JSON, and the number is a property of a whole sweep rather than of one
// analysis. It exists so a test can assert the substitution still happens, since
// nothing in the output distinguishes "the state was known" from "the state was
// missing and we assumed the worst".
var invalidStateAsTop int

func successors(in Instr, st State, sw switchModel) ([]site, string) {
	// A MISSING abstract state is not a state that says "everything is zero", but
	// that is exactly what it used to mean here. Every call site indexes a map --
	// `absStates[at]`, `states[a]` -- and a Go map miss yields the zero State, whose
	// ValueRanges have Top=false, Lo=0, Hi=0: EXACT ZERO. accessOf then reads
	// st.SP.konst() and st.X/st.Y from it, so a PHA is modelled as writing precisely
	// $0100 and `lda table,x` as reading precisely `table`, and switchEdges decides
	// whether the instruction reaches a bank-switch hotspot from that footprint.
	// A narrower footprint can MISS a hotspot, which drops a cross-bank successor,
	// which shortens the predecessor set determineBound maximises over -- an
	// under-approximation, the one direction this package forbids.
	//
	// Measured over roms/techniques + roms/litmus (Prove + BeamIntervals + DefUse +
	// Lint): 1,994,520 calls, 2,572 with no usable state, 212 of those on a
	// bank-switched cartridge where the state is actually read, 124 of which still
	// produced a concrete address, and 6 whose footprint genuinely differs from the
	// sound answer. Six, not zero.
	//
	// A flat image is unaffected: switchEdges returns before touching st when the
	// cartridge has no hotspots, which is why every flat ROM's output is unchanged.
	if !st.valid {
		invalidStateAsTop++
		st = topState()
	}
	edges, keep, refusal := sw.switchEdges(in, st)
	if refusal != "" {
		return nil, refusal
	}
	if len(edges) == 0 {
		return decodeSuccessors(in), ""
	}
	if !keep {
		return edges, ""
	}
	return append(edges, decodeSuccessors(in)...), ""
}

// --- region model + worst-case (DAG longest-path) ---

// Step is one instruction (or one folded loop) on a worst-case path, with the
// cycles it is charged.
type Step struct {
	Addr uint16 `json:"addr"`
	// Bank names which bank's bytes this step executes. BankValid distinguishes
	// "bank 0" from "not a banked image", the same pattern Region uses, so a flat
	// ROM's JSON is unchanged. Without it a worst path that crosses banks prints two
	// entirely different instructions as the same $FF03.
	Bank      int    `json:"bank,omitempty"`
	BankValid bool   `json:"bank_valid,omitempty"`
	Cyc       int    `json:"cyc"`
	Loop      int    `json:"loop,omitempty"` // iteration count if this step is a folded counted loop
	Loc       string `json:"loc,omitempty"`  // srcmap "Label+off (file:line)" when available
}

// Region is one WSYNC-to-WSYNC interval and its proven worst case.
type Region struct {
	Start    uint16 `json:"start"` // address of the WSYNC store that opens the region
	StartLoc string `json:"start_loc,omitempty"`
	Kind     string `json:"kind,omitempty"` // "visible" (budget-checked) or "blank" (VSYNC/VBLANK; skipped)
	// Bank names which bank this region lives in. BankValid distinguishes "bank 0"
	// from "not a banked image", so a flat ROM's JSON is unchanged (omitempty on a
	// plain int would also hide a real bank 0).
	Bank      int    `json:"bank,omitempty"`
	BankValid bool   `json:"bank_valid,omitempty"`
	Worst     int    `json:"worst"` // proven worst-case cycles from here to the next WSYNC
	Budget    int    `json:"budget"`
	Over      bool   `json:"over"`             // Worst > Budget
	Bounded   bool   `json:"bounded"`          // false => could not prove (reported, not passed)
	Reason    string `json:"reason,omitempty"` // why unbounded
	Path      []Step `json:"path,omitempty"`   // worst-case breakdown (filled when Over)

	// Conditional turns "cannot prove" into something a builder can act on.
	// When the ONLY obstacle is a loop whose iteration count could not be
	// established, the region's cost is still a known function of that count, so
	// the largest count that fits the budget is computable — and that number is
	// the useful one: not "unprovable" but "stays inside 76 cycles as long as
	// this loop runs at most 7 times".
	//
	// It is an OBLIGATION, not a proof. Bounded stays false and the region never
	// counts towards Certified; discharging it means proving the bound elsewhere,
	// asserting it at run time, or the author accepting it explicitly.
	Conditional *Obligation `json:"conditional,omitempty"`

	// SwitchEdges is how many MODELLED cross-bank control-flow edges this region's own
	// subgraph contains. It is evidence, not decoration: a region reported as bounded on
	// a bank-switched cartridge whose crossing this analysis was supposed to follow, yet
	// which records no edge, is a visible bug rather than an invisible one. Always 0 on
	// a flat image, so its JSON is unchanged.
	SwitchEdges int `json:"switch_edges,omitempty"`
}

// Obligation is a conditional bound: the region is within budget provided the
// named loop does not exceed MaxIterations.
type Obligation struct {
	LoopHeader    string `json:"loop_header"`
	LoopLoc       string `json:"loop_loc,omitempty"`
	MaxIterations int    `json:"max_iterations"`
	WorstAtMax    int    `json:"worst_at_max"`
	Budget        int    `json:"budget"`
	Statement     string `json:"statement"`
}

type loopInfo struct {
	cost int // worst case: n iterations
	// minCost is the cost of the FEWEST iterations the loop can run. The
	// worst-case prover never needed it, but an interval does: using the
	// worst-case cost as the lower bound too claims the loop always runs its
	// maximum, which for a divide-by-15 positioning loop is only true for one
	// target X. Measured consequence of getting this wrong: 20 observed TIA
	// writes landed EARLIER than their "proven" earliest position.
	minCost int
	exit    site
	n       int
}

type result struct {
	cyc  int
	path []Step
}

const (
	white = 0
	gray  = 1
	black = 2
)

// ctx is the interprocedural call context (2A): the site execution returns to when
// the active subroutine's RTS retires, and whether a call is active at all.
//
// `active` is a separate bool rather than a zero-address sentinel because
// (bank 0, $0000) is an address-shaped value: "no active call" must not be spelled
// as an address, or a legitimate return site could be read as "top level".
//
// The return site is the CALLER's bank — the bank in which the JSR's own fetch
// happened — not whichever bank is current when the RTS retires. litmus_bank does
// exactly this (jsr in bank 0, a switch inside PingPong, the rts fetched back in
// bank 0).
type ctx struct {
	ret    site
	active bool
}

// lkey keys the longest-path memo/colour by (code site, call context), so the same
// subroutine site called from different places is solved per-caller.
type lkey struct {
	at site
	c  ctx
}

type solver struct {
	nodes     map[site]Instr
	sinks     map[site]bool
	folds     map[site]loopInfo
	memo      map[lkey]result
	state     map[lkey]int
	absStates map[site]State // S3: abstract state per site, for page-cross precision
	sw        switchModel    // the one cross-bank flow oracle
	banked    bool           // print the bank in messages only when there is more than one
	cyclic    bool
	unbounded bool   // 2A: a path needs interprocedural support we don't model (nested call / RTS w/o caller)
	unbReason string // why
	sm        *srcmap.Map
	amaxHint  int // ②: `@amax N` = author-declared upper bound of a divide-loop accumulator; used by determineBound when the abstract range is Top
	// allInstrs is the WHOLE decoded program, not just this region's subgraph.
	// determineBound's predecessor scan needs it: a divide loop's entry value is
	// normally established BEFORE the region's opening WSYNC (`lda #SCORE_X` then
	// `jsr SetXPos`, whose first act is `sta WSYNC`), and a scan confined to the
	// region cannot see it. Measured on a probe pair that differ only in whether the
	// `lda #78` sits before or after the opening WSYNC: 116cy NOT CERTIFIED vs 60cy
	// CERTIFIED, for machine behaviour that is identical.
	allInstrs map[site]Instr
	// ctxCall is the JSR this region is being analysed THROUGH, when there is one.
	// analyzeRegionInContexts walks a shared subroutine once per call site, so the
	// entry value of a loop inside it is a property of the context — not of the
	// site-keyed join in absStates, which merges every caller. Zero value means
	// "no context" (the flat walk), and determineBound then behaves as before.
	ctxCall      site
	ctxCallValid bool

	// pending holds a loop whose body is perfectly well understood but whose
	// iteration count could not be established. Keeping it, instead of only
	// reporting that the region is unbounded, is what lets the caller ask the
	// question that actually helps: how many times may this run before the
	// region misses its budget?
	pending *pendingLoop
}

type pendingLoop struct {
	header       site
	latch        Instr // carries its own bank in Instr.Bank
	bodyNoBranch int
	pen          int
	exit         site
}

// costAt returns the loop's cycle cost for exactly n iterations.
func (pl *pendingLoop) costAt(n int) int {
	if n < 1 {
		n = 1
	}
	return n*pl.bodyNoBranch + (n-1)*(pl.latch.Def.Cycles+pl.pen) + pl.latch.Def.Cycles
}

func prepend(s Step, rest []Step) []Step {
	return append([]Step{s}, rest...)
}

// theSuccSites is the successor set of a straight-line (non-branch, non-call)
// instruction. On a flat image that is exactly one site; on a banked image an
// instruction that reaches a bank-switch hotspot continues in the target bank, and
// an over-wide footprint can name several candidates plus the fall-through — so the
// caller takes the MAXIMUM over them.
func (s *solver) theSuccSites(in Instr) ([]site, string) {
	if in.Def.Operator == instructions.JMP { // absolute (indirect was filtered out)
		return []site{{in.Bank, in.Operand}}, ""
	}
	return successors(in, s.absStates[in.site()], s.sw)
}

// longest returns the worst-case cycles (and breakdown) from a code site to the next
// WSYNC sink, within call context c (2A). It assumes the region subgraph is a DAG
// (loops already folded); any remaining back-edge sets cyclic so the caller reports
// the region as unbounded rather than returning a bogus number. Memoised per
// (site, context). Anything we cannot bound soundly (a back-edge, a nested call, an
// RTS with no caller in context, or a bank switch whose edge is not modelled) sets
// s.cyclic / s.unbounded so the caller reports the region unbounded rather than
// trusting a low number.
func (s *solver) longest(at site, c ctx) result {
	k := lkey{at, c}
	if r, ok := s.memo[k]; ok {
		return r
	}
	if s.state[k] == gray {
		s.cyclic = true
		return result{}
	}
	s.state[k] = gray

	var best result
	switch {
	case s.foldHit(at):
		lf := s.folds[at]
		sub := s.longest(lf.exit, c)
		best = result{cyc: lf.cost + sub.cyc,
			path: prepend(s.step(at, lf.cost, lf.n), sub.path)}
	default:
		in := s.nodes[at]
		switch {
		case s.sinks[at]:
			best = result{cyc: in.Def.Cycles, path: []Step{s.step(at, in.Def.Cycles, 0)}}
		case in.Def.Operator == instructions.JSR:
			// 2A: follow into the callee with the return SITE threaded. Only one level
			// deep — a call while a call is already active is not modeled. The callee and
			// the return point are both in the JSR's own bank: its operand fetch already
			// happened there, and a JSR into a hotspot is refused by switchEdges.
			if c.active {
				s.unbounded = true
				s.unbReason = "nested subroutine call (single-level interprocedural only)"
				break
			}
			sub := s.longest(site{in.Bank, in.Operand}, ctx{ret: in.nextSite(), active: true})
			best = result{cyc: in.Def.Cycles + sub.cyc,
				path: prepend(s.step(in.site(), in.Def.Cycles, 0), sub.path)}
		case in.Def.Operator == instructions.RTS || in.Def.Operator == instructions.RTI:
			// 2A: return to the active call site; no caller in context => cannot bound.
			if !c.active {
				s.unbounded = true
				s.unbReason = "RTS/RTI with no caller in context"
				break
			}
			// The return site was recorded in the CALLER's bank. If the callee has
			// switched bank and returns without switching back, the hardware resumes at
			// that address in the NEW bank — different bytes, different cost — and
			// costing the caller's bytes there is an under-approximation, the one
			// direction this package forbids.
			//
			// No ROM in the corpus does it (every trampoline switches back before
			// returning), so there is no witness and nothing would have caught it. Found
			// by adversarial review of the cross-bank rekey, confirmed by reading the
			// ctx the JSR builds at the call site: ctx{ret: in.nextSite()} stamps the
			// caller's bank unconditionally.
			//
			// Refused rather than modelled: following it properly means tracking the
			// bank across the call, which is more than this stage does.
			if in.Bank != c.ret.bank {
				s.unbounded = true
				s.unbReason = fmt.Sprintf("RTS in bank %d returns to a site recorded in bank %d — the "+
					"callee switched bank and did not switch back, so the bytes at the return address "+
					"are not the ones costed", in.Bank, c.ret.bank)
				break
			}
			sub := s.longest(c.ret, ctx{})
			best = result{cyc: in.Def.Cycles + sub.cyc,
				path: prepend(s.step(in.site(), in.Def.Cycles, 0), sub.path)}
		case in.Def.IsBranch():
			// THE SAME PRUNE collectRegion APPLIES, and it has to be the same or the
			// two disagree: collection would leave the dead arm out of `s.nodes` while
			// this walk still asked for its cost, and `longest` on a site that was
			// never collected reports the whole region unbounded. Measured when the
			// prune was added to collection alone: five regions lost their bound
			// (M.A.S.H. $F126, Bermuda Triangle $F4F8, Star Wars $F649 and its bank-1
			// image, Planet of the Apes $F86C) — not because the pruning was wrong but
			// because only half the analysis knew about it.
			onlyTaken, onlyNotTaken := false, false
			if st, ok := s.absStates[at]; ok && st.valid {
				tkS, ntS := st.refineBranch(in)
				onlyTaken = tkS.valid && !ntS.valid
				onlyNotTaken = ntS.valid && !tkS.valid
			}
			var nt, tk result
			if !onlyTaken {
				nt = s.longest(in.nextSite(), c)
			}
			if !onlyNotTaken {
				tk = s.longest(in.branchSite(), c)
			}
			pen := 1
			// The page test stays flat address arithmetic: a branch cannot leave its bank,
			// and the +1 is about which 256-byte page the two addresses sit in.
			if (in.next() >> 8) != (in.branchTarget() >> 8) {
				pen = 2 // taken branch crossing a page costs +2 total over base
			}
			ntTot := in.Def.Cycles + nt.cyc
			tkTot := in.Def.Cycles + pen + tk.cyc
			if onlyTaken {
				ntTot = -1 // the machine cannot fall through here
			}
			if onlyNotTaken {
				tkTot = -1 // the machine cannot take this branch
			}
			if tkTot >= ntTot {
				best = result{cyc: tkTot, path: prepend(s.step(in.site(), in.Def.Cycles+pen, 0), tk.path)}
			} else {
				best = result{cyc: ntTot, path: prepend(s.step(in.site(), in.Def.Cycles, 0), nt.path)}
			}
		default:
			cost := in.baseCost() + in.pagePenalty(s.absStates[at]) // S3: page penalty from the index range
			succ, refusal := s.theSuccSites(in)
			if refusal != "" {
				// The collect pass uses the same oracle, so reaching this means the region
				// should already have been refused. Refuse rather than cost a path whose
				// continuation is unknown.
				s.unbounded = true
				s.unbReason = refusal
				break
			}
			// A switching instruction with an over-wide footprint has more than one
			// possible continuation; the worst case is the maximum over them.
			var bestSub result
			haveSub := false
			for _, nx := range succ {
				sub := s.longest(nx, c)
				if !haveSub || sub.cyc > bestSub.cyc {
					bestSub, haveSub = sub, true
				}
			}
			if !haveSub {
				s.unbounded = true
				s.unbReason = fmt.Sprintf("no successor for the instruction at %s", siteDesc(at, s.banked))
				break
			}
			best = result{cyc: cost + bestSub.cyc,
				path: prepend(s.step(in.site(), cost, 0), bestSub.path)}
		}
	}

	s.state[k] = black
	s.memo[k] = best
	return best
}

// step builds one worst-path row, tagging the bank only on a banked image so a flat
// ROM's JSON is unchanged.
func (s *solver) step(at site, cyc, loop int) Step {
	st := Step{Addr: at.addr, Cyc: cyc, Loop: loop, Loc: s.loc(at)}
	if s.banked {
		st.Bank, st.BankValid = at.bank, true
	}
	return st
}

// collectRegion walks the region subgraph from the instruction after start and
// fills s.nodes / s.sinks. It returns "" on success or the reason the region
// cannot be reasoned about. Shared with the beam-interval pass so both analyses
// see the same subgraph — two collectors would eventually disagree, and the
// disagreement would be invisible.
func (s *solver) collectRegion(instrs map[site]Instr, start Instr) string {
	return s.collectFrom(instrs, start.nextSite())
}

// collectFrom walks the subgraph from one site, adding to whatever is already
// collected. Shared so a caller's continuation can be brought into the same
// region without a second, subtly different walker.
func (s *solver) collectFrom(instrs map[site]Instr, from site) string {
	work := []site{from}
	for len(work) > 0 {
		at := work[len(work)-1]
		work = work[:len(work)-1]
		if _, ok := s.nodes[at]; ok {
			continue
		}
		in, ok := instrs[at]
		if !ok {
			// Naming the bank matters: a cross-bank hole would otherwise read as a
			// same-bank one, and the author would look for the wrong bug.
			return fmt.Sprintf("reaches undecoded address %s", siteDesc(at, s.banked))
		}
		s.nodes[at] = in
		if in.isWSYNC() {
			s.sinks[at] = true
			continue
		}
		switch in.Def.Operator {
		case instructions.RTS, instructions.RTI:
			// 2A: a leaf for collection — the return target is the call context,
			// resolved in longest() via the threaded return site.
			continue
		case instructions.BRK:
			// NAMING THE ADDRESS MATTERS. The region this refusal belongs to starts
			// somewhere else entirely — on pizza_boy.asm the region begins at $F075 and
			// the BRK is at $F490, four hundred bytes away inside a data table. Without
			// the address, finding it meant instrumenting the prover and re-running it.
			return fmt.Sprintf("BRK in region at %s", siteDesc(at, s.banked))
		case instructions.JAM:
			return "JAM (illegal/halt) in region"
		case instructions.JMP:
			if in.Def.AddressingMode == instructions.Indirect {
				return "indirect JMP in region (target not statically known)"
			}
		}
		// One successor function for every walker. JSR contributes both its callee and
		// its return point (the callee's own WSYNC is a sink as usual, and longest()
		// threads the return site); collection terminates via the seen-set.
		// A BRANCH WHOSE FLAG IS ALREADY DECIDED HAS ONE SUCCESSOR, NOT TWO.
		//
		// Collection used to take both arms of every branch unconditionally, so a
		// `lda #0 / sta Dx / beq .exit` — where Z is 1 by construction and the branch
		// is always taken — also walked the fall-through, straight into whatever
		// bytes follow. In `pizza_boy.asm` those bytes are the `Alley3A` snap table,
		// and its first `$00` decodes as BRK: the region was refused for containing
		// an instruction the machine never executes, at an address that holds data.
		//
		// `refineBranch` is the same test the abstract interpreter already applies in
		// `absSuccessors`; collection simply was not asking. It prunes only when the
		// flag is KNOWN — `triUnknown` keeps both arms — so this removes paths the
		// machine provably cannot take and no others. If both arms come back invalid,
		// the state is contradictory rather than informative, and both are kept.
		var succ []site
		var refusal string
		if st, ok := s.absStates[at]; ok && st.valid && in.Def.IsBranch() {
			tk, nt := st.refineBranch(in)
			if tk.valid && !nt.valid {
				succ = []site{in.branchSite()}
			} else if nt.valid && !tk.valid {
				succ = []site{in.nextSite()}
			}
		}
		if succ == nil {
			succ, refusal = successors(in, s.absStates[at], s.sw)
			if refusal != "" {
				return refusal
			}
		}
		work = append(work, succ...)
	}
	return ""
}

// callContexts returns, for a region that begins INSIDE a subroutine, the return
// addresses that could be active when it runs.
//
// A WSYNC inside a callee opens a region whose continuation lives in the CALLER,
// so with no call context the walk hits an RTS, finds no WSYNC ahead of it, and
// the region is reported unbounded. Measured on the corpus: five regions fail as
// "no WSYNC reached" and three as "RTS with no caller in context", and
// shared_setxpos.asm fails twice because its whole positioning routine is built
// that way.
//
// Each JSR contributes its return SITE as a candidate whenever the region start is
// reachable from that JSR's target without first returning. The return site carries
// the CALLER's bank, which is where the RTS's own fetch comes from.
func callContexts(instrs map[site]Instr, regionStart site, sw switchModel) []site {
	var out []site
	seenRet := map[site]bool{}
	for _, in := range instrs {
		if in.Def.Operator != instructions.JSR {
			continue
		}
		ret := in.nextSite()
		if seenRet[ret] || !reachableWithinCallee(instrs, site{in.Bank, in.Operand}, regionStart, sw) {
			continue
		}
		seenRet[ret] = true
		out = append(out, ret)
	}
	sortSites(out)
	return out
}

// displayPreserver returns a predicate answering "can a JSR to this address
// change VSYNC/VBLANK?" — used to keep the display bits alive across a call
// instead of dropping them with the rest of the callee's unmodeled effect.
//
// It is deliberately one-sided. Anything it cannot resolve — an indexed or
// indirect store, a store whose address folds into TIA $00/$01 through any
// mirror, a push (page 1 mirrors the console's own addresses, so SP can aim a
// PHA at VBLANK), or a nested call it has already visited — answers "not
// preserved", which restores the old behaviour for that call. Being wrong in
// this direction only loses precision; being wrong the other way would let a
// region that really does turn the display on be classified as blank and skipped.
// states may be nil on the first pass, when no index ranges are known yet; the
// caller then runs a second pass with the states the first produced, which is
// what makes `sta RESP0,X` — the ordinary way a shared positioning routine picks
// its object — resolvable at all. The second pass only ever ADDS a justified
// fact, and the justification itself comes from a sound first-pass range, so the
// fixpoint stays sound.
// A callee that switches banks must be walked ACROSS the switch, or "provably cannot
// write VSYNC/VBLANK" would be a claim about half the callee. The rule stays
// one-sided: a switch whose edge cannot be followed answers false.
func displayPreserver(instrs map[site]Instr, states map[site]State, sw switchModel) func(site) bool {
	memo := map[site]bool{}
	var walk func(entry site, depth int, onStack map[site]bool) bool
	walk = func(entry site, depth int, onStack map[site]bool) bool {
		if v, ok := memo[entry]; ok {
			return v
		}
		if depth > 8 || onStack[entry] { // recursion / deep nesting: give up, safely
			return false
		}
		onStack[entry] = true
		defer delete(onStack, entry)

		seen := map[site]bool{entry: true}
		work := []site{entry}
		for len(work) > 0 {
			a := work[len(work)-1]
			work = work[:len(work)-1]
			in, ok := instrs[a]
			if !ok {
				continue
			}
			switch in.Def.Operator {
			case instructions.RTS, instructions.RTI, instructions.BRK, instructions.JAM:
				continue
			case instructions.PHA, instructions.PHP:
				if !pushMissesDisplay(in, states[a]) {
					memo[entry] = false
					return false
				}
			case instructions.JSR:
				if !walk(site{in.Bank, in.Operand}, depth+1, onStack) {
					memo[entry] = false
					return false
				}
			case instructions.STA, instructions.STX, instructions.STY:
				if !storeMissesDisplay(in, states[a]) {
					memo[entry] = false
					return false
				}
			}
			succ, refusal := successors(in, states[a], sw)
			if refusal != "" {
				memo[entry] = false
				return false // an unfollowable switch: half the callee is unknown
			}
			for _, nx := range succ {
				if !seen[nx] {
					seen[nx] = true
					work = append(work, nx)
				}
			}
		}
		memo[entry] = true
		return true
	}
	return func(entry site) bool { return walk(entry, 0, map[site]bool{}) }
}

// storeMissesDisplay reports that this store provably cannot reach VSYNC ($00) or
// VBLANK ($01). Only a non-indexed store to a statically known address qualifies,
// and the address is folded through the engine's own memory map first so a write
// via any TIA mirror is caught rather than compared as a raw number.
// It takes the State at the instruction rather than the whole map: the map is keyed
// on a code site now, and passing the map would make every caller re-derive the key.
func storeMissesDisplay(in Instr, st State) bool {
	if in.Def.AddressingMode != instructions.Absolute {
		return indexedStoreMissesDisplay(in, st)
	}
	ma, area := memorymap.MapAddress(in.Operand, false)
	if area != memorymap.TIA {
		return true
	}
	// MapAddress folds any TIA mirror onto the canonical register address, so the
	// comparison is against $00/$01 there rather than against the raw operand.
	return ma != 0x00 && ma != 0x01
}

// pushMissesDisplay reports that a PHA/PHP cannot land on VSYNC/VBLANK.
//
// A push is a store to $0100|SP, and page 1 mirrors the addresses the console
// decodes, so a program that aims SP at the TIA turns a push into a register
// write — the Stack Trick this project uses deliberately. Treating EVERY push as
// display-touching would be sound but would drag every RTS-dispatch and every
// ordinary subroutine call into the visible class. SP is tracked, so the question
// is answerable: only an SP range that can reach $0100/$0101 counts.
func pushMissesDisplay(in Instr, st State) bool {
	if !st.valid || st.SP.Top || st.SP.Hi-st.SP.Lo > 255 {
		return false
	}
	for sp := st.SP.Lo; sp <= st.SP.Hi; sp++ {
		ma, area := memorymap.MapAddress(uint16(0x0100|(sp&0xFF)), false)
		if area == memorymap.TIA && (ma == 0x00 || ma == 0x01) {
			return false
		}
	}
	return true
}

// indexedStoreMissesDisplay resolves `sta base,X` (or ,Y) through the index range
// the previous pass proved, and reports that no address it can reach is VSYNC or
// VBLANK. An unknown or unusably wide range answers false.
func indexedStoreMissesDisplay(in Instr, st State) bool {
	switch in.Def.AddressingMode {
	case instructions.AbsoluteX, instructions.AbsoluteY:
	default:
		return false // indirect: no base to reason from
	}
	if !st.valid {
		return false
	}
	idx := st.storeIndex(in)
	if idx.Top || idx.Hi-idx.Lo > 255 {
		return false
	}
	for i := idx.Lo; i <= idx.Hi; i++ {
		ma, area := memorymap.MapAddress(in.Operand+uint16(i), false)
		if area == memorymap.TIA && (ma == 0x00 || ma == 0x01) {
			return false
		}
	}
	return true
}

// reachableWithinCallee reports whether target is reachable from entry without
// passing through a return — i.e. whether it is part of that subroutine's body.
//
// It follows cross-bank edges. litmus_bank's PingPong is exactly a callee that
// switches banks mid-body; without the edge the region start is not seen inside the
// callee, the call context is dropped, and the region fails as "RTS with no caller in
// context" instead of being bounded. Reachability is a MAY question, so an
// unfollowable switch simply stops that branch of the walk — the answer can only be
// too small, and too small means one fewer call context, which is the direction that
// refuses rather than the one that flatters.
// ctxEntryA answers "what is A when this context enters that loop header?" — or -1
// when it cannot say. It is the whole of K6's narrow fix.
//
// absStates is keyed by SITE, so A at a shared subroutine's loop header is the join
// over every caller. pizza_boy.asm calls SetXPos five times: twice from VBLANK with a
// sprite coordinate out of RAM (Top) and three times from the HUD with a compile-time
// constant. The join is Top, so the constant callers inherited 19 divide iterations
// against the 6 they can actually reach, and the region was reported over budget on a
// path that does not exist.
//
// The region walk already knows which call it is inside. When nothing between the
// callee's entry and the loop header can touch A — `sec` and a WSYNC store, for the
// positioning idiom — the accumulator at the CALL is the accumulator at the header,
// exactly, for this context. Anything less certain returns -1 and the caller falls back
// to the site-keyed scan and then to the 255 floor.
//
// Sound because analyzeRegionInContexts uses the result for THIS context only and takes
// the worst across contexts; a per-context bound narrower than the join is not a claim
// about the other contexts.
func (s *solver) ctxEntryA(header site) int {
	if !s.ctxCallValid || s.allInstrs == nil || s.absStates == nil {
		return -1
	}
	jin, ok := s.allInstrs[s.ctxCall]
	if !ok {
		return -1
	}
	st, ok := s.absStates[s.ctxCall]
	if !ok || !st.valid || st.A.Top {
		return -1
	}
	callee := site{jin.Bank, jin.Operand}
	if !accumulatorSurvives(s.allInstrs, callee, header, s.sw) {
		return -1
	}
	return int(st.A.Hi)
}

// preservesA lists the operators that provably leave the accumulator alone. It is a
// WHITELIST and everything absent from it counts as clobbering A, which is the safe
// default when the answer is "this analysis does not know": being wrong here would let
// a divide loop be bounded from a value the machine no longer holds, and that is an
// UNDER-approximation — the one direction this package forbids.
//
// JSR is deliberately absent. A nested call's effect on A is not modelled, so a route
// that goes through one cannot claim A survived it. So are PLA (loads A from the stack)
// and every arithmetic/logic operator. The shifts appear only in their memory forms:
// `asl` with no operand is Implied and targets A, which is exactly what SetXPos does
// four times AFTER its loop, so getting this distinction wrong would be easy and would
// matter.
func preservesA(in Instr) bool {
	switch in.Def.Operator {
	case instructions.STA, instructions.STX, instructions.STY,
		instructions.LDX, instructions.LDY,
		instructions.INX, instructions.INY, instructions.DEX, instructions.DEY,
		instructions.TAX, instructions.TAY, instructions.TSX, instructions.TXS,
		instructions.PHA, instructions.PHP,
		instructions.CMP, instructions.CPX, instructions.CPY, instructions.BIT,
		instructions.INC, instructions.DEC,
		instructions.CLC, instructions.SEC, instructions.CLI, instructions.SEI,
		instructions.CLV, instructions.CLD, instructions.SED,
		instructions.NOP,
		instructions.JMP,
		instructions.BCC, instructions.BCS, instructions.BEQ, instructions.BNE,
		instructions.BMI, instructions.BPL, instructions.BVC, instructions.BVS:
		return true
	case instructions.ASL, instructions.LSR, instructions.ROL, instructions.ROR:
		// Implied = accumulator form; the memory forms leave A alone.
		return in.Def.AddressingMode != instructions.Implied
	}
	return false
}

// accumulatorSurvives reports whether A provably still holds the caller's value when
// control reaches `target` from `entry`. Every instruction on every route must be one
// preservesA vouches for; an unresolvable successor set, an RTS (control leaving before
// the target), or a step limit all answer false.
//
// This is what makes a call site's A usable as a loop's entry value. `lda #SCORE_X` /
// `jsr SetXPos` / `sec` / `sta WSYNC` / `sbc #15` is the entire positioning idiom, and
// between the call and the loop it executes exactly `sec` — so the answer is yes, and
// the constant the caller loaded is the constant the loop divides.
func accumulatorSurvives(instrs map[site]Instr, entry, target site, sw switchModel) bool {
	const maxSteps = 4096
	seen := map[site]bool{}
	work := []site{entry}
	steps := 0
	for len(work) > 0 {
		steps++
		if steps > maxSteps {
			return false
		}
		a := work[len(work)-1]
		work = work[:len(work)-1]
		if a == target {
			continue // reached the header; this route is clean
		}
		if seen[a] {
			continue
		}
		seen[a] = true
		in, ok := instrs[a]
		if !ok {
			return false
		}
		if !preservesA(in) {
			return false
		}
		succ, refusal := successors(in, topState(), sw)
		if refusal != "" || len(succ) == 0 {
			return false
		}
		for _, nx := range succ {
			work = append(work, nx)
		}
	}
	return true
}

func reachableWithinCallee(instrs map[site]Instr, entry, target site, sw switchModel) bool {
	seen := map[site]bool{entry: true}
	work := []site{entry}
	for len(work) > 0 {
		a := work[len(work)-1]
		work = work[:len(work)-1]
		if a == target {
			return true
		}
		in, ok := instrs[a]
		if !ok {
			continue
		}
		switch in.Def.Operator {
		case instructions.RTS, instructions.RTI, instructions.BRK, instructions.JAM:
			continue
		}
		succ, refusal := successors(in, topState(), sw)
		if refusal != "" {
			continue
		}
		for _, nx := range succ {
			if !seen[nx] {
				seen[nx] = true
				work = append(work, nx)
			}
		}
	}
	return false
}

// analyzeRegionInContexts re-analyses a region once per candidate return address
// and returns the WORST result, or nil when any context fails to resolve. Every
// context must succeed: a region provable from one call site and not from another
// is not provable, and reporting the provable one would be choosing the
// flattering answer.
func analyzeRegionInContexts(instrs map[site]Instr, start Instr, budget, amaxHint int,
	sm *srcmap.Map, states map[site]State, sw switchModel, ctxs []site) *Region {
	var worst *Region
	for _, ret := range ctxs {
		s := &solver{
			nodes: map[site]Instr{}, sinks: map[site]bool{}, folds: map[site]loopInfo{},
			memo: map[lkey]result{}, state: map[lkey]int{}, absStates: states, sm: sm, amaxHint: amaxHint,
			sw: sw, banked: sw.banked, allInstrs: instrs,
		}
		// The JSR that opened this context sits three bytes before the return site;
		// confirm it against the decoded instruction rather than assuming.
		if js := (site{ret.bank, ret.addr - 3}); true {
			if jin, ok := instrs[js]; ok && jin.Def.Operator == instructions.JSR {
				s.ctxCall, s.ctxCallValid = js, true
			}
		}
		if msg := s.collectRegion(instrs, start); msg != "" {
			return nil
		}
		// Bring the caller's continuation in; without it the walk stops at the RTS
		// and never reaches the WSYNC that ends the region.
		if msg := s.collectFrom(instrs, ret); msg != "" {
			return nil
		}
		if len(s.sinks) == 0 {
			return nil
		}
		// Same order as the flat walk, and for the same measured reason: fold
		// first, and let the DAG walk override only the back-edge refusal. See
		// multipleBackEdges. Handling one call site and not the other would leave a
		// region provable at top level and refused inside a subroutine, which is a
		// split the caller cannot see.
		var r result
		msg := s.foldLoops()
		if msg == multipleBackEdges {
			s.resetSolve()
			r = s.longest(start.nextSite(), ctx{ret: ret, active: true})
			if s.cyclic || s.unbounded {
				return nil
			}
			dagFirstAnswered++
			if addressOrderLatches(s) > 1 {
				dagFirstMultiLatch++
			}
		} else {
			if msg != "" {
				return nil
			}
			r = s.longest(start.nextSite(), ctx{ret: ret, active: true})
			if s.cyclic || s.unbounded {
				return nil
			}
		}
		// CLASSIFY THIS CONTEXT, NOT THE JOIN OF ALL OF THEM.
		//
		// This used to read states[start.site()] — the callee's entry state, which for a
		// shared subroutine is the join over every caller. One visible caller therefore
		// made the display bits unknown at the entry and EVERY context came out
		// "visible", including ones that provably run inside VBLANK. That is what "the
		// blank classification does not cross a JSR" actually meant: the JSR edge
		// carries the bits correctly; the loss is at the merge on the other side.
		//
		// Written once before and MEASURED AS A NO-OP (102/157 certified either way, not
		// one bound moved), then reverted rather than shipped unwitnessed. It was a
		// no-op only because loop bounds were context-INSENSITIVE, so every context cost
		// the same and the ranking below could not change anything. With ctxEntryA above
		// giving each context its own entry value, the contexts differ and it does.
		kind := "visible"
		if s.ctxCallValid {
			if st, ok := states[s.ctxCall]; ok && st.displayOff() && !regionTouchesDisplay(s.nodes, states) {
				kind = "blank"
			}
		} else if st, ok := states[start.site()]; ok && st.displayOff() && !regionTouchesDisplay(s.nodes, states) {
			kind = "blank"
		}
		// A VISIBLE context outranks a blank one whatever the cycle counts, because this
		// Region is what the budget gate reads and the gate is a claim about visible
		// lines. A blank context that overruns does not tear a line; it adds one, and
		// that failure mode has its own gate now — `frame_lines_stable` plus the
		// corpus-wide TestNoRomBreathesAcrossFrames — rather than being folded into a
		// tearing verdict, which is a different and much louder claim.
		better := worst == nil ||
			(kind == "visible" && worst.Kind == "blank") ||
			(kind == worst.Kind && r.cyc > worst.Worst)
		if better {
			reg := Region{
				Start: start.Addr, StartLoc: s.loc(start.site()), Budget: budget,
				Bounded: true, Kind: kind, Worst: r.cyc, Over: r.cyc > budget,
			}
			if sw.banked {
				reg.Bank, reg.BankValid = start.Bank, true
			}
			if reg.Over {
				reg.Path = r.path
			}
			worst = &reg
		}
	}
	return worst
}

// solveObligation answers "how many times may this loop run before the region
// misses its budget?" by folding the pending loop at successive trip counts and
// re-solving the longest path. The search is linear from 1 and stops at the
// first count that overruns, so the answer is the largest count that still fits.
//
// A count of 0 means the region cannot meet its budget even with a single
// iteration — a fact worth reporting plainly rather than as "unprovable".
func (s *solver) solveObligation(start Instr, budget int) *Obligation {
	pl := s.pending
	if pl == nil {
		return nil
	}
	const maxTrip = 256
	best := 0
	worstAtBest := 0
	for n := 1; n <= maxTrip; n++ {
		// Fresh solver state per trial: memo and colouring are per-solve.
		s.folds = map[site]loopInfo{pl.header: {cost: pl.costAt(n), minCost: pl.costAt(1), exit: pl.exit, n: n}}
		s.memo = map[lkey]result{}
		s.state = map[lkey]int{}
		s.cyclic, s.unbounded, s.unbReason = false, false, ""
		r := s.longest(start.nextSite(), ctx{})
		if s.cyclic || s.unbounded {
			return nil // something else is wrong too; no clean obligation to state
		}
		if r.cyc > budget {
			break
		}
		best, worstAtBest = n, r.cyc
	}
	where := siteDesc(pl.header, s.banked)
	ob := &Obligation{
		LoopHeader:    where,
		LoopLoc:       s.loc(pl.header),
		MaxIterations: best,
		WorstAtMax:    worstAtBest,
		Budget:        budget,
	}
	if best == 0 {
		ob.Statement = fmt.Sprintf("the region misses its %d-cycle budget even at one iteration of the loop at %s",
			budget, where)
	} else {
		ob.Statement = fmt.Sprintf("within %d cycles provided the loop at %s runs at most %d time(s) (worst %d there)",
			budget, where, best, worstAtBest)
	}
	return ob
}

func (s *solver) foldHit(at site) bool { _, ok := s.folds[at]; return ok }

// loc resolves a code site to "Label+off (file:line)".
//
// On a BANKED image NO bank gets a source location, and that is a limitation with a
// measured cause rather than an oversight.
//
// DASM's listing address column is the PHYSICAL ROM OFFSET, not the RORG'd address
// (litmus_bank's listing shows `0f00 ad f9 ff lda $FFF9` for bank 0 and `1f03 a9 b1
// lda #$B1` for bank 1), and srcmap.Parse drops every row below $1000 — so bank 0's
// line numbers are dropped entirely and banks 1..n's offsets are stored as if they
// were CPU addresses. That much this comment always said, and it concluded bank 0
// could still be resolved safely. That conclusion was WRONG, because it reasoned
// about line numbers and the LABEL list is built differently: labels come from the
// symbol dump, where every bank's labels carry their RORG'd $F0xx address, so the
// banks' labels are interleaved in one list and "the last label at or before this
// address" can belong to either. Measured on roms/techniques/banked_game.asm: two
// BANK 0 regions were reported at `LvTab+0` and `LvTab+2`, and LvTab is a bank 1
// table 4K away in the image. A confidently wrong location is worse than none — the
// author goes and reads correct code looking for a fault that is not there.
//
// So on a banked image every site prints as "bank N $FFxx" and nothing else.
// Report.SourceAnnotations says this out loud. The real repair is a per-bank line
// map, which is recoverable because the listing offset carries the bank
// (bank = offset >> 12); filed, not built.
func (s *solver) loc(at site) string {
	if s.banked {
		// The per-bank map keys on (bank, address) and takes its labels from the
		// SOURCE file rather than the symbol table, so it cannot name another bank's
		// code. When the listing has nothing at this site, the bare address is still
		// the honest answer.
		if l := s.sm.LocateBank(at.bank, at.addr); l != "" {
			return l
		}
		return siteDesc(at, true)
	}
	return s.sm.Locate(at.addr)
}

// dagFirst runs the longest-path walk over the UNFOLDED region subgraph and returns
// its result when the walk finished without meeting a cycle. A nil answer means the
// region needs loop modelling, and the solver is left reset so the caller can fold
// and re-solve exactly as if this had never run.
//
// WHY IT RUNS FIRST. foldLoops decides what is a loop by ADDRESS ORDER: a branch
// counts as a back edge when its target sits at a lower address than the branch and
// is not a WSYNC sink. That is not reachability. A branch backwards to code that
// cannot reach the branch again is not a back edge at all, and a region whose graph
// is a perfectly good DAG was being refused as "multiple back-edges" for having no
// edges back. Measured on the real cartridges: of 33 regions refused that way across
// the corpus (in-tree kernels + 5 commercial images), 4 are acyclic — Seaquest $F1EC
// (DAG 59, machine 53), Chopper Command $FA78 (74, machine 72) and $FAEC (103,
// machine 97 over 2 scanlines), Barnstorming $F3D4 (95). Seaquest $F1EC reads
// plainly: `$F1E8 bpl → $F1BB` and `$F1F6 beq → $F1E5` are both backward BRANCHES,
// but $F1BB reaches only the WSYNC sinks $F1D3/$F1EC and $F1E5 reaches only sinks.
// Nothing returns to either branch.
//
// SOUNDNESS.
//  1. The DAG longest path over s.nodes is an upper bound on the cost of every
//     reachable path, because collectFrom fills s.nodes through the SAME successor
//     oracle longest walks and over-approximates the edge set. This is not a new
//     claim: it is the machinery that produces every bounded region in the report
//     today. What changes is only WHEN it is allowed to run.
//  2. Folding exists solely to cut cycles. With no cycle there is nothing to cut, so
//     trying the DAG first cannot lower a bound that folding would have produced —
//     it can only produce a bound where there was a refusal.
//  3. Cycle detection is delegated to longest's own gray-node colouring rather than
//     to a second, independent notion of "back edge". collectRegion's comment warns
//     that two collectors "would eventually disagree, and the disagreement would be
//     invisible"; the address-order test IS that second, disagreeing notion, and
//     those 4 regions are the measured disagreement. One notion of a cycle, used by
//     the code that has to cost it.
//
// Anything the walk cannot bound for a NON-cycle reason (a nested call, an RTS with
// no caller, an unmodelled switch edge) still answers nil, so the fold path and its
// call-context fallbacks are reached unchanged.
// multipleBackEdges is the ONE refusal the DAG walk is allowed to override, and it
// is named rather than matched as a literal because the override has to be exact.
//
// foldLoops refuses for eight distinct reasons and only this one is about the shape
// of the GRAPH; the other seven are about the loop BODY — it leaves the region, it
// leaves the bank, it holds a WSYNC, it holds a branch, it switches banks, it is
// misaligned, its trip count is unknown. A DAG answer says nothing about any of
// those, and substituting one for them is unsound.
//
// Measured, and it is why this constant exists: an earlier version ran the DAG walk
// BEFORE foldLoops and accepted whenever the walk met no cycle. VideoOlympics
// $F5CA refuses for "WSYNC inside loop body", and that loop is INVISIBLE to the
// walk — the WSYNC is a sink, so the walk stops there and never traverses the edge
// back, leaving a subgraph that looks perfectly acyclic. The walk answered 148
// cycles for an interval the machine takes 163, which the standing corpus gate
// caught. $F61F did the same at 155 for "branch inside loop body". A bound below
// the machine's is worse than no bound, so the refusal order is: fold first,
// override only this.
const multipleBackEdges = "multiple back-edges (nested/complex loops) — not modeled in v1"

func (s *solver) dagFirst(start Instr) *result {
	r := s.longest(start.nextSite(), ctx{})
	if !s.cyclic && !s.unbounded {
		dagFirstAnswered++
		if addressOrderLatches(s) > 1 {
			dagFirstMultiLatch++
		}
		return &r
	}
	s.resetSolve()
	return nil
}

// resetSolve clears the per-solve memo, colouring and verdict so the same collected
// subgraph can be solved again. solveObligation does exactly this between trip
// counts; dagFirst does it after an attempt that met a cycle.
func (s *solver) resetSolve() {
	s.memo = map[lkey]result{}
	s.state = map[lkey]int{}
	s.cyclic, s.unbounded, s.unbReason = false, false, ""
}

// addressOrderLatches counts the branches foldLoops would treat as back edges. It is
// a MEASUREMENT of the old, address-order notion of a loop, kept so a test can prove
// its own premise: a region that the DAG answered while carrying two or more of these
// is a region the `len(latches) > 1` guard refused before this change. Counting is
// side-effect free — it does not fold anything.
func addressOrderLatches(s *solver) int {
	n := 0
	for _, in := range s.nodes {
		if !in.Def.IsBranch() {
			continue
		}
		tgt := in.branchSite()
		if _, ok := s.nodes[tgt]; ok && tgt.addr <= in.Addr && !s.sinks[tgt] {
			n++
		}
	}
	return n
}

// dagFirstAnswered / dagFirstMultiLatch census how often the DAG-first attempt
// settles a region, and how often it settles one the address-order guard would have
// refused outright.
//
// They are MEASUREMENT counters like divEdgeScanned and bplTripCountAdjusted, not
// analysis state. dagFirstAnswered is large and uninteresting on its own — most
// regions have no loop at all and were always answered by this walk, just after a
// fold pass that found nothing. dagFirstMultiLatch is the number that says whether
// this change does anything: it counts regions carrying >= 2 address-order latches
// and no cycle, which is exactly the set the "multiple back-edges" refusal used to
// take. A test that reads it can say out loud when its fixture is the only witness.
var (
	dagFirstAnswered   int
	dagFirstMultiLatch int
)

// foldLoops finds a single simple counted loop in the region subgraph and folds
// it into one synthetic node keyed by the loop header. Returns "" on success (or
// when there is no loop), or a reason string when a loop exists but can't be
// bounded (so the region is reported unbounded).
//
// It runs only AFTER dagFirst has reported a cycle, so the address-order latch test
// below is no longer the thing that decides whether a region has a loop at all — it
// only picks WHICH branch to fold in a region already proven cyclic.
func (s *solver) foldLoops() string {
	var latches []Instr
	for _, in := range s.nodes {
		if in.Def.IsBranch() {
			tgt := in.branchSite() // a branch cannot leave its bank
			// A backward branch to a WSYNC sink is the normal per-scanline loop
			// returning to the next region — a region boundary, not an intra-line
			// loop. Only branches back to a non-sink node are real loops to fold.
			if _, ok := s.nodes[tgt]; ok && tgt.addr <= in.Addr && !s.sinks[tgt] {
				latches = append(latches, in)
			}
		}
	}
	if len(latches) == 0 {
		return ""
	}
	// TWO LOOPS IN A REGION ARE NOT NECESSARILY NESTED, AND THE REFUSAL WAS NAMED
	// AFTER THE RAREST OF THE THREE SHAPES.
	//
	// This used to be `if len(latches) > 1 { return multipleBackEdges }`. Measured
	// across the sixteen-cartridge corpus, of the regions carrying exactly two
	// latches: 22 are SIBLINGS whose intervals do not overlap, 9 overlap
	// irreducibly, and exactly 1 is nested. Nesting is rare for a reason that is
	// obvious once stated — a region is one WSYNC-to-WSYNC interval, so a nest would
	// have to fit two levels of iteration inside a scanline.
	//
	// Siblings are two plain counted loops one after the other. Each is a fold the
	// code below already computes, `s.folds` is keyed by header and holds as many as
	// it is given, and the longest-path walk pays each fold as it reaches it. So the
	// region's cost is a sum of things already known, and the refusal was about the
	// COUNT of back edges rather than about anything the analysis could not do.
	//
	// The rule is all-or-nothing: every latch must fold and no two bodies may share
	// an instruction. A partial fold would leave a real loop in the graph for the
	// walk to meet, and overlapping bodies have no order in which folding one leaves
	// the other a simple counted loop.
	if len(latches) > 1 {
		// Deterministic order: `s.nodes` is a map, so the iteration above is
		// randomised, and a refusal string that depends on which latch was examined
		// first would make the corpus gate flap.
		sort.Slice(latches, func(i, j int) bool {
			if latches[i].Bank != latches[j].Bank {
				return latches[i].Bank < latches[j].Bank
			}
			return latches[i].Addr < latches[j].Addr
		})
		// EVERY FAILURE HERE REPORTS multipleBackEdges, INCLUDING THE SPECIFIC ONES.
		//
		// The first version returned the body refusal instead — "WSYNC inside loop
		// body" is more informative than "multiple back-edges", and a region whose
		// second loop holds a WSYNC should be able to say so. It cost **6 proven
		// regions** across the corpus (Barnstorming $F3D4, Chopper Command $FA78 and
		// $FAEC, Planet of the Apes $F8B9, Seaquest $F1EC, Stampede $F1A5), and the
		// reason is stated thirty lines above this: multipleBackEdges is the ONE
		// refusal `analyzeRegion` lets the DAG walk override. A more precise string
		// is a string the override does not match, so those regions stopped being
		// answered by the walk that had been answering them.
		//
		// Precision in the message is worth less than the answer it suppresses.
		shapes := make([]loopShape, 0, len(latches))
		for _, l := range latches {
			sh, refusal := s.loopShape(l)
			if refusal != "" {
				return multipleBackEdges
			}
			shapes = append(shapes, sh)
		}
		for i := range shapes {
			for j := i + 1; j < len(shapes); j++ {
				if shapes[i].overlaps(shapes[j]) {
					return multipleBackEdges
				}
			}
		}
		// determineBound is deferred to here so that NOTHING is folded until every
		// loop is known to be foldable — a half-folded region hands the walk a graph
		// with a real cycle still in it.
		type ready struct {
			sh loopShape
			n  int
		}
		var rs []ready
		for _, sh := range shapes {
			n := determineBound(s.nodes, s.allInstrs, sh.header, sh.latch, s.absStates, s.sw, s.amaxHint, s.ctxEntryA(sh.header))
			if n <= 0 {
				// s.pending carries ONE loop, and the caller uses it to answer "how
				// many iterations fit the budget?". With several loops in the region
				// that question has no single answer, so it is refused rather than
				// answered about whichever loop happened to be scanned last — and
				// refused as multipleBackEdges, so the DAG walk keeps its override.
				return multipleBackEdges
			}
			rs = append(rs, ready{sh, n})
		}
		for _, r := range rs {
			s.folds[r.sh.header] = r.sh.loopInfo(r.n)
		}
		siblingFolds++
		return ""
	}
	sh, refusal := s.loopShape(latches[0])
	if refusal != "" {
		return refusal
	}
	latch, header, bodyNoBranch, pen := sh.latch, sh.header, sh.bodyNoBranch, sh.pen
	n := determineBound(s.nodes, s.allInstrs, header, latch, s.absStates, s.sw, s.amaxHint, s.ctxEntryA(header))
	if n <= 0 {
		// A TIMER SPIN IS NOT A LOOP MISSING A COUNTER.
		//
		// Measured across the corpus, of the loops refused here for having neither a
		// counted dex/dey nor the divide idiom, TWENTY OF TWENTY-ONE are the same
		// thing: `lda $0284 / bne` — INTIM, the RIOT's interval-timer read port,
		// polled until it reaches zero. The trip count is not a property of any
		// register this analysis tracks; it is whatever the hardware has left to
		// count down, and no counter will ever appear however much the analysis is
		// strengthened.
		//
		// The refusal is right and the generic reason is wrong. "Needs a counted
		// dex/dey" tells a builder to look for an analysis gap that is not there,
		// when what the region needs is to be recognised as a wait. Naming it is the
		// whole of the change: no bound is invented, and `s.pending` is deliberately
		// NOT set, because "how many iterations fit in 76 cycles?" is not a question
		// anyone wants answered about a spin on a clock.
		if spin, port := timerSpin(s.nodes, header, latch); spin {
			timerSpins++
			return fmt.Sprintf("timer wait: the loop polls INTIM ($%04X) until it reaches "+
				"zero, so its length is set by the RIOT timer rather than by any counter "+
				"this analysis can read", port)
		}
		// The body is fully understood; only the trip count is missing. Keep it so
		// the caller can still answer "how many iterations fit the budget?".
		s.pending = &pendingLoop{header: header, latch: latch, bodyNoBranch: bodyNoBranch, pen: pen, exit: latch.nextSite()}
		return "loop bound unknown (need a counted dex/dey or sbc-divide idiom with a proven range)"
	}
	s.folds[header] = sh.loopInfo(n)
	return ""
}

// branchBodyFolds counts loop bodies costed as a DAG because they hold a branch.
// It is a MEASUREMENT counter like siblingFolds and divEdgeScanned: the corpus yield
// of the DAG body is ONE loop, so without this a fixture cannot tell whether it is
// exercising the new path or the old one.
var branchBodyFolds int

// siblingFolds counts regions settled by folding more than one loop. It is a
// MEASUREMENT counter like divEdgeScanned: a fixture can read it to prove its own
// premise, that the region it grades really took the multi-loop path rather than
// being answered by the single-loop path or by the DAG walk.
var siblingFolds int

// loopShape is everything foldLoops learns about one counted loop before it knows
// the trip count: which instructions are in the body, what they cost, and whether
// the back edge crosses a page.
type loopShape struct {
	latch        Instr
	header       site
	body         map[site]bool
	bodyNoBranch int
	pen          int
}

// overlaps reports whether two loops share an instruction. Siblings do not; nests
// and irreducible overlaps do, and both are refused.
//
// IT IS UNREACHABLE TODAY, AND MEASURED TO BE. Disabling it entirely (`return false`)
// leaves the fixture's NestedRow and OverlapRow refused exactly as before — the
// negative control does not fire. The reason is structural: if two loops share an
// instruction, one body contains the other's LATCH, a latch is a branch, and
// `loopShape` refuses a body holding a branch before this is ever consulted.
//
// It is kept, and the emptiness is written down rather than taken as permission to
// delete, because the very next repair to this package removes the check that hides
// it. `branch inside loop body` is the largest refusal left on the corpus; the moment
// a branch in the body is allowed, nests and overlaps reach here and this becomes the
// only thing standing between them and a fold that charges an inner loop once for
// iterations the machine runs many times. Its behaviour is pinned directly by
// TestOverlapsSeparatesSiblingsFromNests rather than through a fixture, since no
// fixture can reach it.
//
// It compares BODY SETS rather than address ranges. Address ranges would call two
// loops disjoint whenever their [header, latch] spans do not intersect, which is the
// same answer for the shapes that matter here but relies on the body being
// contiguous — and `loopShape` follows `nextSite()`, which is not obliged to walk
// addresses in order. The set is what the fold actually charges for, so the set is
// what must not be shared.
func (a loopShape) overlaps(b loopShape) bool {
	if a.header == b.header || a.latch.site() == b.latch.site() {
		return true
	}
	if b.body[a.header] || a.body[b.header] {
		return true
	}
	if b.body[a.latch.site()] || a.body[b.latch.site()] {
		return true
	}
	small, large := a.body, b.body
	if len(large) < len(small) {
		small, large = large, small
	}
	for at := range small {
		if large[at] {
			return true
		}
	}
	return false
}

// loopInfo turns a shape plus a trip count into the folded cost.
func (sh loopShape) loopInfo(n int) loopInfo {
	// n iterations: n bodies, (n-1) taken branches back, 1 final not-taken exit.
	loopCost := n*sh.bodyNoBranch + (n-1)*(sh.latch.Def.Cycles+sh.pen) + sh.latch.Def.Cycles
	// Fewest iterations: the back edge is a bne/bpl AFTER the body, so the body
	// runs at least once and the branch then falls through.
	minLoopCost := sh.bodyNoBranch + sh.latch.Def.Cycles
	return loopInfo{cost: loopCost, minCost: minLoopCost, exit: sh.latch.nextSite(), n: n}
}

// loopShape validates that one back edge closes a simple counted loop and returns
// its shape, or a refusal string naming what is wrong with the body.
//
// It is the whole of the old foldLoops from the body walk down to the page penalty,
// lifted out unchanged so that a region with several back edges can ask the same
// question about each of them.
func (s *solver) loopShape(latch Instr) (loopShape, string) {
	header := latch.branchSite()

	// Validate the body header..latch and find the cost of its LONGEST path.
	//
	// This used to require a straight chain and refuse any branch in it. Measured
	// across the sixteen-cartridge corpus, of the branches that tripped that refusal:
	//
	//	89   forward, target still inside the body   (an if/else skip)
	//	29   forward, target outside the body        (an early exit)
	//	 1   backward, inside the body               (an inner loop)
	//
	// Three quarters are a SKIP, most often `bcc` (64 of 118): add, and if it did not
	// carry, skip the fixup. The body is then not a chain but a small acyclic graph
	// with two ways through that rejoin before the latch — and such a graph has a
	// longest path, which is a sound cost for one iteration because whichever way the
	// machine goes it cannot spend more.
	//
	// The two shapes that are NOT sound to cost this way are refused below by the
	// successor test: a target outside [header, latch] means the loop can end early
	// and the cost after that exit belongs to another part of the region, and a target
	// before the header is an inner loop, which a single longest path charges once for
	// iterations the machine runs many times.
	// PASS 1 — collect the reachable body, validate every instruction in it, and
	// record each edge with its cost. Nothing is summed here: the sum depends on the
	// order the sites are visited, and this walk's order is a work queue.
	body := map[site]bool{}
	type edge struct {
		to   site
		cost int
	}
	edges := map[site][]edge{}
	pending := []site{header}
	for len(pending) > 0 {
		at := pending[0]
		pending = pending[1:]
		if body[at] {
			continue
		}
		in, ok := s.nodes[at]
		if !ok {
			return loopShape{}, "loop body leaves the region"
		}
		if in.Bank != latch.Bank {
			// Should be unreachable — every successor is checked to be in the latch's
			// bank below — but a body in another bank would make the address
			// comparisons meaningless, so it is refused rather than assumed away.
			return loopShape{}, "loop body leaves the latch's bank"
		}
		if in.Addr == latch.Addr {
			continue // the latch closes the body; its own cost is added by the caller
		}
		if in.isWSYNC() {
			return loopShape{}, "WSYNC inside loop body"
		}
		// A CALL OR JUMP IN THE BODY IS NOT A BRANCH, and `IsBranch` does not catch
		// it: that is `AddressingMode == Relative && Effect == Flow`, while a JSR is
		// Absolute/Subroutine and a JMP is Absolute/Flow. So the walk sailed through
		// both, and `nextSite()` sent it to the RETURN address of a JSR or to the
		// bytes textually after a JMP.
		//
		// The arithmetic alone is unsound: a JSR is costed at its own 6 cycles and the
		// callee's cycles are dropped, once per iteration. Measured on
		// litmus_callinloop, whose callee is twelve `nop`s: proven **48** cycles
		// against a machine that spends **168 across 3 scanlines** — 3.5x under, with
		// `certified: true`.
		//
		// The worse case is a callee containing `sta WSYNC`, because then the walk
		// steps over a REGION BOUNDARY. The machine's interval ends at that strobe and
		// the proof's does not, so the two numbers describe different intervals and
		// comparing them is a category error rather than a comparison.
		// `roms/techniques/shared_setxpos.asm` $F054 is exactly that shape — its
		// `jsr SetXPos` calls a routine whose second instruction is `sta WSYNC` — and
		// it reads **proven 98, machine 36**. Sound only by accident: the machine's
		// interval ends early, so the inflated number happens to sit above it.
		// Refusing loses that fold, which was over budget and describing the wrong
		// interval anyway.
		switch in.Def.Operator {
		case instructions.JSR, instructions.JMP,
			instructions.RTS, instructions.RTI, instructions.BRK:
			return loopShape{}, "call or jump inside loop body — the folded cost is not one iteration"
		}
		// A bank switch inside the body is a SOUNDNESS condition, not a precision one:
		// the folded cost n*body + (n-1)*(latch+pen) + latch assumes every iteration
		// executes the same bytes, and after a switch iteration 2 executes different
		// bytes at the same addresses.
		if es, keep, refusal := s.sw.switchEdges(in, s.absStates[at]); refusal != "" || len(es) > 0 || !keep {
			return loopShape{}, "bank switch inside loop body — the second iteration does not execute the same bytes"
		}
		body[at] = true

		// Record each successor edge. A branch has two: falling through costs its
		// base cycles, taking it costs one more — two if it crosses a page — which is
		// the same arithmetic the region-level walk uses.
		add := func(to site, cost int, viaBranch bool) string {
			if to.bank != latch.Bank {
				return "loop body leaves the latch's bank"
			}
			// OUTSIDE [header, latch] IS NOT COSTABLE HERE. Past the latch is an early
			// exit: the trip count is then not the counter's range, and the cost after
			// the exit belongs to another part of the region — the fold replaces
			// header..exit and never charges the escape path, so a long one is simply
			// dropped. Before the header, the walk would collect sites below the
			// header and ascending address would stop being a topological order,
			// which is the premise pass 2 rests on.
			//
			// ITS NEGATIVE CONTROL DOES NOT FIRE, and that is written down rather than
			// taken as permission to delete it. Disabling this test leaves the
			// fixture's ExitCtl refused for "WSYNC inside loop body": the walk follows
			// the escape, and a region is bounded by WSYNC strobes, so an escape
			// inside one runs into a strobe. A backward escape is caught earlier still
			// — a branch below the header is itself a back edge, so the region carries
			// two latches and never reaches here. Both are accidents of other checks,
			// neither states the premise, and the premise is what pass 2 needs.
			if to.addr > latch.Addr || to.addr < header.addr {
				if viaBranch {
					return "branch inside loop body — not a simple counted loop"
				}
				// Straight-line code whose next address is outside the body is the
				// old "misaligned loop body", and it keeps that name: nothing
				// branched anywhere.
				return "misaligned loop body"
			}
			edges[at] = append(edges[at], edge{to, cost})
			pending = append(pending, to)
			return ""
		}
		if in.Def.IsBranch() {
			pen := 1
			if (in.next() >> 8) != (in.branchTarget() >> 8) {
				pen = 2
			}
			if r := add(in.nextSite(), in.Def.Cycles, true); r != "" {
				return loopShape{}, r
			}
			if r := add(in.branchSite(), in.Def.Cycles+pen, true); r != "" {
				return loopShape{}, r
			}
			continue
		}
		if r := add(in.nextSite(), in.nodeCost(), false); r != "" {
			return loopShape{}, r
		}
	}

	// PASS 2 — longest path from the header to the latch, in address order.
	//
	// Every edge recorded above goes to a site in [header, latch], and every edge
	// goes FORWARD: a branch backwards inside the body was refused as an inner loop,
	// and a fall-through always increases the address. So ascending address is a
	// topological order and one pass settles every distance. Doing this in the work
	// queue of pass 1 would not: a site can be dequeued before a longer path into it
	// has been discovered, and its own successors would then carry a stale distance.
	order := make([]site, 0, len(body))
	for at := range body {
		order = append(order, at)
	}
	sort.Slice(order, func(i, j int) bool { return order[i].addr < order[j].addr })
	dist := map[site]int{header: 0}
	for _, at := range order {
		d, reached := dist[at]
		if !reached {
			// Unreachable from the header although collected — impossible, since
			// pass 1 only enqueues successors of reached sites. Refused rather than
			// silently costed as zero.
			return loopShape{}, "loop body has an instruction with no path from the header"
		}
		for _, e := range edges[at] {
			if old, seen := dist[e.to]; !seen || d+e.cost > old {
				dist[e.to] = d + e.cost
			}
		}
	}
	// The witness counter: how many bodies were costed as a graph rather than a
	// chain. Across the corpus this is 1, which is the whole measured yield of the
	// change and the reason the fixture below is the only other witness.
	for at := range body {
		if in, ok := s.nodes[at]; ok && in.Def.IsBranch() {
			branchBodyFolds++
			break
		}
	}
	// THE COST OF ONE ITERATION IS THE LONGEST PATH TO THE LATCH.
	//
	// A body with no branch has exactly one path, so this reproduces the old running
	// sum instruction for instruction — which is what keeps every folded region in
	// the corpus at the bound it already had.
	bodyNoBranch, reached := dist[latch.site()]
	if !reached {
		// Every edge was checked to stay inside [header, latch] and to move forward,
		// so the latch is reachable by construction. Refused rather than assumed.
		return loopShape{}, "loop body does not reach its latch"
	}

	// EVERY WAY IN MUST BE THROUGH THE HEADER.
	//
	// determineBound takes the counter's entry value by maximising over the
	// predecessors of the HEADER. That is the right set only if every execution
	// reaching the back edge passed through the header. An edge landing inside the
	// body arrives at the latch without crossing a scanned predecessor, so the value
	// it carries is not in the maximum, and the fold charges n iterations for a loop
	// entered with a different n.
	//
	// Nothing stated this premise anywhere. The walk above checks that the chain is
	// straight, cheap and single-bank; it never asked who else can arrive in it.
	//
	// Measured on litmus_midentry: the header's only scanned predecessor loads X=2
	// while a `jmp` lands one instruction past the header with X=$50 already set.
	// Proven **40** cycles against a machine that spends **733 across 10 scanlines**
	// — 18.3x under, with `certified: true` and `roll_free: true`.
	//
	// The header itself is excluded: arriving there is the normal way in, and a loop
	// with several such predecessors is fine — the scan sees them all and takes the
	// maximum. Refusing on "more than one predecessor" would lose that, which is what
	// the fixture's JoinCtl row exists to prevent.
	for at, in := range s.nodes {
		if body[at] || at == latch.site() {
			continue // inside the loop: its own edges are the chain and the back edge
		}
		succ, refusal := successors(in, s.absStates[at], s.sw)
		if refusal != "" {
			continue // an instruction whose successors are unknown is handled elsewhere
		}
		for _, nx := range succ {
			if nx != header && body[nx] {
				return loopShape{}, "loop entered past its header — the counter's entry value is not " +
					"the one the scan maximised over"
			}
		}
	}

	pen := 1
	if (latch.next() >> 8) != (header.addr >> 8) {
		pen = 2
	}
	return loopShape{latch: latch, header: header, body: body, bodyNoBranch: bodyNoBranch, pen: pen}, ""
}

// divEdgeScanned counts how many divide folds took their entry bound from the EDGE
// state, i.e. from `absSuccessors` rather than from the old fall-through-plus-address
// -order filter.
//
// It replaces `proxyFallbackHits`, which counted uses of the "closest `lda #imm` below
// the header" proxy. That counter measured 0 across the corpus and the proxy was kept
// on the strength of it — a fact about the corpus, not about the proxy, and
// `litmus_divpred` reaches it on the first try. The proxy is deleted; this counter
// exists so a fixture can prove its own premise, that the region it grades really took
// the scan under test.
var divEdgeScanned int

// divCtxEntryUsed counts loops bounded from the CONTEXT's call-site accumulator rather
// than the site-keyed join (K6). A corpus where this stays 0 is a corpus that never
// exercises the fix, which is a fact worth being able to read off rather than assume.
var divCtxEntryUsed int

// writesX and writesY report whether an operator writes that index register.
//
// Hand-written for the same reason `preservesZN` is: the engine's instruction
// definitions carry no register-effect information — `Effect` is
// Read/Write/Modify/Flow/Subroutine/Interrupt, which describes MEMORY — so there is no
// table to consult, and the safe default with no table is to assume a write.
//
// That default is why these are stated as the SET THAT WRITES rather than the set that
// does not: a new or unrecognised operator must fall through to "does not write" only
// if it genuinely cannot, and every 6502 operator that touches X or Y is listed here.
// The undocumented opcodes the engine decodes (LAX, SAX, DCP…) are absent because the
// decoder marks them and `nodeCost` already refuses; if that ever changes, this list
// must grow with it.
func writesX(op instructions.Operator) bool {
	switch op {
	case instructions.LDX, instructions.INX, instructions.DEX,
		instructions.TAX, instructions.TSX:
		return true
	}
	return false
}

func writesY(op instructions.Operator) bool {
	switch op {
	case instructions.LDY, instructions.INY, instructions.DEY,
		instructions.TAY:
		return true
	}
	return false
}

// preservesZN lists the operators that leave the Z and N flags untouched, so an
// instruction between a loop counter's decrement and the latch that reads it does not
// change which condition the latch tests.
//
// It is a WHITELIST, and deliberately short. The engine's instruction definitions
// carry no flag information (`Effect` is Read/Write/Modify/Flow/Subroutine/Interrupt,
// which is about memory, not status), so there is no table to consult and the safe
// default when a table is missing is to refuse. Everything here is a store or a
// stack/flag operation whose 6502 definition sets neither Z nor N:
//
//	STA/STX/STY  write memory only
//	PHA/PHP      push; PLA and PLP are absent because they LOAD the flags
//	CLC/SEC/CLI/SEI/CLV/CLD/SED  touch C, I, V or D — never Z or N
//	NOP          by definition
//
// Anything else — including every compare, every arithmetic op, every transfer
// (TAX/TXA set Z and N), TSX, and the loads — is treated as clobbering the flags.
// Note TXS is excluded on purpose even though it preserves them: it is the one that
// does NOT set flags among the transfers, and admitting it would invite the reader to
// assume its siblings are fine too. The precision it would buy is 2 instances in the
// whole corpus.
func preservesZN(op instructions.Operator) bool {
	switch op {
	case instructions.STA, instructions.STX, instructions.STY,
		instructions.PHA, instructions.PHP,
		instructions.CLC, instructions.SEC, instructions.CLI, instructions.SEI,
		instructions.CLV, instructions.CLD, instructions.SED,
		instructions.NOP:
		return true
	}
	return false
}

// determineBound returns an iteration upper bound for the simple counted loop
// header..latch, or 0 if undeterminable. Two idioms:
//   - decrement-to-zero (ldx/ldy #N + dex/dey + bne/bpl), and
//   - (2B) divide / sbc-counter (sec; A reduced by a constant; bcs/bcc),
//     bounded from the proven range of A on entry to the loop header.
//
// MERGING TWO BANKS' INSTRUCTIONS INTO ONE NODE SET IS EXACTLY THE CONDITION THAT
// BREAKS AN ADDRESS-ORDER HEURISTIC, which is what SD-9 was. Three consequences are
// handled here explicitly:
//   - the predecessor scan follows CROSS-BANK edges, and returns 0 unless the
//     predecessor set is complete. Maximising over an incomplete predecessor set
//     UNDER-approximates the entry value, hence the trip count, hence the worst case.
//   - every address-order filter is a SAME-BANK comparison. Addresses in different
//     banks have no order at all, so `addr < header` across banks compares two
//     unrelated numbers.
//   - the `lda #imm` fallback on the BCS/BCC path is the same "closest immediate
//     below the header" proxy SD-9 deleted from the dex/dey path. Measured on this
//     corpus with a counter (proxyFallbackHits): 0 ROMs reach it, so it is gated to
//     the same bank rather than deleted, and the counter stays so a future ROM that
//     starts relying on it is visible instead of silent.
func determineBound(nodes, allInstrs map[site]Instr, header site, latch Instr, absStates map[site]State, sw switchModel, amaxHint, ctxAmax int) int {
	// 2B: divide-by-N coarse-positioning idiom — the body subtracts a constant from
	// A and loops while no borrow (BCS) / while borrow (BCC). Max iterations =
	// floor(Amax/const)+1 (with carry set); +1 more covers an unknown entry carry.
	// SOUND: over-approximates the count (more iterations = higher cost). Unknown A
	// range or non-constant subtrahend => 0 (stay unbounded, no false bound).
	if latch.Def.Operator == instructions.BCS || latch.Def.Operator == instructions.BCC {
		sub, nbody := 0, 0
		at := header
		for {
			in, ok := nodes[at]
			if !ok {
				return 0
			}
			if in.Addr == latch.Addr {
				break
			}
			nbody++
			if in.Def.Operator == instructions.SBC && in.Def.AddressingMode == instructions.Immediate {
				sub = int(in.Operand & 0xFF)
			}
			at = in.nextSite()
		}
		// only the canonical single-instruction `sbc #const` body is modeled
		if sub == 0 || nbody != 1 {
			return 0
		}
		// A's LOOP-ENTRY upper bound. absStates[header] is the in-loop JOIN — polluted
		// to Top by the final wrapping subtraction on the back-edge — so read A from
		// the FALL-THROUGH predecessor's post-state instead (the value entering the
		// loop from above). This is where 3A's AND-mask range and 3B's array-element
		// range surface. Max over predecessors (sound). Unknown => 0 (stay unbounded).
		// The comment above says "Unknown => 0 (stay unbounded)" and the code did not
		// do that: a predecessor with no abstract state, or one whose A is Top, was
		// SKIPPED, and the maximum was then taken over the predecessors that happened
		// to be known. Skipping the unknown one is exactly how the maximum comes out
		// too low — an under-approximation, the direction this package forbids, in the
		// same function whose dex/dey path produced SD-9's fortyfold one.
		// THE SAME PREDECESSOR SCAN THE DEX/DEY PATH USES, for the same reason.
		//
		// This filter used to be `in.nextSite() == header && at.addr < header.addr` —
		// textual fall-through plus address order, which is the proxy SD-9 deleted
		// from the other path and left alive here. Its own comment said so and kept
		// it behind a counter that measured zero uses; the counter fires on a
		// twenty-line cartridge.
		//
		// It was wrong in BOTH directions at once. A predecessor arriving by `jmp` or
		// a branch is not `nextSite() == header`, so its value never entered the
		// maximum — an UNDER-approximation. And an instruction that merely SITS before
		// the header has `nextSite() == header` although control never flows there, so
		// a value the machine never holds was admitted.
		//
		// Measured on litmus_divpred, all three with `certified: true`:
		//   the missed predecessor    proven 27, machine 87
		//   the `lda #imm` proxy      proven 28, machine 87
		//   the phantom predecessor   proven 29, machine 89
		//
		// Asking `absSuccessors` which edges reach the header, and reading A from the
		// edge's own state, answers all three at once and deletes a divergence rather
		// than adding a rule — the same argument made when the dex/dey path stopped
		// using `transfer`.
		amax := -1
		unknownPred := false
		// SCAN THE WHOLE PROGRAM, NOT THIS REGION'S SUBGRAPH.
		//
		// A divide loop's entry value is normally established BEFORE the region's
		// opening WSYNC — `lda #SCORE_X` and then `jsr SetXPos`, whose first act is
		// `sta WSYNC`. `nodes` starts AFTER that WSYNC, so the only in-region
		// predecessor of the header was the latch, which this loop excludes, leaving
		// amax = -1 and the 255 floor below. The floor is sound but it is 19 iterations
		// where a sprite coordinate reaches 6, and it is the difference between a
		// certified kernel and a red one.
		//
		// Measured on a probe pair identical except for where `lda #78` sits:
		//   before the opening WSYNC   116cy, NOT CERTIFIED
		//   after it (same machine)     60cy, CERTIFIED
		// Two verdicts for one behaviour, decided by which side of a WSYNC a constant
		// was written on.
		//
		// Widening the scan can only ADD predecessors, and the maximum over a LARGER
		// predecessor set is >= the maximum over a smaller one — so this cannot lower
		// a bound that was already derived, and the direction this package forbids
		// (under-approximation) is not reachable this way. What it CAN do is turn
		// amax = -1 into a real value, which lowers the count from the 255 floor. That
		// is only sound if the predecessor set is COMPLETE, which is exactly what the
		// `unknownPred` guard below is for: any instruction whose successors we cannot
		// resolve might be a predecessor we never saw, and one of those forfeits the
		// inferred bound entirely.
		scan := allInstrs
		if scan == nil {
			scan = nodes
		}
		for at, in := range scan {

			if at == latch.site() {
				continue // the back edge carries the REDUCED A, not the entry value
			}
			st, ok := absStates[at]
			if !ok || !st.valid {
				continue // not known to be a predecessor at all; see below
			}
			if _, refusal := successors(in, st, sw); refusal != "" {
				unknownPred = true
				continue
			}
			for _, e := range absSuccessors(in, st, sw) {
				if e.at != header {
					continue
				}
				if e.state.A.Top {
					unknownPred = true
					break
				}
				if e.state.A.Hi > amax {
					amax = e.state.A.Hi
				}
				break
			}
		}
		if unknownPred {
			// One predecessor we cannot bound makes the maximum over the others a lower
			// bound, not an upper one. The @amax annotation below is the author's own
			// declared ceiling and is still allowed to answer; an inferred range is not.
			amax = -1
		}
		// The `lda #imm` fallback is GONE. It guessed A's entry value from the nearest
		// immediate below the header — the same address proxy SD-9 removed from the
		// dex/dey path — and was kept because a counter measured zero uses across the
		// corpus. Zero uses is a fact about the corpus: `litmus_divpred`'s ProxyDanger
		// row reaches it on the first try and gets 28 cycles for an interval the
		// machine takes 87. A fallback that only fires where the sound scan failed is
		// a fallback that only fires where it is wrong.
		//
		// `@amax` below stays: that is the author DECLARING a ceiling, not the
		// analysis inferring one.
		if amax >= 0 {
			divEdgeScanned++
		}
		// THE CONTEXT'S OWN ENTRY VALUE, when the site-keyed scan could not produce one.
		//
		// It is consulted here rather than earlier on purpose: the scan above is a
		// property of the program and holds for every context, so when it answers, its
		// answer already covers this one. ctxAmax is narrower — true of THIS call site
		// only — and analyzeRegionInContexts is what makes that legitimate, by walking
		// each context separately and taking the worst. Using it in the flat walk would
		// be unsound, which is why the flat walk passes -1.
		//
		// This is what the 255 floor was standing in for on pizza_boy.asm's SetXPos:
		// the HUD contexts pass a constant and the VBLANK ones pass a RAM byte, and the
		// join of those is Top no matter how good the scan is.
		if amax < 0 && ctxAmax >= 0 {
			amax = ctxAmax
			divCtxEntryUsed++
		}
		if amax < 0 && amaxHint > 0 {
			amax = amaxHint // ②: author-declared `@amax N` (the accumulator's proven upper bound) when the abstract range is Top
		}
		// A IS EIGHT BITS WIDE, AND THAT IS NOT AN INFERENCE.
		//
		// Everything above refuses an entry value it cannot pin, and it is right to:
		// SD-9's proxy guessed one and under-approximated by 40x. But refusing ALL of
		// them conflates "the analysis does not know this value" with "this value has
		// no bound", and the second is false about a 6502 accumulator. Whatever the
		// machine holds in A, it holds at most 255 — a fact about the HARDWARE, not a
		// range inferred from the program, which is why it does not reopen the door
		// SD-9 closed: the failure there was reading a number off the wrong
		// instruction, not reading the register's width off the datasheet. `@amax`
		// stays useful because a tighter ceiling gives a tighter bound; this is only
		// the floor under it.
		//
		// Found on the user's own ROM. `pizza_boy.asm`'s `SetXPos` divides A by 15 to
		// place a sprite, and A arrives from `lda px` — a RAM byte, Top by
		// construction, at all five of its call sites. The region was refused for "no
		// WSYNC reached" because every call context died on this line first, and that
		// refusal is what kept the project's own phase0 scenario red after the BRK
		// half was fixed.
		//
		// The bound is loose — 255/15 + 2 = 19 iterations where a sprite coordinate
		// can only reach ~160 — and loose is the correct direction.
		if amax < 0 {
			amax = 255
		}

		// BCC COUNTS UP, so `amax` is the wrong variable for it.
		//
		// The two latches run the loop in opposite directions:
		//
		//   `sbc #N / bcs L`  loops while NO borrow, i.e. while A >= N. A falls by N
		//                     each time, so a LARGER entry value means MORE iterations
		//                     and `amax` is exactly what bounds the count.
		//
		//   `sbc #N / bcc L`  loops while there IS a borrow, i.e. while A < N. The
		//                     subtraction wraps, so A RISES by (255 - N) each time
		//                     until it reaches N. A larger entry value means FEWER
		//                     iterations, and `amax` bounds nothing at all — the count
		//                     depends on N alone, worst case from A = 0.
		//
		// One formula was applied to both. It agrees only while N is small: at N = 15
		// `amax/sub + 2` is loose and safe, at N = 200 it answers 2 for a loop the
		// machine runs 5 times. Measured on litmus_bccdiv: proven **16** cycles
		// against a machine that spends **31**, with `certified: true`.
		//
		// SOUNDNESS. From A the borrowing step gives A' = A + (255 - N); the loop
		// exits when A >= N. From the worst start A = 0 that takes ceil(N/(255-N))
		// borrowing steps, plus the final non-borrowing one, plus one for an entry
		// carry this analysis does not track: ceil(N/(255-N)) + 2.
		//
		// N = 255 never terminates — 255 - 255 = 0, so A does not rise — and is
		// refused rather than given a number.
		if latch.Def.Operator == instructions.BCC {
			up := 255 - sub
			if up <= 0 {
				return 0 // A cannot rise; the loop does not terminate
			}
			return (sub+up-1)/up + 2
		}
		return amax/sub + 2
	}
	if latch.Def.Operator != instructions.BNE && latch.Def.Operator != instructions.BPL {
		return 0
	}
	// THE LATCH MUST READ THE COUNTER'S OWN FLAGS.
	//
	// The trip count derived below says "the counter decrements to zero and the
	// branch exits there". That reasoning is about the DECREMENT's Z/N flags, so it
	// holds only if those are the flags the latch actually tests. Anything that
	// writes Z or N in between silently substitutes its own condition, and the
	// derived count then describes a loop that does not exist.
	//
	// Measured on litmus_latchflags, whose DangerRow is `ldx #1 / ... / dex /
	// cpx #$02 / bne`: after the decrement X is 0, the `cpx` compares 0 against 2,
	// clears Z and takes the branch, so X wraps through $FF and the loop runs 255
	// times. The prover answered **19 cycles** and the machine spent **3829 across 51
	// scanlines** — 201x under, carried out on certified:true and roll_free:true. A
	// bound below the machine's is the one direction this package forbids, and this
	// is the same failure mode as SD-9's fortyfold proxy bug arriving by a second
	// route.
	//
	// Reversing the inequality hides it: with an entry above the compared value the
	// loop exits early and the answer is an OVER-approximation, sound by luck.
	// `roms/exerciser/exerciser.asm $F0C9` (`dex / cpx #$02 / bne`, entry 5) is
	// exactly that, which is why the defect sat in 140 images with none exposing it.
	//
	// The rule is stated as a WHITELIST of instructions known to leave Z and N alone,
	// because the engine's instruction table does not record flag effects and the
	// safe default when a table is missing is to refuse. Cost, measured over the
	// technique/litmus/exerciser corpus plus five commercial cartridges: of 757
	// dex/dey folds **720 are adjacent** and unaffected; of the 37 with something in
	// between, the instructions seen are cpx 19, inx 7, adc 5, jsr 5, bne 15 and
	// similar — all flag-writing. No store appeared at all. So the whitelist costs
	// essentially no precision today while closing the hole for good.
	// EXACTLY ONE INSTRUCTION IN THE BODY MAY TOUCH THE COUNTER, AND IT IS THE
	// DECREMENT.
	//
	// The derivation below reads "entry value v, therefore v iterations". That needs
	// the body's net effect on the counter to be exactly -1, and the previous version
	// only latched a boolean when it saw a DEX or DEY — so a body that also wrote the
	// register was indistinguishable from one that did not.
	//
	// Measured on a probe of `ldx #2 / L: inx / inx / dex / bne L`: proven **22**
	// cycles against a machine that spends **2290 across 31 scanlines**, reported as
	// `certified: true` and `roll_free: true`. **104x under.** Two increments and one
	// decrement leave the counter rising, so the loop never terminates on it; the
	// analysis answered 2.
	//
	// This is the other half of the SD-13 repair. That one added `preservesZN` to
	// guard the window AFTER the decrement, where a compare could substitute its own
	// condition. The window BEFORE it was left open, and it is worse: a write there
	// changes the count itself rather than which flags are read.
	//
	// Two decrements per iteration is the benign form of the same shape and is
	// refused here as well, though it over-approximates rather than under. Refusing
	// costs one real fold — `Pressure Cooker.bin $D801`, whose body ends `dex dex dex
	// dex / bpl` — and buying it back needs the count, not a boolean, which is a
	// precision change and does not belong in a soundness repair.
	decX, decY := false, false
	wroteX, wroteY := false, false
	at := header
	for {
		in, ok := nodes[at]
		if !ok {
			return 0
		}
		if in.Addr == latch.Addr {
			break
		}
		switch in.Def.Operator {
		case instructions.DEX:
			if decX || decY || wroteX {
				return 0 // a second decrement, or a counter already written above
			}
			decX = true
		case instructions.DEY:
			if decX || decY || wroteY {
				return 0
			}
			decY = true
		default:
			// Anything that writes THE COUNTER invalidates the count, wherever it
			// sits — before the decrement as well as after. The other index
			// register moving is not the counter moving: a loop that walks two
			// pointers is a common and perfectly boundable shape, and refusing it
			// would cost precision for nothing. Which register is the counter is
			// only known once the decrement has been seen, so a write BEFORE it is
			// remembered and judged at the end.
			if writesX(in.Def.Operator) {
				wroteX = true
			}
			if writesY(in.Def.Operator) {
				wroteY = true
			}
			if decX && wroteX {
				return 0
			}
			if decY && wroteY {
				return 0
			}
			if (decX || decY) && !preservesZN(in.Def.Operator) {
				return 0 // the latch would be testing THIS instruction's flags
			}
		}
		at = in.nextSite()
	}
	if !decX && !decY {
		return 0
	}
	// The counter's range ENTERING the loop, taken from the abstract state of every
	// predecessor of the header except the back-edge, maximised. Sound: more
	// iterations cost more, so the largest entry value is the worst case.
	//
	// This used to be "the immediate LDX/LDY at the greatest address below the
	// header" — a proxy for "the initialiser that ran most recently", valid only
	// while address order matches execution order. A backward jump breaks that, and
	// it needs no bank switching to do so: measured on litmus_bound_proxy, a flat 4K
	// image whose real `ldx #200` sits ABOVE the header (reached by a forward jump)
	// while a discarded `ldx #2` sits below it, the proxy returned 2. The prover
	// answered certified:true, roll_free:true, max_worst 25, and the machine measured
	// that same interval at 1015 cycles across 14 scanlines in a 273-line frame.
	// A fortyfold under-approximation carried on the roll_free verdict — the one
	// direction this package forbids, and pre-existing rather than anything the bank
	// work introduced.
	//
	// An unknown range now returns 0, i.e. stays unbounded. That is a loss of
	// precision in exchange for the guarantee; a stated refusal is worth more than a
	// number that can be wrong by 40x.
	best := -1
	zeroReachable := false
	for at, in := range nodes {
		if at == latch.site() {
			continue // the back-edge carries the DECREMENTED value, not the entry value
		}
		// Cross-bank successors included: a header entered from another bank would
		// otherwise lose that predecessor, and maximising over an INCOMPLETE predecessor
		// set under-approximates the entry value, hence the trip count, hence the worst
		// case. An instruction whose successors cannot be determined at all makes the
		// predecessor set unknowable, so the bound is refused.
		st, ok := absStates[at]
		if !ok || !st.valid {
			return 0 // a predecessor we know nothing about: no bound
		}
		if _, refusal := successors(in, st, sw); refusal != "" {
			return 0
		}
		// THE STATE ON THE EDGE, not the instruction's own effect.
		//
		// This used to compute `st.transfer(in)` and read the counter from that.
		// `transfer` models what the instruction does to the machine; `absSuccessors`
		// defines what state FLOWS ALONG EACH EDGE, and for a JSR the two disagree by
		// design: transfer models only the push and leaves X and Y untouched, while
		// absSuccessors correctly resets the return point to Top because the callee's
		// effect is not modelled. Reading the former means reading the counter's value
		// from BEFORE the call and treating it as the value at the header.
		//
		// Measured: a region whose header is preceded by `ldx #2 / jsr SetBig`, where
		// SetBig does `ldx #$50`. The scan saw X=2 and answered 36 cycles; the machine
		// spent 738 across 10 scanlines. **20.5x under**, with `certified: true`.
		//
		// Taking the edge state deletes the divergence rather than adding a rule —
		// the same argument `successors` itself makes about having one notion of a
		// successor. A JSR predecessor now yields X.Top and the refusal below fires.
		var post State
		found := false
		for _, e := range absSuccessors(in, st, sw) {
			if e.at == header {
				post, found = e.state, true
				break
			}
		}
		if !found {
			continue // not a predecessor of the header
		}
		r := post.Y
		if decX {
			r = post.X
		}
		if r.Top {
			return 0 // unknown entry value: stay unbounded rather than guess
		}
		if r.Hi > best {
			best = r.Hi
		}
		// Whether ZERO is reachable is a separate fact from the maximum, and the BNE
		// trip count needs both — see the wrap case below. Collected here because
		// this is the only place the per-predecessor ranges exist; `best` alone
		// cannot answer it.
		if r.Lo <= 0 && r.Hi >= 0 {
			zeroReachable = true
		}
	}
	if best < 0 {
		return 0
	}
	// BNE and BPL do NOT end the same countdown on the same iteration, and this
	// function returned `best` for both.
	//
	// `dex; bne L` with X=6 exits when the decrement produces zero: 6 iterations.
	// `dex; bpl L` with X=6 exits only when the decrement produces a NEGATIVE value,
	// so it runs the body once more with X=0 and leaves on X=$FF: 7 iterations. The
	// trip count of the bpl form is best+1, not best.
	//
	// Measured on the real Seaquest cartridge, region $F1FC — `lda #$FF / ldx #$06 /
	// L: sta $B0,x / dex / bpl L` between two WSYNCs. The missing iteration is one
	// body (sta abs,x 4 + dex 2 = 6) plus one taken branch (3) = 9 cycles: proven 66,
	// machine 75. An UNDER-approximation, the one direction this package forbids, and
	// it rode out on Bounded=true.
	//
	// SOUNDNESS. loopCost = n*body + (n-1)*(latch+pen) + latch is monotonically
	// increasing in n (body >= 0, latch+pen > 0), so raising n can only raise the
	// bound. best is the maximum entry value of the counter over the header's
	// predecessors, and for the bpl exit condition the trip count as a function of the
	// entry value v is: v+1 for v <= 128 (v=128 decrements to 127, still non-negative,
	// so it too runs to the wrap), and 1 for v >= 129 (the first decrement is already
	// negative). Its maximum over any range with upper bound `best` is therefore
	// best+1 when best <= 128, and at most 129 <= best+1 when best >= 129. So best+1
	// is an upper bound everywhere, and it is EXACT for the counted-down loops an
	// author actually writes (best <= 128) — the bound rises to the true value rather
	// than merely away from the violation.
	//
	if latch.Def.Operator == instructions.BPL {
		bplTripCountAdjusted++
		return best + 1
	}

	// BNE's trip count is NOT monotone in the entry value, and taking `best` alone
	// assumed it was.
	//
	//     v > 0  ->  v iterations    (the decrement reaches zero after v steps)
	//     v = 0  ->  256 iterations  (the decrement wraps to $FF and counts down)
	//
	// So `Hi` is the maximum only when zero is out of range. When it is in range the
	// analysis was answering for the SMALLEST possible loop while the machine could
	// run the largest — measured on litmus_bnezero, whose header sees X in [0,5]:
	// proven 60 cycles against a machine that spends 2319 across 31 scanlines,
	// **38.7x under**, with `certified: true` and `roll_free: true`.
	//
	// SOUNDNESS. 256 is the exact trip count for v=0 and exceeds it for every other v
	// in a range bounded by 255, so max(256, Hi) is an upper bound over the whole
	// range. `loopCost` is monotone increasing in n, so substituting the maximum can
	// only raise the bound.
	//
	// Returning 256 rather than refusing is deliberate: the region stays BOUNDED, so
	// an author gets a number they can act on ("this loop can cost 256 iterations
	// here") instead of a refusal that says nothing. The number is honest — it is
	// what the hardware does on the path the analysis cannot rule out.
	//
	// Not fixed when the BPL sibling was, because a census over the five commercial
	// cartridges the gate then graded found 3 instances and no violation. Re-censused
	// once the gate stopped grading a hand-picked five: **14 folds across 3 shipped
	// cartridges** — Seaquest x3, Bermuda Triangle x6, Vanguard x5, all [0,15].
	// Latent in every observed run and one input away from live in three real games.
	if zeroReachable && best > 0 {
		bneZeroWrapAdjusted++
		return 256
	}
	return best
}

// bplTripCountAdjusted counts how many times determineBound has raised a dex/dey
// countdown's trip count by one because its latch is BPL rather than BNE.
//
// Like proxyFallbackHits above it is a MEASUREMENT counter, not analysis state. It
// exists because the correction it counts is invisible in the report — a bound 9
// cycles higher looks like any other bound — and because the standing corpus gate was
// blind to this defect for a MEASURED reason. Censused over 140 images: 7 distinct bpl
// folds, only 4 of them in kernels we wrote, and none of the four could expose it —
// rts_dispatch's region $F036 and zone_multiplex's $F033 produce NO ProfileLineWorst
// row at all (the gate compares nothing there), and shared_setxpos's $F054 was proven
// 83 against a measured 36, i.e. 47 cycles of slack to hide 15 cycles of error in. It
// took a tight commercial region to make the gap visible.
//
// A test that reads this counter can prove its own PREMISE — that the region it grades
// really did fold a bpl latch down this path — instead of passing over a region that
// never reached it.
var bplTripCountAdjusted int

// bneZeroWrapAdjusted counts the folds where a `dex; bne` counter could enter at ZERO
// and the trip count therefore had to be the wrap count rather than the range's upper
// bound. Same purpose as its sibling: a fixture can assert it moved, and so prove that
// the region it grades really took this path rather than passing over one that never
// reached it.
var bneZeroWrapAdjusted int

// regionTouchesDisplay reports whether any node stores to VSYNC($00)/VBLANK($01),
// i.e. could change the display state within the region.
// regionTouchesDisplay reports whether anything in the region can turn the
// display on inside itself, which disqualifies it from being skipped as blank.
//
// It shares `storeMissesDisplay` with the JSR rule rather than testing the raw
// operand, because the raw test saw only non-indexed writes: `sta VSYNC,x` is
// AbsoluteX, so it returned no address at all and the region was skipped as blank
// while containing a write to VBLANK. A push counts too — page 1 mirrors the
// console's own addresses, so SP can aim a PHA at the display.
// Because the region subgraph now crosses bank boundaries, a `sta VBLANK` on the far
// side of a switch can no longer be missed when classifying a region blank.
func regionTouchesDisplay(nodes map[site]Instr, states map[site]State) bool {
	for at, in := range nodes {
		switch in.Def.Operator {
		case instructions.PHA, instructions.PHP:
			if !pushMissesDisplay(in, states[at]) {
				return true
			}
		case instructions.STA, instructions.STX, instructions.STY:
			if !storeMissesDisplay(in, states[at]) {
				return true
			}
		}
	}
	return false
}

// analyzeRegion proves the worst case of the WSYNC-to-WSYNC region opened by the
// WSYNC store `start`. states gives the abstract state at each address, used to
// classify the region's display interval (S1).
func analyzeRegion(instrs map[site]Instr, start Instr, budget, amaxHint int, sm *srcmap.Map,
	states map[site]State, sw switchModel) Region {
	s := &solver{
		nodes:     map[site]Instr{},
		sinks:     map[site]bool{},
		folds:     map[site]loopInfo{},
		memo:      map[lkey]result{},
		state:     map[lkey]int{},
		absStates: states, // S3: page-cross penalty resolved from tracked index ranges
		sm:        sm,
		amaxHint:  amaxHint,
		sw:        sw,
		banked:    sw.banked,
		allInstrs: instrs,
	}
	reg := Region{Start: start.Addr, StartLoc: s.loc(start.site()), Budget: budget, Bounded: true, Kind: "visible"}
	if sw.banked {
		reg.Bank, reg.BankValid = start.Bank, true
	}

	// Collect the region subgraph: from the instruction after `start`, follow the
	// CFG; WSYNC stores are sinks (terminators, not expanded); flow we can't
	// reason about makes the whole region unbounded.
	unbounded := func(reason string) Region { reg.Bounded = false; reg.Reason = reason; return reg }
	if msg := s.collectRegion(instrs, start); msg != "" {
		return unbounded(msg)
	}
	// S1: if the beam is provably in VSYNC/VBLANK at region entry AND the region
	// never stores to $00/$01 (so it can't turn the display on inside itself), it
	// is not a visible-scanline timing risk — skip it soundly. Its only failure
	// mode is total frame-line drift, which is a separate check (ntsc_frame_lines).
	if st, ok := states[start.site()]; ok && st.displayOff() && !regionTouchesDisplay(s.nodes, states) {
		reg.Kind = "blank"
		// ①: fall through and STILL compute the worst-case. A blank (VSYNC/VBLANK/
		// overscan) region > budget does not tear a visible line, but it adds a
		// scanline (WSYNC halts to the next line) = frame-line drift / screen dip /
		// roll. Previously this returned Worst=0, hiding it from the ∀ proof.
	}
	ctxs := callContexts(instrs, start.site(), sw)
	// A region inside a subroutine called from more than one place must be walked
	// once PER CALL SITE, not as one merged graph. The flat walk follows the RTS
	// to every return address, so it happily costs a path that enters from call
	// site A and leaves through call site B — an execution that cannot happen. On
	// a two-sprite kernel that positions both players through one shared routine
	// (the ordinary way to write it) that fused path both inflated the worst case
	// and dragged in a `sta VBLANK` belonging to the other context, which flipped
	// the region's classification from blank to visible and reported a violation
	// on a routine that runs entirely inside VBLANK. Measured on a generated
	// two-call clone: 89 cycles and `certified:false`, against 74 and
	// `certified:true` for the identical kernel with one call site.
	//
	// Taking the worst across contexts stays sound — each context is a real
	// execution — while dropping the impossible ones. This used to run only as a
	// fallback when the flat walk found no WSYNC at all, which is exactly the case
	// where it does NOT help: with several call sites the flat walk reaches a sink
	// easily, and answers confidently from a path that does not exist.
	if len(ctxs) > 1 {
		if r := analyzeRegionInContexts(instrs, start, budget, amaxHint, sm, states, sw, ctxs); r != nil {
			return *r
		}
		// Unresolvable in some context: fall through to the flat walk, which is
		// still an over-approximation and therefore still sound.
	}
	if len(s.sinks) == 0 {
		if len(ctxs) > 0 {
			if r := analyzeRegionInContexts(instrs, start, budget, amaxHint, sm, states, sw, ctxs); r != nil {
				return *r
			}
		}
		return unbounded("no WSYNC reached from region start")
	}
	// DAG FIRST. Ask the longest-path walk for an answer BEFORE trying to fold, and
	// fold only when the walk itself reports a cycle (see dagFirstAnswered).
	// Fold FIRST, and let the DAG walk override exactly one refusal. See
	// multipleBackEdges for why the order matters and what it cost to learn.
	// foldLoops returns that message before it has folded anything, so the walk
	// below still runs on the unfolded subgraph.
	if msg := s.foldLoops(); msg != "" {
		if msg == multipleBackEdges {
			if r := s.dagFirst(start); r != nil {
				reg.Worst = r.cyc
				reg.Over = r.cyc > budget
				if reg.Over {
					reg.Path = r.path
				}
				return reg
			}
		}
		reg = unbounded(msg)
		// If the only thing missing was the trip count, the region's cost is still
		// a known function of it, so the largest count that fits is computable.
		if s.pending != nil {
			reg.Conditional = s.solveObligation(start, budget)
		}
		return reg
	}

	r := s.longest(start.nextSite(), ctx{})
	if s.cyclic {
		return unbounded("unbounded loop in region (no counted-loop bound found)")
	}
	if s.unbounded {
		if len(ctxs) > 0 {
			if rc := analyzeRegionInContexts(instrs, start, budget, amaxHint, sm, states, sw, ctxs); rc != nil {
				return *rc
			}
		}
		return unbounded(s.unbReason)
	}
	reg.Worst = r.cyc
	reg.Over = r.cyc > budget
	if reg.Over {
		reg.Path = r.path
	}
	return reg
}

// --- top-level ---

// BankCoverage is how much of one bank the analysis actually reached.
type BankCoverage struct {
	Bank int `json:"bank"`
	// Instructions is the decode reached from this bank's own reset/NMI/IRQ vectors
	// PLUS every cross-bank landing address seeded into it (see decodeUnits).
	Instructions int `json:"instructions"`
	Regions      int `json:"regions"`
	// SeededEntries is how many of those entry points came from a bank switch
	// rather than from this bank's own vectors. A worker bank whose count is 0 was
	// entered by nothing this analysis could see.
	SeededEntries int `json:"seeded_entries,omitempty"`
}

// Report is the proof outcome over all WSYNC-to-WSYNC regions of a kernel.
type Report struct {
	Asm        string   `json:"asm"`
	Budget     int      `json:"budget"`
	Regions    int      `json:"regions"`              // WSYNC-to-WSYNC regions analyzed
	Blank      int      `json:"blank,omitempty"`      // regions skipped as VSYNC/VBLANK (not visible-line risks)
	MaxWorst   int      `json:"max_worst"`            // largest proven worst-case among bounded VISIBLE regions
	Certified  bool     `json:"certified"`            // all visible regions bounded AND <= budget
	Violations []Region `json:"violations,omitempty"` // regions whose worst case exceeds the budget
	Unbounded  []Region `json:"unbounded,omitempty"`  // regions that could not be proven (out of scope)
	Lines      []Region `json:"lines,omitempty"`      // PONG-C3: the COMPLETE per-region table (every visible region incl. passing ones, address order) — "trim by the exact margin", not guess-and-assert
	// --- blank (VSYNC/VBLANK/overscan) region accounting (VV-2b: previously skipped as worst=0) ---
	BlankLines     []Region `json:"blank_lines,omitempty"`     // every blank region with its computed worst
	BlankMaxWorst  int      `json:"blank_max_worst,omitempty"` // largest worst among BOUNDED blank regions
	BlankOver      []Region `json:"blank_over,omitempty"`      // blank regions whose worst exceeds budget×@lines (roll risk, not a visible tear)
	BlankUnbounded []Region `json:"blank_unbounded,omitempty"` // blank regions we could not statically bound (e.g. a divide loop over an un-@amax'd RAM accumulator)
	RollFree       bool     `json:"roll_free"`                 // ∀ roll-freedom: EVERY region (blank+visible) is bounded AND within its budget×@lines span

	// Converged reports that the abstract-interpretation fixpoint reached a fixed
	// point rather than stopping at its iteration cap. A capped run leaves
	// UNDER-approximated states, which every downstream consumer treats as sound,
	// so a report with Converged=false proves nothing and must not be certified.
	Converged bool `json:"converged"`

	// BankedDeclined names the reason the image was not analysed at all.
	//
	// Everything in this package keys on a flat 16-bit address, so on a
	// bank-switched cartridge the vectors, the decode and every analysis built on
	// them describe a program that does not exist. Until this field existed the
	// refusal was not real: Prove analysed the fiction anyway and was saved only by
	// the incidental fact that no STA WSYNC was reachable in whichever bank the
	// flat model happened to fold to, which tripped the "0 regions never certify"
	// backstop. That is luck, not a refusal — a banked ROM whose flat fold DID
	// contain a WSYNC would have been analysed and reported on with confidence.
	BankedDeclined string `json:"banked_declined,omitempty"`

	// Banks is the number of banks analysed, set only for a bank-switched image.
	Banks int `json:"banks,omitempty"`

	// BankCoverage says how much of each bank was actually reached, so a bank that
	// was barely decoded cannot pass for one that was checked.
	//
	// A bank is normally entered by the trampoline that switched to it, not by its
	// own reset vector. Seeding the target bank at the address after each hotspot
	// access closes that (see CrossBankSeeds): measured on litmus_bank, bank 1 went
	// from 4 decoded instructions with 4 executed-but-undecoded, to 99 decoded with
	// 0 undecoded.
	//
	// Read that 99 as over-approximation, not as coverage: the machine executes 4
	// instructions in bank 1, and the other 95 decoded are mostly the $FF filler
	// between that body and the reset stub, decoded as a chain of undocumented
	// opcodes. None of them stores WSYNC, so bank 1 still carries 0 regions — which
	// is why the region count sits beside the instruction count here. A large
	// instruction count on its own says the decoder walked a long way, not that
	// anything was checked.
	BankCoverage []BankCoverage `json:"bank_coverage,omitempty"`

	// CrossBankSeeds lists the decode entry points that came from a bank switch:
	// instruction at (from_bank, from_addr) reaches hotspot `symbol`, so the next
	// fetch is (to_bank, to_addr).
	//
	// This closes the DECODE only. It does not model flow inside a region, so a
	// region that can reach a hotspot is still refused (see hotspotRefusal) and
	// UnmodelledSwitches still blocks certification — the analysis now knows what
	// the other bank CONTAINS, not how the cycles run across the boundary.
	CrossBankSeeds []CrossBankSeed `json:"cross_bank_seeds,omitempty"`

	// CrossBankSeedRounds is how many passes the seeding fixpoint took (the last one
	// adds nothing, which is how it knows it is done).
	CrossBankSeedRounds int `json:"cross_bank_seed_rounds,omitempty"`

	// CrossBankSeedCapped reports that the fixpoint stopped at seedRoundCap instead
	// of closing. The decode is then INCOMPLETE by an unknown amount, and saying so
	// is the whole point of the field: a capped run must not read as a converged one.
	CrossBankSeedCapped bool `json:"cross_bank_seed_capped,omitempty"`

	// UnresolvedHotspots names bank-switch hotspots whose mapper symbol could not be
	// parsed to a bank number (Parker Bros publishes "B0S0", M-Network "RAM0"), so
	// no target bank could be seeded. Guessing one would invent a control-flow edge.
	UnresolvedHotspots []string `json:"unresolved_hotspots,omitempty"`

	// UnresolvableSwitchAccesses counts instructions under a hotspot-bearing mapper
	// whose memory target could not be resolved at all. Each MIGHT be a switch, but
	// no address means no symbol and no bank to seed. They are refused, not seeded.
	UnresolvableSwitchAccesses int `json:"unresolvable_switch_accesses,omitempty"`

	// UnmodelledSwitches counts regions refused because they can change bank and
	// cross-bank flow is not modelled.
	//
	// Certification must require this to be zero, for the same reason it requires
	// Converged: "every region I looked at passed" is not a proof when the reason
	// some were not looked at is that the program leaves for somewhere this
	// analysis does not follow. Before this counter existed, litmus_bank came back
	// certified:true having analysed bank 0 of 2 — a true statement about a
	// fragment presented as a verdict on the cartridge.
	//
	// Its MEANING is now narrower than it was: it counts regions refused because a
	// switch they can perform is not modelled, not regions that merely contain a
	// switch. ModelledSwitchEdges is the other half of the pair.
	UnmodelledSwitches int `json:"unmodelled_switches,omitempty"`

	// ModelledSwitchEdges is how many cross-bank control-flow edges the analysed
	// regions actually followed. It sits beside UnmodelledSwitches so "0 refusals" can
	// be told apart from "0 refusals because nothing was crossed": a bank-switched
	// cartridge reported as certified with 0 modelled edges has not been checked
	// across its banks, it has merely not been caught.
	ModelledSwitchEdges int `json:"modelled_switch_edges,omitempty"`

	// SwitchWidenedSites counts code sites whose abstract state was forced to Top
	// because they are a possible landing point of a bank switch this analysis does not
	// model, and SwitchWidenReasons names why.
	//
	// The reason is a soundness one and it is worth stating rather than inferring. The
	// value domain is a WHOLE-PROGRAM fixpoint while a refusal is per-region, so an
	// unmodelled switch leaves its real landing site with an incomplete set of incoming
	// edges — and a join over an incomplete predecessor set UNDER-approximates, which is
	// exactly the narrow-confident-wrong range that could bound a loop in some OTHER
	// region that was never refused. Refusing the region that contains the switch does
	// not protect the region that contains its landing, so the landing is widened.
	//
	// A non-zero count here does not by itself mean a region was refused: it can come
	// from a decoded-but-never-executed byte sequence (the decode over-approximates on
	// purpose). It means some precision was given up for a soundness reason, and it is
	// reported so that a bound which quietly depends on the difference is visible.
	SwitchWidenedSites int      `json:"switch_widened_sites,omitempty"`
	SwitchWidenReasons []string `json:"switch_widen_reasons,omitempty"`

	// SourceAnnotations says when `@lines` / `@amax` could not be read at all, with the
	// reason. Both are looked up through the source map, and on a bank-switched image
	// DASM's listing address column is the physical ROM offset rather than the RORG'd
	// CPU address — so the lookup silently misses and every region is budgeted at one
	// scanline. A region that legitimately spans two lines then reports as a violation,
	// and the reader has to be told that is the annotation's absence rather than the
	// prover's disagreement.
	SourceAnnotations string `json:"source_annotations,omitempty"`
}

// romByBank builds the ROM-byte reader the value domain uses, routed BY BANK.
//
// Routing is not a nicety. romTableRange folds real cartridge bytes into an EXACT
// value range and that range can bound a loop's trip count, so on a merged program a
// single flat reader would fold whichever bank happened to be bound in the closure —
// and a `lda table,x` in bank 1 would come out bounded by bank 0's table bytes. That
// is a narrow, confident, wrong range feeding a worst case, which is the same failure
// shape as reading a superchip's RAM window as ROM (SD-8c) on a different axis.
func romByBank(decodes []unitDecode) func(bank int, addr uint16) (byte, bool) {
	progs := make(map[int]*program, len(decodes))
	for _, d := range decodes {
		progs[d.bank] = d.prog
	}
	return func(bank int, addr uint16) (byte, bool) {
		p, ok := progs[bank]
		if !ok {
			return 0, false
		}
		return p.byteAt(addr)
	}
}

// Prove assembles asmPath, statically proves every WSYNC-to-WSYNC region's
// worst-case cycle cost, and returns the report. budget<=0 defaults to 76.
func Prove(asmPath string, budget int) (*Report, error) {
	if budget <= 0 {
		budget = DefaultBudget
	}
	// A raw cartridge image is analysed directly. Everything derived from source —
	// `@lines`/`@amax` annotations, label locations, the per-bank line map — is simply
	// absent, and `srcmap.Map` is nil-safe throughout for exactly that case.
	//
	// This entry did not exist, and its absence was the quiet part. SD-0c made the
	// DECODER read real cartridges (Outlaw 2K went 0 instructions to 931, Combat 0 to
	// 838), but nothing public took a `.bin`: Prove and timinglint assemble their
	// input, so a commercial image came back as "Unknown Mnemonic" — measured
	// 2026-07-30 on Adventure, Seaquest, Chopper Command, VideoOlympics and Empire
	// Strikes Back. The capability was there and unreachable, which is the same shape
	// as a description that denies what a tool can do.
	var (
		bin      string
		lst      string
		sm       *srcmap.Map
		srcLines []string
	)
	if strings.EqualFold(filepath.Ext(asmPath), ".bin") {
		bin = asmPath
	} else {
		bin = build.BinPathFor(asmPath)
		out, l, sym, err := build.AssembleWithListing(asmPath, bin)
		if err != nil {
			return nil, fmt.Errorf("assemble %s failed:\n%s", asmPath, out)
		}
		lst = l
		sm = srcmap.Parse(l, sym, asmPath)
		src, _ := os.ReadFile(asmPath)
		srcLines = strings.Split(string(src), "\n")
	}

	// `; @lines N` on the source line that OPENS a WSYNC region declares it spans
	// N scanlines (a 2-line kernel does ~2 lines of CPU work between WSYNCs), so
	// that region's budget is N*budget. Default 1. Greens legitimate 2-line kernels
	// without weakening the proof (an un-annotated over-76 region still flags).
	atLinesRe := regexp.MustCompile(`@lines\s+(\d+)`)
	// The lookup takes the SITE, not a bare address. On a banked image the flat line
	// map holds bank 1's rows under their physical offsets ($1F03 for $FF03), so an
	// address-only lookup misses every banked region and silently returns the default
	// — measured before this: every region on a bank-switched kernel was budgeted at
	// 1 scanline no matter what the source said, which is why litmus_bank_f4's
	// genuinely 2-line crossing region was reported over budget.
	regionLines := func(at site) int {
		ln, ok := lineAt(sm, at)
		if !ok {
			return 1
		}
		// Scan the mapped line and the next: DASM maps a labeled WSYNC to its LABEL
		// line, so `@lines N` written on the `sta WSYNC` line sits one line below.
		for i := ln - 1; i <= ln && i < len(srcLines); i++ {
			if i < 0 {
				continue
			}
			if g := atLinesRe.FindStringSubmatch(srcLines[i]); g != nil {
				if n, e := strconv.Atoi(g[1]); e == nil && n >= 1 {
					return n
				}
			}
		}
		return 1
	}
	// ②: `@amax N` on the WSYNC line that opens a region declares the upper bound of
	// that region's divide-loop accumulator, so a ÷N coarse-positioner whose input is
	// a RAM byte (abstract range Top) can still be bounded. 0 = none.
	atAmaxRe := regexp.MustCompile(`@amax\s+(\d+)`)
	regionAmax := func(at site) int {
		ln, ok := lineAt(sm, at)
		if !ok {
			return 0
		}
		for i := ln - 1; i <= ln && i < len(srcLines); i++ {
			if i < 0 {
				continue
			}
			if g := atAmaxRe.FindStringSubmatch(srcLines[i]); g != nil {
				if n, e := strconv.Atoi(g[1]); e == nil && n >= 1 {
					return n
				}
			}
		}
		return 0
	}

	rom, err := os.ReadFile(bin)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", bin, err)
	}
	if len(rom) < 6 || len(rom) > 0x10000 {
		return nil, fmt.Errorf("unexpected ROM size %d bytes", len(rom))
	}
	// A bank-switched cartridge is decoded one bank at a time and then analysed as ONE
	// MERGED program keyed on (bank, address). Each bank is a 4K image in its own
	// right — same mask, same $F000 base, its own vectors — so the decode runs over it
	// unchanged; what the merge adds is flow BETWEEN banks (see switchModel).
	//
	// A flat ROM is simply the one-bank case and goes down the identical path, so
	// its report is unchanged byte for byte.
	units, unitErr := analysisUnits(rom, bin)
	if unitErr != "" {
		return &Report{Asm: filepath.Base(asmPath), Budget: budget, BankedDeclined: unitErr}, nil
	}

	// Each bank's decode is then closed over bank switches: an instruction reaching
	// a hotspot continues at the following address in the bank the hotspot's own
	// symbol names, so that address seeds the target bank. Without it a worker bank
	// shows only its reset stub: measured, litmus_bank bank 1 decoded 4 instructions
	// and the machine executed 4 OTHERS there ($FF03/$FF05/$FF07/$FF09), none of
	// them among the 4 decoded — the residue this closes.
	// Per-bank source lines. The listing's address column is the PHYSICAL ROM offset,
	// so bank = offset>>12 recovers what a flat parse throws away; see srcmap.BankMap
	// for why the labels come from the source rather than the symbol table.
	if len(units) > 1 {
		sm.AttachBanked(srcmap.ParseBanked(lst, asmPath, len(units)))
	}

	decodes, instrs, entries, seeds := decodeUnits(units)

	sw := switchModel{banked: len(units) > 1, banks: map[int]bool{}}
	if sw.banked {
		sw.hotspots = units[0].hotspots
		for _, u := range units {
			sw.banks[u.bank] = true
		}
	}

	rep := &Report{Asm: filepath.Base(asmPath), Budget: budget, Converged: true}
	if len(units) > 1 {
		rep.Banks = len(units)
		rep.CrossBankSeeds = seeds.seeds
		rep.CrossBankSeedRounds = seeds.rounds
		rep.CrossBankSeedCapped = seeds.capped
		rep.UnresolvedHotspots = seeds.unresolved
		rep.UnresolvableSwitchAccesses = seeds.unresolvable
		// srcmap keys line numbers on DASM's listing address column, which on a banked
		// image is the PHYSICAL ROM OFFSET rather than the RORG'd CPU address. Measured
		// on litmus_bank's listing: bank 0's rows read `0f00 ad f9 ff lda $FFF9` (below
		// $1000, so Parse drops them) and bank 1's read `1f03 a9 b1 lda #$B1` (stored as
		// if $1F03 were a CPU address). So `@lines` and `@amax`, which are looked up
		// THROUGH that map, resolve for nothing on a bank-switched kernel — every region
		// silently gets @lines 1. Saying so is the point of this field: litmus_bank_f4's
		// crossing region really does span 2 scanlines (measured 128 cycles over 2 lines),
		// and it is reported as a budget violation because the annotation that would
		// declare those 2 lines cannot be read, not because the prover disagrees.
		cov := sm.BankLineCoverage()
		total := 0
		for _, n := range cov {
			total += n
		}
		rep.SourceAnnotations = fmt.Sprintf("per-bank: %d addresses resolved to source lines across %d bank(s) "+
			"%v. DASM's listing address column is the PHYSICAL ROM OFFSET, so bank = offset>>12 recovers what a "+
			"flat parse discards; labels come from the source file rather than the symbol table, whose RORG'd "+
			"addresses interleave every bank. @lines and @amax are resolvable here", total, len(cov), cov)
	}

	// ONE abstract-interpretation fixpoint over the merged program. Cross-bank state
	// FLOWS along the switch edge instead of the landing site being re-seeded with Top,
	// which is strictly more precise and still sound — but only because romAt is routed
	// by the instruction's own bank (see absint.romTableRange), or a `lda table,x` in
	// bank 1 would be bounded by bank 0's table bytes.
	//
	// When some instruction can switch banks in a way switchEdges does NOT model, the
	// incoming-edge set of its real landing site is incomplete, and a join over an
	// incomplete predecessor set is an under-approximation. Those landing sites — and
	// only those — are forced to Top (see unmodelledLandings).
	widen, widenWhy := unmodelledLandings(instrs, sw)
	rep.SwitchWidenedSites = len(widen)
	rep.SwitchWidenReasons = widenWhy
	states, converged := computeStates(instrs, entries, romByBank(decodes), sw, widen)
	if !converged {
		rep.Converged = false
	}

	// Region starts as sites, sorted by (bank, address) — the same order the previous
	// per-bank loop produced, so the output ordering is unchanged.
	var starts []site
	for a, in := range instrs {
		if in.isWSYNC() {
			starts = append(starts, a)
		}
	}
	sortSites(starts)

	perBankRegions := map[int]int{}
	for _, sa := range starts {
		reg := analyzeRegion(instrs, instrs[sa], budget*regionLines(sa), regionAmax(sa), sm, states, sw)
		if sw.active() {
			// Every switch this region can perform must have a modelled edge. The residue
			// — an instruction fetched across a hotspot, a jmp/jsr into one, an
			// unresolvable target — is refused and counted, because bounding it on the
			// bytes that follow in THIS bank would be a number about a path the hardware
			// does not take.
			why, edges := residualSwitchRefusal(instrs, sa, sw, states)
			reg.SwitchEdges = edges
			if why != "" {
				reg.Bounded = false
				reg.Reason = why
				reg.Worst, reg.Over, reg.Path = 0, false, nil
				rep.UnmodelledSwitches++
			} else {
				rep.ModelledSwitchEdges += edges
			}
		}
		perBankRegions[sa.bank]++
		rep.Regions++
		if reg.Kind == "blank" {
			// ①: a blank region no longer vanishes as worst=0. It is not a visible-line
			// tear (so it stays OUT of Lines/MaxWorst/Violations/Certified for backward
			// compatibility), but a blank region over its budget adds a scanline = roll,
			// so surface it and feed the ③ roll_free ∀ verdict.
			rep.Blank++
			rep.BlankLines = append(rep.BlankLines, reg)
			if !reg.Bounded {
				rep.BlankUnbounded = append(rep.BlankUnbounded, reg)
			} else {
				if reg.Worst > rep.BlankMaxWorst {
					rep.BlankMaxWorst = reg.Worst
				}
				if reg.Over {
					rep.BlankOver = append(rep.BlankOver, reg)
				}
			}
			continue
		}
		rep.Lines = append(rep.Lines, reg) // PONG-C3: keep EVERY visible region, passing or not
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
	if len(units) > 1 {
		for _, u := range decodes {
			rep.BankCoverage = append(rep.BankCoverage, BankCoverage{
				Bank: u.bank, Instructions: len(u.instrs), Regions: perBankRegions[u.bank],
				SeededEntries: u.seeded,
			})
		}
	}
	if rep.Regions == 0 {
		// No reachable STA WSYNC: a bank-switched kernel (display loop in another
		// bank we don't follow) or a non-kernel ROM. Never vacuously certify "0
		// regions, all safe" — that would be the prover lying. Report it.
		rep.Unbounded = append(rep.Unbounded, Region{Budget: budget,
			Reason: "no STA WSYNC reached from the reset/IRQ/NMI vectors — bank-switched or not a single-bank kernel (out of scope)"})
	}
	// A capped fixpoint leaves under-approximated states, so nothing derived from
	// them may be certified — the bound would rest on values the analysis never
	// finished computing.
	rep.Certified = rep.Converged && rep.Regions > 0 && rep.UnmodelledSwitches == 0 &&
		len(rep.Violations) == 0 && len(rep.Unbounded) == 0
	// ③ roll_free: the ∀ roll-freedom verdict — EVERY region (blank AND visible) is
	// bounded and within its budget×@lines span. Stricter than Certified (visible-only):
	// a blank region over budget or unbounded means the frame's line total is NOT
	// statically proven here (it is delegated to the runtime ∃ ntsc_frame_lines check).
	rep.RollFree = rep.Converged && rep.Regions > 0 && rep.UnmodelledSwitches == 0 &&
		len(rep.Violations) == 0 && len(rep.Unbounded) == 0 &&
		len(rep.BlankOver) == 0 && len(rep.BlankUnbounded) == 0
	return rep, nil
}

// intimAddr is the RIOT's interval-timer read port. A read of it returns whatever the
// timer has left to count down, which is why a loop polling it has no trip count in
// any register this analysis tracks.
//
// ONLY THE CANONICAL ADDRESS IS RECOGNISED, deliberately. The 6532 is decoded from a
// handful of address lines, so INTIM answers at many mirrors, and matching them would
// mean reproducing that decode here. All twenty timer spins in the corpus read exactly
// $0284. A ROM using a mirror is simply not recognised and keeps the generic refusal —
// the failure mode of matching too narrowly is the message a builder already gets,
// while the failure mode of matching too widely is calling an ordinary polling loop a
// timer wait.
const intimAddr = 0x0284

// timerSpins counts loops named as timer waits. A MEASUREMENT counter, so a fixture
// can prove its own premise.
var timerSpins int

// timerSpin reports whether the loop header..latch is a spin on the RIOT timer, and
// the address it polls.
//
// The test is that the LAST instruction in the body to touch Z is a load of INTIM, and
// that the latch branches on Z. It is written that way rather than as "the body
// contains `lda $0284`" because the corpus holds `sta $002A / lda $0284 / bne`, where
// a store sits in the body and leaves the flags alone — and because a body that loads
// INTIM and then does arithmetic on something else is NOT spinning on the timer, and
// must keep the generic refusal.
func timerSpin(nodes map[site]Instr, header site, latch Instr) (bool, uint16) {
	switch latch.Def.Operator {
	case instructions.BNE, instructions.BEQ:
	default:
		return false, 0 // the latch reads C, N or V: not "until the timer is zero"
	}
	var last Instr
	found := false
	at := header
	for i := 0; i < 32; i++ {
		in, ok := nodes[at]
		if !ok {
			return false, 0
		}
		if in.Addr == latch.Addr {
			break
		}
		if !preservesZN(in.Def.Operator) {
			last, found = in, true
		}
		at = in.nextSite()
	}
	if !found {
		return false, 0
	}
	switch last.Def.Operator {
	case instructions.LDA, instructions.LDX, instructions.LDY:
	default:
		return false, 0
	}
	if last.Def.AddressingMode != instructions.Absolute || last.Operand != intimAddr {
		return false, 0
	}
	return true, last.Operand
}
