; litmus_bound_proxy — the loop-bound heuristic's proxy, broken on purpose.
;
; `determineBound` picks a counted loop's trip count from "the immediate LDX/LDY
; at the GREATEST address BELOW the loop header". That is a proxy for "the
; initialiser executed most recently before the header", and it is only as good as
; the assumption that address order matches execution order.
;
; A backward jump breaks that assumption, and no bank switching is needed to do
; it — which is the point of this ROM. If the weakness reproduces on a flat 4K
; image then it is PRE-EXISTING, not something the bank work introduces.
;
; The region below is laid out so that:
;
;   $F0xx  ldx #2      executes, then is thrown away. It sits BELOW the header,
;                      so it is the only initialiser the heuristic can see.
;   $F0yy  ldx #200    the value the loop ACTUALLY runs with. It sits ABOVE the
;                      header, reached by a forward jump, so the heuristic ignores
;                      it entirely.
;
; The loop costs 5 cycles an iteration. Run, that is ~1000 cycles; predicted from
; the decoy, ~10. A `bounded` verdict carrying the small number is an
; under-approximation of a hundredfold with no refusal and no flag — the one
; direction this project forbids. It is decidable by the machine: the interval is
; measured, so proof-versus-measurement settles who is right.
;
; The loop sits in overscan, where an overrun costs scanlines instead of tearing
; the picture, so the ROM stays inspectable while it misbehaves.
    processor 6502
VSYNC  = $00
VBLANK = $01
WSYNC  = $02
COLUBK = $09

    org $F000

Reset:
    sei
    cld
    ldx #$FF
    txs
    lda #0
Clear:
    sta $80,x
    dex
    bne Clear

Frame:
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

    lda #0
    sta VBLANK
    lda #$84
    sta COLUBK
    ldx #192
Vis:
    sta WSYNC
    dex
    bne Vis

    lda #2
    sta VBLANK

    ; ---- the region under test ---------------------------------------------
    sta WSYNC
    ldx #2                    ; DECOY: executed, then discarded. Below the header,
    jmp Init                  ; so it is the only initialiser the proxy can see.

Spin:                         ; the loop header, at a LOWER address than Init
    dex
    bne Spin
    jmp Done

Init:
    ldx #200                  ; the real trip count, ABOVE the header
    jmp Spin

Done:
    ; ---- end of the region --------------------------------------------------
    sta WSYNC
    ldx #26
OS:
    sta WSYNC
    dex
    bne OS
    jmp Frame

    org $FFFA
    .word Reset
    .word Reset
    .word Reset
