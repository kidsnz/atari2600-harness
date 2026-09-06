package emu

import (
	"testing"

	"github.com/jetsetilly/gopher2600/hardware/tia/audio/mix"
)

// TestTheMixerIsCompressiveAndOneChannelSquashesTheOther measures the shape of the TIA's output
// stage, and in doing so settles a report from 2004 that nobody in the thread resolved.
//
// ★What was reported. A two-voice tune sounded *"brilliant and crystal clear"* under an emulator
// and *"horribly distorted"* on a Cuttle Cart, the voice line *"vibrating"*. The author analysed it
// for hours, found *"the driver did precisely what I wanted it to do"*, noticed that *"each channel
// playing on his own sounded fine"*, and cured it by **turning the volume down** — voice at 10,
// bassline at 8 〔stella-list `200405/msg00275`, Manuel Rotschkar〕. He gave a minimal recipe:
// ch0 `AUDC $C / AUDF $0E`, ch1 `AUDC $6 / AUDF $1A`, and *"when both channels are set to a lower
// volume like 8, channel 0 sounds independent from channel 1. But with a volume of $F, channel 1
// seems to modulate channel 0, so that it sounds like channel 0 is set on and off constantly"*
// 〔`msg00285`〕. Thomas Jentzsch asked what happens if the channels are swapped: *"Nothing, same
// problem"* 〔`msg00288`〕. His own guess — that it was PAL mono downmixing — was disproved two days
// later: *"I did your test … on a PAL and a NTSC 2600 Jr. I got the same results on both consoles"*
// 〔`msg00286`, Eckhard Stolberg〕. The word offered for the symptom was *"clipping"*
// 〔`msg00281`, B. Watson〕. **The thread then moved on without a mechanism.**
//
// ★★The mechanism is in the mixer's table. `mix.Mono` builds 31 entries as
// `0x7fff * vol/30 * (30 + 30) / (30 + vol)` — a compressive curve, lifting small sums and flat at
// the top. So the two channels do not add: what channel 0 contributes to the output DEPENDS ON
// WHAT CHANNEL 1 IS DOING, and the louder they both are, the more channel 0's contribution is
// squashed when channel 1 sounds.
//
// ★★★Measured here, and it matches the report in direction and in size:
//
//	volume 8  — channel 0 contributes 6898 with channel 1 silent, 4499 with it sounding: −34.8%
//	volume 15 — channel 0 contributes 10922 with channel 1 silent, 5461 with it sounding: −50.0%
//
// So raising the volume from 8 to $F deepens the modulation from about a third to a half, which is
// exactly the difference the author heard. **And at volume 8 it is already 35%** — "sounds
// independent" was the ear, not the signal. Recovered by the mailing-list distillation (helper-2),
// who computed the same figures from the formula before this was run.
//
// ★★★★A tolerance, and the reason for it: the table is built in `float32`, and this repository
// has twice pinned a machine-specific float as if it were a constant. The percentages are asserted
// to ±1 point, which is far tighter than the effect being measured (35 against 50) and loose enough
// that a rounding boundary cannot turn it red.
func TestTheMixerIsCompressiveAndOneChannelSquashesTheOther(t *testing.T) {
	// What channel 0 adds to the output, given what channel 1 is doing.
	contribution := func(v0, v1 uint8) int {
		return int(mix.Mono(v0, v1)) - int(mix.Mono(0, v1))
	}
	attenuation := func(v uint8) float64 {
		alone := contribution(v, 0)
		with := contribution(v, v)
		return 100 * float64(alone-with) / float64(alone)
	}

	at8, at15 := attenuation(8), attenuation(15)
	t.Logf("channel 0 squashed by a sounding channel 1: %.1f%% at volume 8, %.1f%% at volume 15",
		at8, at15)

	for _, c := range []struct {
		vol  uint8
		got  float64
		want float64
	}{{8, at8, 34.8}, {15, at15, 50.0}} {
		if d := c.got - c.want; d > 1 || d < -1 {
			t.Errorf("at volume %d channel 0 is squashed by %.1f%%, want %.1f%% (±1). The mixer's "+
				"curve has changed shape, which changes what every two-voice ROM in this tree "+
				"sounds like and what `golden_audio` is hashing", c.vol, c.got, c.want)
		}
	}

	// ★The claim that matters is not two numbers but the DIRECTION: louder squashes harder. That is
	// what the 2004 report is about and what "turn the volume down" fixes.
	if at15 <= at8 {
		t.Errorf("raising the volume from 8 to 15 does not deepen the squash (%.1f%% then %.1f%%). "+
			"The mixer is no longer compressive, and the 2004 report — cured by turning the volume "+
			"down — would no longer be explained by it", at8, at15)
	}

	// ★★The negative control, and the first version of it was worthless: it asked what the squash
	// is when BOTH channels are silent, which divides zero by zero and returns NaN. The question
	// that discriminates is what the same arithmetic reports for a mixer that does NOT compress. A
	// linear mixer must give exactly zero at every volume — if it does not, the figures above are
	// an artefact of how contribution is computed rather than a property of the curve.
	// Scale-free on purpose: dividing by 30 truncates, and the first version of this control was
	// off by exactly 1 from that truncation alone — an integer-division artefact wearing the face
	// of a signal, which is the very thing the control exists to rule out.
	linear := func(v0, v1 uint8) int { return (int(v0) + int(v1)) * 0x7fff }
	for _, v := range []uint8{1, 8, 15} {
		alone := linear(v, 0) - linear(0, 0)
		with := linear(v, v) - linear(0, v)
		if alone != with {
			t.Errorf("a LINEAR mixer squashes channel 0 by %d of %d at volume %d — it must squash "+
				"by nothing, so `contribution` is measuring the arithmetic and not the curve",
				alone-with, alone, v)
		}
	}

	// ★★★Superposition, stated as its own fact because it is the thing an author assumes:
	// two channels at 15 are NOT twice one channel at 15.
	sum := int(mix.Mono(15, 0)) + int(mix.Mono(0, 15))
	both := int(mix.Mono(15, 15))
	if sum <= both {
		t.Errorf("Mono(15,0)+Mono(0,15) = %d is not greater than Mono(15,15) = %d — the mixer no "+
			"longer under-adds, so mixing two voices no longer costs headroom", sum, both)
	}
	t.Logf("superposition fails by %d of %d (%.0f%%): Mono(15,0)+Mono(0,15)=%d, Mono(15,15)=%d",
		sum-both, sum, 100*float64(sum-both)/float64(sum), sum, both)
}
