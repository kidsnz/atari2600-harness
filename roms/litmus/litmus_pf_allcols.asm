; litmus_pf_allcols.asm — the playfield column map, ALL TWENTY columns at once.
;
; CLAUDE.md lists the column→register→bit table under "constants you must never get
; wrong": PF0 upper nibble only (col0→D4 … col3→D7), PF1 MSB first (col4→D7 …
; col11→D0), PF2 LSB first (col12→D0 … col19→D7), each column 4 colour clocks wide.
; litmus_pf.asm checks the leftmost bit of each register — col 0, 4 and 12 — which is
; three of the twenty positions, so seventeen bits of that table had nothing behind
; them. A table verified at its three easiest entries is a table verified at three
; entries.
;
; This draws the whole map in one frame: the visible area is split into 20 bands of 9
; scanlines, and band k lights ONLY column k. Reading any line of band k must show the
; playfield lit at clocks 4k..4k+3 and nowhere else in the left half. CTRLPF=$00
; (repeat, not reflect), so the same column repeats at 80+4k..80+4k+3 — which also
; pins the repeat rule and the left/right half boundary at clock 80.
;
; The per-column bytes come from the documented mapping, so a wrong entry in the table
; shows up as a lit column in the wrong place, not as a blank screen.
;
;  The band loop counts with inc/cmp rather than dex, so prove_line_budget declines one
;  region here — determineBound models the dex/dey and sbc-divide idioms, not inc/cmp.
;  That is the prover refusing rather than guessing, and this ROM is a playfield fixture,
;  not a budget one, so its scenario checks the frame length only.
;
; Self-contained (no vcs.h); NTSC 3/37/192/30 = 262 (20 bands x 9 = 180, + 12 filler).

        processor 6502

VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUPF  = $08
COLUBK  = $09
CTRLPF  = $0A
PF0     = $0D
PF1     = $0E
PF2     = $0F

col     = $80           ; current column 0..19
line    = $81           ; lines left in this band

BANDS   = 20
BANDH   = 9

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

        lda #$0E        ; playfield white
        sta COLUPF
        lda #$00        ; black background
        sta COLUBK
        sta CTRLPF      ; D0=0 → repeat (right half copies left), not reflect

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

; --- Visible: 20 bands of 9 lines, band k lighting column k only ---
        lda #0
        sta col
Band:   ldx col
        lda PF0Tab,x
        sta PF0
        lda PF1Tab,x
        sta PF1
        lda PF2Tab,x
        sta PF2
        lda #BANDH
        sta line
BandLine:
        sta WSYNC
        dec line
        bne BandLine
        inc col
        lda col
        cmp #BANDS
        bne Band

        ; --- 12 filler lines so the visible area is exactly 192 ---
        lda #0
        sta PF0
        sta PF1
        sta PF2
        ldx #12
Fill:   sta WSYNC
        dex
        bne Fill

; --- Overscan: 30 lines ---
        lda #2
        sta VBLANK
        ldx #30
OS:     sta WSYNC
        dex
        bne OS

        jmp Main

; The column→register→bit table, written out from the documented mapping.
PF0Tab:
        .byte $10,$20,$40,$80,$00,$00,$00,$00,$00,$00,$00,$00,$00,$00,$00,$00,$00,$00,$00,$00
PF1Tab:
        .byte $00,$00,$00,$00,$80,$40,$20,$10,$08,$04,$02,$01,$00,$00,$00,$00,$00,$00,$00,$00
PF2Tab:
        .byte $00,$00,$00,$00,$00,$00,$00,$00,$00,$00,$00,$00,$01,$02,$04,$08,$10,$20,$40,$80

        org $FFFC
        .word Reset
        .word Reset
