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
