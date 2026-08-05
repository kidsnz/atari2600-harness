; litmus_nusiz_copies — hardware verification of NUSIZ multiple copies (S-2 verification extension)
; Draws an 8px solid sprite with player0 and NUSIZ0=$03 (ThreeCopiesClose). Three copies line up horizontally.
; With three close copies there is 8px of white at base / base+16 / base+32 px.
; Check: read_row shows three 8px white spans on the sprite rows (copy spacing 16px).
; Hardware-verified (Gopher2600): read_row(visible 96)=three 8px white spans at clock 3/19/35 (copy spacing 16px).
; Regression-locked = roms/litmus/scenarios/nusiz_copies.json.
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
        lda #$03          ; NUSIZ0 = ThreeCopiesClose (pkg/sprite.NUSIZPlayer(ThreeCopiesClose))
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

; The sprite occupies (in kernel terms) visible 88..95 = Gopher2600 visible 96..103. All 8 rows are solid $FF.
GfxLine:
        ds 96, 0
        .byte $FF,$FF,$FF,$FF,$FF,$FF,$FF,$FF     ; idx 96..103 (visible 95..88)
        ds 88, 0

        org $FFFC
        .word Start
        .word Start
