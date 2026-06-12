package ingest

import (
	"fmt"
	"image"
)

// マルチフレーム分離（M8）＝汎用精度の本丸。
// 同じシーンのスクショ N 枚（F12 連打）から:
//   静的層 = 画素毎の多数決（動かないもの＝PF/背景/駐機物）
//   動的層 = 各フレームの「静的層との差分」（動くもの＝スプライト）
// 単一画像の原理的限界（重なりの所属・flicker の欠け・縁飾りの誤分類）を、
// 参照パターンに頼らず原理的に解決する。N=2 は不一致画素が多数決できない（穴）ので
// N=3 推奨（穴は行背景で充填し unresolved 率として可視化する）。

// FrameResult は 1 フレームぶんの動的層解析。
type FrameResult struct {
	Sprites  []Sprite `json:"sprites"`
	Fidelity float64  `json:"fidelity"`
}

// UnionObject は全フレームを通したオブジェクト・トラック（位置連続性で連結）。
// 形が変わっても（アニメ）、動いても、近接（≤12px）かつ色が交差すれば同一トラック。
type UnionObject struct {
	Sprite           // 代表（最初の出現）
	SeenFrames []int `json:"seen_frames"`
	Poses      int   `json:"poses"`              // トラック内の異なる GRP 形の数（アニメ検出）
	Flicker    bool  `json:"flicker,omitempty"`  // 同位置でフレームを飛ばして明滅（真の flicker）
}

// MultiReport は静的層レポート＋フレーム毎の動的層＋union。
// （Report は埋め込みでなく named field: MCP の structured output スキーマ生成と相性が良い）
type MultiReport struct {
	Static          *Report       `json:"static"` // 静的層（playfield / 色 / static_object 群）
	NumFrames       int           `json:"num_frames"`
	UnresolvedShare float64       `json:"unresolved_share"` // 多数決が割れた画素率（背景アニメ等のサイン）
	Frames          []FrameResult `json:"frames"`
	Union           []UnionObject `json:"union"`
}

// AnalyzeFrames は複数フレームを静的/動的に分離して解析する。1 枚なら従来解析に等価。
func AnalyzeFrames(frames []*Normalized, q *Quantizer) (*MultiReport, error) {
	if len(frames) == 0 {
		return nil, fmt.Errorf("no frames")
	}
	for i := 1; i < len(frames); i++ {
		if frames[i].Height != frames[0].Height || frames[i].ScaleX != frames[0].ScaleX || frames[i].ScaleY != frames[0].ScaleY {
			return nil, fmt.Errorf("frame %d has different geometry (%dx? scale %dx%d) — re-shoot the sequence without changing Stella's window",
				i, frames[i].Height, frames[i].ScaleX, frames[i].ScaleY)
		}
	}
	h := frames[0].Height

	// --- 静的層: 画素毎の多数決 ---
	static := &Normalized{
		Height: h, ScaleX: frames[0].ScaleX, ScaleY: frames[0].ScaleY,
		TIA:   image.NewRGBA(image.Rect(0, 0, tiaWidth, h)),
		Codes: make([][]uint8, h),
	}
	unresolved := 0
	for y := 0; y < h; y++ {
		static.Codes[y] = make([]uint8, tiaWidth)
		for x := 0; x < tiaWidth; x++ {
			counts := map[uint8]int{}
			var best uint8
			bestN := 0
			for _, f := range frames {
				c := f.Codes[y][x]
				counts[c]++
				if counts[c] > bestN {
					best, bestN = c, counts[c]
				}
			}
			if bestN <= len(frames)/2 && len(frames) > 1 {
				unresolved++ // 過半数なし（N=2 の不一致 or 常時アニメ画素）
			}
			static.Codes[y][x] = best
			static.TIA.SetRGBA(x, y, q.RGB(best))
		}
	}

	// N=2 の不一致画素は「どちらが静的か」決定不能 → 行背景で充填（unresolved に計上済み）
	if len(frames) == 2 {
		fillTiesWithRowBG(static, frames)
	}

	// --- 静的層の解析（PF・色・駐機物）---
	rep := Analyze(static, q)
	for i := range rep.Sprites {
		rep.Sprites[i].Kind = "static_" + rep.Sprites[i].Kind // 動くスプライトと区別
		rep.Sprites[i].Hint = staticHint(rep, rep.Sprites[i])
	}

	mr := &MultiReport{
		Static:          rep,
		NumFrames:       len(frames),
		UnresolvedShare: float64(unresolved) / float64(h*tiaWidth),
	}
	if len(frames) == 1 {
		mr.Frames = []FrameResult{{Sprites: nil, Fidelity: rep.Fidelity}}
		return mr, nil
	}

	// --- 動的層: フレーム毎の差分 → 既存スプライト抽出 ---
	for _, f := range frames {
		residual := make([][]bool, h)
		for y := 0; y < h; y++ {
			residual[y] = make([]bool, tiaWidth)
			for x := 0; x < tiaWidth; x++ {
				if f.Codes[y][x] != static.Codes[y][x] {
					residual[y][x] = true
				}
			}
		}
		sprites := ExtractSprites(f, residual)
		// fidelity: 静的層レポート＋このフレームの動的スプライトで逆描画
		tmp := *rep
		tmp.Sprites = append(append([]Sprite{}, rep.Sprites...), sprites...)
		// static_ プレフィクスは Render に影響しない kind なので戻す必要あり（player_2x 等の stretch 判定）
		for i := range tmp.Sprites[:len(rep.Sprites)] {
			tmp.Sprites[i].Kind = trimStatic(tmp.Sprites[i].Kind)
		}
		mr.Frames = append(mr.Frames, FrameResult{Sprites: sprites, Fidelity: Fidelity(&tmp, f)})
	}

	// --- union: 同形・同色を 1 オブジェクトに（出現フレーム付き）---
	mr.Union = unionObjects(mr.Frames)
	return mr, nil
}

