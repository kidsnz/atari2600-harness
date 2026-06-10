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
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/kidsnz/atari2600-harness/internal/build"
	"github.com/kidsnz/atari2600-harness/internal/emu"
)

// Input はあるフレームで与えるジョイスティック操作（D-2: 入力タイムライン）。
type Input struct {
	Frame   int    `json:"frame"`   // シナリオ開始（warmup 後）からの 0 起点フレーム。このフレームを走らせる前に適用
	Player  int    `json:"player"`  // 0=P0/左, 1=P1/右
	Action  string `json:"action"`  // left|right|up|down|fire|center
	Pressed bool   `json:"pressed"` // 押下保持/解除（center では無視）
}

// Assert はあるフレーム終了時点の瞬時数値条件（D-1）。副作用のある計測は Checks 側で扱う。
type Assert struct {
	AtFrame int    `json:"at_frame"` // このフレームを走らせた直後に評価
	Field   string `json:"field"`    // 語彙（下記 resolve 参照）
	Op      string `json:"op"`       // == != < <= > >=
	Value   int64  `json:"value"`    // 比較値（bool は 0/1）
}

// Checks は run 全体に対する性質（副作用＝フレームを進める計測なのでタイムライン後にまとめて評価）。
type Checks struct {
	NTSCFrameLines *int `json:"ntsc_frame_lines,omitempty"` // StepFrame() == この値（NTSC は 262）
	MaxLineBudget  *int `json:"max_line_budget,omitempty"`  // RunUntilBudget が超過しない（既定予算 76）
	GoldenFrame    bool `json:"golden_frame,omitempty"`     // D-3: タイムラインの描画連鎖ハッシュを <scenario>.golden と照合
	GoldenAudio    bool `json:"golden_audio,omitempty"`     // A-2: タイムラインの音声連鎖ハッシュを <scenario>.audio.golden と照合
}

// Scenario は 1 本のシナリオ定義。
type Scenario struct {
	Rom          string   `json:"rom"`
	TVSpec       string   `json:"tv_spec,omitempty"`       // 省略時 NTSC
	WarmupFrames int      `json:"warmup_frames,omitempty"` // 計測前の空走（省略時 2）
	Inputs       []Input  `json:"inputs,omitempty"`
	Asserts      []Assert `json:"asserts,omitempty"`
	Checks       *Checks  `json:"checks,omitempty"`

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
	GoldenHash string // golden_frame 指定時に算出した描画連鎖ハッシュ（決定性確認用）
	AudioHash  string // golden_audio 指定時に算出した音声連鎖ハッシュ
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

	res := &Result{Pass: true}
	record := func(a Assert, got int64, ok bool, evalErr error) error {
		if evalErr != nil {
			return evalErr
		}
		desc := fmt.Sprintf("%s %s %d", a.Field, a.Op, a.Value)
		res.Asserts = append(res.Asserts, AssertResult{Desc: desc, Got: got, Pass: ok})
		if !ok {
			res.Pass = false
		}
		return nil
	}

	for f := 0; f <= maxF; f++ {
		for _, in := range inByFrame[f] { // このフレームを走らせる前に入力適用
			if err := e.SetInput(in.Player, in.Action, in.Pressed); err != nil {
				return nil, fmt.Errorf("frame %d input: %w", f, err)
			}
		}
		if err := e.RunFrames(1); err != nil {
			return nil, err
		}
		for _, a := range asByFrame[f] { // フレーム終了時点で瞬時評価
			got, err := resolve(e, a.Field)
			if err != nil {
				return nil, fmt.Errorf("frame %d: %w", f, err)
			}
			ok, err := compare(got, a.Op, a.Value)
			if err := record(a, got, ok, err); err != nil {
				return nil, err
			}
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
		return false, fmt.Errorf("unknown op %q (want == != < <= > >=)", op)
	}
}

func b2i(b bool) int64 {
	if b {
		return 1
	}
	return 0
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
