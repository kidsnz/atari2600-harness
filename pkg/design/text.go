package design

// TextTechnique は文字/HUD 表示の技法。「出す文字数」で技法が決まる。
// 〔design-principles.md「HUD/テキストは出す文字数で技法が決まる」/ 採掘 197162〕
type TextTechnique int

const (
	// Text48px は 48px スプライト合成（標準幅の文字・最大12字/行）。
	Text48px TextTechnique = iota
	// TextVenetian は venetian blinds（3px 幅専用の細文字・1行おき・最大32字/行）。
	TextVenetian
)

// MaxChars は技法ごとの 1 行あたり最大文字数を返す（未知技法は 0）。
func MaxChars(t TextTechnique) int {
	switch t {
	case Text48px:
		return 12
	case TextVenetian:
		return 32
	default:
		return 0
	}
}

// FitsText は n 文字がその技法の 1 行に収まるかを返す。
func FitsText(n int, t TextTechnique) bool {
	return n >= 0 && n <= MaxChars(t)
}
