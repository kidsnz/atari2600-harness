// Package build は DASM でのアセンブルを 1 関数に集約する（assemble_and_load ツールと
// シナリオの .asm 直指定で共有＝欠落E のビルドループ短縮）。
package build

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// BinPathFor は .asm パスから既定の .bin 出力パスを返す（拡張子を .bin に）。
func BinPathFor(asmPath string) string {
	return strings.TrimSuffix(asmPath, filepath.Ext(asmPath)) + ".bin"
}

// Assemble は dasm -f3（生バイナリ出力）で asmPath を binPath にアセンブルする。
// 失敗行を含む診断のため stdout+stderr を output で返す（成功時も dasm の "Complete." を含む）。
// `-I<asm のディレクトリ>` を渡すので、cwd に関わらず `include vcs.h` 等が asm 自身の隣から解決する
// （別ディレクトリから .asm シナリオを走らせても通る）。
//
// A crash is rejected but not explained. Measured outside this repository on DASM 2.20.14.1
// (2026-08-28, by bisection, and again on 2026-09-18): one `.byte` line of 129 values (522
// characters) kills DASM with SIGSEGV -- nothing printed, exit 139 in a shell, a 0-byte output
// file left behind -- while 128 values (518 characters) assemble. err is non-nil, so the scratch
// file is removed and nothing is accepted, but output is empty, so the caller gets no line to look
// at. In 2004 the same family showed up as misleading errors on a macro call
// longer than 255 characters (Manuel Rotschkar, stella-list `200410/msg00086`; Thomas Mathys
// pointed at `#define MAXLINE 256` in DASM's main.c, `msg00090`). A generator that writes long
// table rows should fold them. No test here reproduces it -- Not verified in this repository.
func Assemble(asmPath, binPath string) (output string, err error) {
	tmp := scratchPath(binPath, "bin")
	out, err := exec.Command("dasm", asmPath, "-f3", "-o"+tmp, "-I"+filepath.Dir(asmPath)).CombinedOutput()
	if err == nil {
		err = diagnosedFailure(string(out))
	}
	if err == nil {
		err = quietlyIncomplete(string(out), tmp)
	}
	if err != nil {
		os.Remove(tmp)
		return string(out), err
	}
	if err := os.Rename(tmp, binPath); err != nil {
		os.Remove(tmp)
		return string(out), err
	}
	return string(out), nil
}

// scratchPath makes a per-process temporary name next to the real output.
//
// The output path is derived from the .asm, so two processes assembling the same
// source write the same file — and `go test ./...` runs each package as its own
// process, so any two test packages that assemble the same kernel do exactly
// that. One would then load a half-written ROM. It showed up as a test that
// failed only under full-parallel `go test` and passed alone and under -p 1,
// which reads like flakiness and is really a torn file. Writing to a private
// name and renaming makes the swap atomic: a reader sees the old complete file
// or the new one, never a partial one.
func scratchPath(binPath, ext string) string {
	return fmt.Sprintf("%s.tmp%d.%s", strings.TrimSuffix(binPath, filepath.Ext(binPath)), os.Getpid(), ext)
}

// AssembleWithListing は -l/-s 付きでアセンブルし、リスティングとシンボル表の中身も返す
// （U-M9 ソース行デバッグ用。失敗時は output に診断、lst/sym は空）。
func AssembleWithListing(asmPath, binPath string) (output, lst, sym string, err error) {
	// Same reasoning as Assemble: private names, then an atomic rename for the
	// artifact other processes read. The listing and symbol files are consumed
	// here and deleted, so they only need to be private.
	tmpBin := scratchPath(binPath, "bin")
	lstPath := scratchPath(binPath, "lst")
	symPath := scratchPath(binPath, "sym")
	out, err := exec.Command("dasm", asmPath, "-f3", "-o"+tmpBin, "-l"+lstPath, "-s"+symPath, "-I"+filepath.Dir(asmPath)).CombinedOutput()
	// The same two checks as Assemble: an exit status of zero is not taken on its own.
	if err == nil {
		err = diagnosedFailure(string(out))
	}
	if err == nil {
		err = quietlyIncomplete(string(out), tmpBin)
	}
	if err != nil {
		os.Remove(tmpBin)
		os.Remove(lstPath)
		os.Remove(symPath)
		// A missing include and a missing processor can happen together, so both may speak. The
		// storm hint counts only real 6502 instructions, so a missing macro file does not trip it.
		hints := unopenedIncludeHint(string(out)) + mnemonicStormHint(string(out))
		if hints == "" {
			hints = operandAsMnemonicHint(string(out))
		}
		return string(out) + hints, "", "", err
	}
	lb, _ := os.ReadFile(lstPath)
	sb, _ := os.ReadFile(symPath)
	os.Remove(lstPath)
	os.Remove(symPath)
	if err := os.Rename(tmpBin, binPath); err != nil {
		os.Remove(tmpBin)
		return string(out), "", "", err
	}
	return string(out), string(lb), string(sb), nil
}

