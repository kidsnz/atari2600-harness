; framelines_clean — the negative control for framelines_trap: 262 lines, every frame.
;
; Byte-for-byte framelines_trap.asm minus ONE instruction — the `sta WSYNC` the trap
; takes every 128th frame. It keeps the frame counter and the branch so that the only
; difference between the pair is the thing under test, and a gate that fires on both is
; firing on the counter rather than on the line it costs.
;
; Verified: scenarios/framelines_clean.json (frame_lines_stable over 200 frames == 262).

    processor 6502

VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUBK  = $09

fc      = $80       ; frame counter

    org $F000

Reset:
    sei
    cld
    ldx #0
    txa
.clr:
    dex
    txs
    pha
    bne .clr
    lda #$44
    sta COLUBK

Main:
    ; --- VSYNC : 3 lines ---
    lda #2
    sta VSYNC
    sta WSYNC
    sta WSYNC
    sta WSYNC
    lda #0
    sta VSYNC

    ; --- VBLANK : 37 lines, every frame ---
    lda #2
    sta VBLANK
    inc fc
    lda fc
    and #$7F
    bne NoExtra
NoExtra:                ; (framelines_trap.asm spends an extra `sta WSYNC` here)
    ldx #37
VB:
    sta WSYNC
    dex
    bne VB
    lda #0
    sta VBLANK

    ; --- visible : 192 lines ---
    ldx #192
VIS:
    sta WSYNC
    dex
    bne VIS

    ; --- Overscan : 30 lines ---
    lda #2
    sta VBLANK
    ldx #30
OS:
    sta WSYNC
    dex
    bne OS
    jmp Main

    org $FFFA
    .word Reset
    .word Reset
    .word Reset
