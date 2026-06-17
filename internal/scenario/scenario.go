// Package scenario は「入力タイムライン＋数値アサーション」を 1 つの JSON で宣言し、ROM に対して
// 自動 pass/fail する回帰ランナー（欠落D = 検証自動化 / P2 の D-1 + D-2）。
//
// アサーションの語彙（field 文字列）は internal/emu の read 系メソッドに 1 対 1 で対応する
// ＝ ハーネスの観測ツールをそのまま回帰の語彙として使う（鉄則1: 判定は数値）。
package scenario

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/kidsnz/atari2600-harness/internal/build"
	"github.com/kidsnz/atari2600-harness/internal/cyclebound"
	"github.com/kidsnz/atari2600-harness/internal/emu"
	"github.com/kidsnz/atari2600-harness/internal/motion"
	"github.com/kidsnz/atari2600-harness/internal/ocr"
)

// Input はあるフレームで与えるジョイスティック操作（D-2: 入力タイムライン）。
type Input struct {
	Frame   int    `json:"frame"`   // シナリオ開始（warmup 後）からの 0 起点フレーム。このフレームを走らせる前に適用
	Player  int    `json:"player"`  // 0=P0/左, 1=P1/右
	Action  string `json:"action"`  // left|right|up|down|fire|center|paddle|reset|select|color|p0pro|p1pro
	Pressed bool   `json:"pressed"` // 押下保持/解除（center/paddle では無視）
	Value   float64 `json:"value,omitempty"` // action=paddle の位置 0.0〜1.0（V2-4b）
}

// Assert はあるフレーム終了時点の瞬時数値条件（D-1）。副作用のある計測は Checks 側で扱う。
type Assert struct {
	AtFrame int    `json:"at_frame"` // このフレームを走らせた直後に評価
	Field   string `json:"field"`    // 語彙（下記 resolve 参照）
	Op      string `json:"op"`       // == != < <= > >= in
	Value   int64  `json:"value"`    // 比較値（bool は 0/1）
	Lo      int64  `json:"lo,omitempty"` // op="in": got が [Lo,Hi] に入るか（範囲演算子）
	Hi      int64  `json:"hi,omitempty"`
}

// Invariant は run 全体で「毎フレーム」成り立つべき条件（property-based / 契約）。
// 最初に破れたフレーム番号を報告する。docs/testing-playbook.md。
type Invariant struct {
	Field string `json:"field"`
	Op    string `json:"op"` // == != < <= > >= in
	Value int64  `json:"value,omitempty"`
	Lo    int64  `json:"lo,omitempty"` // op="in"
	Hi    int64  `json:"hi,omitempty"`
}

// Monotonic は field が run 全体で一方向にしか動かない性質（score 単調増・lives 単調減など）。
type Monotonic struct {
	Field     string `json:"field"`
	Direction string `json:"direction"` // "up"=非減少 / "down"=非増加
}

// Fuzz は seed 付き乱数入力で Frames フレーム走らせ、毎フレーム invariants/monotonic を監視する
// ＝決定論シミュレーションテスト（FoundationDB/Antithesis 流儀）の 2600 版。決定論的なので
// 同じ seed は同じ入力列＝失敗は再現可能（replay）。docs/testing-playbook.md。
type Fuzz struct {
	Seed    int64    `json:"seed"`
	Frames  int      `json:"frames"`
	Actions []string `json:"actions"` // 乱択する入力プール（例 ["left","right","fire"]）
	Player  int      `json:"player,omitempty"`
}

// Temporal は frame 系列に対する有界時相論理モニタ（VV-5）。瞬時 Assert / 毎フレーム
// Invariant では書けない「系列の性質」＝有界 liveness と期限つき応答を表す。条件 P は既存
// Assert/Invariant と同じ語彙（resolve + condPass）を使う。"always P" は既存 Invariant そのもの
// なので重複実装せず、ここでは系列特有の3種だけを足す（docs/scenarios.md で相互参照）。
//
// Kind:
//   "eventually" — P が run 開始から Within フレーム以内（先頭 Within フレーム f∈[0,Within-1]）に
//                  少なくとも一度成立する（有界 liveness）。窓 [0,Within-1] が run 終了までに
//                  完全に観測されないまま P が成立しなければ INCONCLUSIVE（≠pass）＝空虚な緑を禁止。
//   "response"   — A（AField...）が成立するたび、その frame f から Within フレーム以内（g∈[f,f+Within]）
//                  に P が成立する。各 A 発火は義務。窓が run 終了をまたぐ未充足義務は INCONCLUSIVE。
//   "never_for"  — P が N フレーム連続して成立してはならない（安全性。観測列で完全に判定可能）。
type Temporal struct {
	Kind string `json:"kind"` // eventually | response | never_for

	// P（監視対象の命題。Assert と同形）。
	Field string `json:"field"`
	Op    string `json:"op"`
	Value int64  `json:"value,omitempty"`
	Lo    int64  `json:"lo,omitempty"`
	Hi    int64  `json:"hi,omitempty"`

	// A（先行条件＝トリガ。kind="response" のみ使用）。
	AField string `json:"a_field,omitempty"`
	AOp    string `json:"a_op,omitempty"`
	AValue int64  `json:"a_value,omitempty"`
	ALo    int64  `json:"a_lo,omitempty"`
	AHi    int64  `json:"a_hi,omitempty"`

	Within int `json:"within,omitempty"` // 期限（フレーム数。eventually/response）
	N      int `json:"n,omitempty"`      // 連続フレーム数（never_for）
}

