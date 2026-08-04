; cart_3e — an 8K image the engine fingerprints as 3E (Tigervision plus RAM).
;
; 3E is not shaped like Atari's schemes and every difference is one this repo's
; static analysis has to see:
;
;   * banks are 2K, not 4K. Only the LOW half of the window ($1000-$17FF) is
;     switchable; $1800-$1FFF is permanently the last bank, which is where the
;     reset vectors must live.
;   * the switch is a DATA value written to a non-cartridge address. `sta $3F`
;     selects a ROM bank and `sta $3E` selects a RAM bank, both taking the target
;     from the byte on the bus. The mapper publishes NO hotspot table at all
;     (`// !!TODO: hotspot info for 3e` in mapper_3e.go), so an address-based
;     model does not lose precision here — it misses the switch entirely.
;   * writing $3E maps 1K of cartridge RAM over the low half: $1000-$13FF reads
;     it, $1400-$17FF writes it. The image is then not what the CPU reads there.
;
; The `sta $3E` / `lda #$00` pair below is also literally the engine's 3E
; fingerprint (the byte sequence 85 3E A9 00), which is why the ROM must switch
; RAM in rather than merely mention the address.
;
; Real cartridges of this shape are in the umbrella's reference archive
; (DeathMerchant, three DungeonStalker builds), all 24 banks of 2K.
    processor 6502
VSYNC  = $00
VBLANK = $01
WSYNC  = $02
COLUBK = $09

    org $0000                 ; ---- ROM bank 0 (switchable into $1000-$17FF)
    rorg $1000
    ds 2048, $C0

    org $0800                 ; ---- ROM bank 1
    rorg $1000
    ds 2048, $C1

    org $1000                 ; ---- ROM bank 2
    rorg $1000
    ds 2048, $C2

    org $1800                 ; ---- ROM bank 3, FIXED at $1800-$1FFF
    rorg $1800
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

    ; --- map RAM bank 0 into the low segment, write it, read it back.
    lda #0
    sta $3E                   ; 85 3E ...
    lda #$00                  ; ... A9 00  = the engine's 3E fingerprint
    sta $1400                 ; RAM write port
    lda $1000                 ; RAM read port — NOT an image byte
    sta $84

    ; --- map ROM bank 1 into the low segment and read a byte of it.
    lda #1
    sta $3F
    lda $1000
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
    rorg $1FFA
    .word Reset
    .word Reset
    .word Reset