// staticHint は静的物の解釈支援: 隣接 PF バンドと同色なら「PF の縁飾り」寄り、
// そうでなければ「駐機中の ball/missile/player」寄り。最終判断は作者。
func staticHint(rep *Report, s Sprite) string {
	for _, b := range rep.Playfield {
		if b.Top > s.Y+s.H+1 || b.Top+b.Height < s.Y-1 {
			continue
		}
		for _, c := range s.Colors {
			if uint8(c) == b.ColorLeft || uint8(c) == b.ColorRight {
				return "pf_fringe?"
			}
		}
	}
	return "parked_object?"
}

func trimStatic(k string) string {
	const p = "static_"
	if len(k) > len(p) && k[:len(p)] == p {
		return k[len(p):]
	}
	return k
}

// fillTiesWithRowBG は N=2 の不一致画素を「行の背景色（一致画素の最頻値）」で充填する。
func fillTiesWithRowBG(static *Normalized, frames []*Normalized) {
	h := static.Height
	for y := 0; y < h; y++ {
		counts := map[uint8]int{}
		var bg uint8
		bestN := 0
		for x := 0; x < tiaWidth; x++ {
			if frames[0].Codes[y][x] == frames[1].Codes[y][x] {
				c := frames[0].Codes[y][x]
				counts[c]++
				if counts[c] > bestN {
					bg, bestN = c, counts[c]
				}
			}
		}
		if bestN == 0 {
			continue
		}
		for x := 0; x < tiaWidth; x++ {
			if frames[0].Codes[y][x] != frames[1].Codes[y][x] {
				static.Codes[y][x] = bg
			}
		}
	}
}

// unionObjects は位置連続性トラッキング（M-H）: フレームを跨いで「近接（≤12px）かつ色が
// 交差する」スプライトを 1 トラックに連結する。形ベースの旧実装はアニメ移動体（Pitfall の
// ハリー）をポーズ毎に別エントリへ割り、全部に flicker を誤フラグした。
// 真の flicker ＝「ほぼ同位置でフレームを飛ばして明滅」だけに限定する。
func unionObjects(frames []FrameResult) []UnionObject {
	type track struct {
		u          UnionObject
		lastX, lastY int
		lastFrame  int
		shapes     map[string]bool
		gap        bool // 出現に飛びがあった
		moved      bool // 大きく動いた（>2px）
	}
	sigOf := func(s Sprite) string {
		sig := ""
		for _, g := range s.GRP {
			sig += string(rune(g))
		}
		return sig
	}
	var tracks []*track
	for fi, fr := range frames {
		for _, s := range fr.Sprites {
			var best *track
			bestD := 21 // 速い移動体（Pitfall のハリーは最大 18px/フレーム）も繋ぐ。色交差が誤連結を防ぐ
			for _, t := range tracks {
				if t.lastFrame == fi {
					continue // 同フレームの別物
				}
				dx := s.X - t.lastX
				if dx < 0 {
					dx = -dx
				}
				dy := s.Y - t.lastY
				if dy < 0 {
					dy = -dy
				}
				d := dx
				if dy > d {
					d = dy
				}
				if d < bestD && colorsShared(s.Colors, t.u.Colors) {
					best, bestD = t, d
				}
			}
			if best != nil {
				if fi-best.lastFrame > 1 {
					best.gap = true
				}
				if bestD > 2 {
					best.moved = true
				}
				best.lastX, best.lastY, best.lastFrame = s.X, s.Y, fi
				if best.u.SeenFrames[len(best.u.SeenFrames)-1] != fi {
					best.u.SeenFrames = append(best.u.SeenFrames, fi)
				}
				best.shapes[sigOf(s)] = true
				continue
			}
			t := &track{u: UnionObject{Sprite: s, SeenFrames: []int{fi}},
				lastX: s.X, lastY: s.Y, lastFrame: fi, shapes: map[string]bool{sigOf(s): true}}
			if fi > 0 {
				t.gap = true // 途中から現れた（先頭フレームに居ない）
			}
			tracks = append(tracks, t)
		}
	}
	var out []UnionObject
	for _, t := range tracks {
		if t.lastFrame < len(frames)-1 {
			t.gap = true // 途中で消えた（最終フレームに不在）
		}
		t.u.Poses = len(t.shapes)
		t.u.Flicker = t.gap && !t.moved // 同位置明滅のみ＝真の flicker
		out = append(out, t.u)
	}
	return out
}