// Checks は run 全体に対する性質（副作用＝フレームを進める計測なのでタイムライン後にまとめて評価）。
type Checks struct {
	NTSCFrameLines *int `json:"ntsc_frame_lines,omitempty"` // StepFrame() == この値（NTSC は 262）
	MaxLineBudget  *int `json:"max_line_budget,omitempty"`  // RunUntilBudget が超過しない（runtime・∃: ある1回の実行を観測。既定予算 76）
	ProveLineBudget *int `json:"prove_line_budget,omitempty"` // VV-2: cyclebound が全パスの worst <= 予算を静的に証明（∀。rom が .asm のときのみ）
	GoldenFrame    bool `json:"golden_frame,omitempty"`     // D-3: タイムラインの描画連鎖ハッシュを <scenario>.golden と照合
	GoldenAudio    bool `json:"golden_audio,omitempty"`     // A-2: タイムラインの音声連鎖ハッシュを <scenario>.audio.golden と照合
	Motion         *MotionCheck `json:"motion,omitempty"`   // VV-4: ある object の動きの滑らかさ（jerk_rms）をゲート
	NoTimerWrap    *int `json:"no_timer_wrap,omitempty"`     // VV-10 T-1: この frame 数を監視し read-after-wrap(G8) が起きないことをゲート
	NoHMOVEHazard  *int `json:"no_hmove_hazard,omitempty"`   // VV-10 T-2: HMOVE 後 24cy 以内の HMxx 書き込みが無いことをゲート
	ScoreEqualsRAM *ScoreCheck `json:"score_equals_ram,omitempty"` // VV-9: 描画された2桁BCDスコア == RAM の値
}

// ScoreCheck は VV-9 OCR ゲート：score2 レイアウトで描画された2桁を OCR し、packed BCD が
// 指定 RAM バイトと一致するか検証する。グリフの正本フォントは `<scenario>.font`（80 バイト=
// 10字×8行）から読む（ROM 由来でなく外部スペックなので font-index バグを捕捉できる）。
type ScoreCheck struct {
	Ram    string `json:"ram"`              // スコアの packed BCD を持つ RAM フィールド（例 "ram.0x80"）
	Frames int    `json:"frames,omitempty"` // 計測前に走らせるフレーム数（既定 4）
}

// MotionCheck gates an object's motion smoothness: it tracks the object for
// Frames frames (after Warmup) and requires the chosen axis's jerk_rms (RMS of
// the rendered position's 2nd difference; 0 = constant velocity) to stay <=
// MaxJerkRMS. Catches a judder/stutter regression. Track over a window where the
// object moves on a uniform background; axis "x" (HmovedPixel) is always exact.
type MotionCheck struct {
	Object     string  `json:"object"`               // P0 M0 P1 M1 BL
	Axis       string  `json:"axis,omitempty"`       // "top" (rendered vertical, default) or "x"
	Frames     int     `json:"frames,omitempty"`     // default 40
	Warmup     int     `json:"warmup,omitempty"`     // frames to settle before tracking
	YTop       int     `json:"y_top,omitempty"`      // scanline search window (grid-y)
	YBot       int     `json:"y_bot,omitempty"`      // default 260
	MaxJerkRMS float64 `json:"max_jerk_rms"`         // gate: jerk_rms must be <= this
}

// Scenario は 1 本のシナリオ定義。
type Scenario struct {
	Rom          string   `json:"rom"`
	TVSpec       string   `json:"tv_spec,omitempty"`       // 省略時 NTSC
	WarmupFrames int      `json:"warmup_frames,omitempty"` // 計測前の空走（省略時 2）
	Inputs       []Input  `json:"inputs,omitempty"`
	Asserts      []Assert `json:"asserts,omitempty"`
	Checks       *Checks  `json:"checks,omitempty"`

	// run 全体の性質（毎フレーム監視）。Frames は監視のために走らせる最小フレーム数
	// （省略時は inputs/asserts の最大フレームまで）。
	Frames     int         `json:"frames,omitempty"`
	Invariants []Invariant `json:"invariants,omitempty"`
	Monotonic  []Monotonic `json:"monotonic,omitempty"`
	Temporal   []Temporal  `json:"temporal,omitempty"` // VV-5: 系列に対する有界時相論理モニタ
	Fuzz       *Fuzz       `json:"fuzz,omitempty"`
	Metrics    []string    `json:"metrics,omitempty"` // run 終了時に捕捉する field（metamorphic 比較用）

	srcPath string // Load 時のファイルパス（golden ファイルの場所決めに使う。空＝プログラム生成）
}

// AssertResult は 1 アサーションの評価結果。
type AssertResult struct {
	Desc string // "ram.0x81 < 144"
	Got  int64
	Pass bool
}

// Result はシナリオ全体の結果。
type Result struct {
	Asserts    []AssertResult
	GoldenHash string           // golden_frame 指定時に算出した描画連鎖ハッシュ（決定性確認用）
	AudioHash  string           // golden_audio 指定時に算出した音声連鎖ハッシュ
	Metrics    map[string]int64 // run 終了時に捕捉した field（metamorphic 比較用）
	Pass       bool
}

// Load はファイルからシナリオを読む。
func Load(path string) (*Scenario, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s Scenario
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if s.Rom == "" {
		return nil, fmt.Errorf("%s: \"rom\" is required", path)
	}
	s.srcPath = path
	return &s, nil
}

// goldenPath は scenario ファイルパスから対応する .golden ファイルのパスを返す。
func goldenPath(srcPath string) string {
	return strings.TrimSuffix(srcPath, filepath.Ext(srcPath)) + ".golden"
}

