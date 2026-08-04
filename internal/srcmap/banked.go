package srcmap

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// BankMap is Map for a BANK-SWITCHED image: everything is keyed on (bank, CPU
// address) rather than on an address alone.
//
// Why a second type rather than a flag on Map. On a banked ROM the two inputs say
// different things and only one of them can be trusted:
//
//   - The LISTING's address column is the PHYSICAL ROM OFFSET. Measured on
//     banked_game.asm: bank 0's rows read `0f80`, bank 1's read `1f83`. That is
//     exact — bank = offset>>12, CPU address = $F000 | (offset & $0FFF) — and it is
//     why per-bank line numbers are recoverable at all.
//   - The SYMBOL table's addresses are RORG'd, so every bank's labels land in the
//     same $F0xx range and interleave. "The last label at or before this address"
//     can belong to either bank; measured, the prover reported two BANK 0 regions at
//     `LvTab+0` / `LvTab+2` when LvTab is a bank 1 table 4K away.
//
// So labels here do NOT come from the symbol table. They come from the SOURCE FILE:
// a label defined on source line L appears in the listing on line L, and that row
// carries the offset, which carries the bank. No ambiguity to resolve.
type BankMap struct {
	File   string
	banks  int
	lines  map[bankSite]int      // (bank, addr) -> source line
	labels map[int][]bankedLabel // bank -> labels, address order
}

type bankSite struct {
	bank int
	addr uint16
}

type bankedLabel struct {
	addr uint16
	name string
}

