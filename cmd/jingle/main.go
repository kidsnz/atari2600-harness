// jingle — メロディ記法 → 演奏 ROM 生成（R7 v1.35.0、T1 v1.39.0 で 2 声対応）。
// 「口ずさんだメロディを 30 秒で Stella で鳴らす」ための道具:
//
//	go run ./cmd/jingle -notes "C4:16 E4:16 G4:16 C5:32 R:16" -o jingle.asm
//	go run ./cmd/jingle -notes "..." -notes2 "..."   ; 2 声（ch0=メロディ, ch1=ハーモニー/ベース）
//
// 記法: 音名:フレーム数（R=休符）。音名→(AUDC,AUDF) は pkg/audio.FindNote（Slocum 調律）。
// 音色は声部毎に auto-pick。総フレーム長が違う場合は短い方を休符で自動パディング（ループ同期）。
// 生成物は単独 4K ROM（黒画面・262行・overscan でドライバ tick・曲はループ）。
// dasm が PATH にあれば .bin まで作る。
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/kidsnz/atari2600-harness/pkg/audio"
)

type ev struct {
	name   string
	frames int
	audf   int
	rest   bool
	cents  float64
}

func main() {
	notes := flag.String("notes", "", `voice 0 melody: "C4:16 E4:16 G4:32 R:8 ..." (name:frames, R=rest)`)
	notes2 := flag.String("notes2", "", "voice 1 melody (optional; harmony/bass on TIA channel 1)")
	out := flag.String("o", "jingle.asm", "output .asm path (assembles to .bin if dasm is available)")
	vol := flag.Int("vol", 8, "AUDV0 volume 1-15 (voice 0)")
	vol2 := flag.Int("vol2", 6, "AUDV1 volume 1-15 (voice 1)")
	soundType := flag.Int("type", 0, "force AUDC sound type for voice 0 (0 = auto-pick best per melody)")
	soundType2 := flag.Int("type2", 0, "force AUDC sound type for voice 1 (0 = auto-pick)")
	flag.Parse()
	if *notes == "" {
		fmt.Fprintln(os.Stderr, `usage: jingle -notes "C4:16 E4:16 ..." [-notes2 "..."] [-o jingle.asm] [-vol 8] [-type 4]`)
		os.Exit(2)
	}
	if err := run(*notes, *notes2, *out, *vol, *vol2, *soundType, *soundType2); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
}

func parseMelody(notes string) ([]ev, error) {
	var evs []ev
	for _, tok := range strings.Fields(notes) {
		parts := strings.SplitN(tok, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("bad token %q (want name:frames)", tok)
		}
		fr, err := strconv.Atoi(parts[1])
		if err != nil || fr < 1 || fr > 255 {
			return nil, fmt.Errorf("bad duration in %q", tok)
		}
		if strings.EqualFold(parts[0], "R") {
			evs = append(evs, ev{name: "R", frames: fr, rest: true})
			continue
		}
		evs = append(evs, ev{name: parts[0], frames: fr})
	}
	if len(evs) == 0 {
		return nil, fmt.Errorf("no notes")
	}
	return evs, nil
}

// pickType: 指定が無ければ「全音符の合計セント誤差が最小」の単一 AUDC を選ぶ
func pickType(evs []ev, forceType int) (int, error) {
	types := []int{4, 6, 7, 12}
	if forceType != 0 {
		types = []int{forceType}
	}
	bestType, bestErr := 0, 1e18
	for _, c := range types {
		sum := 0.0
		ok := true
		for _, e := range evs {
			if e.rest {
				continue
			}
			_, _, cents, err := audio.FindNote(e.name, []int{c}, audio.BaseClockNTSC)
			if err != nil {
				return 0, err
			}
			if cents > 60 || cents < -60 {
				ok = false // 半音以上ずれる音色は不採用
			}
			sum += cents * cents
		}
		if ok && sum < bestErr {
			bestType, bestErr = c, sum
		}
	}
	if bestType == 0 {
		return 0, fmt.Errorf("no sound type can play this melody within ±60 cents — try different octaves")
	}
	return bestType, nil
}