// Run はシナリオを実行し pass/fail を返す。updateGoldens=true なら golden_frame の基準ハッシュを書き直す。
func Run(s *Scenario, updateGoldens bool) (*Result, error) {
	spec := s.TVSpec
	if spec == "" {
		spec = "NTSC"
	}
	e, err := emu.New(spec)
	if err != nil {
		return nil, err
	}

	// rom が .asm なら実行前に dasm でアセンブル（ソース1枚→合否を 1 コマンドに＝欠落E）。
	romPath := s.Rom
	if strings.EqualFold(filepath.Ext(romPath), ".asm") {
		bin := build.BinPathFor(romPath)
		out, aerr := build.Assemble(romPath, bin)
		if aerr != nil {
			return nil, fmt.Errorf("assemble %s failed:\n%s", romPath, out)
		}
		romPath = bin
	}
	if err := e.LoadROM(romPath); err != nil {
		return nil, fmt.Errorf("load %s: %w", romPath, err)
	}

	golden := s.Checks != nil && s.Checks.GoldenFrame
	if golden {
		if err := e.EnableVideoDigest(); err != nil {
			return nil, err
		}
	}
	goldenA := s.Checks != nil && s.Checks.GoldenAudio
	if goldenA {
		if err := e.EnableAudioDigest(); err != nil {
			return nil, err
		}
	}

	warmup := s.WarmupFrames
	if warmup == 0 {
		warmup = 2
	}
	if err := e.RunFrames(warmup); err != nil {
		return nil, err
	}
	if golden {
		e.ResetVideoDigest() // warmup を除外＝タイムラインのフレームだけで決定的なハッシュにする
	}
	if goldenA {
		e.ResetAudioDigest()
	}

	// フレーム別にインデックス化。
	inByFrame := map[int][]Input{}
	asByFrame := map[int][]Assert{}
	maxF := 0
	for _, in := range s.Inputs {
		inByFrame[in.Frame] = append(inByFrame[in.Frame], in)
		if in.Frame > maxF {
			maxF = in.Frame
		}
	}
	for _, a := range s.Asserts {
		asByFrame[a.AtFrame] = append(asByFrame[a.AtFrame], a)
		if a.AtFrame > maxF {
			maxF = a.AtFrame
		}
	}

	// invariants / monotonic は inputs/asserts の最大フレームを越えても監視したいので run 長を伸ばす。
	lastF := maxF
	if s.Frames-1 > lastF {
		lastF = s.Frames - 1
	}

	// fuzz: seed 付き乱数入力で Frames フレーム回す（決定論＝再現可能）。
	var rng *rand.Rand
	if s.Fuzz != nil {
		if len(s.Fuzz.Actions) == 0 {
			return nil, fmt.Errorf("fuzz needs a non-empty \"actions\" pool")
		}
		if s.Fuzz.Frames-1 > lastF {
			lastF = s.Fuzz.Frames - 1
		}
		rng = rand.New(rand.NewSource(s.Fuzz.Seed))
	}

	res := &Result{Pass: true}
	record := func(a Assert, got int64, ok bool, evalErr error) error {
		if evalErr != nil {
			return evalErr
		}
		desc := condDesc(a.Field, a.Op, a.Value, a.Lo, a.Hi)
		res.Asserts = append(res.Asserts, AssertResult{Desc: desc, Got: got, Pass: ok})
		if !ok {
			res.Pass = false
		}
		return nil
	}

	// 毎フレーム監視の状態（最初の破れだけ記録してフレーム数ぶん溢れさせない）。
	brokenInv := make([]bool, len(s.Invariants))
	prevMono := make([]int64, len(s.Monotonic))
	seenMono := make([]bool, len(s.Monotonic))
	brokenMono := make([]bool, len(s.Monotonic))

	// VV-5 temporal: 各モニタの P（と response の A）を毎フレーム観測して bool 列に貯め、
	// run 後に系列として判定する（観測と時相評価を分離＝自己テストしやすい）。
	if err := validateTemporal(s.Temporal); err != nil {
		return nil, err
	}
	pTrace := make([][]bool, len(s.Temporal))
	aTrace := make([][]bool, len(s.Temporal))

	jammed := false
	for f := 0; f <= lastF; f++ {
		if s.Fuzz != nil && f < s.Fuzz.Frames { // 乱数入力（scripted の後に適用）
			act := s.Fuzz.Actions[rng.Intn(len(s.Fuzz.Actions))]
			pressed := rng.Intn(2) == 1
			if err := e.SetInput(s.Fuzz.Player, act, pressed); err != nil {
				return nil, fmt.Errorf("fuzz frame %d input %q: %w", f, act, err)
			}
		}
		for _, in := range inByFrame[f] { // このフレームを走らせる前に入力適用
			if in.Action == "paddle" {
				if err := e.SetPaddle(in.Player, in.Value); err != nil {
					return nil, fmt.Errorf("frame %d paddle: %w", f, err)
				}
			} else if in.Action == "reset" || in.Action == "select" || in.Action == "color" ||
				in.Action == "p0pro" || in.Action == "p1pro" {
				if err := e.SetPanel(in.Action, in.Pressed); err != nil {
					return nil, fmt.Errorf("frame %d panel: %w", f, err)
				}
			} else if err := e.SetInput(in.Player, in.Action, in.Pressed); err != nil {
				return nil, fmt.Errorf("frame %d input: %w", f, err)
			}
		}
		if err := e.RunFrames(1); err != nil {
			return nil, err
		}
		if s.Fuzz != nil && e.VCS.CPU.Jammed { // クラッシュ（KIL/不正命令で CPU 停止）を検出
			res.Asserts = append(res.Asserts, AssertResult{
				Desc: fmt.Sprintf("fuzz: CPU jammed (crash) @frame %d [seed=%d]", f, s.Fuzz.Seed),
				Pass: false})
			res.Pass = false
			jammed = true
			break
		}
		for _, a := range asByFrame[f] { // フレーム終了時点で瞬時評価
			got, err := resolve(e, a.Field)
			if err != nil {
				return nil, fmt.Errorf("frame %d: %w", f, err)
			}
			ok, err := condPass(got, a.Op, a.Value, a.Lo, a.Hi)
			if err := record(a, got, ok, err); err != nil {
				return nil, err
			}
		}

		// 毎フレーム: 不変条件（常に真であるべき）。最初に破れたフレームを1回だけ記録。
		for i, inv := range s.Invariants {
			if brokenInv[i] {
				continue
			}
			got, err := resolve(e, inv.Field)
			if err != nil {
				return nil, fmt.Errorf("frame %d invariant %q: %w", f, inv.Field, err)
			}
			ok, err := condPass(got, inv.Op, inv.Value, inv.Lo, inv.Hi)
			if err != nil {
				return nil, err
			}
			if !ok {
				res.Asserts = append(res.Asserts, AssertResult{
					Desc: fmt.Sprintf("invariant %s [broke@frame %d]", condDesc(inv.Field, inv.Op, inv.Value, inv.Lo, inv.Hi), f),
					Got:  got, Pass: false})
				res.Pass = false
				brokenInv[i] = true
			}
		}

		// 毎フレーム: 単調性（一方向にしか動かない）。最初の違反を1回だけ記録。
		for i, m := range s.Monotonic {
			got, err := resolve(e, m.Field)
			if err != nil {
				return nil, fmt.Errorf("frame %d monotonic %q: %w", f, m.Field, err)
			}
			if seenMono[i] && !brokenMono[i] {
				var bad bool
				switch m.Direction {
				case "up":
					bad = got < prevMono[i]
				case "down":
					bad = got > prevMono[i]
				default:
					return nil, fmt.Errorf("monotonic %q: unknown direction %q (want up|down)", m.Field, m.Direction)
				}
				if bad {
					res.Asserts = append(res.Asserts, AssertResult{
						Desc: fmt.Sprintf("monotonic %s %s [broke@frame %d: %d->%d]", m.Field, m.Direction, f, prevMono[i], got),
						Got:  got, Pass: false})
					res.Pass = false
					brokenMono[i] = true
				}
			}
			prevMono[i] = got
			seenMono[i] = true
		}

		// 毎フレーム: temporal モニタの命題を観測（評価は run 後）。
		for i, t := range s.Temporal {
			got, err := resolve(e, t.Field)
			if err != nil {
				return nil, fmt.Errorf("frame %d temporal %q: %w", f, t.Field, err)
			}
			p, err := condPass(got, t.Op, t.Value, t.Lo, t.Hi)
			if err != nil {
				return nil, err
			}
			pTrace[i] = append(pTrace[i], p)
			if t.Kind == "response" {
				gotA, err := resolve(e, t.AField)
				if err != nil {
					return nil, fmt.Errorf("frame %d temporal a_field %q: %w", f, t.AField, err)
				}
				a, err := condPass(gotA, t.AOp, t.AValue, t.ALo, t.AHi)
				if err != nil {
					return nil, err
				}
				aTrace[i] = append(aTrace[i], a)
			}
		}
	}

	// fuzz が完走（クラッシュ無し）したら陽に記録。seed があるので失敗は再現可能。
	if s.Fuzz != nil && !jammed {
		res.Asserts = append(res.Asserts, AssertResult{
			Desc: fmt.Sprintf("fuzz seed=%d frames=%d: no crash", s.Fuzz.Seed, s.Fuzz.Frames),
			Pass: true})
	}

	// 破れなかった invariant / monotonic は「N フレーム成立」と陽に記録（合格を可視化）。
	for i, inv := range s.Invariants {
		if !brokenInv[i] {
			res.Asserts = append(res.Asserts, AssertResult{
				Desc: fmt.Sprintf("invariant %s held [%d frames]", condDesc(inv.Field, inv.Op, inv.Value, inv.Lo, inv.Hi), lastF+1),
				Pass: true})
		}
	}
	for i, m := range s.Monotonic {
		if !brokenMono[i] {
			res.Asserts = append(res.Asserts, AssertResult{
				Desc: fmt.Sprintf("monotonic %s %s held [%d frames]", m.Field, m.Direction, lastF+1),
				Pass: true})
		}
	}

	// VV-5: temporal モニタを観測列から判定（pass / fail / inconclusive）。
	// inconclusive は Pass:false（窓が来ていないのに緑、を禁止）。
	for i, t := range s.Temporal {
		desc, pass := evalTemporal(t, pTrace[i], aTrace[i])
		res.Asserts = append(res.Asserts, AssertResult{Desc: desc, Pass: pass})
		if !pass {
			res.Pass = false
		}
	}

	// metamorphic 用メトリクスは Checks の副作用計測（StepFrame 等で emu が進む）より先に捕捉する。
	if len(s.Metrics) > 0 {
		res.Metrics = make(map[string]int64, len(s.Metrics))
		for _, fld := range s.Metrics {
			v, err := resolve(e, fld)
			if err != nil {
				return nil, fmt.Errorf("metric %q: %w", fld, err)
			}
			res.Metrics[fld] = v
			res.Asserts = append(res.Asserts, AssertResult{Desc: fmt.Sprintf("metric %s", fld), Got: v, Pass: true})
		}
	}

	// run 全体の性質（副作用のある計測はここでまとめて）。
	if s.Checks != nil {
		// ゴールデンは副作用計測（StepFrame/RunUntilBudget）より先にハッシュ確定。
		if golden {
			hash := e.VideoHash()
			res.GoldenHash = hash
			if err := evalGolden(s, hash, updateGoldens, res); err != nil {
				return nil, err
			}
		}
		if goldenA {
			hash := e.AudioHash()
			res.AudioHash = hash
			if err := evalGoldenAudio(s, hash, updateGoldens, res); err != nil {
				return nil, err
			}
		}
		if s.Checks.NTSCFrameLines != nil {
			lines, err := e.StepFrame()
			if err != nil {
				return nil, err
			}
			ok := lines == *s.Checks.NTSCFrameLines
			res.Asserts = append(res.Asserts, AssertResult{
				Desc: fmt.Sprintf("ntsc_frame_lines == %d", *s.Checks.NTSCFrameLines), Got: int64(lines), Pass: ok})
			if !ok {
				res.Pass = false
			}
		}
		if s.Checks.MaxLineBudget != nil {
			over, atSL, _, err := e.RunUntilBudget(2, *s.Checks.MaxLineBudget)
			if err != nil {
				return nil, err
			}
			ok := !over
			got := int64(0)
			if over {
				got = int64(atSL)
			}
			res.Asserts = append(res.Asserts, AssertResult{
				Desc: fmt.Sprintf("max_line_budget %d: no overrun", *s.Checks.MaxLineBudget), Got: got, Pass: ok})
			if !ok {
				res.Pass = false
			}
		}
		if s.Checks.NoTimerWrap != nil {
			// VV-10 T-1: read-after-wrap(G8) を監視。clean なタイマ使用は flag されない。
			hit, err := e.WatchTimerWrap(*s.Checks.NoTimerWrap)
			if err != nil {
				return nil, err
			}
			ok := hit == nil
			got := int64(0)
			desc := fmt.Sprintf("no_timer_wrap over %d frames: clean", *s.Checks.NoTimerWrap)
			if hit != nil {
				got = int64(hit.PC)
				desc = fmt.Sprintf("no_timer_wrap over %d frames: read-after-wrap @frame %d pc 0x%04X",
					*s.Checks.NoTimerWrap, hit.Frame, hit.PC)
			}
			res.Asserts = append(res.Asserts, AssertResult{Desc: desc, Got: got, Pass: ok})
			if !ok {
				res.Pass = false
			}
		}
		if s.Checks.NoHMOVEHazard != nil {
			// VV-10 T-2: HMOVE 後 24cy 以内の HMxx 書き込み（動き不定ハザード）を監視。
			hit, err := e.WatchHMOVEHazard(*s.Checks.NoHMOVEHazard)
			if err != nil {
				return nil, err
			}
			ok := hit == nil
			got := int64(0)
			desc := fmt.Sprintf("no_hmove_hazard over %d frames: clean", *s.Checks.NoHMOVEHazard)
			if hit != nil {
				got = int64(hit.PC)
				desc = fmt.Sprintf("no_hmove_hazard over %d frames: HMxx written %dcy after HMOVE @frame %d pc 0x%04X",
					*s.Checks.NoHMOVEHazard, hit.CyclesAfter, hit.Frame, hit.PC)
			}
			res.Asserts = append(res.Asserts, AssertResult{Desc: desc, Got: got, Pass: ok})
			if !ok {
				res.Pass = false
			}
		}
		if s.Checks.ScoreEqualsRAM != nil {
			// VV-9: 描画した2桁BCDスコアを OCR し RAM 値と一致するか。正本フォントは <scenario>.font。
			sc := s.Checks.ScoreEqualsRAM
			frames := sc.Frames
			if frames <= 0 {
				frames = 4
			}
			font, ferr := loadScoreFont(strings.TrimSuffix(s.srcPath, filepath.Ext(s.srcPath)) + ".font")
			if ferr != nil {
				return nil, ferr
			}
			if err := e.RunFrames(frames); err != nil {
				return nil, err
			}
			img, _ := e.Snapshot()
			r := ocr.ReadScore2(img, font)
			want, err := resolve(e, sc.Ram)
			if err != nil {
				return nil, err
			}
			ok := r.OK && int64(r.ExpectedBCD()) == want
			res.Asserts = append(res.Asserts, AssertResult{
				Desc: fmt.Sprintf("score_equals_ram %s: displayed 0x%02X (decoded %d%d)", sc.Ram, r.ExpectedBCD(), r.Tens, r.Ones),
				Got:  int64(r.ExpectedBCD()), Pass: ok})
			if !ok {
				res.Pass = false
			}
		}
		if s.Checks.ProveLineBudget != nil {
			// VV-2: 全到達パスの worst-case を静的に証明（assert_line_budget の ∀ 版）。
			// ソース必須なので rom が .asm のときだけ実行（.bin 直指定は skip と明記）。
			if !strings.EqualFold(filepath.Ext(s.Rom), ".asm") {
				res.Asserts = append(res.Asserts, AssertResult{
					Desc: "prove_line_budget: skipped (needs .asm source)", Pass: true})
			} else {
				rep, perr := cyclebound.Prove(s.Rom, *s.Checks.ProveLineBudget)
				if perr != nil {
					return nil, perr
				}
				res.Asserts = append(res.Asserts, AssertResult{
					Desc: fmt.Sprintf("prove_line_budget %d: all paths certified (worst %d, %d violation(s), %d unbounded)",
						*s.Checks.ProveLineBudget, rep.MaxWorst, len(rep.Violations), len(rep.Unbounded)),
					Got: int64(rep.MaxWorst), Pass: rep.Certified})
				if !rep.Certified {
					res.Pass = false
				}
			}
		}
		if s.Checks.Motion != nil {
			mc := s.Checks.Motion
			frames := mc.Frames
			if frames == 0 {
				frames = 40
			}
			yBot := mc.YBot
			if yBot == 0 {
				yBot = 260
			}
			if mc.Warmup > 0 {
				if err := e.RunFrames(mc.Warmup); err != nil {
					return nil, err
				}
			}
			tr, err := motion.TrackObject(e, mc.Object, frames, mc.YTop, yBot)
			if err != nil {
				return nil, err
			}
			jerk, axis := tr.Top.JerkRMS, "top"
			if mc.Axis == "x" {
				jerk, axis = tr.X.JerkRMS, "x"
			}
			ok := jerk <= mc.MaxJerkRMS
			res.Asserts = append(res.Asserts, AssertResult{
				Desc: fmt.Sprintf("motion %s.%s jerk_rms %.3g <= %.3g (missing %d)", mc.Object, axis, jerk, mc.MaxJerkRMS, tr.Missing),
				Got:  int64(math.Round(jerk * 1000)), Pass: ok})
			if !ok {
				res.Pass = false
			}
		}
	}

	return res, nil
}

