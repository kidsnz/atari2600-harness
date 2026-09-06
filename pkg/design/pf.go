package design

import "github.com/kidsnz/atari2600-harness/pkg/playfield"

// ColorClockPerColumn は PF 1 列の幅（color clock）。列数は playfield.FullWidth(=40) を再利用する。
// 〔design-principles.md「横 40px × 4clk/px」/ Davie S13〕
const ColorClockPerColumn = 4

// PFTotalColorClocks は PF 全幅の color clock 数（40 列 × 4 = 160 = 可視幅）。
func PFTotalColorClocks() int { return playfield.FullWidth * ColorClockPerColumn }

// CTRLPFScoreBit は CTRLPF の score ビット(D1)。立てると左半PF=COLUP0・右半PF=COLUP1 で
// 独立色になる＝非対称書込みのタイミング不要で「タダの2色PF」。〔design-principles.md / w11〕
const CTRLPFScoreBit = 0x02

// ScoreModeTwoColor は CTRLPF 値が score ビットを立てて左右2色PFになっているかを返す。
func ScoreModeTwoColor(ctrlpf byte) bool { return ctrlpf&CTRLPFScoreBit != 0 }

// ★2026-09-03: この定数を足したとき、★直前にあった ScrollScanlinesConstant の注釈との
// あいだに空行を入れ忘れ、★★注釈が融合した。★`go doc RAM2600` が関数の説明を出し、
// ★★★`go doc ScrollScanlinesConstant` は【1行も出さない】状態だった。
// ★`go build` は exit=0 なので、★★ビルドでは捕まらない種類の事故。★helper-3 が
// ★Go 自身の道具（`go doc`）で見つけた。★注釈の直前には必ず空行を置くこと。
//
// RAM2600 は 2600 の内蔵 RAM の総バイト数（$80–$FF）。★スタックはこの中から上に向かって
// 積まれるので、変数と共有する。〔emu.RAMSize と同じ量。ここは design 側の定数〕
const RAM2600 = 128

// ScrollBackgroundFitsRAM は「スクロール背景の3層（盤面 + 表示バッファ + 差分）」が
// ★内蔵 128 バイトに収まるかを判定する。
//
// ★なぜ要るか（2026-09-03・蒸留が見つけた穴）: `design-principles.md` はこの3層構造を
// 規定して `ScrollScanlinesConstant` を指していたが、★★その関数は走査線数と PAL の偶奇
// しか見ていない。★★★出典 〔200972:14〕 は逆に **「実行中に書き換えたい広域は
// SuperChip/CBS RAM に置く必要があり、内蔵 128 バイト RAM では小さい世界（120 byte 級）
// しか malleable にできない」** と書いている——★3層を数えておいて、それが RAM に入るか
// を誰も検査していなかった。
//
// stackBytes は呼び出しの深さぶん（1段 2 バイト）＋割り込みは無いので純粋に JSR の深さ。
//
// ★boardBytes は cells × bitsPerCell ÷ 8。★★2026-09-06 まで、この関数は四つの数を足すだけで
// 「どう数えるのか」を一行も書いていなかった。★★★リストは書いている——しかも「cell」の意味が
// 毎回違うのに同じ式で通る:
//
//	 60 board squares × 2 bits      =  15 bytes  〔stella-list 200304/msg00033, C. Tumber, 2003〕
//	104 well spots   × 4 bits       =  52 bytes  〔199905/msg00090, R. Perry Jr, 1999〕
//	128 bricks       × 4 bits       =  64 bytes  〔200108/msg00315, E. Mooney, 2001〕
//	  7 bytes/line   × 8 lines      =  56 bytes  〔199906/msg00102, K. Woloch, 1999〕
//	  5 sprites wide × 20 lines deep = 100 bytes 〔200209/msg00045, A. Davie, 2002〕
//
// ★★★★Perry's is the one that proves it is a FORMULA and not remembered numbers: 78 spots at
// 3 bits is 29.25, which does not divide, and he wrote "<30 bytes" rather than a round figure.
//
// ★★And the bit width is not free — it is decided by what each cell has to remember. Mooney:
// one bit per brick is enough for a simple game, but Arkanoid needs "its color, how many hits it
// has taken (for silvers), and whether or not it hides a capsule", so four.
//
// ★false のときは SuperChip/CBS RAM が要る、というのがこの関数の言っていること——★★ただし
// リストはその判断を三択として扱っている。★★★(1) buy the cartridge RAM, which one correspondent
// called cheating: "it is cheating to simply shift a game to the Supercharger that'd normally need
// 500 Bytes of RAM … The challenge is to make it run with 128 Bytes anyway" 〔200104/msg00092〕;
// (2) pack harder — the bit widths above; (3) ★change the GAME so it needs less, which is the
// option a tool cannot suggest: "This can be handled, maybe through constraints, something like
// making the levels not have more than two silver bricks per row" 〔200108/msg00315〕.
// Recovered by the mailing-list distillation (helper-2, helper-3); the arithmetic re-checked here
// on all five, and it holds at bit widths 2, 3, 4 and 8.
func ScrollBackgroundFitsRAM(boardBytes, bufferBytes, deltaBytes, stackBytes int) bool {
	if boardBytes < 0 || bufferBytes < 0 || deltaBytes < 0 || stackBytes < 0 {
		return false
	}
	return boardBytes+bufferBytes+deltaBytes+stackBytes <= RAM2600
}

