; litmus_divpre — the divide loop's entry value is established BEFORE the region's
; opening WSYNC, which is where the ordinary positioning idiom puts it.
;
; `lda #TARGET_X` then `jsr SetXPos`, and SetXPos's first act is `sta WSYNC`. That
; WSYNC opens the region, so the region subgraph starts AFTER it and the `lda` that
; decides how many times the `sbc #15` loop runs is not in it. determineBound's
; predecessor scan used to be confined to that subgraph, found only the latch (which
; it correctly excludes, since the back edge carries the reduced value), and fell
; through to the 255 floor: 19 iterations for a value that is 78.
;
; This ROM is the witness for widening that scan to the whole decoded program. It
; CERTIFIES at budget 76 with the widened scan and does NOT without it — measured
; 62cy vs 122cy for the same instructions.
;
; It is here because the existing corpus does not exercise the path: sweeping all 157
; ROMs before and after the change, ZERO changed their proven bound. Every real kernel
; that shares a positioning routine passes it a RAM byte (`lda xpos,x`), which is Top
; whatever the scan can see, or fails earlier for an unrelated reason —
; shared_setxpos.asm is refused at "no WSYNC reached from region start". A fix whose
; only demonstration is a throwaway probe is a fix nothing will notice losing, so the
; probe is checked in as a gate instead.
;
; Verified: scenarios/divpre.json (prove_line_budget 76 must certify).

    processor 6502

VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUBK  = $09
RESP0   = $10
HMP0    = $20
HMOVE   = $2A

TARGET_X = 78       ; a compile-time constant, exactly as a HUD element's X would be

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
    lda #$92
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

    ; --- VBLANK : 37 lines ---
    lda #2
    sta VBLANK
    ldx #37
VB:
    sta WSYNC
    dex
    bne VB
    lda #0
    sta VBLANK

    ; --- visible : 192 lines, the first two spent positioning P0 ---
    ; The constant lives HERE, on the caller's side of the callee's WSYNC.
    ldx #0
    lda #TARGET_X
    jsr SetXPos
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

; SetXPos — A = target X. Opens its own line, so the caller's `lda` sits outside the
; region this routine's body lives in.
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