// audioGoldenPath は scenario ファイルパスから対応する .audio.golden ファイルのパスを返す。
func audioGoldenPath(srcPath string) string {
	return strings.TrimSuffix(srcPath, filepath.Ext(srcPath)) + ".audio.golden"
}

// evalGolden は描画連鎖ハッシュを <scenario>.golden と照合する（label="golden_frame"）。
func evalGolden(s *Scenario, hash string, update bool, res *Result) error {
	return evalGoldenFile(s.srcPath, goldenPath(s.srcPath), "golden_frame", hash, update, res)
}

// evalGoldenAudio は音声連鎖ハッシュを <scenario>.audio.golden と照合する（label="golden_audio"）。
func evalGoldenAudio(s *Scenario, hash string, update bool, res *Result) error {
	return evalGoldenFile(s.srcPath, audioGoldenPath(s.srcPath), "golden_audio", hash, update, res)
}

// evalGoldenFile は算出ハッシュを golden ファイルと照合する（映像・音声で共有）。ファイルが無い／
// update なら記録。srcPath が空（プログラム生成）ならファイル照合はスキップ。
func evalGoldenFile(srcPath, gp, label, hash string, update bool, res *Result) error {
	if srcPath == "" {
		res.Asserts = append(res.Asserts, AssertResult{Desc: label + " (no file; hash computed)", Pass: true})
		return nil
	}
	existing, err := os.ReadFile(gp)
	switch {
	case update || errors.Is(err, fs.ErrNotExist):
		if err := os.WriteFile(gp, []byte(hash+"\n"), 0o644); err != nil {
			return err
		}
		res.Asserts = append(res.Asserts, AssertResult{Desc: label + " recorded " + filepath.Base(gp), Pass: true})
		return nil
	case err != nil:
		return err
	default:
		want := strings.TrimSpace(string(existing))
		ok := want == hash
		res.Asserts = append(res.Asserts, AssertResult{Desc: label + " matches", Pass: ok})
		if !ok {
			res.Pass = false
		}
		return nil
	}
}

