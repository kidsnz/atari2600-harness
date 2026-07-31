package emu

import "sort"

// covKey identifies an instruction by the bank it was FETCHED from as well as its
// address.
//
// A bare address is not an instruction on a bank-switched cartridge: both banks of
// an 8K image decode $F000-$FFFF, so bank 0's $F123 and bank 1's $F123 are two
// different instructions sharing one number. Keying on the address alone merges
// them, and it does so in both directions at once — measured 2026-07-30 over 200k
// instructions, the exerciser executes 319 distinct (bank, pc) pairs and the old
// PCCount reported 282, with 37 addresses run in BOTH banks; banked_game was 74 vs
// 69 with 5. As a count that under-reports what ran; as a query, Seen() answered
// "covered" for the twin in the bank that never ran, which is the flattering
// direction and the one VV-3's percentage and mutate's "covered" set rest on.
//
// On a flat 4K image every bank is 0, so this key changes nothing there — verified
// by diffing cmd/cover's whole JSON output before and after on flat ROMs.
type covKey struct {
	bank int
	addr uint16
}

// Coverage は実行で踏んだ命令アドレスと分岐エッジを記録する（VV-3）。テスト網羅度の軸＝
// 「どの命令／どちらの分岐方向を実際に実行したか」を数で出す。EnableCoverage で有効化する
// まで Emu.cov は nil＝ゼロコスト（毎命令フックを通らない）。
type Coverage struct {
	pcSeen   map[covKey]bool // 実行された命令（バンク＋先頭アドレス）
	brTaken  map[covKey]bool // 分岐命令→taken エッジを踏んだ
	brNot    map[covKey]bool // 分岐命令→fall-through エッジを踏んだ
	branches map[covKey]bool // 分岐命令と判明したもの（全体集合）

	// addrSeen keeps the bank-less view alive for Seen(addr), whose meaning is
	// "executed at this address in SOME bank". Kept separate rather than derived,
	// so the legacy query stays O(1) and its meaning stays visible in one place.
	addrSeen map[uint16]bool
}

func newCoverage() *Coverage {
	return &Coverage{
		pcSeen:   map[covKey]bool{},
		brTaken:  map[covKey]bool{},
		brNot:    map[covKey]bool{},
		branches: map[covKey]bool{},
		addrSeen: map[uint16]bool{},
	}
}

// record は完了した 1 命令を取り込む（stepInstr から、命令完了時だけ呼ばれる）。
// bank は命令を FETCH したバンク。完了後に問い合わせてはいけない: ホットスポットに触れる
// 命令はその完了と同時にマッピングを変えるので、後から訊くと「切り替えた先」のバンクに
// 帰属してしまう。
func (c *Coverage) record(bank int, addr uint16, isBranch, taken bool) {
	k := covKey{bank, addr}
	c.pcSeen[k] = true
	c.addrSeen[addr] = true
	if isBranch {
		c.branches[k] = true
		if taken {
			c.brTaken[k] = true
		} else {
			c.brNot[k] = true
		}
	}
}

// PCCount は踏んだ相異なる命令数（バンク込み）。
func (c *Coverage) PCCount() int { return len(c.pcSeen) }

// Seen はそのアドレスの命令を **いずれかのバンクで** 実行したか（未踏＝デッドコードの候補）。
// バンクを問うなら SeenIn を使うこと。フラット 4K では両者は同じ。
func (c *Coverage) Seen(addr uint16) bool { return c.addrSeen[addr] }

// SeenIn はその (バンク, アドレス) の命令を実行したか。8K 以上でのみ Seen と差が出る。
func (c *Coverage) SeenIn(bank int, addr uint16) bool { return c.pcSeen[covKey{bank, addr}] }

// BranchCount は分岐命令として観測した数（バンク込み）。
func (c *Coverage) BranchCount() int { return len(c.branches) }

// dedupeAddrs は (bank,addr) 集合を昇順のアドレス列に潰す（外向き API の形を保つため）。
func dedupeAddrs(keys map[covKey]bool) []uint16 {
	seen := map[uint16]bool{}
	out := make([]uint16, 0, len(keys))
	for k := range keys {
		if !seen[k.addr] {
			seen[k.addr] = true
			out = append(out, k.addr)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// BranchAddrs は分岐命令として観測したアドレスを昇順で返す。静的に復号した分岐集合と
// 突き合わせるための観測側＝これに含まれて静的側に無いものは「デコーダが到達していない
// コード」であり、分母が小さすぎる合図になる。同じアドレスが両バンクで分岐なら 1 回だけ出る。
func (c *Coverage) BranchAddrs() []uint16 { return dedupeAddrs(c.branches) }

// EdgeCount は踏んだ分岐エッジの総数（最大 = BranchCount*2＝両方向踏破）。
func (c *Coverage) EdgeCount() int { return len(c.brTaken) + len(c.brNot) }

// OneSidedBranches は片側エッジしか踏んでいない分岐アドレスを昇順で返す
// （taken だけ／not だけ＝その分岐の裏側がテストされていないサイン）。
// バンク切替イメージでは「どれかのバンクで片側」＝報告する。片側性は所見なので、
// 潰すなら見落とす側ではなく報告する側に潰す。
func (c *Coverage) OneSidedBranches() []uint16 {
	oneSided := map[covKey]bool{}
	for k := range c.branches {
		if c.brTaken[k] != c.brNot[k] { // ちょうど片方だけ true
			oneSided[k] = true
		}
	}
	return dedupeAddrs(oneSided)
}

// Signature は coverage-guided fuzz のフィードバック用に、踏んだ「カバレッジ標識」を比較可能な
// キー集合で返す（命令アドレス＋分岐エッジ taken/not）。新しい標識が増える入力＝interesting。
// バンクを含めるので、8K イメージでは別バンクの同アドレスが別の標識として数えられる
// （含めないと、片方のバンクを歩いただけで「新しい被覆なし」に見えて探索が止まる）。
func (c *Coverage) Signature() map[uint64]bool {
	sig := make(map[uint64]bool, len(c.pcSeen)+len(c.brTaken)+len(c.brNot))
	mark := func(tag uint64, k covKey, edge uint64) uint64 {
		return tag | uint64(k.bank)<<40 | uint64(k.addr)<<1 | edge
	}
	for k := range c.pcSeen {
		sig[mark(0, k, 0)] = true
	}
	for k := range c.brTaken {
		sig[mark(1<<32, k, 1)] = true
	}
	for k := range c.brNot {
		sig[mark(1<<32, k, 0)] = true
	}
	return sig
}

// SeenPCs は踏んだ命令アドレスを昇順で返す（dump 用）。
func (c *Coverage) SeenPCs() []uint16 { return dedupeAddrs(c.pcSeen) }

// Reset は記録を空に戻す（guidedfuzz が 1 評価ごとに再ロードせず再利用するため。
// signature 同一性を検証した上で warmup=200 で約 100 倍速）。
func (c *Coverage) Reset() {
	c.pcSeen = map[covKey]bool{}
	c.brTaken = map[covKey]bool{}
	c.brNot = map[covKey]bool{}
	c.branches = map[covKey]bool{}
	c.addrSeen = map[uint16]bool{}
}
