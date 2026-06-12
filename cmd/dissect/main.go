// dissect — 実行トレース × ROM 照合によるアセット抽出（S4, v1.38.0）。
// 画素解析の上位互換となる「ROM がある時の本命」: 実行中に TIA 描画レジスタへ store された
// 値列（PC・scanline 付き）を記録し、ROM バイト列の中から一致するテーブル（正順/逆順）を
// 探して特定する。画面に映った瞬間だけでなく**テーブル全体**（全アニメフレーム等）に到達できる。
//
//	go run ./cmd/dissect -rom game.bin [-frames 3] [-warmup 150] [-out dir]
//	                     [-distella path/to/distella]   ; あれば注釈付き逆アセンブルも出力
//
// クリーンルーム方針: 商用 ROM の解析結果は学習・解析専用（inbox/reference 限定、公開リポへ
// コミットしない）。技として一般化したものだけを自前実装で techniques カタログへ。
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kidsnz/atari2600-harness/internal/emu"
)

var regNames = map[uint16]string{
	0x06: "COLUP0", 0x07: "COLUP1", 0x08: "COLUPF", 0x09: "COLUBK",
	0x0D: "PF0", 0x0E: "PF1", 0x0F: "PF2",
	0x1B: "GRP0", 0x1C: "GRP1",
}

type store struct {
	frame, scanline int
	reg             string
	val             uint8
	pc              uint16
}

func main() {
	rom := flag.String("rom", "", "ROM (.bin)")
	warmup := flag.Int("warmup", 150, "frames before tracing")
	frames := flag.Int("frames", 3, "frames to trace")
	out := flag.String("out", "", "output dir (default: alongside ROM)")
	distella := flag.String("distella", "", "path to distella binary (optional, adds annotated disassembly)")
	flag.Parse()
	if *rom == "" {
		fmt.Fprintln(os.Stderr, "usage: dissect -rom game.bin [...]")
		os.Exit(2)
	}
	if err := run(*rom, *warmup, *frames, *out, *distella); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
}