// compare は got <op> want を評価する。
func compare(got int64, op string, want int64) (bool, error) {
	switch op {
	case "==":
		return got == want, nil
	case "!=":
		return got != want, nil
	case "<":
		return got < want, nil
	case "<=":
		return got <= want, nil
	case ">":
		return got > want, nil
	case ">=":
		return got >= want, nil
	default:
		return false, fmt.Errorf("unknown op %q (want == != < <= > >= in)", op)
	}
}

// condPass は範囲演算子 in（[lo,hi] 内包）を含む条件を評価する。
func condPass(got int64, op string, value, lo, hi int64) (bool, error) {
	if op == "in" {
		return got >= lo && got <= hi, nil
	}
	return compare(got, op, value)
}

// condDesc は条件の人間可読な説明（"in" は範囲表示）。
func condDesc(field, op string, value, lo, hi int64) string {
	if op == "in" {
		return fmt.Sprintf("%s in [%d,%d]", field, lo, hi)
	}
	return fmt.Sprintf("%s %s %d", field, op, value)
}

// validateTemporal は temporal モニタの形を run 前に検証する（不正は即エラー）。
func validateTemporal(ts []Temporal) error {
	for _, t := range ts {
		if t.Field == "" {
			return fmt.Errorf("temporal %q: \"field\" is required", t.Kind)
		}
		switch t.Kind {
		case "eventually", "response":
			if t.Within <= 0 {
				return fmt.Errorf("temporal %q: \"within\" must be > 0", t.Kind)
			}
			if t.Kind == "response" && t.AField == "" {
				return fmt.Errorf("temporal response: \"a_field\" (trigger) is required")
			}
		case "never_for":
			if t.N <= 0 {
				return fmt.Errorf("temporal never_for: \"n\" must be > 0")
			}
		default:
			return fmt.Errorf("temporal: unknown kind %q (want eventually|response|never_for)", t.Kind)
		}
	}
	return nil
}

