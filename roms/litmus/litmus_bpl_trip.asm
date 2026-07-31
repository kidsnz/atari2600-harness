; litmus_bpl_trip.asm — a dex/BPL and a dey/BPL countdown, each alone in a WSYNC
; region, so the prover's trip count for the BPL exit condition has a witness.
;
; `internal/cyclebound`'s determineBound accepted BNE and BPL as the latch of a
; decrement countdown and returned the SAME trip count for both. They do not end on
; the same iteration. `dex; bne L` with X=6 leaves when the decrement produces zero:
; 6 iterations. `dex; bpl L` with X=6 leaves only when the decrement produces a
; NEGATIVE value, so it runs the body once more with X=0 and exits on X=$FF:
; 7 iterations. The prover was short by one body plus one taken branch, every time.
;
; Found on the real Seaquest cartridge, not here — region $F1FC is
; `lda #$FF / ldx #$06 / L: sta $B0,x / dex / bpl L` between two WSYNCs, proven at
; 66 cycles while the machine measured 75. A proven worst case the hardware EXCEEDS
; is the one direction this package forbids; it rode out on Bounded=true, and an
; author trusting it would have believed a 75-cycle line had 10 cycles spare.
;
; The standing corpus gate (TestProvenWorstIsNeverExceededOnCorpus) never saw it, for
; a measured reason. Of 7 bpl folds across the 140 images analysed, only 4 are in
; kernels we wrote, and not one of those 4 could expose it: rts_dispatch's $F036 and
; zone_multiplex's $F033 produce NO ProfileLineWorst row at all, so the gate compares
; nothing there, and shared_setxpos's $F054 was proven 83 against a measured 36 —
; 47 cycles of slack, comfortably more than the 15 the bug cost it. Slack hides an
; under-approximation exactly as well as it hides nothing.
;
; So this ROM exists to make the fold TIGHT and MEASURABLE. Two visible kernels, each
; a single scanline, each containing one countdown and nothing else that varies:
;
;   VisX (95 lines)  ldx #6 / FillX: sta Buf,x / dex / bpl FillX     7 iterations
;                    region = 2 + [7*6 + 6*3 + 2] + 5 + 3 + 3      = 75 cycles
;   VisY (96 lines)  ldy #6 / FillY: sty COLUBK / dey / bpl FillY    7 iterations
;                    region = 2 + [7*5 + 6*3 + 2] + 5 + 3 + 3      = 68 cycles
;
; The dey form is here because the same `dec:` path serves both registers, so a fix
; that only ever ran against dex would be a fix with one witness and two claims. With
; the bug present the two regions prove 66 and 60; the machine takes 75 and 68. The
; test asserts EQUALITY, not `<=`: for this shape the trip count is exactly best+1,
; and a bound that is merely safe would send an author trimming work that was never
; over budget.
;
; Both regions are deliberately under 76 so each is exactly one scanline and the frame
; stays at 262 — an over-budget litmus would be measuring the roll, not the fold.
;
; Self-contained (no vcs.h); NTSC 3/37/192/30 = 262 (visible 95 + 1 bridge + 96).

        processor 6502

VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUBK  = $09

LineCnt = $80          ; outer per-kernel line counter
Buf     = $B0          ; $B0..$B6 — what the dex fill loop writes

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

; --- Visible A: 95 lines, dex + BPL ---
; A = $FF throughout: it is both the byte the fill writes and the byte that strobes
; WSYNC, so nothing inside the region reloads it and the region's only variable cost
; is the loop itself.
        lda #95
        sta LineCnt
        lda #$FF
VisX:   sta WSYNC              ; region opens here
        ldx #6                 ; 2
FillX:  sta Buf,x              ; 4   (zero page,X — no page-cross case exists)
        dex                    ; 2
        bpl FillX              ; 3 taken / 2 on the exit — 7 iterations, not 6
        dec LineCnt            ; 5
        bne VisX               ; 3 taken (back to a WSYNC = a region boundary)
                               ; + 3 for the WSYNC that closes the region = 75

; --- Bridge: 1 line, so 95 + 1 + 96 = 192 ---
Bridge: sta WSYNC
        lda #96
        sta LineCnt

; --- Visible B: 96 lines, dey + BPL ---
VisY:   sta WSYNC              ; region opens here
        ldy #6                 ; 2
FillY:  sty COLUBK             ; 3   (7 background stripes = the count is visible)
        dey                    ; 2
        bpl FillY              ; 3 taken / 2 on the exit — 7 iterations, not 6
        dec LineCnt            ; 5
        bne VisY               ; 3 taken
                               ; + 3 for the closing WSYNC = 68

; --- Overscan: 30 lines ---
; The first overscan WSYNC comes BEFORE the VBLANK write on purpose: it is what closes
; VisY's last region, and keeping that path cheap is what keeps VisY's worst case the
; back-edge (68) rather than the fall-through.
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