func run(rom string, warmup, frames int, out, distella string) error {
	if out == "" {
		base := strings.TrimSuffix(filepath.Base(rom), filepath.Ext(rom))
		out = filepath.Join(filepath.Dir(rom), base+"_dissect")
	}
	romBytes, err := os.ReadFile(rom)
	if err != nil {
		return err
	}
	e, err := emu.New("AUTO")
	if err != nil {
		return err
	}
	if err := e.LoadROM(rom); err != nil {
		return err
	}
	if err := e.RunFrames(warmup); err != nil {
		return err
	}

	// --- 実行トレース: TIA 描画レジスタへの store を記録 ---
	var stores []store
	startF := e.Coords().Frame
	for e.Coords().Frame < startF+frames {
		pc := e.PC()
		op := e.PeekROM(pc)
		var target int = -1
		var valSel byte // 'A','X','Y'
		switch op {
		case 0x85: // STA zp
			target, valSel = int(e.PeekROM(pc+1)), 'A'
		case 0x86: // STX zp
			target, valSel = int(e.PeekROM(pc+1)), 'X'
		case 0x84: // STY zp
			target, valSel = int(e.PeekROM(pc+1)), 'Y'
		case 0x95: // STA zp,X
			target, valSel = int(e.PeekROM(pc+1))+int(e.XReg()), 'A'
		case 0x8D: // STA abs
			target, valSel = int(e.PeekROM(pc+1))|int(e.PeekROM(pc+2))<<8, 'A'
		}
		var name string
		var val uint8
		if target >= 0 {
			if n, ok := regNames[uint16(target&0x3F)]; ok && target < 0x80 {
				name = n
				switch valSel {
				case 'A':
					val = e.A()
				case 'X':
					val = e.XReg()
				case 'Y':
					val = e.YReg()
				}
			}
		}
		c := e.Coords()
		if err := e.StepInstruction(); err != nil {
			return err
		}
		if name != "" {
			stores = append(stores, store{c.Frame, c.Scanline, name, val, pc})
		}
	}

	// --- ストリーム化: レジスタ毎に scanline 連続（gap≤2）の値列へ ---
	type stream struct {
		reg        string
		frame      int
		fromL, toL int
		vals       []uint8
	}
	var streams []stream
	byReg := map[string][]store{}
	for _, s := range stores {
		byReg[s.reg] = append(byReg[s.reg], s)
	}
	for reg, ss := range byReg {
		var cur *stream
		for _, s := range ss {
			if cur == nil || s.frame != cur.frame || s.scanline > cur.toL+2 {
				if cur != nil && len(cur.vals) >= 4 {
					streams = append(streams, *cur)
				}
				cur = &stream{reg: reg, frame: s.frame, fromL: s.scanline, toL: s.scanline}
			}
			cur.vals = append(cur.vals, s.val)
			cur.toL = s.scanline
		}
		if cur != nil && len(cur.vals) >= 4 {
			streams = append(streams, *cur)
		}
	}
	sort.Slice(streams, func(i, j int) bool {
		if streams[i].frame != streams[j].frame {
			return streams[i].frame < streams[j].frame
		}
		return streams[i].fromL < streams[j].fromL
	})

	// --- ROM 照合: 値列（と逆順）を ROM から探す ---
	find := func(seq []uint8) (int, bool, bool) { // offset, found, reversed
		if idx := bytesIndex(romBytes, seq); idx >= 0 {
			return idx, true, false
		}
		rev := make([]uint8, len(seq))
		for i := range seq {
			rev[i] = seq[len(seq)-1-i]
		}
		if idx := bytesIndex(romBytes, rev); idx >= 0 {
			return idx, true, true
		}
		return -1, false, false
	}
	dedup := func(vals []uint8) []uint8 { // 行倍化（連続同値）を 1 つに畳んだ版も試す
		var out []uint8
		for i, v := range vals {
			if i == 0 || v != out[len(out)-1] {
				out = append(out, v)
			}
		}
		return out
	}
	trim := func(vals []uint8) []uint8 { // 前後の 0（消灯行）を落とした版＝スプライト本体
		a, b := 0, len(vals)
		for a < b && vals[a] == 0 {
			a++
		}
		for b > a && vals[b-1] == 0 {
			b--
		}
		return vals[a:b]
	}

	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	var rep strings.Builder
	fmt.Fprintf(&rep, "dissect %s — 実行トレース×ROM照合（%dフレーム, store %d 件, stream %d 本）\n",
		filepath.Base(rom), frames, len(stores), len(streams))
	fmt.Fprintf(&rep, "%s\n", strings.Repeat("=", 70))
	romOrg := 0x10000 - len(romBytes) // 末尾が $FFFF に当たる素朴な ORG（4K=$F000）
	annots := map[int]string{}        // ROM offset → 注釈
	for _, st := range streams {
		fmt.Fprintf(&rep, "\n%s frame%d 行%d-%d（%d値）: ", st.reg, st.frame, st.fromL, st.toL, len(st.vals))
		if c := dedup(trim(st.vals)); len(c) <= 1 { // 全行同値＝テーブルではなく即値（ROM 検索すると偽陽性になる）
			v := uint8(0)
			if len(c) == 1 {
				v = c[0]
			}
			fmt.Fprintf(&rep, "定数 $%02X（テーブルではなく即値/単色）\n", v)
			continue
		}
		seqs := [][]uint8{st.vals}
		tags := []string{""}
		if t := trim(st.vals); len(t) >= 4 && len(t) != len(st.vals) {
			seqs, tags = append(seqs, t), append(tags, "（消灯行を除いた本体で一致）")
			if d := dedup(t); len(d) >= 4 && len(d) != len(t) {
				seqs, tags = append(seqs, d), append(tags, "（消灯行除去＋行倍化を畳んで一致）")
			}
		}
		if d := dedup(st.vals); len(d) >= 4 && len(d) != len(st.vals) {
			seqs, tags = append(seqs, d), append(tags, "（行倍化を畳んで一致）")
		}
		found := false
		for si, seq := range seqs {
			if off, ok, rev := find(seq); ok {
				addr := romOrg + off
				tag := tags[si]
				if rev {
					tag += "（逆順格納）"
				}
				fmt.Fprintf(&rep, "ROM $%04X-$%04X %s\n", addr, addr+len(seq)-1, tag)
				if st.reg == "GRP0" || st.reg == "GRP1" {
					for _, v := range seq {
						fmt.Fprintf(&rep, "      %s\n", artRow(v))
					}
				}
				annots[off] = fmt.Sprintf("%s table (frame%d rows %d-%d)%s", st.reg, st.frame, st.fromL, st.toL, tag)
				found = true
				break
			}
		}
		if !found {
			show := st.vals
			if t := trim(st.vals); len(t) > 0 {
				show = t
			}
			if len(show) > 32 {
				show = show[:32]
			}
			fmt.Fprintf(&rep, "ROM 内に直接一致なし（計算生成 or 加工されたデータ）\n  値（先頭32まで・前後0除去後）: %v\n", show)
		}
	}

	// --- distella 注釈付き逆アセンブル（任意）---
	if distella != "" {
		if outAsm, err := exec.Command(distella, "-a", rom).Output(); err == nil {
			// 注釈先＝「対象アドレス以下で最も近いラベル行」。テーブルは基準オフセット参照
			// されがちで一致開始アドレス自体にラベルが立たないことが多いため。
			lines := strings.Split(string(outAsm), "\n")
			type lbl struct {
				line int
				addr int
			}
			var lbls []lbl
			for i, ln := range lines {
				if len(ln) >= 6 && ln[0] == 'L' && ln[5] == ':' {
					var a int
					if _, err := fmt.Sscanf(ln[1:5], "%04X", &a); err == nil {
						lbls = append(lbls, lbl{i, a})
					}
				}
			}
			ins := map[int][]string{} // 行番号 → 挿入コメント
			hits := 0
			for off, note := range annots {
				addr := romOrg + off
				best := -1
				for bi, l := range lbls { // distella 出力はアドレス昇順
					if l.addr <= addr {
						best = bi
					} else {
						break
					}
				}
				if best >= 0 {
					d := addr - lbls[best].addr
					tag := ""
					if d > 0 {
						tag = fmt.Sprintf("（ラベル+%d＝$%04X から）", d, addr)
					}
					ins[lbls[best].line] = append(ins[lbls[best].line],
						fmt.Sprintf("; ★dissect: %s %s", note, tag))
					hits++
				}
			}
			var sb strings.Builder
			for i, ln := range lines {
				for _, c := range ins[i] {
					sb.WriteString(c + "\n")
				}
				sb.WriteString(ln + "\n")
			}
			os.WriteFile(filepath.Join(out, "disassembly.asm"), []byte(strings.TrimRight(sb.String(), "\n")+"\n"), 0o644)
			fmt.Fprintf(&rep, "\n（distella 逆アセンブル: disassembly.asm — %d 箇所に dissect 注釈）\n", hits)
		} else {
			fmt.Fprintf(&rep, "\n（distella 実行失敗: %v — トレース照合のみ）\n", err)
		}
	}
	os.WriteFile(filepath.Join(out, "dissect.txt"), []byte(rep.String()), 0o644)
	fmt.Print(rep.String())
	fmt.Println("\nwrote", out+"/")
	return nil
}

func bytesIndex(hay []byte, needle []uint8) int {
	return strings.Index(string(hay), string(needle))
}

func artRow(v uint8) string {
	s := fmt.Sprintf("%08b", v)
	return strings.NewReplacer("0", ".", "1", "X").Replace(s)
}
