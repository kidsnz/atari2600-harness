package ingest

// Sprite は PF でも背景でもないピクセルの連結成分から推定したオブジェクト。
type Sprite struct {
	X      int    `json:"x"`      // 最左 clock（バウンディングボックス）
	Y      int    `json:"y"`      // 画像相対の最上行
	W      int    `json:"w"`
	H      int    `json:"h"`
	Kind   string `json:"kind"`   // "player" | "missile_or_ball" | "large_object"
	// []int なのは MCP の structured output 対策（Go の []uint8=[]byte は base64 文字列になる）
	GRP    []int `json:"grp,omitempty"`        // player のとき: 行毎 GRP（pkg/sprite ビット順、X 起点 8px 窓）
	Colors []int `json:"row_colors,omitempty"` // 行毎の色（TIA コード）。2600 スプライトは行毎多色が普通
	Copies  int    `json:"copies"`            // NUSIZ 等間隔コピー（1=単独）
	Spacing int    `json:"spacing,omitempty"` // copies>1 のときの clock 間隔（16/32/64）
	Confidence float64 `json:"confidence"`
}

// ExtractSprites は residual マスク（PF/背景以外）から連結成分を拾い分類する。
// 注意（doc 参照）: スクショ1枚は1フレームの真実。flicker 物は写っている分しか出ない。
// 窓 8px の anchoring はバウンディングボックス基準＝絵の左端が GRP の D7 に来るとは限らない
//（絵がスプライト窓のどこに置かれていたかは画像から決定不能）。
func ExtractSprites(n *Normalized, residual [][]bool) []Sprite {
	h := n.Height
	visited := make([][]bool, h)
	for y := range visited {
		visited[y] = make([]bool, tiaWidth)
	}
	var sprites []Sprite
	for y := 0; y < h; y++ {
		for x := 0; x < tiaWidth; x++ {
			if !residual[y][x] || visited[y][x] {
				continue
			}
			// 8近傍 flood fill
			minX, maxX, minY, maxY := x, x, y, y
			stack := [][2]int{{x, y}}
			visited[y][x] = true
			var px [][2]int
			for len(stack) > 0 {
				p := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				px = append(px, p)
				if p[0] < minX {
					minX = p[0]
				}
				if p[0] > maxX {
					maxX = p[0]
				}
				if p[1] < minY {
					minY = p[1]
				}
				if p[1] > maxY {
					maxY = p[1]
				}
				for dy := -1; dy <= 1; dy++ {
					for dx := -1; dx <= 1; dx++ {
						nx, ny := p[0]+dx, p[1]+dy
						if nx < 0 || nx >= tiaWidth || ny < 0 || ny >= h {
							continue
						}
						if residual[ny][nx] && !visited[ny][nx] {
							visited[ny][nx] = true
							stack = append(stack, [2]int{nx, ny})
						}
					}
				}
			}
			if len(px) < 2 {
				continue // 1px のゴミは捨てる（量子化ノイズ対策）
			}
			sprites = append(sprites, classify(n, px, minX, minY, maxX, maxY))
		}
	}
	return mergeCopies(sprites)
}

func classify(n *Normalized, px [][2]int, minX, minY, maxX, maxY int) Sprite {
	s := Sprite{X: minX, Y: minY, W: maxX - minX + 1, H: maxY - minY + 1, Copies: 1, Confidence: 1.0}
	lit := map[[2]int]bool{}
	for _, p := range px {
		lit[p] = true
	}
	switch {
	case s.W <= 4 && solidColumns(lit, minX, minY, maxX, maxY):
		s.Kind = "missile_or_ball"
	case s.W <= 8:
		s.Kind = "player"
	default:
		s.Kind = "large_object" // PF 非整列の大物 or 拡大 NUSIZ。確定はユーザーと
		s.Confidence = 0.5
	}
	if s.Kind == "player" || s.Kind == "missile_or_ball" {
		for y := minY; y <= maxY; y++ {
			var b uint8
			counts := map[uint8]int{}
			var rowCol uint8
			bestN := 0
			for dx := 0; dx < 8 && minX+dx < tiaWidth; dx++ {
				if lit[[2]int{minX + dx, y}] {
					b |= 1 << (7 - uint(dx)) // pkg/sprite と同じ col0→D7
					c := n.Codes[y][minX+dx]
					counts[c]++
					if counts[c] > bestN {
						rowCol, bestN = c, counts[c]
					}
				}
			}
			s.GRP = append(s.GRP, int(b))
			s.Colors = append(s.Colors, int(rowCol))
		}
	}
	return s
}

// solidColumns は全行が同じ細い縦棒か（missile/ball らしさ）。
func solidColumns(lit map[[2]int]bool, minX, minY, maxX, maxY int) bool {
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			if !lit[[2]int{x, y}] {
				return false
			}
		}
	}
	return true
}

// mergeCopies は「同じ行範囲・同じ絵・等間隔（16/32/64）」の並びを NUSIZ コピーとして 1 件に畳む。
func mergeCopies(in []Sprite) []Sprite {
	used := make([]bool, len(in))
	var out []Sprite
	for i := range in {
		if used[i] {
			continue
		}
		group := []int{i}
		for j := i + 1; j < len(in); j++ {
			if used[j] {
				continue
			}
			if in[j].Y == in[i].Y && in[j].H == in[i].H && in[j].Kind == in[i].Kind && sameGRP(in[i].GRP, in[j].GRP) {
				group = append(group, j)
			}
		}
		if len(group) >= 2 {
			// 等間隔チェック
			sp := in[group[1]].X - in[group[0]].X
			ok := sp == 16 || sp == 32 || sp == 64
			for k := 2; k < len(group) && ok; k++ {
				if in[group[k]].X-in[group[k-1]].X != sp {
					ok = false
				}
			}
			if ok {
				s := in[group[0]]
				s.Copies = len(group)
				s.Spacing = sp
				for _, g := range group {
					used[g] = true
				}
				out = append(out, s)
				continue
			}
		}
		used[i] = true
		out = append(out, in[i])
	}
	return out
}

func sameGRP(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
