package build

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDiagnosedFailure covers the guard that refuses an assembly DASM called an error while exiting
// zero. See `diagnosedFailure`'s comment for why a zero exit is not trusted on its own.
func TestDiagnosedFailure(t *testing.T) {
	if err := diagnosedFailure("\nComplete. (0)\n"); err != nil {
		t.Errorf("a clean run must pass: %v", err)
	}
	// DASM's real wording, from the 2003 report.
	bad := "\nbad.asm (5): error: Branch out of range (200 bytes).\n\nComplete. (0)\n"
	err := diagnosedFailure(bad)
	if err == nil {
		t.Fatal("an output containing `error:` must be rejected even when dasm exits 0 — that is " +
			"the whole point of the guard")
	}
	if !strings.Contains(err.Error(), "Branch out of range") {
		t.Errorf("the error should quote the diagnostic so the caller can see it: %v", err)
	}
	// A warning is not an error, and must not be escalated: DASM prints warnings on healthy builds.
	if err := diagnosedFailure("bad.asm (5): warning: something\nComplete. (0)\n"); err != nil {
		t.Errorf("a warning must not fail the build: %v", err)
	}
}

// TestAssembleRejectsBranchOutOfRange is the end-to-end half: it builds a source with a branch
// beyond ±127 and requires Assemble to refuse it AND to leave no .bin behind. Measured on DASM
// 2.20.14.1, that source exits 3, so this passes through the exit-status path rather than the
// guard above — which is exactly what should be recorded, because the two together are what make
// "a broken image can never become a golden" true whichever way DASM behaves.
func TestAssembleRejectsBranchOutOfRange(t *testing.T) {
	dir := t.TempDir()
	asm := filepath.Join(dir, "far.asm")
	var b strings.Builder
	b.WriteString("        processor 6502\n        org $F000\nStart:\n        lda #0\n        bne Far\n")
	for i := 0; i < 200; i++ {
		b.WriteString("        nop\n")
	}
	b.WriteString("Far:\n        jmp Start\n        org $FFFC\n        .word Start\n        .word Start\n")
	if err := os.WriteFile(asm, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "far.bin")
	out, err := Assemble(asm, bin)
	if err == nil {
		t.Fatalf("a branch 200 bytes out of range must not assemble; dasm said: %s", out)
	}
	if !strings.Contains(out, "Branch out of range") {
		t.Errorf("expected DASM to name the fault, got: %s", out)
	}
	if _, statErr := os.Stat(bin); statErr == nil {
		t.Error("a rejected assembly must leave no .bin — otherwise a later step can pick up the " +
			"partial image and a golden gets recorded from it")
	}
}

