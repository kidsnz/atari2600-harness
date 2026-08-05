; litmus_divctx — one positioning routine, two callers that are nothing alike.
;
; The shape every real kernel with a shared SetXPos has, and the one that defeats a
; state map keyed by SITE alone:
;
;   VBLANK  : lda sprX   / jsr SetXPos   <- a RAM byte. Top, and rightly so.
;   visible : lda #HUD_X / jsr SetXPos   <- a compile-time constant.
;
; A's range AT THE ROUTINE'S LOOP HEADER is the join of those, which is Top. So the
; constant caller inherits the RAM caller's ignorance: the divide is bounded from the
; 255 floor at 19 iterations, the interval comes out over 76 cycles, and the region is
; reported as a visible-line overrun on a path that cannot happen — HUD_X is 78 and the
; loop can only run 6 times for it.
;
; What makes the right answer reachable is that the region walk ALREADY runs once per
; call site (analyzeRegionInContexts). It knows which JSR it came through, and when
; nothing between the callee's entry and the loop header can touch A — here `sec` and
; the WSYNC store — the accumulator at the call IS the accumulator at the header, for
; that context, exactly. K6 in docs/capability-gap-audit.md.
;
; TWO THINGS HAVE TO BE TRUE AT ONCE for this ROM to certify, which is why it is one
; fixture and not two:
;   1. the visible context is bounded from 78 rather than from 255, and
;   2. the VBLANK context — genuinely unbounded, genuinely expensive — is classified
;      `blank` from ITS OWN call site rather than from the callee's joined entry state,
;      so it does not outrank the visible verdict. A blank overrun adds a scanline
;      instead of tearing one, and that failure mode is gated separately by
;      frame_lines_stable and TestNoRomBreathesAcrossFrames.
; Take either half away and this ROM is NOT CERTIFIED at budget 76.
;
; Verified: scenarios/divctx.json (prove_line_budget 76 must certify).

    processor 6502

VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUBK  = $09
RESP0   = $10
RESP1   = $11
HMP0    = $20
HMP1    = $21
HMOVE   = $2A

sprX    = $84       ; a RAM byte: whatever the analysis can see, it is Top
HUD_X   = 78        ; a constant, exactly as a score or face position would be

    org $F000

Reset:
    sei
    cld
    ldx #0
    txa
.clr:
    dex
    txs
    pha
    bne .clr
    lda #40
    sta sprX
    lda #$C4
    sta COLUBK

Main:
    ; --- VSYNC : 3 lines ---
    lda #2
    sta VSYNC
    sta WSYNC
    sta WSYNC
    sta WSYNC
    lda #0
    sta VSYNC

    ; --- VBLANK : 37 lines. The sprite is placed HERE, from RAM. ---
    lda #2
    sta VBLANK
    ldx #0
    lda sprX
    jsr SetXPos         ; context 1: A is Top, and this runs inside VBLANK
    sta WSYNC
    sta HMOVE
    ldx #35
VB:
    sta WSYNC
    dex
    bne VB
    lda #0
    sta VBLANK

    ; --- visible : 192 lines. The HUD element is placed from a CONSTANT. ---
    ldx #1
    lda #HUD_X
    jsr SetXPos         ; context 2: A is 78, and this runs in the visible region
    sta WSYNC
    sta HMOVE
    ldx #190
VIS:
    sta WSYNC
    dex
    bne VIS

    ; --- Overscan : 30 lines ---
    lda #2
    sta VBLANK
    ldx #30
OS:
    sta WSYNC
    dex
    bne OS
    jmp Main

; SetXPos — A = target X, X = object. Between the caller's JSR and the loop header
; there is exactly `sec` and the WSYNC store, neither of which touches A.
SetXPos:
    sec
    sta WSYNC
.wait:
    sbc #15
    bcs .wait
    eor #7
    asl
    asl
    asl
    asl
    sta HMP0,x
    sta.w RESP0,x
    rts

    org $FFFA
    .word Reset
    .word Reset
    .word Reset