// diagnosedFailure reports an assembly that DASM described as an error while still exiting zero.
//
// The exit status is normally enough — measured 2026-09-04, `error: Branch out of range` exits **3**
// on DASM 2.20.14.1, so `Assemble` already rejects it. This guard exists because the failure it
// prevents is silent and permanent, and because someone reported the opposite behaviour on the list.
// Manuel Polik, stella-list `200306/msg00003`: *"Should a source producing that `error: Branch out of
// range (135 bytes).` compile into a working binary or not? **Well I'm asking because it does...**"*
// That does not reproduce on the version we pin — but if any DASM diagnostic ever prints `error:`
// and exits zero, the `.bin` would be accepted, a `golden_frame` would be recorded **from the broken
// image**, and every run afterwards would compare the damage against itself and pass.
//
// This repository has already been bitten once by an input that failed quietly rather than loudly:
// the umbrella `CLAUDE.md` records that pointing a scenario at a `.bin` makes `prove_line_budget`
// and `pf_deadlines` SKIP, "and a skip is recorded as a PASS — Frogger's scenarios read green for
// months that way." Refusing to trust an exit status alone costs one string search.
//
// Found by the mailing-list distillation (helper-2), who could not run DASM and so reported it as a
// question with the command to settle it rather than as a defect.
//
// Two quiet outputs this guard does not see:
//
//   - A SOURCE that could not be opened. The whole output pasted in AtariAge topic/246976 is a
//     `Warning: Unable to open 'kernel.asm'`, an empty symbol list and `Complete.` -- no `error:`
//     line (the DASM version is not given). A warning is deliberately not an error here
//     (diagnosed_test.go), so quietlyIncomplete catches this one by its own wording, measured on
//     DASM 2.20.14.1.
//   - UNRESOLVED symbols (reported on AtariAge, not run here: Cited only, not verified).
//     topic/318210 (DASM 2.20.14) shows `--- 1 Unresolved Symbol` followed by `Complete. (0)`,
//     and the ROM ran in an emulator. The one symbol was NO_ILLEGAL_OPCODES, which macro.h only
//     tests with IFNCONST (internal/emu/oddsleep_test.go), so that list was harmless; a symbol
//     used as an operand ends in `Source is not resolvable` and exit 3 (unopenedIncludeHint). An
//     unresolved list is therefore not a failure mark by itself.
func diagnosedFailure(out string) error {
	for _, ln := range strings.Split(out, "\n") {
		if strings.Contains(ln, "error:") {
			return fmt.Errorf("dasm reported an error but exited 0: %s", strings.TrimSpace(ln))
		}
	}
	return nil
}

// quietlyIncomplete reports an image DASM did not finish although it exited zero.
//
// Measured on DASM 2.20.14.1 (2026-10-02, scratch builds outside this repository, the same flags as
// Assemble; everything goes to stdout, and `-v0` to `-v2` add only pass and segment tables). Each
// of these ends `Complete. (0)` with exit 0:
//
//	incbin "gfx.dat" that cannot be opened -> `unable to open gfx.dat`, a 4096-byte image without
//	                                          the table (lower case, no quotes, no "Warning:")
//	a source file that does not exist      -> `Warning: Unable to open 'x.asm'`, a 0-byte image
//	an include nothing uses, missing       -> `Warning: Unable to open 'x.h'`, a 4096-byte image
//	`SEG.U` left open into the code        -> no message at all, a 0-byte image
//
// The first three are refused by DASM's own wording, the last by the size. An include that IS used
// also prints the warning but ends in exit 3 (unopenedIncludeHint), so this only adds the case where
// nothing went unresolved -- and a header holding code rather than equates would be missing from the
// image just as quietly, so a missing include is refused whether or not anything refers to it.
//
// Two messages that look alike are left alone. `Warning: Unable to [re]open 'x.bin'` is the OUTPUT
// file, and DASM already exits 2 on it. `Warning: Unable to open Symbol Dump file 'x.sym'` exits 0
// with a complete image; only the symbol table is missing.
//
// A 0-byte image is refused whatever the output says: it is not a cartridge. Smaller-than-a-cartridge
// images are not refused, because -f3 of a fragment is legitimately short. No image at all is named
// here too: an old DASM exited 0 and wrote nothing (AtariAge topic/111264, cured by moving to 2.20.07;
// Cited only, not verified). The rename after this would fail on it anyway, but with an error that
// names the scratch file rather than DASM.
//
// No false refusals, measured the same day with DASM directly: all 211 `.asm` files in this
// repository outside Gopher2600/ exit 0 with a non-empty image and none of these messages, and so do
// the 155 `.asm` files that the author's works name in their scenarios.
func quietlyIncomplete(out, binPath string) error {
	for _, re := range []*regexp.Regexp{unopenedIncludeRe, unopenedIncbinRe} {
		if m := re.FindStringSubmatch(out); m != nil {
			return fmt.Errorf("dasm exited 0 but could not open %q, so the image is missing what that "+
				"file holds: %s", strings.TrimSpace(m[1]), strings.TrimSpace(m[0]))
		}
	}
	fi, err := os.Stat(binPath)
	if os.IsNotExist(err) {
		return fmt.Errorf("dasm exited 0 but wrote no image at all")
	}
	if err != nil {
		return err
	}
	if fi.Size() == 0 {
		return fmt.Errorf("dasm exited 0 but wrote an image of 0 bytes. One cause that prints no " +
			"message is a `SEG.U` left open into the code: end the header that opens it with a plain `SEG`")
	}
	return nil
}

