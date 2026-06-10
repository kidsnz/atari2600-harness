// Command scenario は宣言的シナリオ（入力タイムライン＋数値アサーション）を ROM に流して
// 自動 pass/fail する回帰ランナー（P2 / 欠落D）。MCP 不要で CI に乗る。
//
//	go run ./cmd/scenario <scenario.json> [more.json ...]
//
// 全アサーション pass で exit 0、1 つでも fail / エラーで exit 1。
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/kidsnz/atari2600-harness/internal/scenario"
)

func main() {
	update := flag.Bool("update", false, "(re)write golden_frame baselines instead of comparing")
	flag.Parse()
	files := flag.Args()
	if len(files) < 1 {
		fmt.Fprintln(os.Stderr, "usage: scenario [-update] <scenario.json> [more.json ...]")
		os.Exit(2)
	}

	allPass := true
	for _, path := range files {
		s, err := scenario.Load(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR %s: %v\n", path, err)
			allPass = false
			continue
		}
		res, err := scenario.Run(s, *update)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR %s: %v\n", path, err)
			allPass = false
			continue
		}

		status := "PASS"
		if !res.Pass {
			status = "FAIL"
			allPass = false
		}
		fmt.Printf("%s  %s  (%s)\n", status, path, s.Rom)
		for _, a := range res.Asserts {
			mark := "ok  "
			if !a.Pass {
				mark = "FAIL"
			}
			fmt.Printf("    %s %s   (got %d)\n", mark, a.Desc, a.Got)
		}
	}

	if !allPass {
		os.Exit(1)
	}
}
