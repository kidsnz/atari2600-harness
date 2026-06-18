; cb_romtable.asm — VV-2 3D self-test ROM (ROM data-table value range).
; A divide loop is fed by a ROM TABLE read at a known index (`ldx #3; lda Tab,x`).
; The table bytes are constant and in the binary, so the prover reads them and
; bounds the loop from the table's actual value range. A tight budget flips it.
; (Real kernels read such tables with a LOOP-CARRIED index, which is Top at the
; loop header — so they stay unbounded; that is the next, deeper limitation.)

        processor 6502

VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUBK  = $09

        org $F000

Reset:
        sei
        cld
        ldx #$FF
        lda #0
ClearMem:
        sta $00,x
        dex
        bne ClearMem
        txs

Main:
        lda #2
        sta VBLANK
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC

        ldx #37
VBlank:
        sta WSYNC
        dex
        bne VBlank
        lda #0
        sta VBLANK

        ldx #95
Top:
        sta WSYNC
        dex
        bne Top

; --- line under test: divide loop fed by a ROM-table read at a known index ---
        sta WSYNC           ; opens the region
        ldx #3
        lda Tab,x           ; 3D: ROM table read -> value range over the proven index
        sec
RWait:
        sbc #15             ; 2
        bcs RWait           ; 2/3 — bounded once the table value range is known
        sta COLUBK

        ldx #95
Bottom:
        sta WSYNC
        dex
        bne Bottom

        lda #2
        sta VBLANK
        ldx #30
Overscan:
        sta WSYNC
        dex
        bne Overscan

        jmp Main

Tab:    .byte 10, 25, 40, 60, 75, 90   ; small constants (<=90) -> few divide iterations

        org $FFFC
        .word Reset
        .word Reset