// evalTemporal は観測した bool 列（p = 命題, a = response のトリガ）から1モニタの判定を返す。
// L = len(p) = 観測フレーム数。窓が観測列に収まらない liveness は inconclusive（pass=false）。
func evalTemporal(t Temporal, p, a []bool) (desc string, pass bool) {
	L := len(p)
	pDesc := condDesc(t.Field, t.Op, t.Value, t.Lo, t.Hi)

	switch t.Kind {
	case "eventually":
		K := t.Within
		limit := K
		if L < limit {
			limit = L
		}
		for f := 0; f < limit; f++ {
			if p[f] {
				return fmt.Sprintf("eventually (%s) within %d frames: held@frame %d", pDesc, K, f), true
			}
		}
		if L >= K {
			return fmt.Sprintf("eventually (%s) within %d frames: NOT held in window", pDesc, K), false
		}
		return fmt.Sprintf("INCONCLUSIVE eventually (%s) within %d frames: run ended at frame %d before the window elapsed (increase frames)", pDesc, K, L-1), false

	case "never_for":
		N := t.N
		run, maxRun, failAt := 0, 0, -1
		for f := 0; f < L; f++ {
			if p[f] {
				run++
				if run > maxRun {
					maxRun = run
				}
				if run >= N && failAt < 0 {
					failAt = f
				}
			} else {
				run = 0
			}
		}
		if failAt >= 0 {
			return fmt.Sprintf("never (%s) for %d frames: VIOLATED [held frames %d..%d]", pDesc, N, failAt-N+1, failAt), false
		}
		return fmt.Sprintf("never (%s) for %d frames: ok [max %d consecutive]", pDesc, N, maxRun), true

	case "response":
		K := t.Within
		aDesc := condDesc(t.AField, t.AOp, t.AValue, t.ALo, t.AHi)
		triggers, inconclusive := 0, false
		for f := 0; f < L; f++ {
			if !a[f] {
				continue
			}
			triggers++
			deadline := f + K
			last := deadline
			if last > L-1 {
				last = L - 1
			}
			found := false
			for g := f; g <= last; g++ {
				if p[g] {
					found = true
					break
				}
			}
			if found {
				continue
			}
			if deadline <= L-1 { // 窓は完全に観測した→違反
				return fmt.Sprintf("response (%s -> %s) within %d frames: VIOLATED [trigger@frame %d, no P within window]", aDesc, pDesc, K, f), false
			}
			inconclusive = true // 窓が run 終了をまたぐ
		}
		if inconclusive {
			return fmt.Sprintf("INCONCLUSIVE response (%s -> %s) within %d frames: an obligation's window extends past the run end (increase frames)", aDesc, pDesc, K), false
		}
		return fmt.Sprintf("response (%s -> %s) within %d frames: ok [%d trigger(s) satisfied]", aDesc, pDesc, K, triggers), true
	}
	return fmt.Sprintf("temporal: unknown kind %q", t.Kind), false
}

