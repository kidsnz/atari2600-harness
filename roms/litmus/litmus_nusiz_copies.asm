; litmus_nusiz_copies — NUSIZ 多コピーの実機裏取り（S-2 検証拡張）
; 8px の solid スプライトを player0・NUSIZ0=$03（ThreeCopiesClose）で描く。3 コピーが横に並ぶ。
; 3 コピー近接なら base / base+16 / base+32 px に 8px 白が 3 つ出る。
; 検証: read_row でスプライト行に 8px 白が 3 スパン（コピー間隔16px）。
; 実機裏取り済（Gopher2600）: read_row(可視96)=clock 3/19/35 に 8px 白が 3 スパン（コピー間隔16px）。
; 回帰固定 = roms/litmus/scenarios/nusiz_copies.json。
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
NUSIZ0  = $04
COLUP0  = $06
COLUBK  = $09
RESP0   = $10
GRP0    = $1B

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
        lda #$03          ; NUSIZ0 = ThreeCopiesClose（pkg/sprite.NUSIZPlayer(ThreeCopiesClose)）
        sta NUSIZ0
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
        ldx #37
VBlank: sta WSYNC
        cpx #37
        bne NoPos
        sta RESP0
NoPos:  dex
        bne VBlank
        lda #0
        sta VBLANK

        ldx #192
Visible:
        sta WSYNC
        lda GfxLine-1,x
        sta GRP0
        dex
        bne Visible
        lda #0
        sta GRP0

        lda #2
        sta VBLANK
        ldx #30
OScan:  sta WSYNC
        dex
        bne OScan
        jmp NextFrame

; スプライトは（カーネル基準）可視 88..95＝Gopher2600 可視 96..103。8 行とも solid $FF。
GfxLine:
        ds 96, 0
        .byte $FF,$FF,$FF,$FF,$FF,$FF,$FF,$FF     ; idx 96..103（可視 95..88）
        ds 88, 0

        org $FFFC
        .word Start
        .word Start
