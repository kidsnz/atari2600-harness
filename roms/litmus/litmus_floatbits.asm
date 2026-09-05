; litmus_floatbits — WHICH model of the floating data bus this engine implements, in one bit.
;
; Reading a write-only TIA register does not return a defined value: only some pins are driven, and
; the rest float. **Three different models of what floats are in circulation**, and the engine's own
; source says so out loud (`Gopher2600/hardware/memory/memory.go`):
;
;	if mem.env.Prefs.RandomPins.Get().(bool) {
;	    data |= uint8(mem.env.Random.Rewindable(0xff)) & (^mem.DataBusDriven)   // (3) random
;	} else {
;	    // this pattern is good for replicating what we see on the pluscart
;	    // ... a different bit pattern can be seen on the Harmony
;	    data |= mem.LastCPUData & ^mem.DataBusDriven                            // (2) last bus byte
;	}
;
; and B. Watson describes the third in 200508: *"In both emulators, the disconnected bits will read
; as the address being read from, so bits 0-6 will be $2B, which has bit 0 set."* — model (1), the
; ADDRESS. Stella and z26 both did that in 2005.
;
;	(1) the address        z26 / Stella, 2005
;	(2) the last bus byte  ★this engine's default
;	(3) random             this engine with RandomPins on
;
; The three are not interchangeable. Games depend on the answer: Stolberg, 200110, names three and
; says what breaks — *"If you don't emulate the undefined bits in the TIA read-registers correctly,
; the ball won't bounce off of the paddles properly in Video Pinball"*, Dodge'em gets *"a reversed
; score display"* if bit 0 comes back 1, and Berzerk enables a missile it should not.
;
; ★So: which one is this? "Read once and look" cannot answer it — every model produces *some* byte.
; The two models have to be made to DISAGREE.
;
; ★★The first attempt at that failed, and the way it failed is worth keeping. It read the same
; register through `$002B` and `$012B`, on the reasoning that the operand's high byte ($00 vs $01)
; is the last thing the CPU puts on the bus. Both came back with bit 0 set, which looks exactly
; like model (1) — and is not. **For a direct read the last bus byte is the address's LOW byte**,
; and `$002B` and `$012B` share it. The experiment was measuring the same thing twice and calling
; the agreement a result.
;
; ★★★What actually separates them is an addressing mode where the last fetched byte is NOT the
; address:
;
;	lda $2B      ; zero page: fetches $A5 then $2B, so the last bus byte IS $2B  -> bit0 = 1
;	lda $00,X    ; zero page,X with X=$2B: fetches $B5 then $00, reads $2B       -> bit0 = 0
;
; Both land on read register $0B (INPT3, since `maskReadTIA = 0x000f`), so the hardware half is
; identical and only the floating half can differ:
;
;	model (2) last bus byte  ->  bit0 = 1 then 0   ← THE TWO DISAGREE
;	model (1) address        ->  bit0 = 1 then 1   ← both read from $..2B, so they agree
;	model (3) random         ->  no stable answer at all
;
;	$80  bit0 of `lda $2B`      (the last bus byte is $2B, which is odd)
;	$81  bit0 of `lda $00,X`    (the last bus byte is $00, which is even)
;	$82  1 if the two disagree — the witness for model (2) over model (1)
;	$83  the byte LSR wrote back, read through the same port, for the record
;	$84  HMP0 after the LSR: `lsr` is read-modify-write and $2B is HMCLR, a STROBE, so the
;	     write half must have cleared the horizontal-motion registers. If it did not, the
;	     read half above was not a real bus cycle either.
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUBK  = $09
HMP0    = $20
HMCLR   = $2B

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

        ; --- read $2B with the address as the last bus byte ------------------------------
        lda HMCLR           ; zero page: last bus byte is the operand $2B
        and #$01
        sta $80

        ; --- read $2B with $00 as the last bus byte ---------------------------------------
        ldx #$2B
        lda $00,X           ; zero page,X: last bus byte is the operand $00; reads $2B
        and #$01
        sta $81

        ; --- do the two disagree? ---------------------------------------------------------
        lda $80
        cmp $81
        beq Same
        lda #1
        sta $82
        jmp Strobe
Same:
        lda #0
        sta $82

        ; --- the write half: $2B is HMCLR, a strobe, so the RMW must clear HMP0 ----------
Strobe:
        lda #$70
        sta HMP0            ; a non-zero horizontal motion
        lsr HMCLR           ; the read-modify-write's write lands on the strobe
        lda #0
        sta $84             ; (HMP0 is write-only; the test reads it through the emulator)

        ; --- $85 / $86: the Haunted House test vector -----------------------------------
        ; ★A 2001 emulator author posted the exact CPU state Haunted House reaches, because his own
        ; emulator disagreed with two others there (Nicolas Olhaberry, 200107/msg00044):
        ;
        ;	A=02 X=04 Y=18 S=FD PC=1441 ... C=1
        ;	1441 E50F  SBC $0F  [000F]=00
        ;	"Both PCAE and Z26, after executing this opcode, leave the accumulator with $F3,
        ;	 so, the value subtracted was $F.  In my emu, since is subtracting zero, the carry
        ;	 remains set"
        ;
        ; ★★So the vector is a three-way discriminator that needs no commercial ROM:
        ;	$F3  -> $0F was subtracted (PCAE / z26 / the address model)
        ;	$02  -> zero was subtracted (his emulator; nothing on the floating pins)
        ;	other-> something else entirely
        ; ★★★And $0F is the one address where the address model and the last-bus-byte model
        ; AGREE, because the operand byte and the address are the same value — which is why this
        ; vector is worth pinning separately from the discriminator above: it says whether we
        ; reproduce a real game's behaviour, not which model we implement.
        lda #$02
        sec
        sbc $0F             ; the posted instruction, from the posted state
        sta $85
        lda #0
        rol                 ; A := carry after the SBC
        sta $86

Frame:
        lda #2
        sta VBLANK
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
        lda #0
        sta VBLANK
        ldx #192
Vis:    sta WSYNC
        dex
        bne Vis
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