// loadScoreFont は <scenario>.font（80 バイト = 10字 × 8行）を ocr.Font に読み込む。
func loadScoreFont(path string) (ocr.Font, error) {
	var f ocr.Font
	b, err := os.ReadFile(path)
	if err != nil {
		return f, fmt.Errorf("score font %s: %w", path, err)
	}
	if len(b) < 80 {
		return f, fmt.Errorf("score font %s: need 80 bytes, got %d", path, len(b))
	}
	for d := 0; d < 10; d++ {
		copy(f[d][:], b[d*8:d*8+8])
	}
	return f, nil
}

func b2i(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

// ResolveField は field（語彙）の現在値を int64 で返す（副作用なし）。
// 他パッケージ（invariant マイニング等）が同じ観測語彙を使うための公開ラッパ。
func ResolveField(e *emu.Emu, field string) (int64, error) {
	return resolve(e, field)
}

// resolve は瞬時フィールド（副作用なし）を int64 で返す。未知フィールドはエラー（タイポを握り潰さない）。
func resolve(e *emu.Emu, field string) (int64, error) {
	parts := strings.Split(field, ".")
	switch parts[0] {
	case "frame":
		return int64(e.Coords().Frame), nil
	case "scanline":
		return int64(e.Coords().Scanline), nil
	case "clock":
		return int64(e.Coords().Clock), nil
	case "cycles_total":
		return e.TotalCycles(), nil
	case "cpu":
		return resolveCPU(e, parts)
	case "ram":
		return resolveRAM(e, parts)
	case "tia":
		return resolveTIA(e, parts)
	case "tiareg":
		return resolveTIAReg(e, parts)
	case "collisions":
		return resolveCollision(e, parts)
	case "audio":
		return resolveAudio(e, parts)
	case "bank":
		// bank.number / bank.is_ram — 現在 PC のカートリッジバンク（bankswitch 検証用）
		if len(parts) != 2 {
			return 0, fmt.Errorf("bank field needs bank.number or bank.is_ram")
		}
		n, isRAM := e.Bank()
		switch parts[1] {
		case "number":
			return int64(n), nil
		case "is_ram":
			return b2i(isRAM), nil
		}
		return 0, fmt.Errorf("unknown bank field %q", parts[1])
	default:
		return 0, fmt.Errorf("unknown field %q", field)
	}
}

func resolveCPU(e *emu.Emu, parts []string) (int64, error) {
	if len(parts) != 2 {
		return 0, fmt.Errorf("cpu field needs cpu.<pc|a|x|y|sp>")
	}
	cpu := e.VCS.CPU
	switch parts[1] {
	case "pc":
		return int64(cpu.PC.Value()), nil
	case "a":
		return int64(cpu.A.Value()), nil
	case "x":
		return int64(cpu.X.Value()), nil
	case "y":
		return int64(cpu.Y.Value()), nil
	case "sp":
		return int64(cpu.SP.Address()), nil
	default:
		return 0, fmt.Errorf("unknown cpu field %q", parts[1])
	}
}

func resolveRAM(e *emu.Emu, parts []string) (int64, error) {
	if len(parts) != 2 {
		return 0, fmt.Errorf("ram field needs ram.<addr> (e.g. ram.0x81)")
	}
	addr, err := strconv.ParseUint(parts[1], 0, 16) // 0x.. / 10進 両対応
	if err != nil {
		return 0, fmt.Errorf("ram addr %q: %w", parts[1], err)
	}
	b, err := e.PeekRAM(uint16(addr))
	if err != nil {
		return 0, err
	}
	return int64(b), nil
}

func resolveTIA(e *emu.Emu, parts []string) (int64, error) {
	if len(parts) != 3 {
		return 0, fmt.Errorf("tia field needs tia.<obj>.<reset_pixel|hmoved_pixel>")
	}
	v := e.VCS.TIA.Video
	pix := func(reset, hmoved int) (int64, error) {
		switch parts[2] {
		case "reset_pixel":
			return int64(reset), nil
		case "hmoved_pixel":
			return int64(hmoved), nil
		default:
			return 0, fmt.Errorf("unknown tia pixel %q", parts[2])
		}
	}
	switch parts[1] {
	case "player0":
		return pix(v.Player0.ResetPixel, v.Player0.HmovedPixel)
	case "player1":
		return pix(v.Player1.ResetPixel, v.Player1.HmovedPixel)
	case "missile0":
		return pix(v.Missile0.ResetPixel, v.Missile0.HmovedPixel)
	case "missile1":
		return pix(v.Missile1.ResetPixel, v.Missile1.HmovedPixel)
	case "ball":
		return pix(v.Ball.ResetPixel, v.Ball.HmovedPixel)
	default:
		return 0, fmt.Errorf("unknown tia obj %q", parts[1])
	}
}

func resolveTIAReg(e *emu.Emu, parts []string) (int64, error) {
	if len(parts) != 3 {
		return 0, fmt.Errorf("tiareg field needs tiareg.<obj>.<reg>")
	}
	r := e.ReadTIARegisters()
	switch parts[1] {
	case "player0", "player1":
		p := r.Player0
		if parts[1] == "player1" {
			p = r.Player1
		}
		switch parts[2] {
		case "color":
			return int64(p.Color), nil
		case "nusiz":
			return int64(p.Nusiz), nil
		case "reflected":
			return b2i(p.Reflected), nil
		case "vertical_delay":
			return b2i(p.VerticalDelay), nil
		}
	case "playfield":
		pf := r.Playfield
		switch parts[2] {
		case "pf0":
			return int64(pf.PF0), nil
		case "pf1":
			return int64(pf.PF1), nil
		case "pf2":
			return int64(pf.PF2), nil
		case "foreground_color":
			return int64(pf.ForegroundColor), nil
		case "background_color":
			return int64(pf.BackgroundColor), nil
		case "ctrlpf":
			return int64(pf.Ctrlpf), nil
		case "reflected":
			return b2i(pf.Reflected), nil
		}
	case "ball":
		switch parts[2] {
		case "color":
			return int64(r.Ball.Color), nil
		case "enabled":
			return b2i(r.Ball.Enabled), nil
		}
	}
	return 0, fmt.Errorf("unknown tiareg field %q.%q", parts[1], parts[2])
}

func resolveCollision(e *emu.Emu, parts []string) (int64, error) {
	if len(parts) != 2 {
		return 0, fmt.Errorf("collisions field needs collisions.<pair>")
	}
	c, err := e.ReadCollisions()
	if err != nil {
		return 0, err
	}
	m := map[string]bool{
		"p0_p1": c.P0P1, "m0_m1": c.M0M1,
		"m0_p0": c.M0P0, "m0_p1": c.M0P1, "m1_p0": c.M1P0, "m1_p1": c.M1P1,
		"p0_pf": c.P0PF, "p0_bl": c.P0BL, "p1_pf": c.P1PF, "p1_bl": c.P1BL,
		"m0_pf": c.M0PF, "m0_bl": c.M0BL, "m1_pf": c.M1PF, "m1_bl": c.M1BL,
		"bl_pf": c.BLPF,
	}
	v, ok := m[parts[1]]
	if !ok {
		return 0, fmt.Errorf("unknown collision pair %q", parts[1])
	}
	return b2i(v), nil
}

func resolveAudio(e *emu.Emu, parts []string) (int64, error) {
	if len(parts) != 3 {
		return 0, fmt.Errorf("audio field needs audio.<ch0|ch1>.<control|freq|volume>")
	}
	a := e.ReadAudio()
	ch := a.Channel0
	switch parts[1] {
	case "ch0":
		ch = a.Channel0
	case "ch1":
		ch = a.Channel1
	default:
		return 0, fmt.Errorf("unknown audio channel %q", parts[1])
	}
	switch parts[2] {
	case "control":
		return int64(ch.Control), nil
	case "freq":
		return int64(ch.Freq), nil
	case "volume":
		return int64(ch.Volume), nil
	default:
		return 0, fmt.Errorf("unknown audio field %q", parts[2])
	}
}
