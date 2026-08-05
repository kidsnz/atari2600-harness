// Package srcmap builds a "PC → source line / nearest label+offset" mapping from DASM's
// listing (-l) and symbol table (-s) (U-M9: source-line debugging).
// It is the foundation for printing `at Label+2 (file.asm:123)` alongside the output of
// trace_clocks / watch_ram / assert_line_budget / read_cpu for a ROM loaded via
// assemble_and_load.
// Scope, corrected to what the code does (2026-08-04). Map itself is FLAT 2K/4K only:
// DASM's listing address column is the PHYSICAL ROM OFFSET rather than the RORG'd CPU
// address, so on a banked image Parse drops bank 0's rows entirely (they are below
// $1000, filtered as TIA/RIOT equates) and stores banks 1..n's offsets as if they were
// CPU addresses ($1F03 for bank 1's $FF03). A banked image is therefore keyed on
// (bank, address) by BankMap (banked.go), which a caller that knows the image is
// bank-switched builds with ParseBanked and hangs here with AttachBanked; LineBank /
// LocateBank / BankLineCoverage go through it and never fall back to the flat lookup,
// whose label list interleaves every bank's labels and can name the wrong bank's code.
// internal/cyclebound reports the per-bank coverage in Report.SourceAnnotations so a
// caller can tell "resolved nothing" from "resolved everything".
package srcmap

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Map is the PC mapping table for a single .asm file.
type Map struct {
	File   string // file name for display (base name)
	lines  map[uint16]int
	labels []label           // ascending by address, ROM range ($1000+) only = for Locate
	syms   map[string]uint16 // all symbols (incl. RAM equates) = for Symbol (symbol resolution for watch/patch)
	bank   *BankMap          // non-nil only for a bank-switched image (AttachBanked)
}

// AttachBanked gives this Map a per-bank companion. Callers that know the image is
// bank-switched build one with ParseBanked and attach it here, so every existing
// consumer keeps its signature and only the banked path changes.
func (m *Map) AttachBanked(b *BankMap) {
	if m != nil {
		m.bank = b
	}
}

// LocateBank renders a (bank, address) through the per-bank map, or "" when there is
// none. It never falls back to the flat lookup: on a banked image the flat label list
// interleaves every bank's labels and can name the wrong bank's code, which is the
// whole reason BankMap exists.
func (m *Map) LocateBank(bank int, addr uint16) string {
	if m == nil || m.bank == nil {
		return ""
	}
	return m.bank.Locate(bank, addr)
}

// LineBank resolves (bank, address) through the per-bank map. ok=false when the
// image is not banked or the listing had nothing there, so a caller can fall back.
func (m *Map) LineBank(bank int, addr uint16) (int, bool) {
	if m == nil || m.bank == nil {
		return 0, false
	}
	return m.bank.Line(bank, addr)
}

// BankLineCoverage reports how many addresses resolved per bank (empty when not banked).
func (m *Map) BankLineCoverage() map[int]int {
	if m == nil || m.bank == nil {
		return map[int]int{}
	}
	return m.bank.Coverage()
}

type label struct {
	addr uint16
	name string
}

var lstRe = regexp.MustCompile(`^\s*(\d+)\s+([0-9a-fA-F]{4})\s`)
var symRe = regexp.MustCompile(`^(\S+)\s+([0-9a-fA-F]{4})\s*`)

// Parse builds a Map from the listing and symbol-table text.
func Parse(lst, sym, asmPath string) *Map {
	m := &Map{File: filepath.Base(asmPath), lines: map[uint16]int{}}
	for _, ln := range strings.Split(lst, "\n") {
		g := lstRe.FindStringSubmatch(ln)
		if g == nil {
			continue
		}
		addr64, err := strconv.ParseUint(g[2], 16, 16)
		if err != nil {
			continue
		}
		addr := uint16(addr64)
		if addr < 0x1000 { // exclude TIA/RIOT equ etc. (code lives in the $F000 range)
			continue
		}
		srcLine, _ := strconv.Atoi(g[1])
		if _, seen := m.lines[addr]; !seen { // first occurrence wins (duplicates from macro expansion etc.)
			m.lines[addr] = srcLine
		}
	}
	m.syms = map[string]uint16{}
	for _, ln := range strings.Split(sym, "\n") {
		g := symRe.FindStringSubmatch(ln)
		if g == nil || g[1] == "---" {
			continue
		}
		addr64, err := strconv.ParseUint(g[2], 16, 16)
		if err != nil {
			continue
		}
		addr := uint16(addr64)
		if _, seen := m.syms[g[1]]; !seen {
			m.syms[g[1]] = addr // RAM equates (<$1000) are kept too = for watch/poke symbol resolution
		}
		if addr < 0x1000 {
			continue // Locate's nearest-label search covers the ROM range only (TIA/RIOT equ must not pollute code locations)
		}
		m.labels = append(m.labels, label{addr, g[1]})
	}
	sort.Slice(m.labels, func(i, j int) bool { return m.labels[i].addr < m.labels[j].addr })
	return m
}

// Symbol returns the address for a label name from the symbol table (for the symbol form of the
// patch/watch options). Besides ROM labels, RAM equates (BallRow=$83 etc.) can be looked up too.
func (m *Map) Symbol(name string) (uint16, bool) {
	if m == nil {
		return 0, false
	}
	a, ok := m.syms[name]
	return a, ok
}

// Line returns the source line number (1-based) for a PC. ok=false when there is no mapping.
func (m *Map) Line(pc uint16) (int, bool) {
	if m == nil {
		return 0, false
	}
	ln, ok := m.lines[pc]
	return ln, ok
}

// Locate renders a PC as "Label+off (file:line)". Empty string when there is no mapping.
// ROM mirrors (a PC outside the $F000 range) are matched naively as-is, without normalizing the
// low 13 bits to $E000|….
func (m *Map) Locate(pc uint16) string {
	if m == nil {
		return ""
	}
	line, okLine := m.lines[pc]
	// nearest preceding label
	best := -1
	for i, l := range m.labels {
		if l.addr <= pc {
			best = i
		} else {
			break
		}
	}
	switch {
	case best >= 0 && okLine:
		off := pc - m.labels[best].addr
		if off == 0 {
			return fmt.Sprintf("%s (%s:%d)", m.labels[best].name, m.File, line)
		}
		return fmt.Sprintf("%s+%d (%s:%d)", m.labels[best].name, off, m.File, line)
	case okLine:
		return fmt.Sprintf("%s:%d", m.File, line)
	case best >= 0:
		return fmt.Sprintf("%s+%d", m.labels[best].name, pc-m.labels[best].addr)
	}
	return ""
}
