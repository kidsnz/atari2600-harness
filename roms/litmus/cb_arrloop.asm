; cb_arrloop.asm — VV-2 3B self-test ROM (zero-page array-element range).
; ZP is cleared at init; a masked value [0,127] is written to a ZP array; a divide
; loop then reads it back THROUGH AN INDEX (`lda arr,x`). The prover must return
; the zero-page value range for the indexed load and bound the loop. A tight
; budget must flip it. Self-contained. (Real kernels that init an array from an
; unmasked ROM table read Top into ZP first, so they stay unbounded — sound.)

        processor 6502

VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUBK  = $09
arr     = $90

        org $F000

Reset:
        sei
        cld
        ldx #$FF
        lda #0
ClearMem:
        sta $00,x           ; canonical ZP clear -> ZP initialised to 0
        dex
        bne ClearMem
        txs

Main:
; --- VSYNC: 3 lines ---
        lda #2
        sta VBLANK
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC

; --- VBLANK: write a masked value into the ZP array ---
        ldx #4
        lda #200
        and #$7F            ; [0,127] -> stored to the array
        sta arr,x
        ldx #37
VBlank:
        sta WSYNC
        dex
        bne VBlank
        lda #0
        sta VBLANK

; --- Visible top ---
        ldx #95
Top:
        sta WSYNC
        dex
        bne Top

; --- line under test: read the array THROUGH AN INDEX, divide ---
        sta WSYNC           ; opens the region
        ldx #4
        lda arr,x           ; 3B: indexed ZP load -> zero-page value range [0,127]
        sec
AWait:
        sbc #15             ; 2
        bcs AWait           ; 2/3 — bounded once arr's range is known
        sta COLUBK

; --- Visible bottom ---
        ldx #95
Bottom:
        sta WSYNC
        dex
        bne Bottom

; --- Overscan ---
        lda #2
        sta VBLANK
        ldx #30
Overscan:
        sta WSYNC
        dex
        bne Overscan

        jmp Main

        org $FFFC
        .word Reset
        .word Reset
