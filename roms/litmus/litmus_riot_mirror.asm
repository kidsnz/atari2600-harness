; litmus_riot_mirror — which address lines the RIOT's DATA registers actually decode.
;
; The 2600's RIOT sits in a 32-byte window at $0280, and the four registers there occupy
; only the bottom two address lines: SWCHA $0280, SWACNT $0281, SWCHB $0282, SWBCNT $0283.
; What the other lines do is not decoration — a kernel that reads a port through a mirror
; is reading the same latch, and one that thinks it found a NEW register is reading a
; ghost.
;
; The list settled this on real hardware. Eckhard Stolberg, measuring a 7800 in 2600 mode
; after the person asking insisted an emulator would not do 〔stella-list, found by the
; mailing-list distillation〕, reported `$288` — SWCHA with A3 set — reading back as the
; port. This ROM measures the same thing on this engine, and measures it in FOUR input
; states, because a mirror that is only ever sampled while the port reads $FF cannot be
; told apart from a register that always reads $FF.
;
; ★It carries its own negative control. A1 IS decoded — SWCHA and SWCHB are different
; registers — so if the test's mirrors all agree AND $0280 still differs from $0282, the
; agreement is a property of A3/A4 and not of a decoder that has stopped discriminating.
        processor 6502
SWCHA   = $0280
SWACNT  = $0281
SWCHB   = $0282
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
        sta SWACNT          ; every joystick line an INPUT, so the port reflects the stick

; Read in the LOOP, not once at reset: an input applied after the first frame must reach
; the recorded bytes. Reading at reset was this ROM's first version and it reported $FF in
; every state, which is exactly the failure the header warns about.
Loop:
        lda SWCHA
        sta $80             ; the register itself
        lda $0288
        sta $81             ; + A3
        lda $0290
        sta $82             ; + A4
        lda $0298
        sta $83             ; + A3 + A4
        lda SWCHB
        sta $84             ; ★the negative control: a DIFFERENT register, one line away
        lda $028A
        sta $85             ; SWCHB + A3
        jmp Loop

        org $FFFC
        .word Start
        .word Start