// unopenedIncbinRe matches the line DASM prints for an `incbin` it cannot open. It is a different
// message from an include's: lower case, the name unquoted, and no "Warning:" (DASM 2.20.14.1).
var unopenedIncbinRe = regexp.MustCompile(`(?m)^unable to open (.+)$`)

// mnemonicStormHint names the cause when DASM rejects the whole instruction set at once.
//
// A source with no working `processor` directive assembles nothing, and DASM reports it by calling
// every instruction it meets an unknown mnemonic:
//
//	x.asm (4): error: Unknown Mnemonic 'lda'.
//	x.asm (5): error: Unknown Mnemonic 'sta'.
//	x.asm (6): error: Unknown Mnemonic 'jmp'.
//
// **Nothing in that output names the cause**, which is one character on line 1. Measured on DASM
// 2.20.14.1, both ways of losing the directive produce the same storm:
//
//	`processor` in COLUMN 1   -> 4 unknown mnemonics, exit 5. DASM reads `processor` as a LABEL and
//	                             `6502` as the mnemonic, so line 1 appears as `Unknown Mnemonic '6502'`
//	                             -- a clue that names the argument rather than the mistake.
//	`processor` line MISSING  -> 3 unknown mnemonics, exit 5, and **no mention of line 1 at all**.
//
// The exit status is non-zero either way, so nothing silently succeeds; what is lost is time. The
// symptom points at the instruction set and the cause is the first line, and the two look nothing
// alike. Manuel Polik diagnosed it on the list in 2001 for someone who had pasted a source into an
// email: *"You need to TAB both lines for DASM. Now it's assuming 'processor' as label and '6502' as
// mnemonic"* 〔stella-list `200102/msg00253`〕. The mailing-list distillation (helper-1) lost an
// afternoon to the second form on 2026-09-07 -- eighteen illegal-opcode probes failed, and so did the
// `lda #$01` negative control, which is what finally gave it away.
//
// The threshold is three because a real source can have one or two genuine typos; a run of three
// unrecognised mnemonics is not a typo, it is a missing processor.
//
// ★Only tokens that start with a LETTER are counted (2026-09-29). A data row that lost its `.byte`
// storms too — three indented rows of `%00011000` give three unknown mnemonics and exit 5 — but the
// tokens it names are values, and sending that reader to line 1 was wrong. Those are left to
// operandAsMnemonicHint. The column-1 form still counts: `6502` is a value, but `lda`, `sta` and
// `jmp` are three instructions.
//
// ★And only the 6502's own instruction names (2026-09-30). A missing `macro.h` rejects
// `VERTICAL_SYNC` and `SLEEP` -- letter-led, but macros, and the cause is the include, not line 1.
// A two-letter typo (`lxx`, `stx2`) is not an instruction name either, so it still cannot count.
func mnemonicStormHint(out string) string {
	var n int
	for _, m := range unknownMnemonicRe.FindAllStringSubmatch(out, -1) {
		if opcodes6502[strings.ToLower(m[1])] {
			n++
		}
	}
	if n < 3 {
		return ""
	}
	return fmt.Sprintf("\n\nhint: %d instructions were rejected as unknown mnemonics. That is not a "+
		"typo -- it is a source with no active `processor 6502` directive. Check line 1: the "+
		"directive must be PRESENT and must be INDENTED. In column 1 DASM reads it as a label and "+
		"the CPU is never selected. 〔stella-list 200102/msg00253〕", n)
}

