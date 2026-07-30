// Package srcmap は DASM のリスティング（-l）とシンボル表（-s）から
// 「PC → ソース行・直近ラベル+オフセット」の対応を作る（U-M9: ソース行デバッグ）。
// assemble_and_load 経由でロードした ROM に対し、trace_clocks / watch_ram /
// assert_line_budget / read_cpu の出力へ `at Label+2 (file.asm:123)` を併記するための基盤。
// Scope, corrected to what the code does (2026-07-29). Flat 2K/4K only. On a
// BANK-SWITCHED ROM this does NOT return an empty string, as the previous note claimed:
// DASM's listing address column is the PHYSICAL ROM OFFSET rather than the RORG'd CPU
// address, so Parse drops bank 0's rows entirely (they are below $1000, filtered as
// TIA/RIOT equates) and stores banks 1..n's offsets as if they were CPU addresses
// ($1F03 for bank 1's $FF03). Locate therefore still answers, from the .sym file's
// labels alone, with a label-only string and no `(file:line)` — measured: litmus_bank
// regions come back "Vis+0" while flat cb_clean gives "Main+8 (cb_clean.asm:33)". Line()
// misses on every banked address, which makes `@lines`/`@amax` inert on a bank-switched
// kernel; internal/cyclebound reports that in Report.SourceAnnotations rather than
// letting it look like the prover disagreeing. Fixing it means tracking each segment's
// ORG/RORG pair and keying on (bank, cpuAddr) = (offset/4096, offset%4096|base).
package srcmap

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Map は 1 本の .asm に対する PC 対応表。
type Map struct {
	File   string // 表示用ファイル名（ベース名）
	lines  map[uint16]int
	labels []label           // アドレス昇順・ROM 域（$1000+）のみ＝Locate 用
	syms   map[string]uint16 // 全シンボル（RAM equate 含む）＝Symbol 用（watch/patch のシンボル解決）
	bank   *BankMap          // バンク切替イメージのときだけ非 nil（AttachBanked）
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

// Parse はリスティングとシンボル表のテキストから Map を作る。
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
		if addr < 0x1000 { // TIA/RIOT equ 等は除外（コードは $F000 域）
			continue
		}
		srcLine, _ := strconv.Atoi(g[1])
		if _, seen := m.lines[addr]; !seen { // 最初の出現（マクロ展開等の重複は先勝ち）
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
			m.syms[g[1]] = addr // RAM equate（<$1000）も保持＝watch/poke のシンボル解決用
		}
		if addr < 0x1000 {
			continue // Locate の直近ラベル探索は ROM 域のみ（TIA/RIOT equ がコード位置を汚さない）
		}
		m.labels = append(m.labels, label{addr, g[1]})
	}
	sort.Slice(m.labels, func(i, j int) bool { return m.labels[i].addr < m.labels[j].addr })
	return m
}

// Symbol はシンボル表のラベル名からアドレスを返す（patch/watch オプションの symbol 指定用）。
// ROM ラベルに加え RAM equate（BallRow=$83 等）も引ける。
func (m *Map) Symbol(name string) (uint16, bool) {
	if m == nil {
		return 0, false
	}
	a, ok := m.syms[name]
	return a, ok
}

// Line は PC に対応するソース行番号（1 起点）を返す。対応が無ければ ok=false。
func (m *Map) Line(pc uint16) (int, bool) {
	if m == nil {
		return 0, false
	}
	ln, ok := m.lines[pc]
	return ln, ok
}

// Locate は PC を「Label+off (file:line)」へ。対応が無ければ空文字。
// ROM ミラー（$F000 域以外の PC）は下位 13bit を $E000|… に正規化せず素朴に直照合のみ。
func (m *Map) Locate(pc uint16) string {
	if m == nil {
		return ""
	}
	line, okLine := m.lines[pc]
	// 直近の先行ラベル
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
