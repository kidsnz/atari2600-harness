; litmus_nusiz — hardware verification of NUSIZ double width (hardening-roadmap S-2)
; Draws an 8px solid sprite (GRP=$FF for 8 rows) with player0 and NUSIZ0=$05 (DoubleWidth).
; At double width each design px becomes 2px on screen, so it comes out as continuous white 8px→16px wide.
; Check: read_row shows the sprite rows 16px wide (8px at normal width) = verification of the pkg/sprite.DoubleWidth semantics.
; Hardware-verified (Gopher2600): read_row(visible 96)=clock 4-19 white, len16 (=16px) / read_tia_registers player0.nusiz=5.
; Regression-locked = roms/litmus/scenarios/nusiz.json (nusiz assert + golden).
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
        lda #$05          ; NUSIZ0 = DoubleWidth (pkg/sprite.NUSIZPlayer(DoubleWidth))
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
