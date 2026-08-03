; litmus_latchflags — the latch must read the COUNTER's flags, not somebody else's.
;
; `determineBound` derives a loop's trip count from a `dex`/`dey` it finds in the
; body and a `bne`/`bpl` latch, on the reasoning that the counter decrements to zero
; and the branch exits there. It never checked that the LATCH READS THAT DECREMENT'S
; FLAGS. Any flag-writing instruction placed between them silently substitutes its own
; condition, and the derived trip count then describes a loop that does not exist.
;
; This is not hypothetical and it is not merely imprecise. Measured on this kernel:
; the prover answers a worst case of ~19 cycles and reports certified:true and
; roll_free:true, while the machine spends thousands in the same interval — the loop
; never terminates on the counter at all. A bound BELOW what the hardware takes is the
; one direction this package forbids, and it rides out under a certificate.
;
; THE MECHANISM. X enters at 1. `dex` makes it 0 and sets Z. Then `cpx #$02` compares
; 0 against 2, clearing Z (0 != 2) and taking the branch. X wraps to $FF, $FE, ... and
; only ever equals 2 on the way down from $FF — 255 iterations, not one. The prover
; saw `dex` + `bne` and answered 1.
;
; Reversing the inequality hides it: with `ldx #$05` and `cpx #$02` the loop exits at
; X=2 after 3 iterations while the prover still says 5, which is an OVER-approximation
; and therefore sound by luck. `roms/exerciser/exerciser.asm $F0C9` is exactly that
; shape, which is why 140 images contained the defect and none exposed it.
;
; THE TWIN. `SafeRow` is the same loop with the `cpx` removed and replaced by two
; `nop`s — same instruction count, same body cost, latch reading the decrement's own
; flags. It must stay exact. Without it, a "fix" that simply refused every counted
; loop would pass the dangerous half of this fixture.
;
; A non-flag instruction between the two is fine and must NOT be refused: `stx` writes
; memory and leaves the flags alone. `StoreRow` pins that, because the cheap repair
; ("require the decrement to be the last instruction before the latch") would reject a
; loop the hardware bounds perfectly well — Chopper Command $F39D is one.
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

; --- Visible ---
; Region A — the defect. A `cpx` sits between the decrement and the latch, so the
; branch tests the COMPARISON, not the counter. Entry 1 is below the compared value,
; so X wraps and the loop runs 255 times instead of 1.
DangerRow:
        sta WSYNC
        ldx #$01
DangerL:
        lda scratch,x
        nop
        nop
        dex
        cpx #$02        ; <- the flags the latch reads come from HERE
        bne DangerL

; Region B — the control. Identical shape, identical instruction count, but the latch
; reads the decrement's own flags. Proven and measured must agree exactly.
SafeRow:
        sta WSYNC
        ldx #$01
SafeL:
        lda scratch,x
        nop
        nop
        dex
        nop
        nop
        bne SafeL

; Region C — a non-flag instruction between the decrement and the latch. This is
; SOUND and must keep its bound: `stx` touches memory, not the flags. It exists so a
; repair cannot be bought by refusing everything that is not adjacent.
StoreRow:
        sta WSYNC
        ldx #$03
StoreL:
        lda scratch,x
        dex
        stx scratch     ; writes memory, leaves Z/N from the dex intact
        bne StoreL

        lda #0
        sta COLUBK
        ldx #188
Fill:   sta WSYNC
        dex
        bne Fill

; --- Overscan: 30 lines ---
        sta WSYNC
        lda #2
        sta VBLANK
        ldx #29
OS:     sta WSYNC
        dex
        bne OS

        jmp Main

        org $FFFC
        .word Reset
        .word Reset
