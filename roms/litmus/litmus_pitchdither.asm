; litmus_pitchdither — can the TIA play a pitch it has no register for?
;
; The TIA's pitches are a fixed, uneven ladder: freq = clock / divisor / (AUDF+1). In
; the bass the rungs are far apart, and a note that falls between two of them cannot be
; played. Measured, that is not a rounding annoyance but a musical wall -- D and E are
; more than 23 cents off in every bass octave on every waveform, which is what forced a
; reproduction of a real record into a different key from the record's.
;
; The way round it is to write one AUDF for a few frames and the adjacent one for the
; next few, so the tone's PERIOD alternates and the ear integrates the mean. Whether
; that produces a pitch or a trill depends on how the swap period compares with the
; NOTE's period, which changes with every note -- so this ROM takes the note and the
; swap rate as parameters instead of hard-coding one case:
;
;   $80  mode   0 = hold the flat rung        (control)
;               1 = hold the sharp rung       (control)
;               2 = alternate between them every $84 frames
;               3 = play both at once, one per channel
;   $82  AUDC   the waveform
;   $83  AUDF   the FLAT rung of the pair; the sharp one is $83-1
;   $84  swap   frames each rung is held, for mode 2
;
; All four are read every frame, so one machine produces every case rather than several
; builds that are assumed to be alike. Power-on values give the case this was written
; for: AUDC 6, AUDF 24/23, swapping every 2 frames, which is E1.
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
AUDC0   = $15
AUDC1   = $16
AUDF0   = $17
AUDF1   = $18
AUDV0   = $19
AUDV1   = $1A

mode    = $80
frame   = $81           ; free-running frame counter
audc    = $82
flat    = $83           ; the rung BELOW the target (the larger AUDF)
swap    = $84           ; frames per rung in mode 2
phase   = $85           ; counts down to the next swap
cur     = $86           ; 0 = flat is sounding, 1 = sharp
delay   = $87           ; scanlines to wait before the AUDF write lands. The swap's
                        ; result turned out to depend on this, which is the difference
                        ; between a usable rule and a coincidence, so it is a parameter.
tmp     = $88

VOL     = 8

        org $F000
Start:  sei
        cld
        ldx #$FF
        txs
        lda #0
Clr:    sta $00,x
        dex
        bne Clr
        lda #6                  ; the defaults are the E1 case this was written for
        sta audc
        lda #24
        sta flat
        lda #2
        sta swap
        lda #1
        sta phase

Frame:  lda #2
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC
        lda #2
        sta VBLANK

        inc frame

        lda audc                ; ch0 carries the note in every mode
        sta AUDC0
        lda #VOL
        sta AUDV0
        lda #0                  ; ch1 is silent unless mode 3 turns it on
        sta AUDC1
        sta AUDV1
        sta WSYNC               ; the mode dispatch does not fit on the same line as the
                                ; channel setup: measured 90 cycles of 76 without this
        lda mode
        beq Flat                ; 0: the flat rung, held
        cmp #1
        beq Sharp               ; 1: the sharp rung, held
        cmp #2
        beq Alt                 ; 2: alternate
        ; 3 and above: both rungs at once, one per channel
        lda flat
        sta AUDF1
        lda audc
        sta AUDC1
        lda #VOL
        sta AUDV1
        jmp Sharp

Alt:    dec phase               ; hold each rung for `swap` frames
        bne Keep
        lda swap
        sta phase
        lda cur
        eor #1
        sta cur
Keep:   lda cur
        bne Sharp
Flat:   lda flat
        jmp Put
Sharp:  lda flat
        sec
        sbc #1
Put:    sta tmp

        ldx delay               ; wait `delay` lines, write, then use up the rest, so the
        beq Now                 ; VBLANK stays the same length whatever the delay is
Wait:   sta WSYNC
        dex
        bne Wait
Now:    lda tmp
        sta AUDF0
        lda #31
        sec
        sbc delay
        tax
VB:     sta WSYNC
        dex
        bne VB
        lda #0
        sta VBLANK
        ; One flat field. The whole ROM is an instrument; a kernel that drew anything
        ; would only add writes for the cycle prover to reason about.
        ldx #192
Pic:    sta WSYNC
        dex
        bne Pic
        lda #2
        sta VBLANK
        ldx #30
OS:     sta WSYNC
        dex
        bne OS
        jmp Frame

        org $FFFC
        .word Start
        .word Start
