package emu

import "fmt"

// RespondsToInput answers the question every measurement on a commercial ROM should
// ask first: is this program actually running, or is it sitting in attract mode
// returning the same numbers forever?
//
// WHY THIS EXISTS. Measuring Outlaw's gunman produced a clean, stable, plausible
// answer three separate times — y_top pinned at 101, x pinned at 7, over hundreds of
// frames — and all three were wrong, because the cartridge had not been RESET and
// nothing was moving. A stuck ROM does not error and does not look stuck: it hands
// back a confident constant, which is the most dangerous shape a wrong measurement
// can take. The third of those three was taken by the author of this comment, in the
// same hour as writing down that the trap exists.
//
// THE TEST. Run `frames` with the input HELD, then rewind to the same state and run
// the same `frames` with nothing held. If the resulting RAM is byte-identical, the
// program did not react to the input. That is a decision about behaviour, not about
// any particular byte, so it needs no game-specific knowledge.
//
// It is a one-sided test and says so: `false` means "this input changed nothing",
// which is strong. `true` means "something changed" — necessary for liveness, not
// sufficient, since a title screen with an animation also changes RAM. Use it to
// REFUSE a measurement, not to bless one.
func (e *Emu) RespondsToInput(player int, action string, frames int) (responds bool, detail string, err error) {
	if frames <= 0 {
		return false, "", fmt.Errorf("frames must be positive, got %d", frames)
	}
	before := e.SaveState()
	if before == nil {
		return false, "", fmt.Errorf("could not snapshot the machine")
	}

	run := func(hold bool) ([RAMSize]uint8, error) {
		if err := e.RestoreState(before); err != nil {
			return [RAMSize]uint8{}, err
		}
		if hold {
			if err := e.SetInput(player, action, true); err != nil {
				return [RAMSize]uint8{}, err
			}
		}
		for i := 0; i < frames; i++ {
			if _, err := e.StepFrame(); err != nil {
				return [RAMSize]uint8{}, err
			}
		}
		ram, err := e.CurrentRAM()
		if hold {
			// Leave the stick centred whichever way this returns, so a caller that
			// keeps measuring afterwards is not silently holding a direction.
			_ = e.SetInput(player, action, false)
		}
		return ram, err
	}

	held, err := run(true)
	if err != nil {
		return false, "", err
	}
	idle, err := run(false)
	if err != nil {
		return false, "", err
	}
	if err := e.RestoreState(before); err != nil {
		return false, "", err
	}

	var diffs []int
	for i := range held {
		if held[i] != idle[i] {
			diffs = append(diffs, 0x80+i)
		}
	}
	if len(diffs) == 0 {
		return false, fmt.Sprintf("holding %q for %d frames changed no RAM byte at all — the program "+
			"is not reacting to input (attract mode, a title screen waiting on RESET, or a frozen "+
			"state). Any position or timing read here is a constant, not a measurement", action, frames), nil
	}
	show := diffs
	if len(show) > 8 {
		show = show[:8]
	}
	return true, fmt.Sprintf("holding %q for %d frames changed %d RAM byte(s) (e.g. %s) — necessary for "+
		"liveness, not sufficient: an animating title screen also changes RAM",
		action, frames, len(diffs), hexAddrs(show)), nil
}

func hexAddrs(a []int) string {
	out := ""
	for i, v := range a {
		if i > 0 {
			out += " "
		}
		out += fmt.Sprintf("$%02X", v)
	}
	return out
}
