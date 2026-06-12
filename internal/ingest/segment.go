package ingest

import (
	"github.com/kidsnz/atari2600-harness/pkg/playfield"
)

// PFBand は playfield の縦バンド（同一パターン・同一色の連続行）。
type PFBand struct {
	Top    int    `json:"top"`    // 画像相対の開始行
	Height int    `json:"height"` // 行数
	Mode   string `json:"mode"`   // "repeat" | "reflect" | "asymmetric"
	// 対称（repeat/reflect）時: 左半 20 列ぶんの 3 バイト（CTRLPF D0 でどちらにも見える）
	PF0 uint8 `json:"pf0"`
	PF1 uint8 `json:"pf1"`
	PF2 uint8 `json:"pf2"`
	// 非対称時のみ: 右半の 3 バイト（左半は上の PF0-2）
	PF0B uint8 `json:"pf0b,omitempty"`
	PF1B uint8 `json:"pf1b,omitempty"`
	PF2B uint8 `json:"pf2b,omitempty"`
	// 色（TIA コード）。左右が違えば score-mode の徴候。
	ColorLeft  uint8 `json:"color_left"`
	ColorRight uint8 `json:"color_right"`
	ScoreMode  bool  `json:"score_mode"` // 左右同パターン・別色
	// 確信度: 1.0=全列が綺麗に 4clk 整列・単色。重なり等で列を捨てた行があると下がる。
	Confidence float64 `json:"confidence"`
	Repaired   bool    `json:"repaired,omitempty"` // M7: 参照バンドから重なり修復済み
	// R3: 行中 COLUPF 書換（timed write）。点灯列の色が途中で変わるバンドはこれで忠実に表現する
	//（Pitfall の林冠フリンジ等）。先頭要素は clock0 の初期色（=ColorLeft）。レジスタ意味論に忠実。
	ColorWrites []ColorWrite `json:"color_writes,omitempty"`
}

// ColorWrite は可視 clock 位置での COLUPF 書換。
type ColorWrite struct {
	Clock int `json:"clock"`
	Color int `json:"color"`
}

// rowPF は 1 行ぶんの PF 解析結果。
type rowPF struct {
	bits     [40]bool
	colColor [40]uint8 // 列毎の色（文脈降格の同色判定用）
	colorL   uint8
	colorR   uint8
	lit      bool    // 1 列でも点灯あり
	conf     float64 // この行の確信度
}

// pfMisaligned は「非背景なのに 4clk 整列・単色の列として読めなかった」ピクセル数を返しつつ
// 1 行を PF 列に畳む。読めなかったピクセルはスプライト層（M3）の担当。
func analyzeRowPF(codes []uint8, bg uint8) (r rowPF, residual int) {
	colColor := map[int]uint8{}
	for c := 0; c < 40; c++ {
		x0 := c * 4
		col := codes[x0]
		uniform := true
		for dx := 1; dx < 4; dx++ {
			if codes[x0+dx] != col {
				uniform = false
				break
			}
		}
		if col == bg {
			if !uniform {
				// 列の途中から非背景＝非整列ピクセル → residual
				for dx := 0; dx < 4; dx++ {
					if codes[x0+dx] != bg {
						residual++
					}
				}
			}
			continue
		}
		if !uniform {
			for dx := 0; dx < 4; dx++ {
				if codes[x0+dx] != bg {
					residual++
				}
			}
			continue
		}
		r.bits[c] = true
		colColor[c] = col
		r.colColor[c] = col
		r.lit = true
	}
	// 半面ごとの色（最頻）。混色は確信度を下げる（重なり物が PF 色を侵食しているサイン）。
	r.colorL, r.conf = halfColor(colColor, 0, 20, 1.0)
	r.colorR, r.conf = halfColor(colColor, 20, 40, r.conf)
	return r, residual
}

func halfColor(colColor map[int]uint8, from, to int, conf float64) (uint8, float64) {
	counts := map[uint8]int{}
	total := 0
	var best uint8
	bestN := 0
	for c := from; c < to; c++ {
		if col, ok := colColor[c]; ok {
			counts[col]++
			total++
			if counts[col] > bestN {
				best, bestN = col, counts[col]
			}
		}
	}
	if total > 0 && bestN < total {
		conf *= float64(bestN) / float64(total)
	}
	return best, conf
}

