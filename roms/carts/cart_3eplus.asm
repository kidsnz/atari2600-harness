; cart_3eplus — a 4K image the engine fingerprints as 3E+ (the "TJ3E" signature).
;
; 3E+ divides the window into FOUR 1K segments, each independently pointed at a
; 1K ROM or RAM bank. The switch value carries the segment in its top two bits:
; `sta $3F` with $41 puts ROM bank 1 in segment 1, `sta $3E` with $80 puts RAM
; bank 0 in segment 2. Nothing about that is visible to an address-based model,
; and the mapper publishes no hotspot table.
;
; The geometry is the part that matters most to this repo's static analysis: the
; engine hands back four 1024-byte banks, each of which can appear at ANY of four
; origins ($F000/$F400/$F800/$FC00). The cross-bank edge the cyclebound package
; models — "the same address in the target bank" — is simply not a true sentence
; here, on two counts at once.
;
; At reset every segment points at bank 0, so the whole window mirrors the first
; 1K of the image and the reset vectors live at the end of it (file offset $3FC).
;
; No cartridge of this type exists anywhere on this machine — measured over 493
; images under the umbrella, 0 fingerprint as 3E+ — so this fixture is the only
; witness there is.
    processor 6502
VSYNC  = $00
VBLANK = $01
WSYNC  = $02
COLUBK = $09

    org $0000                 ; ---- bank 0; at reset it fills the whole window
    rorg $1C00
Sig:
    .byte "TJ3E"              ; the engine's 3E+ fingerprint
Reset:
    sei
    cld
    ldx #$FF
    txs
    lda #0
Clr:
    sta $80,x
    dex
    bne Clr

Loop:
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
VB:
    sta WSYNC
    dex
    bne VB

    ; --- ROM bank 1 into segment 1 ($1400-$17FF); read a byte of it.
    lda #$41                  ; segment = data>>6 = 1, bank = data&$3F = 1
    sta $3F
    lda $1400
    sta $84

    ; --- RAM bank 0 into segment 2 ($1800-$1BFF); write it, read it back.
    lda #$80                  ; segment = 2, RAM bank 0
    sta $3E
    lda #$5A
    sta $1A00                 ; RAM write half
    lda $1800                 ; RAM read half — NOT an image byte
    sta $85

    lda #0
    sta VBLANK
    lda #$44
    sta COLUBK
    ldx #192
Vis:
    sta WSYNC
    dex
    bne Vis
    lda #2
    sta VBLANK
    ldx #30
OS:
    sta WSYNC
    dex
    bne OS
    jmp Loop

    org $03FA                 ; the vectors sit at the end of the FIRST 1K
    rorg $1FFA
    .word Reset
    .word Reset
    .word Reset

    org $0400                 ; ---- bank 1
    rorg $1000
    ds 1024, $D1

    org $0800                 ; ---- bank 2
    rorg $1000
    ds 1024, $D2

    org $0C00                 ; ---- bank 3
    rorg $1000
    ds 1024, $D3
