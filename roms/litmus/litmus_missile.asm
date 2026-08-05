; litmus_missile — hardware verification of missile0 / ball positions (deepening verification coverage)
; missile0 and ball are strobed to different positions in the visible area and enabled. read_tia gives each position, read_row shows them as vertical lines.
; Against the player position (litmus_pos, X=3N-54), this verifies the position readout of the missile/ball family (X=3N-55).
; Hardware-verified (Gopher2600): read_tia missile0=38 / ball=140, and read_row(100) shows 1px of white at each clock.
; Regression-locked = roms/litmus/scenarios/missile.json.
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
NUSIZ0  = $04
COLUP0  = $06
COLUPF  = $08
COLUBK  = $09
RESM0   = $12
RESBL   = $14
ENAM0   = $1D
ENABL   = $1F

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
        sta COLUP0      ; missile0 white
        sta COLUPF      ; ball white
        lda #0
        sta COLUBK
        lda #2
        sta ENAM0       ; missile0 enabled
        sta ENABL       ; ball enabled

NextFrame:
        lda #2
        sta VBLANK
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC
        ; VBLANK line 1: positioning (strobe in the visible area)
        sta WSYNC
        ldy #6
DM:     dey
        bne DM          ; ~29cy delay
        sta RESM0       ; missile0 into the visible area
        ldy #6
DB:     dey
        bne DB          ; another ~29cy
        sta RESBL       ; ball further right
        ldx #36
VBlank: sta WSYNC
        dex
        bne VBlank
        lda #0
        sta VBLANK
        ldx #192
Visible:
        sta WSYNC
        dex
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
