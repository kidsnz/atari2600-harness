package ingest

// 逆描画＝忠実度メトリクス。抽出した report（背景行色＋PF バンド＋スプライト）から
// 160×H を再構成し、正規化入力と比較する。「綺麗に読めた」を数字にする装置。
// 自前 ROM のラウンドトリップは 100% でなければバグ（CI で assert）。
// 実画像（Stella 由来）はパレット量子化後の比較なので、残差は抽出漏れ＝改善対象を直接指す。

// RenderReport は report を 160×H の TIA 色コード平面へ逆描画する。
func RenderReport(rep *Report) [][]uint8 {
	h := rep.Height
	out := make([][]uint8, h)
	for y := 0; y < h; y++ {
		out[y] = make([]uint8, tiaWidth)
		bg := uint8(0)
		if y < len(rep.RowBG) {
			bg = uint8(rep.RowBG[y])
		}
		for x := 0; x < tiaWidth; x++ {
			out[y][x] = bg
		}
	}
	// PF バンド（ColorWrites があれば timed write を再生して列毎色を決める）
	for _, b := range rep.Playfield {
		bits := decodeBand(b)
		var colColors [40]uint8
		for c := 0; c < 40; c++ {
			if c < 20 {
				colColors[c] = b.ColorLeft
			} else {
				colColors[c] = b.ColorRight
			}
		}
		if len(b.ColorWrites) > 0 {
			cur := uint8(b.ColorWrites[0].Color)
			wi := 1
			for c := 0; c < 40; c++ {
				for wi < len(b.ColorWrites) && b.ColorWrites[wi].Clock <= c*4 {
					cur = uint8(b.ColorWrites[wi].Color)
					wi++
				}
				colColors[c] = cur
			}
		}
		for y := b.Top; y < b.Top+b.Height && y < h; y++ {
			for c := 0; c < 40; c++ {
				if !bits[c] {
					continue
				}
				for dx := 0; dx < 4; dx++ {
					out[y][c*4+dx] = colColors[c]
				}
			}
		}
	}
	// スプライト（PF より手前＝通常優先度を仮定）
	for _, s := range rep.Sprites {
		stretch := 1
		switch s.Kind {
		case "player_2x":
			stretch = 2
		case "player_4x":
			stretch = 4
		}
		for k := 0; k < s.Copies; k++ {
			x0 := s.X + k*s.Spacing
			for r, g := range s.GRP {
				y := s.Y + r
				if y < 0 || y >= h {
					continue
				}
				for bit := 0; bit < 8; bit++ {
					if g&(1<<(7-uint(bit))) == 0 {
						continue
					}
					for dx := 0; dx < stretch; dx++ {
						x := x0 + bit*stretch + dx
						if x >= 0 && x < tiaWidth {
							out[y][x] = uint8(s.Colors[r])
						}
					}
				}
			}
		}
	}
	return out
}

// decodeBand は PF バイトを 40 列へ展開（pkg/playfield の検証済みビット順の逆）。
func decodeBand(b PFBand) [40]bool {
	var bits [40]bool
	left := decodeHalf(b.PF0, b.PF1, b.PF2)
	copy(bits[:20], left[:])
	switch b.Mode {
	case "reflect":
		for i := 0; i < 20; i++ {
			bits[20+i] = left[19-i]
		}
	case "asymmetric":
		right := decodeHalf(b.PF0B, b.PF1B, b.PF2B)
		copy(bits[20:], right[:])
	default: // repeat
		copy(bits[20:], left[:])
	}
	return bits
}

func decodeHalf(pf0, pf1, pf2 uint8) [20]bool {
	var cells [20]bool
	for col := 0; col < 20; col++ {
		switch {
		case col <= 3:
			cells[col] = pf0&(1<<uint(4+col)) != 0
		case col <= 11:
			cells[col] = pf1&(1<<uint(11-col)) != 0
		default:
			cells[col] = pf2&(1<<uint(col-12)) != 0
		}
	}
	return cells
}

// Fidelity は再構成と正規化入力（量子化コード）の一致率（0..1）を返す。
func Fidelity(rep *Report, n *Normalized) float64 {
	rendered := RenderReport(rep)
	match, total := 0, 0
	for y := 0; y < n.Height; y++ {
		for x := 0; x < tiaWidth; x++ {
			total++
			if rendered[y][x] == n.Codes[y][x] {
				match++
			}
		}
	}
	if total == 0 {
		return 0
	}
	return float64(match) / float64(total)
}
