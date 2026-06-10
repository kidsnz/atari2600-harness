; litmus_collide_mp — missile0-player0 衝突（CXM0P）の実機裏取り（衝突カバレッジ拡張）
; player0(GRP=$FF) と missile0 を HBLANK ストローブでX=2..9 の 8px missile を X=3..10 の player に重ね、両方描画。CXM0P D7（M0-P0）がセット。
; read_collisions.m0_p0 で裏取り。CXCLR せず sticky に任せる。
; 実機裏取り済（Gopher2600）: read_collisions m0_p0=true（8px missile×player）。回帰固定=scenarios/collide_mp.json。
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
NUSIZ0  = $04
COLUP0  = $06
COLUBK  = $09
RESP0   = $10
RESM0   = $12
GRP0    = $1B
ENAM0   = $1D
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
        lda #0
        sta COLUBK
        lda #2
        sta ENAM0         ; missile0 有効
        lda #$30
        sta NUSIZ0        ; missile0 幅 8px（D5:4=11）→ player と重なる
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
        sta RESP0         ; P0 最左(X=3)
        sta RESM0         ; missile0 最左(X=3)＝重なる
        ldx #36
VBlank: sta WSYNC
        dex
        bne VBlank
        lda #0
        sta VBLANK
        ldx #192
Visible:
        sta WSYNC
        cpx #100
        bcs Blank
        cpx #92
        bcc Blank
        lda #$FF
        sta GRP0
        jmp Next
Blank:  lda #0
        sta GRP0
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
