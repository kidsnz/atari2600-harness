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
}

// rowPF は 1 行ぶんの PF 解析結果。
type rowPF struct {
	bits   [40]bool
	colorL uint8
	colorR uint8
	lit    bool    // 1 列でも点灯あり
	conf   float64 // この行の確信度
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
func ExtractPlayfield(n *Normalized) (bands []PFBand, residualMask [][]bool) {
	globalBG := globalMode(n.Codes)
	residualMask = make([][]bool, n.Height)
	rows := make([]rowPF, n.Height)
	for y := 0; y < n.Height; y++ {
		bg := rowMode(n.Codes[y])
		if cnt := countCode(n.Codes[y], globalBG); cnt >= 16 {
			bg = globalBG
		}
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
		bands = append(bands, makeBand(rows[top], top, y-top))
	}
	return bands, residualMask
}

func makeBand(r rowPF, top, height int) PFBand {
	b := PFBand{Top: top, Height: height, ColorLeft: r.colorL, ColorRight: r.colorR, Confidence: r.conf}
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
