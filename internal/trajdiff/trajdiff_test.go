package trajdiff

import (
	"os"
	"path/filepath"
	"testing"
)

const smoke = "../../roms/litmus/smoke.bin"

// mutant は smoke.bin の 1 バイトを書き換えた一時 ROM を作ってパスを返す。
func mutant(t *testing.T, off int, xor byte) string {
	t.Helper()
	b, err := os.ReadFile(smoke)
	if err != nil {
		t.Fatal(err)
	}
	b[off] ^= xor
	p := filepath.Join(t.TempDir(), "mutant.bin")
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// TestTrajdiffSelfTest は VV-8 の falsifiable 自己テスト：
//  1. 同一 ROM 同士は必ず MATCH（決定性ガードも兼ねる）。
//  2. reset ベクタを壊すと実行経路が変わり、RAM 軌跡が必ず発散する（diff は挙動に敏感）。
//  3. 実行・参照されないバイトを変えても MATCH（diff はバイトでなく挙動を見る）。
func TestTrajdiffSelfTest(t *testing.T) {
	o := Options{Frames: 4, Warmup: 2}

	// 1) identity = MATCH
	if r, err := Compare(smoke, smoke, o); err != nil {
		t.Fatal(err)
	} else if !r.Match {
		t.Fatalf("identity must MATCH, diverged: %+v", r.Diverged)
	}

	// 2) reset ベクタ(0xFFFC) の下位バイトを壊す → 別のエントリから実行 → 発散。
	b, err := os.ReadFile(smoke)
	if err != nil {
		t.Fatal(err)
	}
	resetLo := len(b) - 4 // 0xFFFC = ROM 末尾から 4 バイト目
	rv := mutant(t, resetLo, 0x20)
	if r, err := Compare(smoke, rv, o); err != nil {
		t.Fatal(err)
	} else if r.Match {
		t.Fatalf("a corrupted reset vector must diverge, got MATCH")
	} else if r.Diverged == nil {
		t.Fatalf("expected a divergence record")
	}

	// 3) 挙動に効かない(=デッド)バイトの変更は MATCH。窓内に必ず 1 つは存在するはず。
	foundDead := false
	for off := 0x20; off < 0x20+40 && !foundDead; off++ {
		m := mutant(t, off, 0x01)
		r, err := Compare(smoke, m, o)
		if err != nil {
			t.Fatal(err)
		}
		if r.Match {
			foundDead = true
		}
	}
	if !foundDead {
		t.Fatalf("expected at least one behaviorally-dead byte (flip -> MATCH) in the scan window")
	}
}
