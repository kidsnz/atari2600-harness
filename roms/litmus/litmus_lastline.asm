; litmus_lastline — a player parked near the RIGHT EDGE of every visible line.
;
; A planted known quantity for the generated-kernel line budget. A table-driven
; replay kernel that runs its loop-exit cleanup INSIDE the last visible line
; clears GRP0/GRP1 while the beam is still crossing that line — around clock 130
; by hand-count — so a sprite drawn to the right of that point survives on all
; 213 earlier lines and vanishes on the last one.
;
; Which register bites is not a guess: PF2 (the only playfield register covering
; the right edge, clocks 128-159) is cleared AFTER the line has ended, so a
; full-width playfield shows nothing at all here. GRP0/GRP1 are cleared early
; enough. Hence a player, not a playfield.
;
; P0 is positioned by a delay loop and its X is deliberately NOT asserted from
; the 3N-54 formula — the offset is kernel-specific, so the position is whatever
; the machine reports; all this ROM needs is "well right of clock 130".
;
; 262 lines: 3 VSYNC + 26 VBLANK + 214 visible + 19 overscan. The visible run is
; sized to FILL the emulator's 214-line snapshot window (top 29), so the last
; extracted line is a drawn one — otherwise the window's trailing blank lines
; hide the very pixel this ROM exists to expose.
    processor 6502
VSYNC  = $00
VBLANK = $01
WSYNC  = $02
COLUP0 = $06
COLUPF = $08
COLUBK = $09
CTRLPF = $0A
PF0    = $0D
PF1    = $0E
PF2    = $0F
RESP0  = $10
GRP0   = $1B

    org $F000

Reset:
    sei
    cld
    ldx #$FF
    txs
    lda #0
Clear:
    sta $00,x
    dex
    bne Clear

    lda #$0E            ; white player, mid-grey playfield, black background:
    sta COLUP0          ; three distinguishable elements in the attribution
    lda #$06
    sta COLUPF
    lda #0
    sta COLUBK
    sta CTRLPF

    ; Park P0 near the right edge. Done once, outside the frame loop, so the
    ; position cannot drift line to line.
    sta WSYNC
    ldx #12
Delay:
    dex
    bne Delay
    sta RESP0

Frame:
    lda #2
    sta VSYNC
    sta WSYNC
    sta WSYNC
    sta WSYNC
    lda #0
    sta VSYNC
    lda #2
    sta VBLANK
    ldx #26
VB:
    sta WSYNC
    dex
    bne VB

    lda #0
    sta VBLANK
    lda #$F0            ; left columns lit, so the frame is not player-only and
    sta PF0             ; the object attribution has something to disagree about
    lda #$FF
    sta GRP0            ; solid 8-pixel player, every visible line
    ldx #214
Vis:
    sta WSYNC
    dex
    bne Vis

    lda #0
    sta GRP0
    sta PF0
    lda #2
    sta VBLANK
    ldx #19
OS:
    sta WSYNC
    dex
    bne OS
    jmp Frame

    org $FFFA
    .word Reset
    .word Reset
    .word Reset
