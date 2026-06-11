; litmus_collide_all — 15 衝突ペアを一括で実機裏取り（V2-8, 残り12ペア）
; P0/P1/M0/M1/BL を全て左端で重ね、PF0 も左端点灯 → 全 15 ペアが衝突するはず。
; read_collisions の全フィールド true を確認。既存 litmus_collide(bl_pf)/_pp(p0_p1)/_mp(m0_p0) を包含。
; 実機裏取り済（Gopher2600, v0.48.0）: 全6オブジェクト+PF を左端で重ね、read_collisions の【15ペア全部 true】を確認。
; （ball幅8で p0_bl/p1_bl も成立）。回帰固定=scenarios/collide_all.json。既存 collide/_pp/_mp を包含。
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
NUSIZ0  = $04
NUSIZ1  = $05
COLUP0  = $06
COLUP1  = $07
COLUPF  = $08
COLUBK  = $09
CTRLPF  = $0A
RESP0   = $10
RESP1   = $11
RESM0   = $12
RESM1   = $13
RESBL   = $14
GRP0    = $1B
GRP1    = $1C
ENAM0   = $1D
ENAM1   = $1E
ENABL   = $1F
PF0     = $0D

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
        sta $2C             ; CXCLR
        ; 全オブジェクト有効化
        lda #$FF
        sta GRP0
        sta GRP1            ; P0/P1 = 8px
        lda #$02
        sta ENAM0
        sta ENAM1
        sta ENABL          ; M0/M1/BL = on
        lda #$30
        sta NUSIZ0
        sta NUSIZ1          ; missile 幅8px（重なり確保）
        lda #$F0
        sta PF0            ; 左端 PF 点灯（clock 0-15）
        lda #$30
        sta CTRLPF         ; ball 幅8px（プレイヤーと重なり p0_bl/p1_bl 確保）
        lda #$0E
        sta COLUP0
        sta COLUP1
        sta COLUPF
        ; 全部 HBLANK でストローブ → 左端に集合（P0/P1=X3, M/BL=X2、幅で重なる）
        sta WSYNC
        sta RESP0
        sta RESP1
        sta RESM0
        sta RESM1
        sta RESBL

NextFrame:
        lda #2
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC
        lda #2
        sta VBLANK
        ldx #37
VB:     sta WSYNC
        dex
        bne VB
        lda #0
        sta VBLANK
        ldy #192
Vis:    sta WSYNC
        dey
        bne Vis
        lda #2
        sta VBLANK
        ldx #30
OS:     sta WSYNC
        dex
        bne OS
        jmp NextFrame

        org $FFFC
        .word Start
        .word Start
