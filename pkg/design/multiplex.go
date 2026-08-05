package design

// MovableObjects は 1 走査線で TIA が同時に出せる可動オブジェクト数（P0,P1,M0,M1,BL）。
const MovableObjects = 5

// DistinctPlayerSprites は同一 Y で重ならず**フリッカ無し**に出せる「別形状プレイヤー」数
// （P0,P1 の2体）。NUSIZ のコピーは同一形状なので別形状にはカウントしない。
const DistinctPlayerSprites = 2

// MaxMultiSprite AND FitsMultiSprite WERE HERE AND ARE DELETED (2026-08-05).
//
// The constant was 5, documented as bB's multisprite/flickersort ceiling and cited to
// AtariAge thread 107063. Reading that thread: it never mentions bB or the number 5. It
// is a 2007 raw-assembly discussion of Venetian blinds for chess pieces, and the numbers
// it actually gives are "8 distinct sprites per line is infeasible" (supercat) and
// "4 pieces per line by rewriting GRP0/GRP1 mid-line, the Video Chess method" (hornpipe2,
// measured). A citation that does not support its claim is worse than none: it makes the
// number look sourced.
//
// It was also 5 — numerically identical to MovableObjects above — so nothing about the
// value distinguished the two concepts, and TestMultiplexConstants asserted only that the
// definition equals itself. And a bB-derived ceiling is a thing this project has decided
// on purpose not to inherit: bB is a lower bound, not the hardware's limit.
//
// The thread's real content is kept where it belongs, in
// reference/atariage/107063-interlacing-multi-sprites/notes.ja.md, not compressed into a
// wrong constant. NeedsFlicker below still cites it correctly.

// NeedsFlicker は同一 Y 帯に sameYSprites 個の「別形状スプライト」を置くと多重化(フリッカ)が
// 要るかを返す。2(P0/P1)まではフリッカ無し、3以上は Y 再配置 or フリッカが必要。
func NeedsFlicker(sameYSprites int) bool {
	return sameYSprites > DistinctPlayerSprites
}

// RepositionCostScanlines は、可動オブジェクトを横へ再配置（RESPx ストロボ）するのに
// 消費する走査線数。1 本の Y 帯境界で 1 走査線を使う＝帯間に空き Y レーンが要る理由。
// 〔design-principles.md「横再配置は1走査線消費」/ Bumbershoot〕
const RepositionCostScanlines = 1

// NeedsEmptyYLane は帯多重化で再配置を挟む時に空 Y レーンが必須かを返す。再配置が
// 1 走査線消費するため、別形状を Y 帯で切り替えるなら境界に空き行が要る。
func NeedsEmptyYLane(sameYSprites int) bool { return NeedsFlicker(sameYSprites) }
