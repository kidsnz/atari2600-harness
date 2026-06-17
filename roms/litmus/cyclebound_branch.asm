; cyclebound_branch.asm — VV-2 self-test ROM (planted branch-dependent overrun).
; One visible line reads a RAM flag (always 0 at runtime) and branches:
;   - flag==0 path: ~10cy of work, so a LIVE run never overruns and the runtime
;     guard assert_line_budget passes — a "lucky" pass.
;   - flag!=0 path: ~90cy of straight-line work, so that logical line's WORST
;     case is ~101cy > 76.
; The static prover (cmd/cyclebound) must FLAG this region over ALL paths even
; though the runtime guard, observing only the flag==0 path it actually takes,
; does not. This is exactly the ∃-(one run)-vs-∀-(all paths) distinction VV-2
; exists for. Self-contained (no include).

        processor 6502

VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUBK  = $09
flag    = $90           ; RAM; zeroed by ClearMem and never written -> always 0 at runtime

        org $F000

Reset:
        sei
        cld
        ldx #$FF
        txs
        lda #0
ClearMem:
        sta $00,x
        dex
        bne ClearMem

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

; --- VBLANK: 37 lines ---
        ldx #37
VBlank:
        sta WSYNC
        dex
        bne VBlank
        lda #0
        sta VBLANK

; --- Visible top: 95 normal lines ---
        lda #$1E
        sta COLUBK
        ldx #95
Top:
        sta WSYNC
        dex
        bne Top

; --- the branch-dependent line (region under test) ---
        sta WSYNC           ; opens the region
        lda flag            ; 3cy; flag is always 0 at runtime
        bne Heavy           ; runtime: not taken (light) / taken path is heavy (~90cy)
        jmp Done            ; light path
Heavy:
        REPEAT 18
        lda #0              ; 2cy  } 5cy x 18 = 90cy of straight-line work,
        sta COLUBK          ; 3cy  } reached only when flag!=0 (never at runtime)
        REPEND
Done:

; --- Visible bottom: 96 normal lines ---
        ldx #96
Bottom:
        sta WSYNC
        dex
        bne Bottom

; --- Overscan: 30 lines ---
        lda #2
        sta VBLANK
        ldx #30
Overscan:
        sta WSYNC
        dex
        bne Overscan

        jmp Main

; --- vectors ---
        org $FFFC
        .word Reset
        .word Reset
