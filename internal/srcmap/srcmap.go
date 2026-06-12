// Package srcmap は DASM のリスティング（-l）とシンボル表（-s）から
// 「PC → ソース行・直近ラベル+オフセット」の対応を作る（U-M9: ソース行デバッグ）。
// assemble_and_load 経由でロードした ROM に対し、trace_clocks / watch_ram /
// assert_line_budget / read_cpu の出力へ `at Label+2 (file.asm:123)` を併記するための基盤。
// 制限: フラット 2K/4K 前提（バンク ROM は RORG 重複のため対象外＝空文字を返す）。
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
	labels []label // アドレス昇順
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
		if addr < 0x1000 {
			continue
		}
		m.labels = append(m.labels, label{addr, g[1]})
	}
	sort.Slice(m.labels, func(i, j int) bool { return m.labels[i].addr < m.labels[j].addr })
	return m
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
