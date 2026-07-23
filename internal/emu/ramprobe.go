package emu

import (
	"fmt"
	"image"
	"sort"
)

// --- RAM semantics probe (C1) ---
//
// 「$XX を書き換えると画面のどこがどう動くか」を総当たりで測る。ソースの無い商用 ROM から
// RAM の意味（このバイトはスプライトの X 座標か／Y 座標か／見た目か／画面に出ないか）を
// **推測でなく実測で**引き出すための道具。casebook（商用ゲームを逆解析して学ぶ）の入口。
//
// 出典: 手法そのものは kisonecat/deep-atari の play.py（ALE の setRAM で Freeway の
// $B3=鶏Y・$CD..$D7=6車線の車X を叩いて画面を見る）を一般化したもの。
// https://github.com/kisonecat/deep-atari （2022-02-15, GPL 系ではない個人リポジトリ・手法のみ参照）
//
// 手続き（SaveState/RestoreState があるので **完全に非破壊**。呼び出し後、機械は開始位置に戻る）:
//
//	S0 = SaveState()
//	base = restore(S0) → frames 進める → 画面
//	各 addr・各 value: restore(S0) → poke(addr,value) → frames 進める → 画面を base と差分
//	restore(S0)
//
// 判定は「変化した画素の重心が probe 値に対してどれだけ動いたか」。X 座標バイトなら重心が
// 横に流れ、Y 座標バイトなら縦に流れる。重心は「変化領域（旧位置∪新位置）」の重心なので
// オブジェクトの絶対位置そのものではないが、**value に対して単調に動く**ので位置バイトの
// 判別には十分。数値は全サンプル返すので、分類を信じずに生データで確認できる。

// ProbeOptions は probe_ram_semantics の入力。
type ProbeOptions struct {
	Addrs  []uint16 // 空なら $80..$FF 全部
	Values []uint8  // 空なら DefaultProbeValues
	Frames int      // poke 後に進めるフレーム数（既定 1）
	MinPix int      // これ未満の変化画素数はノイズとして "none" 扱い（既定 1）
}

// DefaultProbeValues は既定の探り値。0..255 をほぼ等間隔に刻む（座標バイトなら画面上を
// 端から端まで動くので重心の傾きが出る）。
var DefaultProbeValues = []uint8{0x00, 0x30, 0x60, 0x90, 0xC0, 0xF0}

// ProbeSample は 1 (addr,value) の実測 1 点。
type ProbeSample struct {
	Value   uint8   `json:"value"`
	Changed int     `json:"changed_pixels"`
	BBox    [4]int  `json:"bbox"` // x0,y0,x1,y1（変化画素の外接矩形・変化なしは全0）
	CX      float64 `json:"centroid_x"`
	CY      float64 `json:"centroid_y"`
}

// ProbeResult は 1 アドレス分の結論。
type ProbeResult struct {
	Addr       uint16        `json:"addr"`
	AddrHex    string        `json:"addr_hex"`
	Class      string        `json:"class"` // x_position | y_position | appearance | none
	MaxChanged int           `json:"max_changed_pixels"`
	SpanX      float64       `json:"centroid_span_x"` // 重心 X の振れ幅（probe 値をまたいだ最大−最小）
	SpanY      float64       `json:"centroid_span_y"`
	Samples    []ProbeSample `json:"samples"`
}