// operandAsMnemonicHint names the cause when DASM rejects an OPERAND as if it were an instruction.
//
// A single instruction that lost its indentation produces one error and points at a token that is
// not an instruction anywhere:
//
//	x.asm (5): error: Unknown Mnemonic '#0'.
//
// DASM reads the first field of a line as a label unless the line is indented. So `lda #0` in column
// 1 becomes the label `lda` and the mnemonic `#0`. ★Measured 2026-09-07: this is a **different
// signature** from a missing `processor` directive, which produces a storm of ordinary-looking
// mnemonics — here there is exactly ONE error and, for this case, the token it names begins with `#` or `$`.
//
// ★That shape has TWO causes, not one (corrected 2026-09-29). A value reaches the mnemonic position
// either because the field before it was eaten as a label (the line starts in column 1), or because
// it is the first field of an indented DATA row that lost its `.byte`: `\t$AA` gives
// `Unknown Mnemonic '$AA'`, `\t#%00000000` gives `'%00000000'` (DASM drops the `#`), and a labelled
// row `Gfx #%00000000` gives `'#%00000000'` (AtariAge topic/72751). DASM's output does not tell the
// two causes apart, so the hint names both. Values that start with `%` or a digit are matched too.
//
// The mailing list has the same diagnosis from 2001, for the `processor` line rather than an
// instruction: *"You need to TAB both lines for DASM. Now it's assuming 'processor' as label and
// '6502' as mnemonic"* 〔stella-list `200102/msg00253`, Manuel Polik〕. It is one mistake with two
// error messages, and neither of them says "indentation".
func operandAsMnemonicHint(out string) string {
	for _, m := range unknownMnemonicRe.FindAllStringSubmatch(out, -1) {
		if strings.ContainsRune("#$%0123456789", rune(m[1][0])) {
			return fmt.Sprintf("\n\nhint: `%s` is a VALUE, not an instruction. Two mistakes put a value "+
				"where DASM expects an instruction: the line starts in column 1, so the field before "+
				"it was read as a LABEL (indent it), or it is a data row that lost its `.byte` (add "+
				"it).", m[1])
		}
	}
	return ""
}

// opcodes6502 is the documented instruction set: the names a source with no `processor` line still
// uses, and so the names DASM rejects when the CPU was never selected.
var opcodes6502 = func() map[string]bool {
	m := map[string]bool{}
	for _, op := range strings.Fields("adc and asl bcc bcs beq bit bmi bne bpl brk bvc bvs clc cld cli clv " +
		"cmp cpx cpy dec dex dey eor inc inx iny jmp jsr lda ldx ldy lsr nop ora pha php pla plp rol " +
		"ror rti rts sbc sec sed sei sta stx sty tax tay tsx txa txs tya") {
		m[op] = true
	}
	return m
}()

// unknownMnemonicRe captures the token DASM names in each "Unknown Mnemonic" error.
var unknownMnemonicRe = regexp.MustCompile(`Unknown Mnemonic '([^']+)'`)

// unopenedIncludeHint names the cause when DASM could not open an include file.
//
// DASM does not stop. It prints a warning at the TOP and keeps going without the file, and what fills
// the screen below is the damage, not the cause. Measured on DASM 2.20.14.1:
//
//	Warning: Unable to open 'vcs.h'          <- the cause (once per pass)
//	--- Unresolved Symbol List
//	WSYNC                    0000 ????         (R )
//	Fatal assembly error: Source is not resolvable.   (exit 3)
//
// A missing `macro.h` adds `Unknown Mnemonic 'VERTICAL_SYNC'`, `'SLEEP'`, `'SLEEP'` -- three
// letter-led tokens, but not 6502 instructions, so mnemonicStormHint stays quiet. When the
// `processor` line is missing as well, both hints speak.
// The negative control: a source that merely uses an undefined symbol prints the same list with
// nothing above it, and gets no hint. (An include nothing uses gives the same warning and exit 0;
// quietlyIncomplete refuses that build, and on AssembleWithListing this hint then names the file.)
//
// The shape -- the list is the consequence, the cause is above it -- is AtariAge topic/72751 (the
// real error sat above "12 references to unknown symbols") and topic/287020.
func unopenedIncludeHint(out string) string {
	var names []string
	for _, m := range unopenedIncludeRe.FindAllStringSubmatch(out, -1) {
		if !slices.Contains(names, m[1]) {
			names = append(names, m[1])
		}
	}
	if len(names) == 0 {
		return ""
	}
	return fmt.Sprintf("\n\nhint: DASM could not open `%s` (the warning at the top). Unresolved "+
		"symbols, and unknown mnemonics that are macro names, are what that file would have defined. Put the "+
		"file next to the .asm (the build adds that directory with -I) or fix the name.",
		strings.Join(names, "`, `"))
}

// unopenedIncludeRe matches the warning DASM prints for an `include` it cannot open. It does not match
// the output file's `Unable to [re]open`, whose message already names the file and the cause.
var unopenedIncludeRe = regexp.MustCompile(`Unable to open '([^']+)'`)