func resolve(evs []ev, audc int) error {
	for i := range evs {
		if evs[i].rest {
			continue
		}
		_, f, cents, err := audio.FindNote(evs[i].name, []int{audc}, audio.BaseClockNTSC)
		if err != nil {
			return err
		}
		evs[i].audf, evs[i].cents = f, cents
	}
	return nil
}

func total(evs []ev) int {
	t := 0
	for _, e := range evs {
		t += e.frames
	}
	return t
}

// tables: Notes/Durs の .byte 列とコメント（声部ラベル付き）
func tables(evs []ev, voice int) (data, durs, cmts string) {
	var d, u, c strings.Builder
	for i, e := range evs {
		if e.rest {
			d.WriteString("$FF")
			c.WriteString(fmt.Sprintf("; v%d %2d: R (%df)\n", voice, i, e.frames))
		} else {
			d.WriteString(fmt.Sprintf("$%02X", e.audf))
			c.WriteString(fmt.Sprintf("; v%d %2d: %-4s AUDF=%d (%+.1f cents, %df)\n", voice, i, e.name, e.audf, e.cents, e.frames))
		}
		u.WriteString(fmt.Sprintf("$%02X", e.frames))
		if i < len(evs)-1 {
			d.WriteString(",")
			u.WriteString(",")
		}
	}
	return d.String(), u.String(), c.String()
}

