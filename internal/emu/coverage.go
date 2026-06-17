package emu

import "sort"

// Coverage は実行で踏んだ命令アドレスと分岐エッジを記録する（VV-3）。テスト網羅度の軸＝
// 「どの命令／どちらの分岐方向を実際に実行したか」を数で出す。EnableCoverage で有効化する
// まで Emu.cov は nil＝ゼロコスト（毎命令フックを通らない）。
type Coverage struct {
	pcSeen   map[uint16]bool // 実行された命令の先頭アドレス
	brTaken  map[uint16]bool // 分岐命令アドレス→taken エッジを踏んだ
	brNot    map[uint16]bool // 分岐命令アドレス→fall-through エッジを踏んだ
	branches map[uint16]bool // 分岐命令と判明したアドレス（全体集合）
}

func newCoverage() *Coverage {
	return &Coverage{
		pcSeen:   map[uint16]bool{},
		brTaken:  map[uint16]bool{},
		brNot:    map[uint16]bool{},
		branches: map[uint16]bool{},
	}
}

// record は完了した 1 命令を取り込む（stepInstr から、命令完了時だけ呼ばれる）。
func (c *Coverage) record(addr uint16, isBranch, taken bool) {
	c.pcSeen[addr] = true
	if isBranch {
		c.branches[addr] = true
		if taken {
			c.brTaken[addr] = true
		} else {
			c.brNot[addr] = true
		}
	}
}

// PCCount は踏んだ相異なる命令アドレス数。
func (c *Coverage) PCCount() int { return len(c.pcSeen) }

// Seen はそのアドレスの命令を実行したか（未踏＝デッドコードの候補）。
func (c *Coverage) Seen(addr uint16) bool { return c.pcSeen[addr] }

// BranchCount は分岐命令として観測したアドレス数。
func (c *Coverage) BranchCount() int { return len(c.branches) }

// EdgeCount は踏んだ分岐エッジの総数（最大 = BranchCount*2＝両方向踏破）。
func (c *Coverage) EdgeCount() int { return len(c.brTaken) + len(c.brNot) }

// OneSidedBranches は片側エッジしか踏んでいない分岐アドレスを昇順で返す
// （taken だけ／not だけ＝その分岐の裏側がテストされていないサイン）。
func (c *Coverage) OneSidedBranches() []uint16 {
	var out []uint16
	for a := range c.branches {
		if c.brTaken[a] != c.brNot[a] { // ちょうど片方だけ true
			out = append(out, a)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// SeenPCs は踏んだ命令アドレスを昇順で返す（dump 用）。
func (c *Coverage) SeenPCs() []uint16 {
	out := make([]uint16, 0, len(c.pcSeen))
	for a := range c.pcSeen {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
