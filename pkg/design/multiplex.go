package design

// MovableObjects は 1 走査線で TIA が同時に出せる可動オブジェクト数（P0,P1,M0,M1,BL）。
const MovableObjects = 5

// DistinctPlayerSprites は同一 Y で重ならず**フリッカ無し**に出せる「別形状プレイヤー」数
// （P0,P1 の2体）。NUSIZ のコピーは同一形状なので別形状にはカウントしない。
const DistinctPlayerSprites = 2

// MaxMultiSprite は bB multisprite 系（flickersort で P1 を Y 帯ごとに再配置）で出せる可動
// スプライト上限。これを超えると同一 Y 帯の重なりでフリッカが悪化する。〔Pizza Boy / 採掘 107063〕
const MaxMultiSprite = 5

// NeedsFlicker は同一 Y 帯に sameYSprites 個の「別形状スプライト」を置くと多重化(フリッカ)が
// 要るかを返す。2(P0/P1)まではフリッカ無し、3以上は Y 再配置 or フリッカが必要。
func NeedsFlicker(sameYSprites int) bool {
	return sameYSprites > DistinctPlayerSprites
}

// RepositionCostScanlines は、可動オブジェクトを横へ再配置（RESPx ストロボ）するのに
// 消費する走査線数。1 本の Y 帯境界で 1 走査線を使う＝帯間に空き Y レーンが要る理由。
// 〔design-principles.md「横再配置は1走査線消費」/ Bumbershoot〕
const RepositionCostScanlines = 1

// FitsMultiSprite は同一 P1 を Y 帯ごとに再配置して count 体のスプライトを出す構成が、
// multisprite カーネルの上限内かを返す。〔design-principles.md / Pizza Boy〕
func FitsMultiSprite(count int) bool { return count <= MaxMultiSprite }

// NeedsEmptyYLane は帯多重化で再配置を挟む時に空 Y レーンが必須かを返す。再配置が
// 1 走査線消費するため、別形状を Y 帯で切り替えるなら境界に空き行が要る。
func NeedsEmptyYLane(sameYSprites int) bool { return NeedsFlicker(sameYSprites) }