// ScrollScanlinesConstant は縦/横スクロール背景の鉄則「総スキャンライン数をフレーム間で
// 一定に保つ」を判定する。frameLines は各フレームの総ライン数。pal=true なら各フレームが
// 偶数ラインであることも要求する（PAL は偶数必須）。〔design-principles.md / 採掘 200972〕
func ScrollScanlinesConstant(frameLines []int, pal bool) bool {
	if len(frameLines) == 0 {
		return true
	}
	first := frameLines[0]
	for _, n := range frameLines {
		if n != first {
			return false
		}
		if pal && n%2 != 0 {
			return false
		}
	}
	return true
}

// PFReg は playfield レジスタ（PF0/PF1/PF2）。
type PFReg int

const (
	PF0 PFReg = iota
	PF1
	PF2
)

// AsymRightWindow は非対称PFで「右半分の値を同一走査線内に書き直す」時の、各レジスタの
// 安全な書込みサイクル窓 [start,end]（WSYNC=cycle 0 基準・repeated モード）を返す。
// repeated: RPF0 27–48 / RPF1 37–53 / RPF2 48–64。reflected モードは RPF2 を「ちょうど 48」で
// 完了させる（STA 3cy なら begin=45。design-principles の "PF2 begin=cy45" と整合）。
// 出典: woodgrain Playfield_Timing.html の definitive table（docs/fundamentals-audit.md:66-69）。
// 備考: これは「文書化された権威テーブル」由来。我々の litmus_pf_async は左右窓の一部のみ実機ロック済。
func AsymRightWindow(reg PFReg) (start, end int) {
	switch reg {
	case PF0:
		return 27, 48
	case PF1:
		return 37, 53
	case PF2:
		return 48, 64
	}
	return 0, 0
}

// FitsAsymRightWrite は右半 PF 再書込みを cycle で行うのが窓内に収まるかを返す。
func FitsAsymRightWrite(reg PFReg, cycle int) bool {
	s, e := AsymRightWindow(reg)
	return cycle >= s && cycle <= e
}

// ═══ Asymmetric playfield vs. repositioning ═══════════════════════════════════════════════
//
// Thomas Jentzsch, stella-list 200409/msg00258 (2004-09-17), on Jumpman's kernel:
//
//	"With assymetrical, non striped playfields, you won't be able to reposition at all.
//	 So, how about striping it?"
//
// That reads like advice. It is arithmetic, and it follows from two tables this package already
// carries: AsymRightWindow (and its left-half twin, docs/fundamentals-audit.md:120) says WHEN each
// playfield store must complete, and sprite-placement.md rule 1 says a reposition's strobe must
// land on ONE specific cycle to reach a given x. A line that rewrites a full asymmetric playfield
// spends six loads and six stores inside those windows; whether a three-cycle strobe still fits at
// the one cycle that reaches your x is a scheduling question with a yes/no answer.
//
// AsymPFLineFits answers it. Measured consequence (locked by TestAsymPFRepositionBudget):
//
//	graphics writes | cycles used | strobe positions still reachable (of 53)
//	              0 |       53/76 | 53 — every one
//	              1 |       60/76 | 49
//	              2 |       67/76 | 16, and all of them on the RIGHT (x >= 102)
//	              3 |       74/76 |  0  <- "you won't be able to reposition at all"
//
// So the 2004 sentence is exact, and it is exact for the kernel Jentzsch was writing: Jumpman puts
// several objects on a line. **The playfield alone does not cost you the reposition** — six PF
// writes plus a strobe fit at every x with 23 cycles to spare. It is the graphics registers, added
// to the playfield, that close the window. That distinction is not in the source and is worth
// having: it names the escape routes (his own "stripe it", or fewer objects on the repositioning
// line) and it says which one buys what.
//
// This is a DERIVATION over two documented tables, not a measurement of silicon. The windows are
// woodgrain's (📖 in fundamentals-audit), the instruction costs are the engine's table. The 2004
// sentence is the independent check on it, which is why it is quoted above rather than cited.
const (
	// pfWriteCost is `lda table,y` (4) + `sta PFn` (3). A non-striped asymmetric playfield reads
	// its values from a table every line, so the immediate-operand form does not apply.
	pfWriteCost = 7
	// strobeCost is `sta RESPx` — three cycles, writing on the last.
	strobeCost = 3
	// wsyncCost and loopCost are what a per-line kernel spends on nothing but being a loop.
	wsyncCost = 3
	loopCost  = 5 // dey (2) + bne (3)
)

