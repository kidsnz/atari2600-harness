; framelines_trap — a kernel whose frame is 262 lines except every 128th, which is 263.
;
; The witness for the scenario check `frame_lines_stable`. It exists because the four
; ROMs that used to demonstrate this fault for real (banked_game, exerciser and the two
; lint_bank fixtures) were repaired, and a gate whose only witness has been fixed is a
; gate that certifies nothing. Paired with framelines_clean.asm, which differs by ONE
; instruction: the `sta WSYNC` below.
;
; The period is deliberately LONG (128 frames, not 2). A stability gate covers only the
; frames it measures, and that is the failure mode this fixture has to be able to
; demonstrate: a 60-frame window PASSES this ROM and a 130-frame window catches it.
; TestFrameLinesStable pins both directions so nobody "fixes" a red check by shrinking
; the window. This mirrors the real defect it stands in for — banked_game switched level
; on a 120-frame period, which a 60-frame window would have sailed past.
;
; The extra line lands in VBLANK, ahead of the fixed 37-line WSYNC loop, which is the
; general shape of the fault: variable-cost work outside the region whose length is
; fixed leaks into the frame total instead of being absorbed by it.
;
; Deliberately has NO scenario file: every scenario in roms/**/scenarios is run and
; required to pass by TestEveryScenarioRuns, and this ROM exists to FAIL. The verdict is
; pinned by TestFrameLinesStable instead. Its twin carries scenarios/framelines_clean.json.

    processor 6502

VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUBK  = $09

fc      = $80       ; frame counter

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

    ; --- VBLANK : 37 lines, plus one more every 128th frame ---
    lda #2
    sta VBLANK
    inc fc
    lda fc
    and #$7F
    bne NoExtra
    sta WSYNC           ; THE TRAP: one extra line, once every 128 frames
NoExtra:
    ldx #37
VB:
    sta WSYNC
    dex
    bne VB
    lda #0
    sta VBLANK

    ; --- visible : 192 lines ---
    ldx #192
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

    org $FFFA
    .word Reset
    .word Reset
    .word Reset
