; cart_f6sc — a 16K image the engine fingerprints as F6SC: FOUR 4K banks, each
; with 128 bytes of Superchip RAM overlaid on the bottom of the window.
;
; litmus_superchip already witnesses the two-bank case (F8SC). It is not enough,
; and the reason is measured rather than argued: the superchip fingerprint checks
; the mirrored first page of EVERY bank (`hasSuperchip` in the engine's
; fingerprint.go loops `loader.Slice(0x1000, 0, 0x100)`), so the number of banks
; is part of what is being detected. A guard verified on one bank count is a guard
; verified on one bank count.
;
; The point of this ROM is what the static analysis must REFUSE. $F000-$F07F is a
; write port and $F080-$F0FF a read port onto the same 128 RAM cells, so the image
; is not what the CPU reads there — and `romTableRange` folds real cartridge bytes
; into an EXACT value range that can bound a loop's trip count. Folding a RAM
; address yields a narrow, confident, wrong number in the one direction the
; cyclebound package forbids.
;
; Each bank opens with 128 bytes of $A5 followed by 128 more, which is exactly the
; pattern a real cartridge dumper leaves behind and exactly what the engine looks
; for. The code lives above the overlay. Nothing here is meant to look interesting
; on screen; what it proves is read out of the analysis.
    processor 6502
VSYNC  = $00
VBLANK = $01
WSYNC  = $02
COLUBK = $09

    MAC SCPAGE                ; the mirrored first page that makes the fingerprint fire
    ds 128, $A5
    ds 128, $A5
    ENDM

    org $0000                 ; ---------------- BANK 0 ----------------
    rorg $F000
    SCPAGE
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
    lda $FFF7                 ; hand over to bank 1 (a read switches)
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
    jmp Frame0

    org $0FFA
    rorg $FFFA
    .word Reset0
    .word Reset0
    .word Reset0

    org $1000                 ; ---------------- BANK 1 ----------------
    rorg $F000
    SCPAGE
Reset1:
    sei
    cld
    ldx #$FF
    txs
    lda #$B1
    sta $81
    lda $FFF6                 ; back to bank 0
    jmp Reset1
    org $1FFA
    rorg $FFFA
    .word Reset1
    .word Reset1
    .word Reset1

    org $2000                 ; ---------------- BANK 2 ----------------
    rorg $F000
    SCPAGE
Reset2:
    sei
    cld
    lda #$B2
    sta $82
    lda $FFF6
    jmp Reset2
    org $2FFA
    rorg $FFFA
    .word Reset2
    .word Reset2
    .word Reset2

    org $3000                 ; ---------------- BANK 3 ----------------
    rorg $F000
    SCPAGE
Reset3:
    sei
    cld
    lda #$B3
    sta $83
    lda $FFF6
    jmp Reset3
    org $3FFA
    rorg $FFFA
    .word Reset3
    .word Reset3
    .word Reset3
