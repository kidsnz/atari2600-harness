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

// UnionObject は全フレームを通したオブジェクト（同形・同色を1件に統合）。
type UnionObject struct {
	Sprite
	SeenFrames []int `json:"seen_frames"`
	Flicker    bool  `json:"flicker,omitempty"` // 一部フレームにしか出ない＝30Hz flicker の徴候
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

// unionObjects は全フレームのスプライトを「形＋色＋サイズ」で束ね、出現フレームと flicker を付ける。
func unionObjects(frames []FrameResult) []UnionObject {
	type key struct {
		w, h int
		sig  string
	}
	sigOf := func(s Sprite) key {
		sig := ""
		for i, g := range s.GRP {
			sig += string(rune(g))
			if i < len(s.Colors) {
				sig += string(rune(s.Colors[i]))
			}
		}
		return key{s.W, s.H, sig}
	}
	order := []key{}
	m := map[key]*UnionObject{}
	for fi, fr := range frames {
		for _, s := range fr.Sprites {
			k := sigOf(s)
			if u, ok := m[k]; ok {
				if u.SeenFrames[len(u.SeenFrames)-1] != fi {
					u.SeenFrames = append(u.SeenFrames, fi)
				}
				continue
			}
			m[k] = &UnionObject{Sprite: s, SeenFrames: []int{fi}}
			order = append(order, k)
		}
	}
	var out []UnionObject
	for _, k := range order {
		u := m[k]
		u.Flicker = len(u.SeenFrames) < len(frames)
		out = append(out, *u)
	}
	return out
}
