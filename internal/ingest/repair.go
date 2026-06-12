package ingest

// 重なり修復（sprite-guided PF inpainting, M7）。
// スプライトが playfield に重なった行では、局所的には画素の所属（PF か sprite か）が決定不能。
// しかし同じ PF 構造が画面内で繰り返すなら（ビル群・タイル・HUD 枠）、クリーンな参照バンドとの
// 差分で一意に復元できる：
//   - 参照より「余分に lit」な列 = スプライト画素が PF に化けたもの → residual（sprite 層）へ
//   - 参照より「欠けている」列 = スプライトが覆った PF bit → 参照どおりに復元
// 参照が見つからない・差分がスプライト範囲外に及ぶ場合は触らない（誤修復より無修復）。

// RepairOverlaps は bands/residual を**その場で**修復し、修復が起きたかを返す。
func RepairOverlaps(n *Normalized, bands []PFBand, residual [][]bool, sprites []Sprite) bool {
	changed := false
	for _, s := range sprites {
		c0 := s.X/4 - 1
		if c0 < 0 {
			c0 = 0
		}
		c1 := (s.X+s.W-1)/4 + 1
		if c1 > 39 {
			c1 = 39
		}
		for bi := range bands {
			b := &bands[bi]
			if b.Top+b.Height <= s.Y || b.Top >= s.Y+s.H {
				continue // 行が重ならない
			}
			if b.Height > 4 && b.Confidence >= 1.0 && b.Mode != "asymmetric" {
				continue // 厚い・クリーン・対称＝汚染の徴候なし
			}
			ref := findReference(bands, bi, c0, c1)
			if ref < 0 {
				continue
			}
			if repairBand(n, b, &bands[ref], residual, c0, c1) {
				changed = true
			}
		}
	}
	return changed
}

// findReference は「スプライト列範囲の外側が完全一致する」最も信頼できる別バンドを探す。
func findReference(bands []PFBand, bi, c0, c1 int) int {
	target := decodeBand(bands[bi])
	best := -1
	for i := range bands {
		if i == bi || bands[i].Confidence < 1.0 || bands[i].Height < bands[bi].Height {
			continue
		}
		cand := decodeBand(bands[i])
		ok := true
		diffInside := false
		for c := 0; c < 40; c++ {
			if c >= c0 && c <= c1 {
				if cand[c] != target[c] {
					diffInside = true
				}
				continue
			}
			if cand[c] != target[c] {
				ok = false
				break
			}
		}
		if !ok || !diffInside {
			continue // 外側不一致 or 差分なし（修復不要）
		}
		if best < 0 || bands[i].Height > bands[best].Height {
			best = i
		}
	}
	return best
}

// repairBand は B を参照 M のパターンへ修復し、差分画素を正しい層へ再帰属する。
func repairBand(n *Normalized, b, m *PFBand, residual [][]bool, c0, c1 int) bool {
	bitsB := decodeBand(*b)
	bitsM := decodeBand(*m)
	for y := b.Top; y < b.Top+b.Height && y < n.Height; y++ {
		for c := c0; c <= c1; c++ {
			if bitsB[c] == bitsM[c] {
				continue
			}
			pfCol := m.ColorLeft
			if c >= 20 {
				pfCol = m.ColorRight
			}
			for dx := 0; dx < 4; dx++ {
				x := c*4 + dx
				px := n.Codes[y][x]
				if bitsB[c] && !bitsM[c] {
					// 汚染列: PF に化けていたスプライト画素 → residual へ
					if px != pfCol {
						residual[y][x] = true
					}
				} else {
					// 欠け列: PF bit を復元。PF 色の画素は residual から除く（部分列の取り込み戻し）、
					// スプライト色の画素は residual のまま（覆っているのはスプライト）。
					if px == pfCol {
						residual[y][x] = false
					} else if px != 0 {
						residual[y][x] = true
					}
				}
			}
		}
	}
	// バンドを参照パターン・参照色で置換（汚染色を引き継がない）
	b.Mode, b.PF0, b.PF1, b.PF2 = m.Mode, m.PF0, m.PF1, m.PF2
	b.PF0B, b.PF1B, b.PF2B = m.PF0B, m.PF1B, m.PF2B
	b.ColorLeft, b.ColorRight, b.ScoreMode = m.ColorLeft, m.ColorRight, m.ScoreMode
	b.Confidence = 0.9
	b.Repaired = true
	return true
}
