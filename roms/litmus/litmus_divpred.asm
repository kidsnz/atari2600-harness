; litmus_divpred — the divide idiom's predecessor scan was still SD-9.
;
; The `sec / sbc #N / bcs` divide is the coarse-positioning idiom, and its trip count
; comes from A's upper bound entering the loop. `determineBound` found that bound with
;
;       in.nextSite() == header && at.bank == header.bank && at.addr < header.addr
;
; which is **textual fall-through plus address order** — exactly the proxy SD-9 deleted
; from the dex/dey path, left alive on this one. Its own comment said so, and kept it
; behind a counter that measured zero uses. The counter fires on a twenty-line
; cartridge.
;
; The filter is wrong in both directions at once:
;
;   MISSED    a predecessor that reaches the header by `jmp` or by a branch is not
;             `nextSite() == header`, so its value is not in the maximum. The maximum
;             then comes out too low, which is an UNDER-approximation.
;   PHANTOM   an instruction that merely SITS before the header — a `jmp` elsewhere,
;             say — has `nextSite() == header` although control never flows there, so
;             a value the machine never holds is admitted to the maximum.
;
; And when the filter finds nothing at all, a third path answers: the "closest
; `lda #imm` below the header" fallback, the same address proxy, guessing an entry
; value from a constant that may belong to unrelated code.
;
; Measured before the fix (proven vs machine):
;
;   DivDanger    27 vs 87   the big-A path arrives by `jmp` and is not scanned
;   ProxyDanger  28 vs 87   nothing is adjacent, so the `lda #imm` proxy answers
;   PhantomDanger 29 vs 89  a `jmp` sits before the header and is read as a predecessor
;
; all three with `certified: true`. Around 3x under each, and the shape is the one this
; package has already been bitten by twice.
;
; DivCtl is the control: a single entry that falls through into the header. It must
; stay bounded, so the repair is measured to cost nothing on the idiom it exists for.
; All 17 divide folds in the corpus are of that shape, which is the only reason none of
; them was wrong.
;
; The predicates read SWCHB so the abstract interpreter cannot fold an arm away — a
; constant would be pinned and the fixture would exercise a different branch, a mistake
; this repo has made twice in fixtures already.
;
; Self-contained (no vcs.h); NTSC 3/37/192/30 = 262.

        processor 6502

VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUBK  = $09
SWCHB   = $0282

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
; MISSED: the arm the machine takes reaches the header by jmp, so the scan never sees
; its A=200 and maximises over the A=10 arm instead.
DivDanger:
        sta WSYNC
        lda SWCHB
        and #$02
        beq DivSmall
        lda #$C8        ; 200 — the value the machine actually divides
        sec
        jmp DivL
DivSmall:
        lda #$0A        ; 10 — the only predecessor the old filter accepted
        sec
DivL:   sbc #$0F
        bcs DivL

; PROXY: no predecessor falls through into the header at all, so the `lda #imm`
; fallback answers with the nearest immediate below it.
ProxyDanger:
        sta WSYNC
        lda SWCHB
        and #$02
        beq ProxSmall
        lda #$C8
        sec
        jmp ProxL
ProxSmall:
        lda #$01        ; the nearest `lda #imm` below the header
        sec
        jmp ProxL
        .byte $EA       ; unreachable filler, so nothing is textually adjacent
ProxL:  sbc #$0F
        bcs ProxL

; PHANTOM: `jmp PhSkip` sits immediately before the header, so its nextSite() IS the
; header although control never goes there. The old filter read it as a predecessor.
PhantomDanger:
        sta WSYNC
        lda SWCHB
        and #$02
        beq PhSmall
        lda #$C8
        sec
        jmp PhL
PhSmall:
        lda #$0A
        sec
        jmp PhSkip
PhL:    sbc #$0F
        bcs PhL
PhSkip: nop

; Control — one entry, falling through into the header. Every divide fold in the
; corpus is this shape; it must stay bounded.
DivCtl: sta WSYNC
        lda #$3C        ; 60
        sec
DivCL:  sbc #$0F
        bcs DivCL

        lda #0
        sta COLUBK
        ldx #135
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
