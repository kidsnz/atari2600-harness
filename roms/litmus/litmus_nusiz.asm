; litmus_nusiz — NUSIZ 倍幅の実機裏取り（hardening-roadmap S-2）
; 8px の solid スプライト（GRP=$FF 8 行）を player0・NUSIZ0=$05（DoubleWidth）で描く。
; 倍幅なら各 design px が画面 2px になり、8px→16px 幅の連続白として出る。
; 検証: read_row でスプライト行が 16px 幅（通常幅なら 8px）＝pkg/sprite.DoubleWidth セマンティクスの裏取り。
; 実機裏取り済（Gopher2600）: read_row(可視96)=clock 4-19 が白 len16（=16px）／read_tia_registers player0.nusiz=5。
; 回帰固定 = roms/litmus/scenarios/nusiz.json（nusiz アサート＋golden）。
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
        lda #$05          ; NUSIZ0 = DoubleWidth（pkg/sprite.NUSIZPlayer(DoubleWidth)）
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