// A label definition in DASM source: a name starting in column 1, optionally
// followed by a colon. Anything indented is an instruction or directive.
var srcLabelRe = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*):?(?:\s|$)`)

// Not every listing row that shows an address PUT SOMETHING THERE, and on a banked
// image the difference decides whether a line number is a fact or a fabrication.
// DASM's own marker is `????` in place of the byte column: a comment, a `processor`
// directive and an `=` equate all print one, and the address they print alongside it
// is whatever the PC happened to be — for a source whose first ORG has not run yet,
// that is offset $0000, i.e. bank 0's first byte.
//
// Measured on litmus_bank_f4 before this rule existed: `bank 0 $F000` resolved to
// line 1, the file's opening comment, because line 1's row reads `0000 ????` and
// first-occurrence wins over the `sei` on line 14 that actually assembles there.
// The equates on lines 6-8 (VSYNC/VBLANK/WSYNC) start in column 1, so they were also
// placed as bank 0 LABELS at $F000 — three names for an address none of them names.
//
// So rows are classified, not merely matched:
//
//   - lstEmitRe — the row emitted bytes at that offset. Layout measured on
//     banked_game.lst and litmus_bank_f4.lst: `<line>  <offset>`, two tabs, the byte
//     column, then the source text. ONLY these define a (bank,addr) -> line mapping,
//     which makes the mapping mean "this line assembles to this address".
//   - lstPosRe — the row carries a position but no bytes: ORG/RORG, or a label alone
//     on its line. These place LABELS (a label needs an address, not bytes) but never
//     a line number.
//   - lstNoOutRe — `????`: neither. Its address is stale.
//
// Two more rows print bytes without putting them in the image, and both were caught by
// the corpus check rather than by reading DASM:
//
//   - An EQUATE AFTER THE FIRST ORG prints its VALUE in the byte column and the live PC
//     in the address column, with no `????` to mark it. Measured on exerciser.asm:
//     `0e10  fd 10   ZHmove = ZHmoveEnd - 256` made bank 0 $FE10 name line 814. lstEquRe
//     matches the `NAME<pad>=<tab>` shape DASM prints for one.
//   - A MACRO EXPANSION is listed with the MACRO BODY's line numbers, restarting at 0.
//     Measured on exerciser.asm, whose GLYPH macro made fourteen bank 1 addresses name
//     line 2 — a comment in the file's header. File rows are numbered monotonically, so
//     a row that does not advance past every line accepted so far belongs to an expansion
//     (or to an included file, whose line numbers are not this file's either) and cannot
//     be attributed to asmPath.
var lstEmitRe = regexp.MustCompile(`^\s*(\d+)\s+([0-9a-fA-F]{4})\t\t\s+[0-9a-f]{2}(?: [0-9a-f]{2})*(?:[ \t*]|$)`)
var lstPosRe = regexp.MustCompile(`^\s*(\d+)\s+([0-9a-fA-F]{4})(?:\s|$)`)
var lstNoOutRe = regexp.MustCompile(`^\s*\d+\s+[0-9a-fA-F]{4} \?\?\?\?`)
var lstEquRe = regexp.MustCompile("\t\\s*[A-Za-z_][A-Za-z0-9_]*\\s+=\t")

// ParseBanked builds a per-bank map. banks is the number of 4K banks; rows whose
// offset lies beyond them are ignored rather than folded, because folding is exactly
// the mistake this type exists to avoid.
func ParseBanked(lst, asmPath string, banks int) *BankMap {
	m := &BankMap{
		File:   filepath.Base(asmPath),
		banks:  banks,
		lines:  map[bankSite]int{},
		labels: map[int][]bankedLabel{},
	}
	if banks < 1 {
		return m
	}

	// listing: source line -> physical offset (first occurrence wins, as in Parse)
	lineOff := map[int]int{}
	maxLine := 0
	for _, ln := range strings.Split(lst, "\n") {
		if lstNoOutRe.MatchString(ln) {
			continue // `????`: this row assembled nothing, and its address is stale
		}
		g := lstPosRe.FindStringSubmatch(ln)
		if g == nil {
			continue
		}
		srcLine, _ := strconv.Atoi(g[1])
		if srcLine <= maxLine {
			continue // a macro expansion or an included file: not asmPath's line numbering
		}
		maxLine = srcLine
		off64, err := strconv.ParseUint(g[2], 16, 32)
		if err != nil {
			continue
		}
		off := int(off64)
		if off >= banks*4096 {
			continue // past the image: DASM prints the end PC on trailing comment rows
		}
		if lstEmitRe.MatchString(ln) && !lstEquRe.MatchString(ln) {
			site := bankSite{off / 4096, uint16(0xF000 | (off & 0x0FFF))}
			if _, seen := m.lines[site]; !seen {
				m.lines[site] = srcLine
			}
		}
		if _, seen := lineOff[srcLine]; !seen {
			lineOff[srcLine] = off
		}
	}

	// labels: read them from the source, then place each one through its own line's
	// offset. A label's bank is therefore a measurement, not a guess.
	if src, err := os.ReadFile(asmPath); err == nil {
		for i, text := range strings.Split(string(src), "\n") {
			g := srcLabelRe.FindStringSubmatch(text)
			if g == nil {
				continue
			}
			off, ok := lineOff[i+1] // listing line numbers are 1-based
			if !ok {
				continue
			}
			bank := off / 4096
			m.labels[bank] = append(m.labels[bank], bankedLabel{uint16(0xF000 | (off & 0x0FFF)), g[1]})
		}
	}
	for b := range m.labels {
		ls := m.labels[b]
		sort.Slice(ls, func(i, j int) bool { return ls[i].addr < ls[j].addr })
		m.labels[b] = ls
	}
	return m
}

// Line returns the source line for a (bank, address), if the listing had one.
func (m *BankMap) Line(bank int, addr uint16) (int, bool) {
	if m == nil {
		return 0, false
	}
	l, ok := m.lines[bankSite{bank, addr}]
	return l, ok
}

// Coverage reports how many (bank, address) pairs resolved, per bank. A caller that
// prints locations should say so: a map that resolved nothing looks exactly like a
// map that resolved everything to the same place.
func (m *BankMap) Coverage() map[int]int {
	out := map[int]int{}
	if m == nil {
		return out
	}
	for s := range m.lines {
		out[s.bank]++
	}
	return out
}

// Labels reports how many labels were placed in each bank.
func (m *BankMap) Labels() map[int]int {
	out := map[int]int{}
	if m == nil {
		return out
	}
	for b, ls := range m.labels {
		out[b] = len(ls)
	}
	return out
}

// LabelBank reports which bank a label was defined in. ok=false when the source had
// no such label. Used to check that a rendered location cannot name another bank's
// code — the defect BankMap exists to prevent.
func (m *BankMap) LabelBank(name string) (int, bool) {
	if m == nil {
		return 0, false
	}
	for b, ls := range m.labels {
		for _, l := range ls {
			if l.name == name {
				return b, true
			}
		}
	}
	return 0, false
}

// Locate renders "bank N Label+off (file:line)" for a (bank, address), or "" when
// the listing has nothing there. The label is the nearest one at or before addr IN
// THE SAME BANK, so it cannot name another bank's code.
func (m *BankMap) Locate(bank int, addr uint16) string {
	if m == nil {
		return ""
	}
	line, okLine := m.Line(bank, addr)
	best := -1
	for i, l := range m.labels[bank] {
		if l.addr <= addr {
			best = i
		} else {
			break
		}
	}
	// A label is only useful while it is NEAR. Measured on banked_game: $FF86 sits in
	// a trampoline block that carries no label of its own, and the nearest preceding
	// label in that bank is the level table 3949 bytes back — "LvTab+3949" points a
	// reader at something with no relationship to the address. Past a page, drop the
	// label and print the source line alone, which is exact.
	if best >= 0 && int(addr)-int(m.labels[bank][best].addr) > 0xFF {
		best = -1
	}
	switch {
	case best >= 0 && okLine:
		off := addr - m.labels[bank][best].addr
		if off == 0 {
			return fmt.Sprintf("bank %d %s (%s:%d)", bank, m.labels[bank][best].name, m.File, line)
		}
		return fmt.Sprintf("bank %d %s+%d (%s:%d)", bank, m.labels[bank][best].name, off, m.File, line)
	case best >= 0:
		off := addr - m.labels[bank][best].addr
		if off == 0 {
			return fmt.Sprintf("bank %d %s", bank, m.labels[bank][best].name)
		}
		return fmt.Sprintf("bank %d %s+%d", bank, m.labels[bank][best].name, off)
	case okLine:
		return fmt.Sprintf("bank %d (%s:%d)", bank, m.File, line)
	}
	return ""
}