// ProbeRAMSemantics は addrs×values を総当たりし、各アドレスが画面に及ぼす作用を実測する。
// 実行後、エミュレータは呼び出し時点の状態に戻る（非破壊）。
func (e *Emu) ProbeRAMSemantics(opt ProbeOptions) ([]ProbeResult, error) {
	addrs := opt.Addrs
	if len(addrs) == 0 {
		for a := uint16(0x80); a <= 0xFF; a++ {
			addrs = append(addrs, a)
		}
	}
	values := opt.Values
	if len(values) == 0 {
		values = DefaultProbeValues
	}
	frames := opt.Frames
	if frames <= 0 {
		frames = 1
	}
	minPix := opt.MinPix
	if minPix <= 0 {
		minPix = 1
	}

	s0 := e.SaveState()
	defer e.RestoreState(s0) // 何が起きても開始状態へ戻す

	if err := e.RestoreState(s0); err != nil {
		return nil, err
	}
	if err := e.RunFrames(frames); err != nil {
		return nil, fmt.Errorf("probe baseline: %w", err)
	}
	base, _ := e.Snapshot()

	out := make([]ProbeResult, 0, len(addrs))
	for _, a := range addrs {
		if a < 0x80 || a > 0xFF {
			return nil, fmt.Errorf("probe: address $%04X outside user RAM $80-$FF", a)
		}
		r := ProbeResult{Addr: a, AddrHex: fmt.Sprintf("$%02X", a), Class: "none"}
		minCX, maxCX := 0.0, 0.0
		minCY, maxCY := 0.0, 0.0
		seen := false

		for _, v := range values {
			if err := e.RestoreState(s0); err != nil {
				return nil, err
			}
			if err := e.Poke(a, v); err != nil {
				return nil, fmt.Errorf("probe poke $%02X: %w", a, err)
			}
			if err := e.RunFrames(frames); err != nil {
				return nil, fmt.Errorf("probe run $%02X=$%02X: %w", a, v, err)
			}
			img, _ := e.Snapshot()
			s := diffSample(base, img)
			s.Value = v
			r.Samples = append(r.Samples, s)

			if s.Changed < minPix {
				continue
			}
			if s.Changed > r.MaxChanged {
				r.MaxChanged = s.Changed
			}
			if !seen {
				minCX, maxCX, minCY, maxCY, seen = s.CX, s.CX, s.CY, s.CY, true
				continue
			}
			minCX, maxCX = minf(minCX, s.CX), maxf(maxCX, s.CX)
			minCY, maxCY = minf(minCY, s.CY), maxf(maxCY, s.CY)
		}

		if seen {
			r.SpanX, r.SpanY = maxCX-minCX, maxCY-minCY
			r.Class = classify(r.SpanX, r.SpanY)
		}
		out = append(out, r)
	}

	sort.SliceStable(out, func(i, j int) bool { return out[i].MaxChanged > out[j].MaxChanged })
	return out, e.RestoreState(s0)
}

// classify は重心の振れ方から役割を当てる。閾値の根拠: 2600 のスプライトは 8px 幅なので、
// 重心が 8px 以上流れれば「同じ絵が別の場所に出た」= 位置バイトと見なせる。片軸がもう一方の
// 2 倍以上動いたときだけ軸を断定し、そうでなければ位置と言い切らない（"appearance"）。
func classify(spanX, spanY float64) string {
	const move = 8.0
	switch {
	case spanX >= move && spanX > 2*spanY:
		return "x_position"
	case spanY >= move && spanY > 2*spanX:
		return "y_position"
	default:
		return "appearance"
	}
}

// diffSample は 2 フレームの差分画素を数え、外接矩形と重心を返す。
func diffSample(a, b *image.RGBA) ProbeSample {
	var s ProbeSample
	w := a.Bounds().Dx()
	h := a.Bounds().Dy()
	if b.Bounds().Dx() != w || b.Bounds().Dy() != h {
		return s // 解像度が変わった（フレーム仕様の変化）= 比較不能。変化0として扱う
	}
	x0, y0, x1, y1 := w, h, -1, -1
	var sx, sy float64
	for y := 0; y < h; y++ {
		ra := a.PixOffset(a.Bounds().Min.X, a.Bounds().Min.Y+y)
		rb := b.PixOffset(b.Bounds().Min.X, b.Bounds().Min.Y+y)
		for x := 0; x < w; x++ {
			i, j := ra+x*4, rb+x*4
			if a.Pix[i] == b.Pix[j] && a.Pix[i+1] == b.Pix[j+1] && a.Pix[i+2] == b.Pix[j+2] {
				continue
			}
			s.Changed++
			sx += float64(x)
			sy += float64(y)
			if x < x0 {
				x0 = x
			}
			if x > x1 {
				x1 = x
			}
			if y < y0 {
				y0 = y
			}
			if y > y1 {
				y1 = y
			}
		}
	}
	if s.Changed == 0 {
		return s
	}
	s.BBox = [4]int{x0, y0, x1, y1}
	s.CX = sx / float64(s.Changed)
	s.CY = sy / float64(s.Changed)
	return s
}

func minf(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func maxf(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
