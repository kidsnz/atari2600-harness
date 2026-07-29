; litmus_superchip — an F8 image the engine fingerprints as F8SC, so the static
; analysis must decline it BY NAME rather than fold its bottom page as ROM.
;
; A superchip overlays 128 bytes of RAM on the bottom of the cartridge window:
; $F000-$F07F is a write port, $F080-$F0FF a read port, both reaching the same
; 128 cells. The image is therefore NOT what the CPU reads there.
;
; That matters because `romTableRange` folds real cartridge bytes into an EXACT
; value range, that range bounds a loop's trip count, and the trip count sets the
; proven worst case. Folding a RAM address produces a narrow, confident, wrong
; number in the one direction this project forbids. The hole was measured:
; `analysisUnits` accepted any mapper with more than one bank that published
; hotspots, and never asked whether RAM was overlaid.
;
; Superchip presence is FINGERPRINTED from the bytes, not declared — Gopher2600's
; `hasSuperchip` requires that in every bank's first page the $00-$7F half equals
; the $80-$FF half — so the same bytes are F8 or F8SC depending on the engine's own
; decision, which is precisely why the analysis has to ask instead of infer. This
; ROM satisfies that pattern deliberately: each bank opens with 128 bytes of $A5
; followed by 128 bytes of $A5, and its code lives above the overlay.
;
; The ROM is not meant to render anything interesting. What it proves is read out
; of the analysis, not out of the picture.
    processor 6502
VSYNC  = $00
VBLANK = $01
WSYNC  = $02
COLUBK = $09

    org $D000                 ; ---------------- BANK 0 ----------------
    rorg $F000
    ; The mirrored first page that makes the fingerprint fire.
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
    lda $FFF9                 ; hand over to bank 1 (a read switches)
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

    org $DFFA
    rorg $FFFA
    .word Reset0
    .word Reset0
    .word Reset0

    org $E000                 ; ---------------- BANK 1 ----------------
    rorg $F000
    ds 128, $A5
    ds 128, $A5

Reset1:
    sei
    cld
    ldx #$FF
    txs
    lda #$B1
    sta $81
    lda $FFF8                 ; back to bank 0
    jmp Reset1

    org $EFFA
    rorg $FFFA
    .word Reset1
    .word Reset1
    .word Reset1
