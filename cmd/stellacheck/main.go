// stellacheck — Stella オラクル照合（V2-17 / F-4）。
// 同じ ROM を Stella と Gopher2600（harness）で「電源投入から N フレーム」走らせ、RAM ($80-$FF) を突き合わせる。
//
// 仕組み（対話セッションで実測した Stella 7.0 の挙動に基づく）:
//  1. ~/Library/Application Support/Stella/autoexec.script に「reset / frame N / dump 80 ff 7」を書く
//     （autoexec はデバッガ突入時に自動実行される。reset で電源投入に揃うため、突入タイミングは任意でよい）。
//  2. Stella を起動 → 【人間がデバッガキー(`)を1回押す】（-debug フラグは突入しないと実測）。
//  3. dump はファイル直書き（~/Desktop/<rom>_dbg_<hash>.dump、実測）。出現をポーリング → パース。
//  4. harness 側で同じ ROM を N フレーム実行 → RAM を比較 → 一致/差分を報告。
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/kidsnz/atari2600-harness/internal/emu"
)

const stellaBin = "/Applications/Stella.app/Contents/MacOS/Stella"

func main() {
	romPath := flag.String("rom", "", "ROM (.bin) path")
	frames := flag.Int("frames", 5, "frames from power-on to compare at")
	timeout := flag.Duration("timeout", 60*time.Second, "wait for the human keypress + dump")
	dumpFile := flag.String("dump", "", "compare against an existing Stella dump file (skip launching Stella)")
	flag.Parse()
	if *romPath == "" {
		fmt.Fprintln(os.Stderr, "usage: stellacheck -rom <path> [-frames N] [-dump <file>]")
		os.Exit(2)
	}
	var err error
	if *dumpFile != "" {
		err = compare(*romPath, *frames, *dumpFile)
	} else {
		err = run(*romPath, *frames, *timeout)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
}

func run(romPath string, frames int, timeout time.Duration) error {
	home, _ := os.UserHomeDir()
	scriptDir := filepath.Join(home, "Library", "Application Support", "Stella")
	scriptPath := filepath.Join(scriptDir, "autoexec.script")
	desktop := filepath.Join(home, "Desktop")

	// 1) autoexec.script（既存があれば退避→終了時に復元）
	if err := os.MkdirAll(scriptDir, 0o755); err != nil {
		return err
	}
	backup, hadBackup := []byte(nil), false
	if b, err := os.ReadFile(scriptPath); err == nil {
		backup, hadBackup = b, true
	}
	script := fmt.Sprintf("reset\nframe %d\ndump 80 ff 7\n", frames)
	if err := os.WriteFile(scriptPath, []byte(script), 0o644); err != nil {
		return err
	}
	defer func() {
		if hadBackup {
			os.WriteFile(scriptPath, backup, 0o644)
		} else {
			os.Remove(scriptPath)
		}
	}()

	// 2) 既存 dump の把握（新規出現を検出するため）
	romBase := strings.TrimSuffix(filepath.Base(romPath), filepath.Ext(romPath))
	pattern := filepath.Join(desktop, romBase+"_dbg_*.dump")
	old := map[string]bool{}
	if m, _ := filepath.Glob(pattern); m != nil {
		for _, f := range m {
			old[f] = true
		}
	}

	// 3) Stella 起動
	cmd := exec.Command(stellaBin, romPath)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("launch stella: %w", err)
	}
	defer func() {
		cmd.Process.Kill()
		cmd.Wait()
	}()
	fmt.Println("Stella を起動しました。")
	fmt.Println("★ Stella のウィンドウで【 ` （バッククォート）キー】を1回押してデバッガに入ってください。")
	fmt.Println("  （autoexec が reset → frame", frames, "→ dump を自動実行します）")

	// 4) dump 出現を待つ
	var dumpFile string
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		time.Sleep(500 * time.Millisecond)
		m, _ := filepath.Glob(pattern)
		for _, f := range m {
			if !old[f] {
				dumpFile = f
				break
			}
		}
		if dumpFile != "" {
			break
		}
	}
	if dumpFile == "" {
		return fmt.Errorf("dump ファイルが %v 以内に出現しませんでした（キーは押しましたか?）", timeout)
	}
	time.Sleep(300 * time.Millisecond) // 書き込み完了待ち
	fmt.Println("dump 取得:", dumpFile)
	return compare(romPath, frames, dumpFile)
}

// compare は Stella の dump ファイルと harness 実行結果の RAM ($80-$FF) を突き合わせる。
func compare(romPath string, frames int, dumpFile string) error {
	stellaRAM, err := parseDump(dumpFile)
	if err != nil {
		return err
	}
	e, err := emu.New("NTSC")
	if err != nil {
		return err
	}
	if err := e.LoadROM(romPath); err != nil {
		return err
	}
	if err := e.RunFrames(frames); err != nil {
		return err
	}
	var ours [128]uint8
	for i := 0; i < 128; i++ {
		v, err := e.PeekRAM(uint16(0x80 + i))
		if err != nil {
			return err
		}
		ours[i] = v
	}
	diffs := 0
	for i := 0; i < 128; i++ {
		if ours[i] != stellaRAM[i] {
			fmt.Printf("DIFF $%02X: harness=%02X stella=%02X\n", 0x80+i, ours[i], stellaRAM[i])
			diffs++
		}
	}
	if diffs > 0 {
		return fmt.Errorf("RAM 不一致 %d バイト（frame %d）", diffs, frames)
	}
	fmt.Printf("PASS: RAM $80-$FF 全128バイトが一致（電源投入から %d フレーム, Gopher2600 vs Stella）\n", frames)
	return nil
}

var ramLine = regexp.MustCompile(`^([0-9a-f]{2}): ((?:[0-9a-f]{2} ){8})- ((?:[0-9a-f]{2} ?){8})`)

func parseDump(path string) ([128]uint8, error) {
	var ram [128]uint8
	b, err := os.ReadFile(path)
	if err != nil {
		return ram, err
	}
	seen := 0
	for _, line := range strings.Split(string(b), "\n") {
		m := ramLine.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			continue
		}
		base, err := strconv.ParseUint(m[1], 16, 16)
		if err != nil || base < 0x80 {
			continue // XC/XS 行などは除外（先頭が 80-f0 の RAM 行のみ）
		}
		hexes := strings.Fields(m[2] + " " + m[3])
		for i, h := range hexes {
			v, err := strconv.ParseUint(h, 16, 8)
			if err != nil {
				return ram, fmt.Errorf("parse %q: %w", line, err)
			}
			ram[int(base)-0x80+i] = uint8(v)
		}
		seen++
	}
	if seen != 8 {
		return ram, fmt.Errorf("RAM 行が %d 行しか見つかりません（期待 8）", seen)
	}
	return ram, nil
}
