package build

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMnemonicStormNamesTheCause drives real DASM through both ways of losing the `processor`
// directive and requires the hint to appear, then requires it to stay away from sources that
// merely have a typo or two. See mnemonicStormHint for the measurement and the 2001 source.
func TestMnemonicStormNamesTheCause(t *testing.T) {
	const body = "\torg $F000\nStart\n\tlda #$00\n\tsta $80\n\tjmp Start\n\torg $FFFC\n\t.word Start\n\t.word Start\n"

	cases := []struct {
		name string
		src  string
		want bool
	}{
		{"processor in column 1", "processor 6502\n" + body, true},
		{"processor line missing", body, true},
		{"correct source", "\tprocessor 6502\n" + body, false},
		// The counter-bait the threshold exists for: a source that IS well formed and has two
		// genuine typos. Three is the line between "someone mistyped" and "the CPU was never
		// selected", and a hint that fires here would send a reader to line 1 for nothing.
		{"two real typos", "\tprocessor 6502\n\torg $F000\nStart\n\tlxx #$00\n\tstx2 $80\n\tjmp Start\n" +
			"\torg $FFFC\n\t.word Start\n\t.word Start\n", false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			asm := filepath.Join(dir, "t.asm")
			if err := os.WriteFile(asm, []byte(c.src), 0o644); err != nil {
				t.Fatal(err)
			}
			out, _, _, err := AssembleWithListing(asm, filepath.Join(dir, "t.bin"))
			if c.want && err == nil {
				t.Fatalf("this source cannot assemble, but Assemble returned no error:\n%s", out)
			}
			got := strings.Contains(out, "no active `processor 6502` directive")
			if got != c.want {
				t.Errorf("hint present = %v, want %v. DASM said:\n%s", got, c.want, out)
			}
		})
	}
}

// TestMnemonicStormThreshold pins the boundary itself, without DASM, so a later edit to the count
// cannot slide past the integration test above by one.
func TestMnemonicStormThreshold(t *testing.T) {
	line := "x.asm (4): error: Unknown Mnemonic 'lda'.\n"
	for n := 0; n <= 4; n++ {
		hint := mnemonicStormHint(strings.Repeat(line, n))
		if want := n >= 3; (hint != "") != want {
			t.Errorf("%d unknown mnemonics: hint=%v, want %v", n, hint != "", want)
		}
	}
	// And it must not fire on an unrelated storm of errors.
	other := strings.Repeat("x.asm (4): error: Branch out of range (200 bytes).\n", 9)
	if mnemonicStormHint(other) != "" {
		t.Error("fired on nine branch-range errors — the hint claims a specific cause and must " +
			"only speak when that cause is the one DASM's output describes")
	}
}
