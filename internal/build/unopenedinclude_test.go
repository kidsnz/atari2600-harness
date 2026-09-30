package build

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestUnopenedIncludeNamesTheCause drives real DASM through an include it cannot open and requires
// the hint to name the file, then requires silence when the same list has no cause above it. See
// unopenedIncludeHint for the measurement.
func TestUnopenedIncludeNamesTheCause(t *testing.T) {
	const tail = "\torg $F000\nStart\n%s\tjmp Start\n\torg $FFFC\n\t.word Start\n\t.word Start\n"
	body := func(code string) string { return strings.Replace(tail, "%s", code, 1) }

	cases := []struct {
		name  string
		src   string
		extra string // written next to the .asm as "here.h" when non-empty
		want  string // the file the hint must name; "" means no hint
		storm bool   // whether the missing-processor hint must also appear
	}{
		{"missing vcs.h leaves WSYNC unresolved",
			"\tprocessor 6502\n\tinclude \"vcs.h\"\n" + body("\tsta WSYNC\n\tsta VBLANK\n"), "", "vcs.h", false},
		// The storm case: three letter-led unknown mnemonics, which on their own read as a missing
		// `processor` directive. The processor line is fine; the macro file is missing.
		{"missing macro.h storms",
			"\tprocessor 6502\n\tinclude \"macro.h\"\nWSYNC = $02\n" +
				body("\tVERTICAL_SYNC\n\tSLEEP 3\n\tSLEEP 5\n\tsta WSYNC\n"), "", "macro.h", false},
		// The negative control: the same Unresolved Symbol List with nothing above it.
		{"undefined symbol, no include", "\tprocessor 6502\n" + body("\tsta WSYNC\n"), "", "", false},
		{"include that opens",
			"\tprocessor 6502\n\tinclude \"here.h\"\n" + body("\tsta WSYNC\n"), "WSYNC = $02\n", "", false},
		// ★2026-09-30: both causes at once. The include hint alone would explain lda/sta/jmp as
		// what vcs.h should have defined; the processor line is the other half.
		{"missing include and no processor",
			"\tinclude \"vcs.h\"\n" + body("\tlda #0\n\tsta WSYNC\n"), "", "vcs.h", true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			asm := filepath.Join(dir, "t.asm")
			if err := os.WriteFile(asm, []byte(c.src), 0o644); err != nil {
				t.Fatal(err)
			}
			if c.extra != "" {
				if err := os.WriteFile(filepath.Join(dir, "here.h"), []byte(c.extra), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			out, _, _, err := AssembleWithListing(asm, filepath.Join(dir, "t.bin"))
			if c.name != "include that opens" && err == nil {
				t.Fatalf("this source cannot assemble, but it returned no error:\n%s", out)
			}
			got := strings.Contains(out, "DASM could not open")
			if got != (c.want != "") {
				t.Fatalf("hint present = %v, want %v. DASM said:\n%s", got, c.want != "", out)
			}
			if got && !strings.Contains(out, "`"+c.want+"`") {
				t.Errorf("the hint does not name %s. DASM said:\n%s", c.want, out)
			}
			if storm := strings.Contains(out, "no active `processor 6502` directive"); storm != c.storm {
				t.Errorf("missing-processor hint present = %v, want %v. DASM said:\n%s", storm, c.storm, out)
			}
		})
	}
}
