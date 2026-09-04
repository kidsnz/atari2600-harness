; litmus_audc_carrier — which AUDC settings produce NO tone, only a DC level.
;
; Eckhard Stolberg, stella-list 199902/msg00036:
;   "If you set the AUDCx register to 0 or 11, the output will always be high."
;
; This project builds TIA PCM on AUDC=0 alone — `docs/techniques/tia-pcm.md` says "Silence the tone
; generators. AUDC=0 on both channels", and `litmus_pcm.asm`'s header says the same. If 11 is also a
; silent carrier then there are TWO, which is an escape route when AUDC is wanted for something else
; on the channel you are using as a DC output.
;
; The ROM takes AUDC from RAM $82 and AUDF from RAM $83 and writes them with the CPU every frame, so
; the register write is a real store and only the *choice* comes from the test. AUDV0 is held at 10,
; so a channel that never toggles reports a constant sample of 10 = the volume written, which is the
; property PCM needs (AUDV IS the amplitude).
;
; Why the frequency has to be swept as well, and this is the finding the source does not carry:
; **AUDC 2, 6, 10 and 14 also read as constant — but only at AUDF=31.** Sample one frequency and
; four settings join the club that do not belong in it. 0 and 11 are constant at every AUDF tried.
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
AUDC0   = $15
AUDF0   = $17
AUDV0   = $19
CTRL    = $82          ; AUDC value, written by the test
FREQ    = $83          ; AUDF value, written by the test

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

NextFrame:
        lda #2
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC
        ; the CPU performs the register writes — the test only chooses the values
        lda CTRL
        sta AUDC0
        lda FREQ
        sta AUDF0
        lda #10
        sta AUDV0
        ; 259 more lines: 3 (VSYNC) + 259 = 262
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