// TestAssembleRejectsQuietlyIncompleteImages drives real DASM through the inputs it does not finish
// but still exits 0 on, and requires both Assemble and AssembleWithListing to refuse each one, name
// the cause, and leave no .bin behind. Each has a negative control that differs only in the missing
// piece being present, so a check that refused everything would fail the controls. Measured on DASM
// 2.20.14.1 (2026-10-02), every bad case below prints `Complete. (0)` and exits 0; see
// quietlyIncomplete for the output of each.
func TestAssembleRejectsQuietlyIncompleteImages(t *testing.T) {
	const tail = "\torg $F000\nStart\n\tjmp Start\n%s\torg $FFFC\n\t.word Start\n\t.word Start\n"
	code := func(s string) string { return strings.Replace(tail, "%s", s, 1) }

	cases := []struct {
		name  string
		src   string            // "" means the .asm is never written
		extra map[string]string // files written next to the .asm
		want  string            // what the error must name; "" means the build must succeed
	}{
		// A 4096-byte image with the table missing: the size is right, only the message tells.
		{"incbin that cannot be opened",
			"\tprocessor 6502\n" + code("Gfx\n\tincbin \"gfx.dat\"\n"), nil, "gfx.dat"},
		{"incbin that opens",
			"\tprocessor 6502\n" + code("Gfx\n\tincbin \"gfx.dat\"\n"), map[string]string{"gfx.dat": "ABC"}, ""},
		// A 0-byte image.
		{"source that does not exist", "", nil, "t.asm"},
		// A 4096-byte image. Nothing here refers to the header, so nothing goes unresolved; a header
		// that held code instead of equates would be missing from the image just as quietly.
		{"include nothing uses, missing",
			"\tprocessor 6502\n\tinclude \"unused.h\"\n" + code(""), nil, "unused.h"},
		{"include nothing uses, present",
			"\tprocessor 6502\n\tinclude \"unused.h\"\n" + code(""), map[string]string{"unused.h": "UNUSED = 1\n"}, ""},
		// A 0-byte image with no message at all (docs/known-traps.md, the zero-byte binary row).
		{"SEG.U left open into the code",
			"\tprocessor 6502\n\tSEG.U vars\n\torg $80\nFoo\tds 1\n" + code(""), nil, "0 bytes"},
		{"SEG.U closed by SEG",
			"\tprocessor 6502\n\tSEG.U vars\n\torg $80\nFoo\tds 1\n\tSEG\n" + code(""), nil, ""},
	}

	assemblers := []struct {
		name string
		run  func(asm, bin string) (string, error)
	}{
		{"Assemble", Assemble},
		{"AssembleWithListing", func(asm, bin string) (string, error) {
			out, _, _, err := AssembleWithListing(asm, bin)
			return out, err
		}},
	}

	for _, c := range cases {
		for _, a := range assemblers {
			t.Run(c.name+"/"+a.name, func(t *testing.T) {
				dir := t.TempDir()
				asm := filepath.Join(dir, "t.asm")
				if c.src != "" {
					if err := os.WriteFile(asm, []byte(c.src), 0o644); err != nil {
						t.Fatal(err)
					}
				}
				for name, body := range c.extra {
					if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
						t.Fatal(err)
					}
				}
				bin := filepath.Join(dir, "t.bin")
				out, err := a.run(asm, bin)
				left, _ := filepath.Glob(filepath.Join(dir, "*.bin"))

				if c.want == "" {
					if err != nil {
						t.Fatalf("the control must assemble, got %v. DASM said:\n%s", err, out)
					}
					if fi, serr := os.Stat(bin); serr != nil || fi.Size() != 4096 {
						t.Fatalf("the control must leave a 4096-byte .bin: %v %v", fi, serr)
					}
					// The incbin control must really carry its bytes, or it does not control anything.
					if c.extra["gfx.dat"] != "" {
						b, _ := os.ReadFile(bin)
						if string(b[3:6]) != "ABC" {
							t.Errorf("the incbin bytes are not at $F003: % x", b[:8])
						}
					}
					return
				}
				if err == nil {
					t.Fatalf("DASM did not finish this image, but it was accepted. DASM said:\n%s", out)
				}
				if !strings.Contains(err.Error(), c.want) {
					t.Errorf("the error does not name %q: %v", c.want, err)
				}
				if len(left) != 0 {
					t.Errorf("a refused image must leave no .bin behind, found %v", left)
				}
				// The listing path explains a missing include with the hint its non-zero exits get.
				if a.name == "AssembleWithListing" && c.want == "unused.h" &&
					!strings.Contains(out, "DASM could not open `unused.h`") {
					t.Errorf("the include hint is missing:\n%s", out)
				}
			})
		}
	}
}

// TestQuietlyIncompleteLeavesLookalikesAlone pins the two DASM messages quietlyIncomplete must not
// read as a missing input, and the case DASM 2.20.14.1 cannot be made to produce: exit 0 with no
// file at all.
func TestQuietlyIncompleteLeavesLookalikesAlone(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "t.bin")
	if err := os.WriteFile(bin, make([]byte, 4096), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, out := range []string{
		"\nComplete. (0)\n",
		// exit 0 with a complete image; only the symbol table is missing
		"\nComplete. (0)\nWarning: Unable to open Symbol Dump file 'nodir/x.sym'\n",
		// the output file; DASM exits 2 on it, so this function never sees it in practice
		"Warning: Unable to [re]open 'nodir/x.bin'\n",
	} {
		if err := quietlyIncomplete(out, bin); err != nil {
			t.Errorf("a complete image was refused on %q: %v", out, err)
		}
	}
	// The negative control for the loop above: the same image with an include's warning is refused.
	if err := quietlyIncomplete("Warning: Unable to open 'x.h'\n\nComplete. (0)\n", bin); err == nil {
		t.Error("an include DASM could not open must be refused even when the image is full size")
	}
	if err := quietlyIncomplete("\nComplete. (0)\n", filepath.Join(dir, "none.bin")); err == nil ||
		!strings.Contains(err.Error(), "no image") {
		t.Errorf("exit 0 with no file must be refused and say so: %v", err)
	}
}
