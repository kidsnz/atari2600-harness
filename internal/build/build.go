// Package build は DASM でのアセンブルを 1 関数に集約する（assemble_and_load ツールと
// シナリオの .asm 直指定で共有＝欠落E のビルドループ短縮）。
package build

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
func Assemble(asmPath, binPath string) (output string, err error) {
	tmp := scratchPath(binPath, "bin")
	out, err := exec.Command("dasm", asmPath, "-f3", "-o"+tmp, "-I"+filepath.Dir(asmPath)).CombinedOutput()
	if err == nil {
		err = diagnosedFailure(string(out))
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
	if err != nil {
		os.Remove(tmpBin)
		os.Remove(lstPath)
		os.Remove(symPath)
		return string(out), "", "", err
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
func diagnosedFailure(out string) error {
	for _, ln := range strings.Split(out, "\n") {
		if strings.Contains(ln, "error:") {
			return fmt.Errorf("dasm reported an error but exited 0: %s", strings.TrimSpace(ln))
		}
	}
	return nil
}
