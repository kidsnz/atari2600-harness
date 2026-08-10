; litmus_pitchdither — can the TIA play a pitch it has no register for?
;
; The TIA's pitches are a fixed, uneven ladder: freq = clock / divisor / (AUDF+1). In
; the bass the rungs are far apart, and a note that falls between two of them cannot be
; played. Measured, that is not a rounding annoyance but a musical wall -- D and E are
; more than 23 cents off in every bass octave on every waveform, which is what forced a
; reproduction of a real record into a different key from the record's.
;
; This ROM exists to test, on the machine, whether two ways round it work:
;
;   ALTERNATION  write a different AUDF on alternate frames, so the tone's PERIOD
;                alternates between two rungs. The mean period lands between them, and
;                the arithmetic says that puts D and E within 8 cents. What it does to
;                the SOUND is the question -- a 60 Hz frame rate means a 30 Hz
;                modulation, which is squarely in the range the ear hears as roughness
;                rather than as pitch.
;
;   DETUNE       play the two rungs at once, one per channel. No modulation sidebands,
;                just a beat at the difference frequency (1.7 Hz for the pair here),
;                but it costs BOTH channels for one note.
;
; The mode is read from RAM $80 every frame, so a test can poke it and capture each
; case from the same machine rather than trusting five different builds to be alike:
;
;   0  AUDF 24 held        40.57 Hz   the flat rung        (control)
;   1  AUDF 23 held        42.26 Hz   the sharp rung       (control)
;   2  23/24 every frame              alternation, 30 Hz modulation
;   3  23/24 every 2 frames           alternation, 15 Hz modulation
;   4  ch0 23 + ch1 24                detune, both channels
;
; E1 is 41.203 Hz. Mode 0 is -26.9 cents and mode 1 is +43.9; the mean period of the
; pair is 41.396 Hz, +8.1 cents. Modes 0 and 1 are the controls that make the rest
; meaningful: if the measurement cannot recover THEM, it cannot judge anything else.
;
; Every mode holds AUDV0 (and AUDV4 in mode 4) at 8, so nothing here is an envelope.
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUBK  = $09
AUDC0   = $15
AUDC1   = $16
AUDF0   = $17
AUDF1   = $18
AUDV0   = $19
AUDV1   = $1A

mode    = $80           ; poked by the test; 0..4
frame   = $81           ; free-running frame counter, the alternation's clock

FLAT    = 24            ; the rung below the target
SHARP   = 23            ; the rung above it
VOICE   = 6             ; divisor 31, the voice a 2600 bassline uses
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
        sta mode
        sta frame

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

        ; every mode uses the same waveform and volume on ch0; only AUDF differs
        lda #VOICE
        sta AUDC0
        lda #VOL
        sta AUDV0
        lda #0
        sta AUDC1
        sta AUDV1

        lda mode
        beq M0
        cmp #1
        beq M1
        cmp #2
        beq M2
        cmp #3
        beq M3
        ; mode 4 and anything above: both channels, one rung each
        lda #SHARP
        sta AUDF0
        lda #VOICE
        sta AUDC1
        lda #FLAT
        sta AUDF1
        lda #VOL
        sta AUDV1
        jmp Set

M0:     lda #FLAT
        jmp Put
M1:     lda #SHARP
        jmp Put
M2:     lda frame               ; bit 0: a new value every frame, 30 Hz
        and #1
        jmp Pick
M3:     lda frame               ; bit 1: a new value every second frame, 15 Hz
        and #2
Pick:   beq PFlat
        lda #SHARP
        jmp Put
PFlat:  lda #FLAT
Put:    sta AUDF0
Set:

        ldx #34                 ; the rest of VBLANK
VB:     sta WSYNC
        dex
        bne VB
        lda #0
        sta VBLANK
        ; The picture is one flat field. There is nothing to see here; the whole ROM is
        ; an instrument, and a kernel that drew anything would only add writes for the
        ; cycle prover to reason about.
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
