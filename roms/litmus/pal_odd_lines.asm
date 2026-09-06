; pal_odd_lines — a PAL frame that is STABLE and ODD, which a PAL set answers by dropping colour.
;
; The witness for the parity half of `frame_lines_stable`. `design.ScrollScanlinesConstant` requires
; a constant line count and, on PAL, an EVEN one; the scenario runner passes it every frame's length
; (see `internal/scenario/scenario.go`). Until this ROM existed the parity branch had been shown
; reachable only by mutation — inverting the test turned `pal.json` red — and nobody had seen a real
; odd PAL frame fail. A branch demonstrated only by breaking it is not demonstrated.
;
; 311 lines: VSYNC 3 + VBLANK 45 + picture 228 + overscan 35. Every frame identical, so the
; CONSTANCY half passes and the PARITY half is the only thing that can fail — which is what makes
; this a witness for one assertion rather than for two.
;
; The rule is seven years deep on the list. Eckhard Stolberg wrote a purpose-made ROM for it in
; 1997: *"The first one, PALLINES.BIN, is for the loss of colour problem … It seems that a program
; looses its colour signal only when it is doing an odd number of overall lines"* — having retracted
; a wrong version of his own claim ten days earlier, *"The thing about PAL systems producing wrong
; colours … was a wrong theory by me. Actually they only loose the colour signal completely."*
; 〔stella-list `199703/msg00258`, `199703/msg00204`〕. Note "overall": the count is the whole
; frame, VBLANK and overscan included 〔`200001/msg00009`〕.
;
; ★Our television takes a digital sync flag, so it cannot SHOW the colour loss — the engine renders
; this ROM happily. That is exactly why the count is checked instead: the rule is enforceable even
; where the symptom is not observable. See the configuration surface in `docs/known-traps.md`.
    processor 6502

VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUBK  = $09

    processor 6502
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
    lda #$44
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

    ; --- VBLANK : 45 lines ---
    lda #2
    sta VBLANK
    ldx #45
VB: sta WSYNC
    dex
    bne VB
    lda #0
    sta VBLANK

    ; --- visible : 228 lines ---
    ldx #228
VIS:
    sta WSYNC
    dex
    bne VIS

    ; --- overscan : 35 lines ---
    lda #2
    sta VBLANK
    ldx #35
OS: sta WSYNC
    dex
    bne OS

    ; 3 + 45 + 228 + 35 = 311, and 311 is ODD. Every frame is identical, so `frame_lines_stable`
    ; passes on constancy and can only fail on parity.
    jmp Main

    org $FFFC
    .word Reset
    .word Reset
