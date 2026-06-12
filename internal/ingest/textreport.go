package ingest

import (
	"fmt"
	"strings"
)

// 人間向けテキストレポート（pizzaboy_report.txt の形式を正式化）。
// ASCII アート＋TIA 色コード＋PF バンド表＋DASM 片を 1 ファイルに。

func artRow(g int, w int) string {
	s := fmt.Sprintf("%08b", uint8(g))
	if w < 8 {
		s = s[:w]
	}
	return strings.NewReplacer("0", ".", "1", "X").Replace(s)
}

// spriteArt はスプライト 1 体の ASCII（同一行は xN で圧縮、倍幅は伸ばす）。
func spriteArt(s Sprite, indent string) string {
	stretch := 1
	switch s.Kind {
	case "player_2x", "static_player_2x":
		stretch = 2
	case "player_4x", "static_player_4x":
		stretch = 4
	}
	var b strings.Builder
	i := 0
	for i < len(s.GRP) {
		j := i
		for j+1 < len(s.GRP) && s.GRP[j+1] == s.GRP[i] && s.Colors[j+1] == s.Colors[i] {
			j++
		}
		line := artRow(s.GRP[i], 8)
		if stretch > 1 {
			var sb strings.Builder
			for _, ch := range line {
				for k := 0; k < stretch; k++ {
					sb.WriteRune(ch)
				}
			}
			line = sb.String()
			if len(line) > s.W {
				line = line[:s.W]
			}
		}
		rep := "   "
		if j > i {
			rep = fmt.Sprintf(" x%d", j-i+1)
		}
		fmt.Fprintf(&b, "%s%s%s  $%02X\n", indent, line, rep, uint8(s.Colors[i]))
		i = j + 1
	}
	return b.String()
}

func spriteHeader(idx int, s Sprite) string {
	h := fmt.Sprintf("--- %d: %s x=%d y=%d %dx%d", idx, s.Kind, s.X, s.Y, s.W, s.H)
	if s.Copies > 1 {
		h += fmt.Sprintf(" copies=%d 間隔%d", s.Copies, s.Spacing)
	}
	if s.Shape > 0 {
		h += fmt.Sprintf(" shape=%d", s.Shape)
	}
	if s.Hint != "" {
		h += " " + s.Hint
	}
	if s.Confidence < 1 {
		h += fmt.Sprintf(" conf=%.2f", s.Confidence)
	}
	return h + " ---"
}

func bandRow(b PFBand) string {
	bits := decodeBand(b)
	var sb strings.Builder
	for _, lit := range bits {
		if lit {
			sb.WriteRune('#')
		} else {
			sb.WriteRune('.')
		}
	}
	return sb.String()
}

// TextReport は単一フレーム（または静的層）の人間向けレポート。
func TextReport(rep *Report, title string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s (fidelity %.1f%%)\n%s\n", title, rep.Fidelity*100, strings.Repeat("=", 60))
	for _, w := range rep.Warnings {
		fmt.Fprintf(&b, "WARN: %s\n", w)
	}
	fmt.Fprintf(&b, "\n=== sprites（%d 体）===\n", len(rep.Sprites))
	for i, s := range rep.Sprites {
		fmt.Fprintln(&b, spriteHeader(i, s))
		b.WriteString(spriteArt(s, "  "))
	}
	if len(rep.Groups) > 0 {
		fmt.Fprintf(&b, "\n=== groups ===\n")
		for _, g := range rep.Groups {
			fmt.Fprintf(&b, "  %s: members=%v at (%d,%d) %dx%d\n", g.Label, g.Members, g.X, g.Y, g.W, g.H)
		}
	}
	fmt.Fprintf(&b, "\n=== playfield（バンド毎 40列）===\n")
	for _, band := range rep.Playfield {
		mode := band.Mode
		if len(mode) > 4 {
			mode = mode[:4]
		}
		flag := ""
		if band.Repaired {
			flag = " repaired"
		}
		if band.ScoreMode {
			flag += " SCORE"
		}
		fmt.Fprintf(&b, "  y%3d h%2d %-4s pf=%02X/%02X/%02X colL=$%02X colR=$%02X %s%s\n",
			band.Top, band.Height, mode, band.PF0, band.PF1, band.PF2,
			band.ColorLeft, band.ColorRight, bandRow(band), flag)
	}
	fmt.Fprintf(&b, "\n=== DASM 片 ===\n%s\n%s", rep.PlayfieldASM, rep.SpritesASM)
	return b.String()
}

// TextReportMulti はマルチフレームの人間向けレポート（静的層＋フレーム毎動的層＋union）。
func TextReportMulti(mr *MultiReport, title string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s — マルチフレーム %d 枚 (unresolved %.2f%%)\n%s\n",
		title, mr.NumFrames, mr.UnresolvedShare*100, strings.Repeat("=", 60))
	for i, fr := range mr.Frames {
		fmt.Fprintf(&b, "frame %d: 動的スプライト %d 体, fidelity %.2f%%\n", i, len(fr.Sprites), fr.Fidelity*100)
	}
	fmt.Fprintf(&b, "\n############ 動的層（フレーム毎・動くもの＝本物のスプライト）############\n")
	for i, fr := range mr.Frames {
		if len(fr.Sprites) == 0 {
			continue
		}
		fmt.Fprintf(&b, "\n--- frame %d ---\n", i)
		for j, s := range fr.Sprites {
			fmt.Fprintln(&b, spriteHeader(j, s))
			b.WriteString(spriteArt(s, "  "))
		}
	}
	if len(mr.Union) > 0 {
		fmt.Fprintf(&b, "\n=== union（全フレーム通しのオブジェクト）===\n")
		for i, u := range mr.Union {
			fl := ""
			if u.Flicker {
				fl = " ※一部フレームのみ（flicker かアニメ姿勢差）"
			}
			fmt.Fprintf(&b, "  %d: %s %dx%d seen=%v%s\n", i, u.Kind, u.W, u.H, u.SeenFrames, fl)
		}
	}
	fmt.Fprintf(&b, "\n############ 静的層（動かないもの＝PF/背景/駐機物）############\n\n")
	b.WriteString(TextReport(mr.Static, "static layer"))
	return b.String()
}
