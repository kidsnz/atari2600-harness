; litmus_amax_floor — "the analysis cannot pin A" is not "A has no bound".
;
; The `sec / sbc #N / bcs` divide is how a sprite gets positioned, and its trip count
; comes from A's value entering the loop. `determineBound` maximises that over the
; header's predecessors and REFUSES when any of them carries a Top accumulator —
; correctly, because SD-9's proxy guessed one and under-approximated by 40x.
;
; But refusing every unpinned entry conflates two different statements:
;
;       "this analysis does not know the value"      true, and unavoidable
;       "this value has no upper bound"              FALSE about a 6502 accumulator
;
; A is eight bits. Whatever the machine puts in it, it is at most 255. That is a fact
; about the HARDWARE, not a range inferred from the program, which is why using it
; does not reopen the door SD-9 closed: the failure there was reading a number off
; the wrong instruction, not reading the register's width off the datasheet.
;
; FOUND ON THE USER'S OWN ROM. `sandbox/practice/pizza-boy-tokyo/build/pizza_boy.asm`
; positions its bike and taxis through
;
;       lda px
;       jsr SetXPos          ; sec / sta WSYNC / .wait: sbc #15 / bcs .wait / ...
;
; and `px` is a RAM byte — Top by construction, at all five call sites. Every call
; context died on this line, so the region came back "no WSYNC reached from region
; start", and that refusal is what kept the project's own `phase0` scenario red after
; the BRK half of the same failure was fixed.
;
; THE BOUND IS LOOSE ON PURPOSE. 255/15 + 2 = 19 iterations, where a real sprite
; coordinate cannot exceed about 160. Loose is the correct direction — the author can
; tighten it with `@amax`, and that annotation is still the only way to declare a
; ceiling the analysis could not derive.
;
; THE ROWS:
;
;   FloorRow    A is read from INTIM while the RIOT is counting, so nothing static
;               can pin it. Must be BOUNDED, at the 255-derived count. Before the
;               change: refused, and the whole region with it.
;
;               Two earlier attempts at this row FAILED AS FIXTURES and are worth
;               recording, because both were bounded before the change and proved
;               nothing: `lda SWCHB / and #$0F` (the interpreter follows the mask and
;               knows [0,15]) and two different `sta $90` on the arms of an undecidable
;               branch (it joins them into [7,200]). The abstract interpreter is good
;               at ranges; the value has to come from HARDWARE that moves on its own.
;   KnownCtl    A comes from an immediate the analysis CAN pin (`lda #60`). Its bound
;               must be TIGHTER than FloorRow's — if the two come out equal, the
;               floor has replaced the real scan instead of standing under it.
;
; Self-contained (no vcs.h); NTSC 3/37/192/30 = 262.

        processor 6502

VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUBK  = $09
SWCHB   = $0282
INTIM   = $0284
TIM64T  = $0296

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
        ; Arm the timer; INTIM below is read while it is still counting.
        lda #40
        sta TIM64T

; --- Visible ---
; THE ROW: A is loaded from RAM whose value nothing can pin. Bounded at the
; 255-derived count, not refused.
FloorRow:
        sta WSYNC
        lda INTIM       ; the RIOT is counting; nothing static can pin this
        sec
FloorL: sbc #15
        bcs FloorL

; CONTROL — A is an immediate the scan CAN read. Its bound must be tighter, or the
; floor has replaced the scan rather than standing under it.
KnownCtl:
        sta WSYNC
        lda #60
        sec
KnownL: sbc #15
        bcs KnownL

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
