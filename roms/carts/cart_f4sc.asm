; cart_f4sc — a 32K image the engine fingerprints as F4SC: EIGHT 4K banks, each
; with 128 bytes of Superchip RAM overlaid on the bottom of the window.
;
; The largest of the three separate-RAM Atari schemes, and the reason the whole
; family is worth witnessing separately rather than assumed from F8SC: the
; superchip fingerprint inspects the mirrored first page of EVERY bank, so the
; bank count is part of the detection, and "verified on 2 banks" is a statement
; about 2 banks. Real cartridges of this shape exist — 12 F4SC images sit in the
; umbrella's reference archive against 3 F8SC and 1 F6SC — so this is the common
; case, not the exotic one.
;
; What must happen to it is a REFUSAL. $F000-$F07F writes and $F080-$F0FF reads
; reach the same 128 RAM cells, so any static value range folded from the image
; there describes bytes the hardware never holds.
    processor 6502
VSYNC  = $00
VBLANK = $01
WSYNC  = $02
COLUBK = $09

    ; {1} = physical org of the bank, {2} = a marker byte it writes to RAM.
    MAC WORKER
    org {1}
    rorg $F000
    ds 128, $A5               ; the mirrored first page (superchip fingerprint)
    ds 128, $A5
.reset
    sei
    cld
    lda #{2}
    sta $84
    lda $FFF4                 ; back to bank 0
    jmp .reset
    org {1}+$0FFA
    rorg $FFFA
    .word .reset
    .word .reset
    .word .reset
    ENDM

    org $0000                 ; ---------------- BANK 0 ----------------
    rorg $F000
    ds 128, $A5
    ds 128, $A5
Reset0:
    sei
    cld
    ldx #$FF
    txs
    lda #0
Clr0:
    sta $80,x
    dex
    bne Clr0

Frame0:
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
VB0:
    sta WSYNC
    dex
    bne VB0
    lda #0
    sta VBLANK
    lda #$44
    sta COLUBK
    ldx #192
Vis0:
    sta WSYNC
    dex
    bne Vis0
    lda #2
    sta VBLANK
    ldx #30
OS0:
    sta WSYNC
    dex
    bne OS0
    lda $FFF5                 ; hand over to bank 1
    jmp Frame0

    org $0FFA
    rorg $FFFA
    .word Reset0
    .word Reset0
    .word Reset0

    WORKER $1000, $B1         ; ------- BANKS 1-7 -------
    WORKER $2000, $B2
    WORKER $3000, $B3
    WORKER $4000, $B4
    WORKER $5000, $B5
    WORKER $6000, $B6
    WORKER $7000, $B7
