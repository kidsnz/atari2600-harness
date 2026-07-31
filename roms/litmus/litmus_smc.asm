; litmus_smc.asm — a store whose target is decoded CODE, planted on purpose.
;
; SD-3 says a store landing in decoded code space is a fact worth reporting, and the
; detector for it had no witness: measured 2026-07-30, **133 ROMs in the corpus and
; ZERO that write into the cartridge window at all**. A detector whose branch nothing
; reaches is not a check, so it ships with this.
;
; The store here is `sta Target`, and `Target` is the address of a real, decoded,
; executed instruction in the visible loop. On the machine the write does nothing —
; cartridge ROM is read-only and a 4K image has no bank-switch hotspots to trip — so
; the ROM runs identically with and without it. That is the point: the FACT the
; analysis reports (this instruction's effective address is code) is true regardless
; of whether the hardware honours the write, and it is exactly the fact that would
; matter on a cartridge with RAM, where the write would land.
;
; Pairs with litmus_smc_clean.asm, which differs only in the store's target — RAM
; instead of code — so a report that fires on both is not reporting what it claims.
;
; Self-contained (no vcs.h); NTSC 3/37/192/30 = 262.

        processor 6502

VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUBK  = $09

scratch = $80

        org $F000

Reset:
        sei
        cld
        ldx #$FF
        txs
        lda #0
Clr:    sta $00,x
        dex
        bne Clr

Main:
; --- VSYNC: 3 lines ---
        lda #2
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC

; --- VBLANK: 37 lines ---
        lda #2
        sta VBLANK
        ldx #37
VB:     sta WSYNC
        dex
        bne VB
        lda #0
        sta VBLANK

        ; The planted store: its effective address is Target, which is an instruction.
        lda #$EA
        sta Target

; --- Visible: 192 lines ---
        ldx #192
Vis:    sta WSYNC
Target: stx COLUBK      ; <- decoded, executed, and the store above aims at it
        dex
        bne Vis

; --- Overscan: 30 lines ---
        lda #2
        sta VBLANK
        ldx #30
OS:     sta WSYNC
        dex
        bne OS

        jmp Main

        org $FFFC
        .word Reset
        .word Reset
