package design

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
