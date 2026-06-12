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
	"github.com/kidsnz/atari2600-harness/pkg/audio"
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
	audioN := flag.Int("audio", 0, "music transcription: sample audio registers for N frames from reset (0=off)")
	flag.Parse()
	if *rom == "" {
		fmt.Fprintln(os.Stderr, "usage: dissect -rom game.bin [...]")
		os.Exit(2)
	}
	if err := run(*rom, *warmup, *frames, *out, *distella, *audioN); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
}

func run(rom string, warmup, frames int, out, distella string, audioN int) error {
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
	fmt.Fprintf(&rep, "dissect %s — runtime trace × ROM matching (%d frames, %d stores, %d streams)\n",
		filepath.Base(rom), frames, len(stores), len(streams))
	fmt.Fprintf(&rep, "%s\n", strings.Repeat("=", 70))
	romOrg := 0x10000 - len(romBytes) // 末尾が $FFFF に当たる素朴な ORG（4K=$F000、2K=$F800）
	banked := len(romBytes) > 4096    // F8/F6/F4: 4K バンクが $F000-$FFFF 窓に入る
	fmtRange := func(off, n int) string {
		if !banked {
			return fmt.Sprintf("ROM $%04X-$%04X", romOrg+off, romOrg+off+n-1)
		}
		bank, a := off/4096, 0xF000+off%4096
		return fmt.Sprintf("ROM bank %d $%04X-$%04X", bank, a, a+n-1)
	}
	if banked {
		fmt.Fprintf(&rep, "(banked cart: %dK = %d banks of 4K; addresses below are bank-relative in the $F000-$FFFF window)\n",
			len(romBytes)/1024, len(romBytes)/4096)
	}
	annots := map[int]string{} // ROM offset → 注釈
	for _, st := range streams {
		fmt.Fprintf(&rep, "\n%s frame%d rows %d-%d (%d values): ", st.reg, st.frame, st.fromL, st.toL, len(st.vals))
		if c := dedup(trim(st.vals)); len(c) <= 1 { // 全行同値＝テーブルではなく即値（ROM 検索すると偽陽性になる）
			v := uint8(0)
			if len(c) == 1 {
				v = c[0]
			}
			fmt.Fprintf(&rep, "constant $%02X (an immediate / solid color, not a table)\n", v)
			continue
		}
		seqs := [][]uint8{st.vals}
		tags := []string{""}
		if t := trim(st.vals); len(t) >= 4 && len(t) != len(st.vals) {
			seqs, tags = append(seqs, t), append(tags, "(matched after trimming blank rows)")
			if d := dedup(t); len(d) >= 4 && len(d) != len(t) {
				seqs, tags = append(seqs, d), append(tags, "(matched after trimming blanks + collapsing doubled rows)")
			}
		}
		if d := dedup(st.vals); len(d) >= 4 && len(d) != len(st.vals) {
			seqs, tags = append(seqs, d), append(tags, "(matched after collapsing doubled rows)")
		}
		found := false
		for si, seq := range seqs {
			if off, ok, rev := find(seq); ok {
				tag := tags[si]
				if rev {
					tag += "(stored reversed)"
				}
				fmt.Fprintf(&rep, "%s %s\n", fmtRange(off, len(seq)), tag)
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
			fmt.Fprintf(&rep, "no direct ROM match (computed or transformed data)\n  values (first 32, blanks trimmed): %v\n", show)
		}
	}

	// --- 音声採譜（任意）: レジスタをフレーム粒度でサンプル → jingle 記法へ ---
	if audioN > 0 {
		music, err := transcribe(rom, audioN)
		if err != nil {
			return err
		}
		rep.WriteString(music)
	}

	// --- distella 注釈付き逆アセンブル（任意・2K/4K のみ）---
	if distella != "" && banked {
		fmt.Fprintf(&rep, "\n(distella annotation skipped: DiStella v2.10 supports 2K/4K only; this is a %dK banked cart)\n",
			len(romBytes)/1024)
		distella = ""
	}
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
						tag = fmt.Sprintf("(label+%d = from $%04X)", d, addr)
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
			fmt.Fprintf(&rep, "\n(distella disassembly: disassembly.asm — %d dissect annotations)\n", hits)
		} else {
			fmt.Fprintf(&rep, "\n(distella failed: %v — trace matching only)\n", err)
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

// transcribe は ROM をリセットから audioN フレーム走らせ、TIA 音声レジスタの状態系列から
// 各チャンネルのメロディを jingle 記法（"D6:20 R:6 ..."）へ採譜する。
// レジスタ上は同音の連続（レガート）と 1 音の持続が区別できないため、同音連続は 1 音に併合される
// （音響的には同一）。
func transcribe(rom string, audioN int) (string, error) {
	e, err := emu.New("AUTO")
	if err != nil {
		return "", err
	}
	if err := e.LoadROM(rom); err != nil {
		return "", err
	}
	states := make([]emu.AudioState, 0, audioN)
	for i := 0; i < audioN; i++ {
		if err := e.RunFrames(1); err != nil {
			return "", err
		}
		states = append(states, e.ReadAudio())
	}

	var b strings.Builder
	fmt.Fprintf(&b, "\n=== music (audio register trace, %d frames) ===\n", audioN)
	for ch := 0; ch < 2; ch++ {
		get := func(st emu.AudioState) emu.AudioChannel {
			if ch == 0 {
				return st.Channel0
			}
			return st.Channel1
		}
		type ev struct {
			c, f, v uint8
			frames  int
		}
		var evs []ev
		for _, st := range states {
			a := get(st)
			key := a
			if a.Volume == 0 {
				key = emu.AudioChannel{} // 休符は (c,f) を無視して併合
			}
			if n := len(evs); n > 0 && evs[n-1].c == key.Control && evs[n-1].f == key.Freq && evs[n-1].v == key.Volume {
				evs[n-1].frames++
				continue
			}
			evs = append(evs, ev{key.Control, key.Freq, key.Volume, 1})
		}
		// 前後の休符をトリム
		for len(evs) > 0 && evs[0].v == 0 {
			evs = evs[1:]
		}
		for len(evs) > 0 && evs[len(evs)-1].v == 0 {
			evs = evs[:len(evs)-1]
		}
		if len(evs) == 0 {
			fmt.Fprintf(&b, "channel %d: silent\n", ch)
			continue
		}
		ctrls := map[uint8]bool{}
		vols := map[uint8]bool{}
		for _, e := range evs {
			if e.v > 0 {
				ctrls[e.c] = true
				vols[e.v] = true
			}
		}
		var hdr []string
		for c := range ctrls {
			hdr = append(hdr, fmt.Sprintf("AUDC=%d %s", c, audio.Name(int(c))))
		}
		sort.Strings(hdr)
		fmt.Fprintf(&b, "channel %d (%s, %d volume level(s)):\n  ", ch, strings.Join(hdr, " / "), len(vols))
		uniq := map[string]string{}
		curCtrl := uint8(255)
		for i, e := range evs {
			if i > 0 {
				b.WriteString(" ")
			}
			if e.v == 0 {
				fmt.Fprintf(&b, "R:%d", e.frames)
				continue
			}
			name, cents := audio.NearestNote(audio.Freq(int(e.c), int(e.f), audio.BaseClockNTSC))
			if name == "" {
				name = fmt.Sprintf("?f%d", e.f)
			}
			if len(ctrls) > 1 && e.c != curCtrl {
				fmt.Fprintf(&b, "[AUDC=%d]", e.c)
				curCtrl = e.c
			}
			fmt.Fprintf(&b, "%s:%d", name, e.frames)
			uniq[name] = fmt.Sprintf("AUDF=%d, %+.1f cents", e.f, cents)
		}
		b.WriteString("\n  notes used: ")
		var keys []string
		for k := range uniq {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for i, k := range keys {
			if i > 0 {
				b.WriteString(", ")
			}
			fmt.Fprintf(&b, "%s (%s)", k, uniq[k])
		}
		b.WriteString("\n")
	}
	return b.String(), nil
}
