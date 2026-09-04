; litmus_swacnt — what writing the port-A direction register actually does.
;
; `fundamentals-audit.md` carried this as ⬜ with the measurement already named: *"Measure what
; writing SWACNT does, which is the truth table … not the default."* No ROM in this repository
; wrote SWACNT at all, so the direction register had never been driven here.
;
; The list drove it. stella-list `poor-man-s-cart-dumper` (2005-08, Fred Quimby) is a cartridge
; dumper in which **the 2600 itself talks serial out of a joystick port** at 1200 baud — it sets
; SWACNT to make port A an output and then writes SWCHA. A third party reports it working:
; *"I was able to get this and your original send.bin program WORKING! … The 1200 baud timing seems
; to work out just fine."* (Glenn Saunders). That is a working report, not a document, and it is why
; this ROM exists. Only the fact that SWACNT is written was taken from that post; no code was.
;
; Four bands, each recording SWCHA into a different RAM byte so `read_ram` reads the whole table:
;   $80  SWACNT=$00 (all inputs)   -> the peripheral's value; idle stick reads $FF
;   $81  SWACNT=$FF (all outputs), SWCHA<-$A5  -> does the write read back?
;   $82  SWACNT=$F0 (high nibble out), SWCHA<-$5A -> split port: latch above, peripheral below
;   $83  SWACNT=$00 again          -> does the latched value survive going back to input?
;
; The last band is the one worth having: it says whether a program that drives the port can hand it
; back, which is what any two-console link (`ask #11`) has to do to reverse direction.
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUBK  = $09
SWCHA   = $0280
SWACNT  = $0281

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

        ; band 1 — all inputs (the power-on state the engine gives us)
        lda #$00
        sta SWACNT
        lda SWCHA
        sta $80

        ; band 2 — all outputs, then write a pattern
        lda #$FF
        sta SWACNT
        lda #$A5
        sta SWCHA
        lda SWCHA
        sta $81

        ; band 3 — half output. The high nibble should come from what we wrote, the low nibble
        ; from the peripheral, which is what makes a 4-bit bidirectional link possible at all.
        lda #$F0
        sta SWACNT
        lda #$5A
        sta SWCHA
        lda SWCHA
        sta $82

        ; band 4 — hand the port back
        lda #$00
        sta SWACNT
        lda SWCHA
        sta $83

        ; band 5 — the SAME as band 2, but waiting the 400 us the Programmer's Guide demands
        ; between writing this port and reading it. 400 us x 1.19318 MHz = 477 cycles = 6.3 lines.
        ; Bands 1-4 read on the very next instruction, ~4 cycles, so they all violate it. If band 5
        ; equals band 2, the engine models no delay at all and the table above is the engine's
        ; answer rather than the console's.
        lda #$FF
        sta SWACNT
        ldy #96                 ; 96 iterations x 5 cy = 480 cycles > 477
Wait:   dey
        bne Wait
        lda #$A5
        sta SWCHA
        lda SWCHA
        sta $84

NextFrame:
        lda #2
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC
        lda $81           ; put band 2 on the screen so the picture is not blank
        sta COLUBK
        ldy #0
Outer:  ldx #0
Line:   sta WSYNC
        inx
        cpx #133
        bne Line
        iny
        cpy #1
        beq Outer2
        jmp NextFrame
Outer2: ldx #0
Line2:  sta WSYNC
        inx
        cpx #126
        bne Line2
        jmp NextFrame

        org $FFFC
        .word Start
        .word Start
