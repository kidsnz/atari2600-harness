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
