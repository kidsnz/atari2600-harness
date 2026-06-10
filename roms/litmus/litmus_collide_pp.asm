; litmus_collide_pp — P0-P1 衝突（CXPPMM）の実機裏取り（衝突カバレッジ拡張）
; P0 と P1 を HBLANK ストローブで同位置（最左クランプ X=3）に重ね、両方 GRP=$FF で描画（CXCLR せず sticky に任せる）。
; 重なるので CXPPMM D7（P0-P1）がセット。read_collisions.p0_p1 で裏取り（Frogger の OnPad 判定が使うペア）。
; 既存 litmus_collide は BL-PF のみ → P0-P1 を追加。
; 実機裏取り済（Gopher2600）: read_collisions p0_p1=true（重なった P0/P1 で CXPPMM D7）。回帰固定 = scenarios/collide_pp.json。
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUP0  = $06
COLUP1  = $07
COLUBK  = $09
RESP0   = $10
RESP1   = $11
GRP0    = $1B
GRP1    = $1C
        org $F000
Start:
        sei
        cld
        ldx #$FF
        txs
        lda #0
Clr:    sta $00,x
        dex
        bne Clr
        lda #$0E
        sta COLUP0
        lda #$46
        sta COLUP1
        lda #0
        sta COLUBK
NextFrame:
        lda #2
        sta VBLANK
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC
        sta WSYNC
        sta RESP0         ; HBLANK ストローブ → P0 最左(X=3)
        sta RESP1         ; HBLANK ストローブ → P1 も最左(X=3)＝重なる
        ldx #36
VBlank: sta WSYNC
        dex
        bne VBlank
        lda #0
        sta VBLANK
        ; 可視: 中央付近の 8 ライン両方 $FF、他は 0
        ldx #192
Visible:
        sta WSYNC
        cpx #100
        bcs Blank
        cpx #92
        bcc Blank
        lda #$FF
        sta GRP0
        sta GRP1
        jmp Next
Blank:  lda #0
        sta GRP0
        sta GRP1
Next:   dex
        bne Visible
        lda #2
        sta VBLANK
        ldx #30
OScan:  sta WSYNC
        dex
        bne OScan
        jmp NextFrame
        org $FFFC
        .word Start
        .word Start
