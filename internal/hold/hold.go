// Package hold keeps buttons of the left joystick down while a ROM runs, for the
// commands that draw or record a ROM from the command line (cmd/still, cmd/sound).
//
// A picture or a sound that answers to a button cannot be taken by a tool that presses
// none, and until these commands took -hold the works wrote their own (one work's
// shot_held.py and trace_audio.py, another's measuring driver).
//
// The spec is "fire,left@60": a name alone is down from power-on, name@N from frame N
// on. Frames count from 1, and a button whose first frame is N is put down BEFORE frame
// N is stepped, so it is down for the whole of frame N and of every frame after it.
package hold

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/kidsnz/atari2600-harness/internal/emu"
)

// A Hold is one button of the left joystick, down from a frame on.
type Hold struct {
	Action string
	From   int // the first frame it is down for; 1 = from power-on
}

// Parse reads "fire,left@60": names SetInput knows, each with an optional first frame.
func Parse(spec string) ([]Hold, error) {
	var out []Hold
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		h := Hold{Action: part, From: 1}
		if name, at, ok := strings.Cut(part, "@"); ok {
			n, err := strconv.Atoi(at)
			if err != nil || n < 1 {
				return nil, fmt.Errorf("-hold %q: the frame after @ has to be a number from 1 up", part)
			}
			h = Hold{Action: name, From: n}
		}
		switch h.Action {
		case "fire", "left", "right", "up", "down":
		default:
			return nil, fmt.Errorf("-hold %q: want fire, left, right, up or down (the left joystick)", part)
		}
		out = append(out, h)
	}
	return out, nil
}

// Press puts down every button whose first frame is f. Call it before frame f is stepped.
func Press(e *emu.Emu, holds []Hold, f int) error {
	for _, h := range holds {
		if h.From == f {
			if err := e.SetInput(0, h.Action, true); err != nil {
				return err
			}
		}
	}
	return nil
}

// Note says, for a command's one line of output, which buttons were down, so that what
// was taken with a button held cannot be mistaken for the ROM at rest.
func Note(spec string) string {
	if strings.TrimSpace(spec) == "" {
		return ""
	}
	return "  held: " + spec
}
