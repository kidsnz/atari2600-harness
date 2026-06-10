// Package build は DASM でのアセンブルを 1 関数に集約する（assemble_and_load ツールと
// シナリオの .asm 直指定で共有＝欠落E のビルドループ短縮）。
package build

import (
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
func Assemble(asmPath, binPath string) (output string, err error) {
	out, err := exec.Command("dasm", asmPath, "-f3", "-o"+binPath).CombinedOutput()
	return string(out), err
}
