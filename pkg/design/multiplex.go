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

// NTSCFrameRateHz is the engine's own NTSC refresh, measured rather than the nominal 60:
// 15734.26 / 262. Kept here so the rate below and `subpixel-velocity.md`'s conversion factor
// cannot drift apart.
const NTSCFrameRateHz = 60.0544

// FlickerRateHz reports how often each object is drawn when sameYSprites objects share the two
// player slots, at 60 Hz frames.
//
// `NeedsFlicker` answers yes or no and says nothing about HOW MUCH — it returns the same answer for
// three objects and for twenty. This is the missing half, and the archive gives both ends of it.
//
// Glenn Saunders, 1997, arguing flicker is a tool rather than a defect: *"Some of the most impressive
// 2600 games have flicker (Solaris, Radar Lock, Stargate, Star Wars: The Arcade Game, even
// Adventure). It frees up the 2600 to do more independently moving sprites, and have more of them.
// **It's never really necessary to drop below 30hz** and still manage to fill the screen with
// sprites"* 〔stella-list `199709/msg00139`〕.
//
// Piero Cavina answered five days later with the counter-example, and it is not a compliment:
// *"**'Adventure' must be the king of flicker**… I remember that you could put all the objects (dot
// included) in the same room and get an incredible amount of flicker"* 〔`199709/msg00218`〕.
//
// So the ladder is the frame rate over the number of subsets, and the two named points are:
//
//	 3-4 objects    2 subsets   30.03 Hz   the rate Saunders says is enough for a screen of sprites
//	24   objects   12 subsets    5.00 Hz   Adventure's crowded room, named by the person who
//	                                       watched it as excessive
//
// The second line is arithmetic meeting an eyewitness: twenty-four objects sharing two slots is
// exactly the "5hZ, maybe?" Cavina guessed at, which is the sort of agreement worth writing down
// because neither side was derived from the other.
//
// There is no hardware limit here to return — this is a judgement, and the number exists so the
// judgement is made against one. `HardwareCollisionUsable` is the other half of the same decision:
// past two subsets the TIA's collision latches stop being trustworthy, so the cost of a high N is
// not only visual.
func FlickerRateHz(sameYSprites int) float64 {
	subsets := SubsetsFor(sameYSprites)
	if subsets <= 0 {
		return 0
	}
	return NTSCFrameRateHz / float64(subsets)
}

// SubsetsFor reports how many frames a cycle takes when sameYSprites objects share the player slots:
// one when they all fit, otherwise the number of groups of DistinctPlayerSprites needed to hold them.
func SubsetsFor(sameYSprites int) int {
	if sameYSprites <= 0 {
		return 0
	}
	if !NeedsFlicker(sameYSprites) {
		return 1
	}
	return (sameYSprites + DistinctPlayerSprites - 1) / DistinctPlayerSprites
}

// RepositionCostScanlines は、可動オブジェクトを横へ再配置（RESPx ストロボ）するのに
// 消費する走査線数。1 本の Y 帯境界で 1 走査線を使う＝帯間に空き Y レーンが要る理由。
// 〔design-principles.md「横再配置は1走査線消費」/ Bumbershoot〕
const RepositionCostScanlines = 1

// NeedsEmptyYLane は帯多重化で再配置を挟む時に空 Y レーンが必須かを返す。再配置が
// 1 走査線消費するため、別形状を Y 帯で切り替えるなら境界に空き行が要る。
func NeedsEmptyYLane(sameYSprites int) bool { return NeedsFlicker(sameYSprites) }

// HardwareCollisionUsable reports whether the TIA's own collision latches (CXxx) can
// be trusted for an object that is drawn with flicker multiplexing.
//
// They cannot, and the reason is a MISS rather than an inaccuracy: two objects that are
// colliding may never be drawn on the SAME FRAME, so `CXPPMM` and its siblings simply
// never latch. To the player that is "I hit it and nothing happened" — the failure is
// silent and intermittent, which is the worst shape a collision bug can take.
//
// So the choice to flicker also decides the collision architecture: it has to move into
// software. The cheapest first step, from the same source, is to test collisions only
// for a sprite that MOVED this frame, since most sprites are stationary; the price is
// that an overlap already present when a screen appears goes unnoticed until something
// moves. 〔blogs 8429 SpiceWare/Frantic〕
func HardwareCollisionUsable(flickered bool) bool { return !flickered }
