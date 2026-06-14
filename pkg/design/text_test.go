package design

import "testing"

// TestMaxChars は技法ごとの上限（48px=12 / venetian=32）。〔197162〕
func TestMaxChars(t *testing.T) {
	if got := MaxChars(Text48px); got != 12 {
		t.Errorf("Text48px MaxChars=%d want 12", got)
	}
	if got := MaxChars(TextVenetian); got != 32 {
		t.Errorf("TextVenetian MaxChars=%d want 32", got)
	}
}

// TestFitsText は境界（ちょうど上限OK・超過NG・負はNG）。
func TestFitsText(t *testing.T) {
	cases := []struct {
		name string
		n    int
		tech TextTechnique
		want bool
	}{
		{"48px_12_ok", 12, Text48px, true},
		{"48px_13_over", 13, Text48px, false},
		{"venetian_32_ok", 32, TextVenetian, true},
		{"venetian_33_over", 33, TextVenetian, false},
		{"negative", -1, Text48px, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := FitsText(c.n, c.tech); got != c.want {
				t.Errorf("FitsText(%d,%v)=%v want %v", c.n, c.tech, got, c.want)
			}
		})
	}
}
