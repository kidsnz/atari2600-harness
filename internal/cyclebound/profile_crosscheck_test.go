package cyclebound

import (
	"testing"

	"github.com/kidsnz/atari2600-harness/internal/build"
	"github.com/kidsnz/atari2600-harness/internal/emu"
)

// TestProveLinesTable は PONG-C3 の静的側＝Report.Lines が「全可視 region の完全な表」
// であることを固定する：通過 region も含まれ、MaxWorst は表の bounded 最大値と一致する。
func TestProveLinesTable(t *testing.T) {
	rep, err := Prove("../../roms/litmus/cb_clean.asm", 76)
	if err != nil {
		t.Fatal(err)
	}
	if !rep.Certified {
		t.Fatalf("cb_clean must certify; got %+v", rep)
	}
	if len(rep.Lines) == 0 {
		t.Fatal("Report.Lines is empty — the per-line table must include PASSING regions")
	}
	if len(rep.Lines) != rep.Regions-rep.Blank {
		t.Fatalf("Lines has %d rows, want %d (regions %d - blank %d)", len(rep.Lines), rep.Regions-rep.Blank, rep.Regions, rep.Blank)
	}
	max := 0
	for _, r := range rep.Lines {
		if r.Bounded && r.Worst > max {
			max = r.Worst
		}
	}
	if max != rep.MaxWorst {
		t.Fatalf("max of Lines = %d, want MaxWorst %d", max, rep.MaxWorst)
	}
}

// TestProfileMatchesProof は決定論カーネル（cb_clean）で、動的実測（emu.ProfileLineWorst）と
// 静的証明（Prove）の per-region ワーストが「同じ strobe PC で厳密一致」することを裏取りする。
// 静的の定義＝開き WSYNC 解放から閉じ WSYNC ストア完了まで。動的も同じ定義でビーム座標から
// 算出しているので、全パスを実際に踏む決定論 ROM では両者は一致しなければならない。
// （動的 ≤ 静的 は常に成立。等号は worst パスが実行された証拠＝相互検証。）
func TestProfileMatchesProof(t *testing.T) {
	const asm = "../../roms/litmus/cb_clean.asm"
	rep, err := Prove(asm, 76)
	if err != nil {
		t.Fatal(err)
	}

	e, err := emu.New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM(build.BinPathFor(asm)); err != nil {
		t.Fatal(err)
	}
	rows, _, err := e.ProfileLineWorst(4, nil)
	if err != nil {
		t.Fatal(err)
	}
	dyn := map[uint16]emu.LineWorst{}
	for _, r := range rows {
		dyn[r.StrobePC] = r
	}

	matched := 0
	for _, reg := range rep.Lines {
		if !reg.Bounded {
			continue
		}
		d, ok := dyn[reg.Start]
		if !ok {
			continue // 実行されなかった region（このROMでは想定外だが健全にスキップ）
		}
		matched++
		if d.WorstCycles > reg.Worst {
			t.Fatalf("dynamic worst %dcy EXCEEDS static proof %dcy at $%04X (%s) — one of the two is wrong",
				d.WorstCycles, reg.Worst, reg.Start, reg.StartLoc)
		}
		if d.WorstCycles != reg.Worst {
			t.Fatalf("dynamic worst %dcy != static worst %dcy at $%04X (%s) — deterministic kernel must match exactly",
				d.WorstCycles, reg.Worst, reg.Start, reg.StartLoc)
		}
	}
	if matched < 3 {
		t.Fatalf("only %d regions cross-checked — expected the kernel's visible regions to execute", matched)
	}
}