// asymOp is one scheduled write: cost cycles long, its write landing on start+writeOff, which must
// fall in [lo,hi].
type asymOp struct {
	cost, writeOff, lo, hi int
}

// AsymPFLineFits reports whether one 76-cycle line can rewrite a full asymmetric playfield (all six
// windows), write nGraphics graphics registers from a table, run WSYNC and the loop, AND strobe a
// reposition whose write cycle is strobeCycle. Pass strobeCycle < 0 to ask only whether the line
// fits without a reposition.
func AsymPFLineFits(nGraphics, strobeCycle int) bool {
	ops := []asymOp{{wsyncCost, 2, 0, 2}}
	// Left half of the line, from the same table (fundamentals-audit.md:120, repeated mode):
	// LPF0 must complete by 21, LPF1 by 27, LPF2 by 37.
	for _, dl := range []int{21, 27, 37} {
		ops = append(ops, asymOp{pfWriteCost, pfWriteCost - 1, 0, dl})
	}
	for _, r := range []PFReg{PF0, PF1, PF2} {
		s, e := AsymRightWindow(r)
		ops = append(ops, asymOp{pfWriteCost, pfWriteCost - 1, s, e})
	}
	// A graphics write takes effect at screen x = 3w - 64 (sprite-placement.md rule 6), so writing
	// for an object around mid-screen means completing by cycle (40+64)/3.
	for i := 0; i < nGraphics; i++ {
		ops = append(ops, asymOp{pfWriteCost, pfWriteCost - 1, 0, (40 + 64) / 3})
	}
	ops = append(ops, asymOp{loopCost, loopCost - 1, 0, 75})
	if strobeCycle >= 0 {
		ops = append(ops, asymOp{strobeCost, 2, strobeCycle, strobeCycle})
	}
	// Every ordering is allowed; what matters is that each write lands in its own window. Held as
	// a subset DP over "earliest cycle still free" rather than permutations, which blows up at 12.
	const infeasible = 1 << 30
	n := len(ops)
	free := make([]int, 1<<n)
	for i := range free {
		free[i] = infeasible
	}
	free[0] = 0
	for mask := 0; mask < 1<<n; mask++ {
		t := free[mask]
		if t >= infeasible {
			continue
		}
		for i, op := range ops {
			if mask>>i&1 == 1 {
				continue
			}
			s := t
			if op.lo-op.writeOff > s {
				s = op.lo - op.writeOff
			}
			if s < 0 {
				s = 0
			}
			if s > op.hi-op.writeOff || s+op.cost-1 > 75 {
				continue
			}
			if nm := mask | 1<<i; s+op.cost < free[nm] {
				free[nm] = s + op.cost
			}
		}
	}
	return free[1<<n-1] < infeasible
}

// AsymPFReachableX returns every screen x a NORMAL-width player can still be repositioned to on a
// line that also rewrites a full asymmetric playfield and writes nGraphics graphics registers.
// Empty means Jentzsch's sentence, literally.
func AsymPFReachableX(nGraphics int) []int {
	var out []int
	for x := 0; x < 160; x++ {
		if (x+60)%3 != 0 {
			continue // rule 1: placement is on a 3-clock grid
		}
		c := (x + 60) / 3
		if c < 16 || c > 72 {
			continue // outside the cycles a line can strobe on at all
		}
		if AsymPFLineFits(nGraphics, c) {
			out = append(out, x)
		}
	}
	return out
}
