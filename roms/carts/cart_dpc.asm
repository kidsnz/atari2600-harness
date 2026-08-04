; cart_dpc — a 10240-byte image the engine fingerprints as DPC (Pitfall II's
; mapper): two 4K ROM banks followed by 2K of graphics data the CPU can never
; address directly.
;
; THIS IS THE DANGEROUS ONE, and the danger is the opposite of obvious. Measured:
;
;   * banks = 2, each exactly 4096 bytes, each at exactly origin $F000;
;   * it publishes $1FF8:BANK0 and $1FF9:BANK1 as HotspotBankSwitch;
;   * `mapper_dpc.go bankswitch` responds to $0FF8/$0FF9 and takes the target from
;     the ADDRESS ALONE — which is precisely the Atari rule the cyclebound package
;     models;
;   * and `GetBank` reports `IsRAM: false` for every address.
;
; So DPC clears every geometric gate the analysis has, and its bank-switch rule
; genuinely IS the modelled one. Anyone who read mapper_dpc.go and promoted "DPC"
; into `verifiedEdgeSemantics` would be doing the correct thing about the switch
; and the wrong thing about the cartridge — because the bottom of the window is
; not image bytes at all. $1000-$1007 are the random-number and music registers,
; $1008-$103F the eight data fetchers, $1040-$107F their pointer/mask writes. Those
; are computed values from cartridge state; the image's bytes there are never what
; the CPU reads. Fold them into a value range and the range bounds a loop on data
; the hardware never holds.
;
; The refusal must therefore rest on "the window is not the image", not on "this
; mapper is not in the table" — the second is true today and is one plausible
; source-reading away from being edited out.
;
; No DPC cartridge exists on this machine (0 of 493 umbrella images fingerprint as
; DPC; Pitfall II is not present), so this fixture is the only witness there is.
    processor 6502
VSYNC  = $00
VBLANK = $01
WSYNC  = $02
COLUBK = $09

    org $0000                 ; ---- ROM bank 0
    rorg $F000
    ds 4096, $C0

    org $1000                 ; ---- ROM bank 1 (the boot bank)
    rorg $F000
    ds 128, $00               ; the DPC register window; never fetched as code
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

    lda #$00
    sta $F040                 ; DF0 pointer low
    lda #$00
    sta $F050                 ; DF0 pointer high

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

    lda $F008                 ; DF0 data — a register read, not an image byte
    sta $84
    lda $F000                 ; RNG — likewise
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

    org $1FFA
    rorg $FFFA
    .word Reset
    .word Reset
    .word Reset

    org $2000                 ; ---- 2K of DPC graphics data (not in the window)
    ds 2048, $3C
