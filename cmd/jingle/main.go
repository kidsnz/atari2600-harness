// jingle — メロディ記法 → 演奏 ROM 生成（R7, v1.35.0 = 作曲セッションの足回り）。
// 「口ずさんだメロディを 30 秒で Stella で鳴らす」ための道具:
//   go run ./cmd/jingle -notes "C4:16 E4:16 G4:16 C5:32 R:16" -o jingle.asm
// 記法: 音名:フレーム数（R=休符）。音名→(AUDC,AUDF) は pkg/audio.FindNote（Slocum 調律）。
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

func main() {
	notes := flag.String("notes", "", `melody: "C4:16 E4:16 G4:32 R:8 ..." (name:frames, R=rest)`)
	out := flag.String("o", "jingle.asm", "output .asm path (assembles to .bin if dasm is available)")
	vol := flag.Int("vol", 8, "AUDV volume 1-15")
	soundType := flag.Int("type", 0, "force AUDC sound type (0 = auto-pick best per melody)")
	flag.Parse()
	if *notes == "" {
		fmt.Fprintln(os.Stderr, `usage: jingle -notes "C4:16 E4:16 ..." [-o jingle.asm] [-vol 8] [-type 4]`)
		os.Exit(2)
	}
	if err := run(*notes, *out, *vol, *soundType); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
}

func run(notes, out string, vol, forceType int) error {
	type ev struct {
		name   string
		frames int
		audf   int
		rest   bool
		cents  float64
	}
	var evs []ev
	for _, tok := range strings.Fields(notes) {
		parts := strings.SplitN(tok, ":", 2)
		if len(parts) != 2 {
			return fmt.Errorf("bad token %q (want name:frames)", tok)
		}
		fr, err := strconv.Atoi(parts[1])
		if err != nil || fr < 1 || fr > 255 {
			return fmt.Errorf("bad duration in %q", tok)
		}
		if strings.EqualFold(parts[0], "R") {
			evs = append(evs, ev{name: "R", frames: fr, rest: true})
			continue
		}
		evs = append(evs, ev{name: parts[0], frames: fr})
	}
	if len(evs) == 0 {
		return fmt.Errorf("no notes")
	}
	// 音色決定: 指定が無ければ「全音符の合計セント誤差が最小」の単一 AUDC を選ぶ
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
				return err
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
		return fmt.Errorf("no sound type can play this melody within ±60 cents — try different octaves")
	}
	for i := range evs {
		if evs[i].rest {
			continue
		}
		_, f, cents, err := audio.FindNote(evs[i].name, []int{bestType}, audio.BaseClockNTSC)
		if err != nil {
			return err
		}
		evs[i].audf, evs[i].cents = f, cents
	}

	var data, durs, cmts strings.Builder
	for i, e := range evs {
		if e.rest {
			data.WriteString("$FF")
			cmts.WriteString(fmt.Sprintf("; %2d: R (%df)\n", i, e.frames))
		} else {
			data.WriteString(fmt.Sprintf("$%02X", e.audf))
			cmts.WriteString(fmt.Sprintf("; %2d: %-4s AUDF=%d (%+.1f cents, %df)\n", i, e.name, e.audf, e.cents, e.frames))
		}
		durs.WriteString(fmt.Sprintf("$%02X", e.frames))
		if i < len(evs)-1 {
			data.WriteString(",")
			durs.WriteString(",")
		}
	}

	asm := fmt.Sprintf(`; jingle — cmd/jingle 生成（音色 AUDC=%d, 音量 %d, %d イベント・ループ演奏）
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
        ; --- overscan: ドライバ tick ---
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
`, bestType, vol, len(evs), cmts.String(), bestType, len(evs), vol, data.String(), durs.String())

	if err := os.WriteFile(out, []byte(asm), 0o644); err != nil {
		return err
	}
	fmt.Printf("wrote %s (AUDC=%d %s)\n", out, bestType, audio.Name(bestType))
	bin := strings.TrimSuffix(out, ".asm") + ".bin"
	if _, err := exec.LookPath("dasm"); err == nil {
		if err := exec.Command("dasm", out, "-f3", "-o"+bin).Run(); err != nil {
			return fmt.Errorf("dasm: %w", err)
		}
		fmt.Println("assembled", bin)
	}
	return nil
}