// ExtractPlayfield は正規化画面から PF バンド列を抽出する。
// residualMask[y][x] = true は「PF でも背景でもない」ピクセル（M3 スプライト層の入力）。
//
// 背景推定: まず画像全体の最頻色（グローバル背景）。各行にそれが十分（16px 以上）あれば
// その行の背景として採用し、無ければ行内最頻色（per-scanline COLUBK 変化に対応）。
// 行内最頻だけだと、塗りが 50% を超える行（密な山など）で図と地が反転する（実測で発見）。
// なお「行全体が単一色」は原理的に背景か全面 PF か決定不能 → 背景扱い（doc 参照）。
func ExtractPlayfield(n *Normalized) (bands []PFBand, residualMask [][]bool, rowBG []uint8) {
	globalBG := globalMode(n.Codes)
	residualMask = make([][]bool, n.Height)
	rowBG = make([]uint8, n.Height)
	rows := make([]rowPF, n.Height)
	for y := 0; y < n.Height; y++ {
		bg := rowMode(n.Codes[y])
		if cnt := countCode(n.Codes[y], globalBG); cnt >= 16 {
			bg = globalBG
		}
		rowBG[y] = bg
		r, _ := analyzeRowPF(n.Codes[y], bg)
		rows[y] = r
		// residual マスク（PF 列に取り込まれなかった非背景ピクセル）
		residualMask[y] = make([]bool, tiaWidth)
		for x := 0; x < tiaWidth; x++ {
			c := x / 4
			if n.Codes[y][x] != bg && !r.bits[c] {
				residualMask[y][x] = true
			}
		}
	}
	// バンド圧縮
	y := 0
	for y < n.Height {
		if !rows[y].lit {
			y++
			continue
		}
		top := y
		for y < n.Height && rows[y].lit && rows[y].bits == rows[top].bits &&
			rows[y].colorL == rows[top].colorL && rows[y].colorR == rows[top].colorR {
			y++
		}
		b := makeBand(rows[top], top, y-top)
		// 調停1: 高さ≤2 かつ点灯列≤2 の極小バンドは「4clk 整列したスプライト」の公算が高い。
		// 調停2（文脈つき・列単位）: 高さ≤2 のバンドで、直上/直下の行に同色 residual が縦に
		// 接している**列だけ**をスプライト層へ降格（バンド丸ごと降格すると、汚染と無関係な
		// クリーン列まで巻き込む——合成オーバーラップテストで発覚）。
		// 全点灯列が接触していれば従来どおり全降格（スコア桁の水平ストローク）。
		demote := b.Height <= 2 && litCols(rows[top].bits) <= 2
		if !demote && b.Height <= 2 {
			contact := contactColumns(n, residualMask, rows[top], top, b.Height)
			if len(contact) > 0 {
				if len(contact) == litCols(rows[top].bits) {
					demote = true
				} else {
					// 部分降格: 接触列の画素を residual へ移し、残りでバンドを作り直す
					r2 := rows[top]
					for c := range contact {
						for yy := top; yy < y; yy++ {
							for dx := 0; dx < 4; dx++ {
								x := c*4 + dx
								if n.Codes[yy][x] != rowBG[yy] {
									residualMask[yy][x] = true
								}
							}
						}
						r2.bits[c] = false
					}
					b = makeBand(r2, top, y-top)
				}
			}
		}
		if demote {
			for yy := top; yy < top+b.Height; yy++ {
				for c := 0; c < 40; c++ {
					if rows[top].bits[c] {
						for dx := 0; dx < 4; dx++ {
							residualMask[yy][c*4+dx] = true
						}
					}
				}
			}
			continue
		}
		bands = append(bands, b)
	}
	return bands, residualMask, rowBG
}