func run(notes, notes2, out string, vol, vol2, forceType, forceType2 int) error {
	evs, err := parseMelody(notes)
	if err != nil {
		return err
	}
	t0, err := pickType(evs, forceType)
	if err != nil {
		return fmt.Errorf("voice 0: %w", err)
	}
	if err := resolve(evs, t0); err != nil {
		return err
	}

	var evs2 []ev
	t1 := 0
	if notes2 != "" {
		evs2, err = parseMelody(notes2)
		if err != nil {
			return fmt.Errorf("voice 1: %w", err)
		}
		t1, err = pickType(evs2, forceType2)
		if err != nil {
			return fmt.Errorf("voice 1: %w", err)
		}
		if err := resolve(evs2, t1); err != nil {
			return err
		}
		// ループ同期: 総フレーム長を休符パディングで揃える
		if d := total(evs) - total(evs2); d != 0 {
			pad := d
			tgt := &evs2
			if d < 0 {
				pad, tgt = -d, &evs
			}
			for pad > 0 {
				n := pad
				if n > 255 {
					n = 255
				}
				*tgt = append(*tgt, ev{name: "R", frames: n, rest: true})
				pad -= n
			}
			fmt.Printf("note: voices differ in length; padded %d frame(s) of rest for loop sync\n", abs(d))
		}
	}

	var asm string
	if notes2 == "" {
		asm = singleVoiceASM(evs, t0, vol)
	} else {
		asm = dualVoiceASM(evs, evs2, t0, t1, vol, vol2)
	}
	if err := os.WriteFile(out, []byte(asm), 0o644); err != nil {
		return err
	}
	if notes2 == "" {
		fmt.Printf("wrote %s (voice0 AUDC=%d %s)\n", out, t0, audio.Name(t0))
	} else {
		fmt.Printf("wrote %s (voice0 AUDC=%d %s / voice1 AUDC=%d %s)\n", out, t0, audio.Name(t0), t1, audio.Name(t1))
	}
	bin := strings.TrimSuffix(out, ".asm") + ".bin"
	if _, err := exec.LookPath("dasm"); err == nil {
		if err := exec.Command("dasm", out, "-f3", "-o"+bin).Run(); err != nil {
			return fmt.Errorf("dasm: %w", err)
		}
		fmt.Println("assembled", bin)
	}
	return nil
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func singleVoiceASM(evs []ev, audc, vol int) string {
	data, durs, cmts := tables(evs, 0)
	return fmt.Sprintf(`; jingle — generated by cmd/jingle (1 voice, AUDC=%d, vol %d, %d events, looping)
%s        processor 6502
VSYNC = $00
VBLANK = $01
WSYNC = $02
AUDC0 = $15
AUDF0 = $17
AUDV0 = $19
idx  = $80
dur  = $81
        org $F000
Start:  sei
        cld
        ldx #$FF
        txs
        lda #0
Clr:    sta $00,x
        dex
        bne Clr
        lda #%d
        sta AUDC0
        jsr Advance
Frame:  lda #2
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC
        lda #2
        sta VBLANK
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
        ; --- overscan: driver tick ---
        dec dur
        bne Hold
        inc idx
        jsr Advance
Hold:   ldx #29
OS:     sta WSYNC
        dex
        bne OS
        sta WSYNC
        jmp Frame
Advance:
        ldx idx
        cpx #%d
        bcc Adv2
        ldx #0
        stx idx
Adv2:   lda Durs,x
        sta dur
        lda Notes,x
        cmp #$FF
        beq Rest
        sta AUDF0
        lda #%d
        sta AUDV0
        rts
Rest:   lda #0
        sta AUDV0
        rts
Notes:  byte %s
Durs:   byte %s
        org $FFFC
        .word Start
        .word Start
`, audc, vol, len(evs), cmts, audc, len(evs), vol, data, durs)
}

func dualVoiceASM(evs0, evs1 []ev, audc0, audc1, vol0, vol1 int) string {
	d0, u0, c0 := tables(evs0, 0)
	d1, u1, c1 := tables(evs1, 1)
	return fmt.Sprintf(`; jingle — generated by cmd/jingle (2 voices, looping)
; voice0: AUDC=%d vol=%d %d events / voice1: AUDC=%d vol=%d %d events
%s%s        processor 6502
VSYNC = $00
VBLANK = $01
WSYNC = $02
AUDC0 = $15
AUDC1 = $16
AUDF0 = $17
AUDF1 = $18
AUDV0 = $19
AUDV1 = $1A
idx0 = $80
dur0 = $81
idx1 = $82
dur1 = $83
        org $F000
Start:  sei
        cld
        ldx #$FF
        txs
        lda #0
Clr:    sta $00,x
        dex
        bne Clr
        lda #%d
        sta AUDC0
        lda #%d
        sta AUDC1
        jsr Adv0
        jsr Adv1
Frame:  lda #2
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC
        lda #2
        sta VBLANK
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
        ; --- overscan: driver tick (both channels, independent) ---
        dec dur0
        bne Hold0
        inc idx0
        jsr Adv0
Hold0:  dec dur1
        bne Hold1
        inc idx1
        jsr Adv1
Hold1:  ldx #29
OS:     sta WSYNC
        dex
        bne OS
        sta WSYNC
        jmp Frame
Adv0:   ldx idx0
        cpx #%d
        bcc Adv0b
        ldx #0
        stx idx0
Adv0b:  lda Durs0,x
        sta dur0
        lda Notes0,x
        cmp #$FF
        beq Rest0
        sta AUDF0
        lda #%d
        sta AUDV0
        rts
Rest0:  lda #0
        sta AUDV0
        rts
Adv1:   ldx idx1
        cpx #%d
        bcc Adv1b
        ldx #0
        stx idx1
Adv1b:  lda Durs1,x
        sta dur1
        lda Notes1,x
        cmp #$FF
        beq Rest1
        sta AUDF1
        lda #%d
        sta AUDV1
        rts
Rest1:  lda #0
        sta AUDV1
        rts
Notes0: byte %s
Durs0:  byte %s
Notes1: byte %s
Durs1:  byte %s
        org $FFFC
        .word Start
        .word Start
`, audc0, vol0, len(evs0), audc1, vol1, len(evs1), c0, c1,
		audc0, audc1, len(evs0), vol0, len(evs1), vol1, d0, u0, d1, u1)
}
