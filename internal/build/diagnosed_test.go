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