// contactColumns は薄バンドの点灯列のうち「上下の行に同色の residual が縦に接している」
// 列の集合（＝スプライトの水平ストロークである強い徴候を列単位で返す）。
func contactColumns(n *Normalized, residual [][]bool, r rowPF, top, height int) map[int]bool {
	out := map[int]bool{}
	for c := 0; c < 40; c++ {
		if !r.bits[c] {
			continue
		}
		col := r.colColor[c]
		for _, yy := range []int{top - 1, top + height} {
			if yy < 0 || yy >= n.Height {
				continue
			}
			for dx := 0; dx < 4; dx++ {
				x := c*4 + dx
				if residual[yy][x] && n.Codes[yy][x] == col {
					out[c] = true
				}
			}
		}
	}
	return out
}

func litCols(bits [40]bool) int {
	n := 0
	for _, b := range bits {
		if b {
			n++
		}
	}
	return n
}

func makeBand(r rowPF, top, height int) PFBand {
	b := PFBand{Top: top, Height: height, ColorLeft: r.colorL, ColorRight: r.colorR, Confidence: r.conf}
	// 点灯列の色変化を timed write 列として抽出（R3）。半面1色なら writes 無し（後方互換）。
	var writes []ColorWrite
	var cur uint8
	started := false
	multi := false
	for c := 0; c < 40; c++ {
		if !r.bits[c] {
			continue
		}
		col := r.colColor[c]
		if !started {
			writes = append(writes, ColorWrite{Clock: 0, Color: int(col)})
			cur, started = col, true
			continue
		}
		if col != cur {
			writes = append(writes, ColorWrite{Clock: c * 4, Color: int(col)})
			cur = col
			multi = true
		}
	}
	// 半面境界のみの変化（score-mode 等）は ColorLeft/Right で表現済み → writes 不要
	if multi && !onlyHalfBoundaryChange(writes) {
		b.ColorWrites = writes
		if b.Confidence < 1.0 {
			b.Confidence = 0.95 // 多色はモデル化された＝混色ペナルティを緩和
		}
	}
	left := r.bits[:20]
	right := r.bits[20:]
	mirror := true
	same := true
	for i := 0; i < 20; i++ {
		if right[i] != left[i] {
			same = false
		}
		if right[i] != left[19-i] {
			mirror = false
		}
	}
	switch {
	case same:
		b.Mode = "repeat"
	case mirror:
		b.Mode = "reflect"
	default:
		b.Mode = "asymmetric"
	}
	if b.Mode == "asymmetric" {
		row := playfield.EncodeAsymmetric(boolSlice(r.bits[:]))
		b.PF0, b.PF1, b.PF2 = row.PF0A, row.PF1A, row.PF2A
		b.PF0B, b.PF1B, b.PF2B = row.PF0B, row.PF1B, row.PF2B
	} else {
		b.PF0, b.PF1, b.PF2 = playfield.EncodeSymmetric(boolSlice(left))
	}
	b.ScoreMode = b.Mode == "repeat" && r.colorL != r.colorR && r.colorL != 0 && r.colorR != 0
	return b
}

func boolSlice(b []bool) []bool { return b }

// onlyHalfBoundaryChange は「色変化が clock80 の半面境界 1 箇所だけ」か（=Left/Right で表現可能）。
func onlyHalfBoundaryChange(w []ColorWrite) bool {
	if len(w) != 2 {
		return false
	}
	return w[1].Clock == 80
}

// globalMode は画像全体の最頻色。
func globalMode(codes [][]uint8) uint8 {
	counts := map[uint8]int{}
	var best uint8
	bestN := 0
	for _, row := range codes {
		for _, c := range row {
			counts[c]++
			if counts[c] > bestN {
				best, bestN = c, counts[c]
			}
		}
	}
	return best
}

func countCode(row []uint8, code uint8) int {
	n := 0
	for _, c := range row {
		if c == code {
			n++
		}
	}
	return n
}

// rowMode は行の最頻色（背景推定。COLUBK は行毎に変わり得る）。
func rowMode(codes []uint8) uint8 {
	counts := map[uint8]int{}
	var best uint8
	bestN := 0
	for _, c := range codes {
		counts[c]++
		if counts[c] > bestN {
			best, bestN = c, counts[c]
		}
	}
	return best
}
