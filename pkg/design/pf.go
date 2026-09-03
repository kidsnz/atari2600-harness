package design

import "github.com/kidsnz/atari2600-harness/pkg/playfield"

// ColorClockPerColumn は PF 1 列の幅（color clock）。列数は playfield.FullWidth(=40) を再利用する。
// 〔design-principles.md「横 40px × 4clk/px」/ Davie S13〕
const ColorClockPerColumn = 4

// PFTotalColorClocks は PF 全幅の color clock 数（40 列 × 4 = 160 = 可視幅）。
func PFTotalColorClocks() int { return playfield.FullWidth * ColorClockPerColumn }

// CTRLPFScoreBit は CTRLPF の score ビット(D1)。立てると左半PF=COLUP0・右半PF=COLUP1 で
// 独立色になる＝非対称書込みのタイミング不要で「タダの2色PF」。〔design-principles.md / w11〕
const CTRLPFScoreBit = 0x02

// ScoreModeTwoColor は CTRLPF 値が score ビットを立てて左右2色PFになっているかを返す。
func ScoreModeTwoColor(ctrlpf byte) bool { return ctrlpf&CTRLPFScoreBit != 0 }

// ★2026-09-03: この定数を足したとき、★直前にあった ScrollScanlinesConstant の注釈との
// あいだに空行を入れ忘れ、★★注釈が融合した。★`go doc RAM2600` が関数の説明を出し、
// ★★★`go doc ScrollScanlinesConstant` は【1行も出さない】状態だった。
// ★`go build` は exit=0 なので、★★ビルドでは捕まらない種類の事故。★helper-3 が
// ★Go 自身の道具（`go doc`）で見つけた。★注釈の直前には必ず空行を置くこと。
//
// RAM2600 は 2600 の内蔵 RAM の総バイト数（$80–$FF）。★スタックはこの中から上に向かって
// 積まれるので、変数と共有する。〔emu.RAMSize と同じ量。ここは design 側の定数〕
const RAM2600 = 128

// ScrollBackgroundFitsRAM は「スクロール背景の3層（盤面 + 表示バッファ + 差分）」が
// ★内蔵 128 バイトに収まるかを判定する。
//
// ★なぜ要るか（2026-09-03・蒸留が見つけた穴）: `design-principles.md` はこの3層構造を
// 規定して `ScrollScanlinesConstant` を指していたが、★★その関数は走査線数と PAL の偶奇
// しか見ていない。★★★出典 〔200972:14〕 は逆に **「実行中に書き換えたい広域は
// SuperChip/CBS RAM に置く必要があり、内蔵 128 バイト RAM では小さい世界（120 byte 級）
// しか malleable にできない」** と書いている——★3層を数えておいて、それが RAM に入るか
// を誰も検査していなかった。
//
// stackBytes は呼び出しの深さぶん（1段 2 バイト）＋割り込みは無いので純粋に JSR の深さ。
// ★false のときは SuperChip/CBS RAM が要る、というのがこの関数の言っていること。
func ScrollBackgroundFitsRAM(boardBytes, bufferBytes, deltaBytes, stackBytes int) bool {
	if boardBytes < 0 || bufferBytes < 0 || deltaBytes < 0 || stackBytes < 0 {
		return false
	}
	return boardBytes+bufferBytes+deltaBytes+stackBytes <= RAM2600
}

// ScrollScanlinesConstant は縦/横スクロール背景の鉄則「総スキャンライン数をフレーム間で
// 一定に保つ」を判定する。frameLines は各フレームの総ライン数。pal=true なら各フレームが
// 偶数ラインであることも要求する（PAL は偶数必須）。〔design-principles.md / 採掘 200972〕
func ScrollScanlinesConstant(frameLines []int, pal bool) bool {
	if len(frameLines) == 0 {
		return true
	}
	first := frameLines[0]
	for _, n := range frameLines {
		if n != first {
			return false
		}
		if pal && n%2 != 0 {
			return false
		}
	}
	return true
}

// PFReg は playfield レジスタ（PF0/PF1/PF2）。
type PFReg int

const (
	PF0 PFReg = iota
	PF1
	PF2
)

// AsymRightWindow は非対称PFで「右半分の値を同一走査線内に書き直す」時の、各レジスタの
// 安全な書込みサイクル窓 [start,end]（WSYNC=cycle 0 基準・repeated モード）を返す。
// repeated: RPF0 27–48 / RPF1 37–53 / RPF2 48–64。reflected モードは RPF2 を「ちょうど 48」で
// 完了させる（STA 3cy なら begin=45。design-principles の "PF2 begin=cy45" と整合）。
// 出典: woodgrain Playfield_Timing.html の definitive table（docs/fundamentals-audit.md:66-69）。
// 備考: これは「文書化された権威テーブル」由来。我々の litmus_pf_async は左右窓の一部のみ実機ロック済。
func AsymRightWindow(reg PFReg) (start, end int) {
	switch reg {
	case PF0:
		return 27, 48
	case PF1:
		return 37, 53
	case PF2:
		return 48, 64
	}
	return 0, 0
}

// FitsAsymRightWrite は右半 PF 再書込みを cycle で行うのが窓内に収まるかを返す。
func FitsAsymRightWrite(reg PFReg, cycle int) bool {
	s, e := AsymRightWindow(reg)
	return cycle >= s && cycle <= e
}
