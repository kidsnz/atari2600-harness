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
	comps := collectComponents(n, residual)
	comps = mergeFragments(comps)
	var sprites []Sprite
	for _, c := range comps {
		if len(c.px) < 2 {
			continue // マージ後も孤立している 1px は量子化ノイズとして捨てる
		}
		sprites = append(sprites, classify(n, c.px, c.minX, c.minY, c.maxX, c.maxY))
	}
	return mergeCopies(sprites)
}

// component は連結成分（マージ前の生の塊）。
type component struct {
	px                     [][2]int
	minX, minY, maxX, maxY int
	colors                 map[uint8]bool
}

func collectComponents(n *Normalized, residual [][]bool) []component {
	h := n.Height
	visited := make([][]bool, h)
	for y := range visited {
		visited[y] = make([]bool, tiaWidth)
	}
	var comps []component
	for y := 0; y < h; y++ {
		for x := 0; x < tiaWidth; x++ {
			if !residual[y][x] || visited[y][x] {
				continue
			}
			c := component{minX: x, maxX: x, minY: y, maxY: y, colors: map[uint8]bool{}}
			stack := [][2]int{{x, y}}
			visited[y][x] = true
			for len(stack) > 0 {
				p := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				c.px = append(c.px, p)
				c.colors[n.Codes[p[1]][p[0]]] = true
				if p[0] < c.minX {
					c.minX = p[0]
				}
				if p[0] > c.maxX {
					c.maxX = p[0]
				}
				if p[1] < c.minY {
					c.minY = p[1]
				}
				if p[1] > c.maxY {
					c.maxY = p[1]
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
			comps = append(comps, c)
		}
	}
	return comps
}

// mergeFragments は「bbox の間隙 ≤2px かつ 色集合が交差」する成分を安定するまで統合する。
// 配達員の離れた手・キャブの車輪・桁の断片など、1 オブジェクトの分裂を 1 つに戻す（F1）。
func mergeFragments(comps []component) []component {
	for {
		merged := false
		for i := 0; i < len(comps) && !merged; i++ {
			for j := i + 1; j < len(comps) && !merged; j++ {
				if gap(comps[i], comps[j]) <= 2 && colorsIntersect(comps[i].colors, comps[j].colors) {
					comps[i] = fuse(comps[i], comps[j])
					comps = append(comps[:j], comps[j+1:]...)
					merged = true
				}
			}
		}
		if !merged {
			return comps
		}
	}
}

// gap は 2 つの bbox 間のチェビシェフ距離（重なれば 0）。
func gap(a, b component) int {
	dx := 0
	if b.minX > a.maxX {
		dx = b.minX - a.maxX - 1
	} else if a.minX > b.maxX {
		dx = a.minX - b.maxX - 1
	}
	dy := 0
	if b.minY > a.maxY {
		dy = b.minY - a.maxY - 1
	} else if a.minY > b.maxY {
		dy = a.minY - b.maxY - 1
	}
	if dx > dy {
		return dx
	}
	return dy
}

func colorsIntersect(a, b map[uint8]bool) bool {
	for c := range a {
		if b[c] {
			return true
		}
	}
	return false
}

func fuse(a, b component) component {
	a.px = append(a.px, b.px...)
	for c := range b.colors {
		a.colors[c] = true
	}
	if b.minX < a.minX {
		a.minX = b.minX
	}
	if b.maxX > a.maxX {
		a.maxX = b.maxX
	}
	if b.minY < a.minY {
		a.minY = b.minY
	}
	if b.maxY > a.maxY {
		a.maxY = b.maxY
	}
	return a
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
