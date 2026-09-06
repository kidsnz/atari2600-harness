; litmus_ramstrip — what a RAM-strip sprite costs per scanline, measured.
;
; `zone-multiplexing.md` lists the routes to more than two figures on a line and prices each in the
; resource it spends: FLICKER pays in frames, the missile/ball/PF pays in objects, a wide `NUSIZ`
; interleave pays "in width rather than in frames", and a missile-as-character pays in the missile.
; A fifth route pays in RAM, and the page did not have it.
;
; Andrew Davie proposed it on the list in 2003, in a thread whose subject is the whole idea —
; "[stella] What would you do with more RAM?" 〔stella-list `200305/msg00000`, 2003-05-01〕:
;
;       if you had 256 bytes reserved for each player, then you could have 'strip' sprites
;       basically like the Atari 400/800.  To modify the vertical position of a player requries
;       you to 'draw' the player shape in the correct spot in the strip of RAM - but, and here's
;       the beautiful bit, to actually draw the player in the kernel it's just...
;               lda P0strip,y
;               sta GRP0
;       That's just, what, 8 cycles per player.  Neat.  No skipdraw trickery, and to have multiple
;       players vertically, you just draw multiple players in the RAM strip.  YOu can have as many
;       as you want, and they can overlap fine, and they won't flicker when vertically overlapping.
;
; Glenn Saunders priced the other side four months later 〔`200309/msg00071`〕: "Sure, it wastes RAM
; on lines the sprites don't appear in, but it's worth it … all those RAM strips do add up and some
; SC games might want to use that RAM for other purposes".
;
; ★What this ROM measures is the eight. It is arithmetic from the instruction table — `lda abs,Y`
; is 4 and `sta abs` is 4 — and arithmetic is not a measurement, which is the rule this repository
; keeps. So the pair is timed against an empty interval, and against the same pair written zero-page
; and absolute-indexed-by-X, because the interesting question for a kernel author is not "is it 8"
; but "is there a cheaper way to write the same line".
;
;       $80  `lda Strip,y` alone, no page cross        (want 4)
;       $81  `sta GRP0` alone                           (want 3 — GRP0 is $1B, a ZERO PAGE address)
;       $82  the pair, no page cross                    (want 7)
;       $83  the pair where the indexed read CROSSES a page (want 8)
;       $8F  the empty interval, so every figure above is read as (value - $8F)
;
; ★★★And the number in the quotation is wrong by one, for a reason worth having. `sta GRP0` is a
; store to $1B — the TIA lives in the zero page, so it assembles to `sta zp`, three cycles, not
; four. Davie's pair is **seven**. The first version of this ROM tried to compare three spellings
; of the pair and got 7 three times, because every one of them ends in the same zero-page store;
; that is recorded here because the comparison looked like a comparison and was not.
;
; ★★★★But eight is reachable, and a 256-byte strip is exactly where it happens: a strip that size
; SPANS A PAGE, so an indexed read into it crosses one whenever `Y` carries. `$83` measures that —
; the same pair, +1. So the figure on the list is right for the wrong reason about half the time,
; and the kernel author's actual choice is whether the strip can be page-aligned.
;
; ★★And the constraint that decides whether the route is available at all: Davie's 256 bytes per
; player do not fit in a 2600. `design.ScrollBackgroundFitsRAM(256,0,0,0)` is false against
; `RAM2600 = 128` — this route needs a SuperChip, which is what separates it from the other four.
; That half is asserted in the Go test rather than here, because it is a fact about the machine
; rather than about this ROM.
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
GRP0    = $1B
INTIM   = $0284
TIM1T   = $0294

        org $F000
Start:
        sei
        cld
        ldx #$FF
        txs
        lda #0
Clr:    sta $00,x
        dex
        bne Clr

        ldy #4
        ldx #4

        ; --- $8F: the empty interval ---
        lda #$80
        sta TIM1T
        lda INTIM
        eor #$FF
        sta $8F

        ; --- $80: the indexed read alone ---
        lda #$80
        sta TIM1T
        lda Strip,y
        tax                 ; keep the load honest: consume A so nothing can be folded away
        lda INTIM
        eor #$FF
        sec
        sbc #2              ; ...and pay back the `tax` that the other bands do not have
        sta $80

        ; --- $81: the store alone ---
        ldy #4
        lda #$80
        sta TIM1T
        sta GRP0
        lda INTIM
        eor #$FF
        sta $81

        ; --- $82: the pair, no page cross ---
        lda #$80
        sta TIM1T
        lda Strip,y
        sta GRP0
        lda INTIM
        eor #$FF
        sta $82

        ; --- $83: the same pair, but the indexed read crosses a page ---
        ldy #$F0            ; Cross+$F0 lands beyond the page boundary below
        lda #$80
        sta TIM1T
        lda Cross,y
        sta GRP0
        lda INTIM
        eor #$FF
        sta $83

Hold:   lda #2
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC
        ldx #37
VB:     sta WSYNC
        dex
        bne VB
        ldx #192
Pic:    sta WSYNC
        dex
        bne Pic
        ldx #30
OS:     sta WSYNC
        dex
        bne OS
        jmp Hold

; A stand-in for the strip. Its contents do not matter — the cost of reading it does. It is placed
; well away from a page boundary so no measurement here pays a page-cross penalty; that penalty is
; real and is measured in `litmus_6502`, and mixing the two would confuse this ROM's subject.
        align 64
Strip:  .byte $00,$18,$3C,$7E,$FF,$7E,$3C,$18

; ★A second table placed so that `Cross,y` with y=$F0 reaches into the NEXT page — the shape a
; 256-byte strip has by construction. `align 256` then backing off 16 bytes puts the base at
; $xxF0, so any index of $10 or more carries.
        align 256
        ds 240, $00
Cross:  ds 32, $A5

        org $FFFC
        .word Start
        .word Start
