package main

// The visual-ceiling ladder (visual_ceiling). Kept in its own file so the tool
// surface can grow without everyone editing main.go — the same arrangement as
// tools_state.go.

import (
	"context"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/kidsnz/atari2600-harness/internal/ceiling"
)

type CeilingIn struct {
	Target  string   `json:"target,omitempty" jsonschema:"a .png to grade; omit to grade the CURRENT emulator frame (load_rom + step_frame first). A .bin is NOT accepted here — loading a ROM would disturb the machine you are working on."`
	Rungs   []string `json:"rungs,omitempty" jsonschema:"which rungs to compute: C1, C2, C3 (default all three)"`
	Columns int      `json:"columns,omitempty" jsonschema:"playfield columns (default 40 = the real grid); only change this to study the grid itself"`
	PNGPath string   `json:"png_path,omitempty" jsonschema:"also write the rendered ceiling picture here (see render_rung)"`
	Render  string   `json:"render_rung,omitempty" jsonschema:"which rung png_path renders (default C1)"`
	TVSpec  string   `json:"tv_spec,omitempty" jsonschema:"TV spec whose palette to quantise against (default NTSC). This MUST be the spec the frame was rendered with."`
}

type CeilingRung struct {
	Rung  string  `json:"rung"`
	Model string  `json:"model"`
	RMSE  float64 `json:"rmse"`
	SumSq float64 `json:"sum_sq_err"`
}

type CeilingDelta struct {
	From     string  `json:"from"`
	To       string  `json:"to"`
	RMSEDrop float64 `json:"rmse_drop"`
	Question string  `json:"question"`
}

type CeilingOut struct {
	Source      string         `json:"source"`
	TVSpec      string         `json:"tv_spec"`
	Width       int            `json:"width"`
	Height      int            `json:"height"`
	Pixels      int            `json:"pixels"`
	Columns     int            `json:"columns"`
	UniqueLines int            `json:"unique_lines"`
	Flat        CeilingRung    `json:"flat"`
	Rungs       []CeilingRung  `json:"rungs"`
	Deltas      []CeilingDelta `json:"deltas"`
	PNGPath     string         `json:"png_path,omitempty"`
	Note        string         `json:"note"`
}

const ceilingNote = "Each rmse is the error of the BEST picture obtainable under that rung's constraint set, " +
	"measured against the target in RGB units. It is a DENOMINATOR, not a score for any kernel, and it is a " +
	"property of (this picture, that constraint set) — scoring a sprite-drawn game under C1 says nothing about " +
	"its kernel. Read the deltas, not the rungs."

func handleCeiling(ctx context.Context, req *mcp.CallToolRequest, in CeilingIn) (*mcp.CallToolResult, CeilingOut, error) {
	mu.Lock()
	defer mu.Unlock()

	spec := in.TVSpec
	if spec == "" {
		spec = "NTSC"
	}
	pal, err := ceiling.PaletteFor(spec)
	if err != nil {
		return nil, CeilingOut{}, err
	}

	var img *image.RGBA
	source := in.Target
	switch {
	case in.Target == "":
		e, err := get()
		if err != nil {
			return nil, CeilingOut{}, err
		}
		img, _ = e.Snapshot()
		source = "current emulator frame"
		if ceiling.LooksUnrendered(img) {
			return nil, CeilingOut{}, fmt.Errorf("visual_ceiling: the framebuffer is still the cleared one (every pixel pure black, " +
				"a value the renderer never writes — its blank is (6,6,6)). Call step_frame first. Grading it would return " +
				"rmse 6.00 on every rung, which looks like an answer")
		}
	case strings.EqualFold(filepath.Ext(in.Target), ".png"):
		f, err := os.Open(in.Target)
		if err != nil {
			return nil, CeilingOut{}, err
		}
		defer f.Close()
		decoded, err := png.Decode(f)
		if err != nil {
			return nil, CeilingOut{}, err
		}
		if r, ok := decoded.(*image.RGBA); ok {
			img = r
		} else {
			b := decoded.Bounds()
			img = image.NewRGBA(b)
			for y := b.Min.Y; y < b.Max.Y; y++ {
				for x := b.Min.X; x < b.Max.X; x++ {
					img.Set(x, y, decoded.At(x, y))
				}
			}
		}
	default:
		return nil, CeilingOut{}, fmt.Errorf("visual_ceiling: target must be a .png or omitted (got %q). "+
			"To grade a ROM's frame, load_rom + step_frame it and call with no target", in.Target)
	}

	var rungs []ceiling.Rung
	for _, r := range in.Rungs {
		rungs = append(rungs, ceiling.Rung(strings.ToUpper(strings.TrimSpace(r))))
	}
	a, err := ceiling.Compute(img, pal, ceiling.Options{Columns: in.Columns, Rungs: rungs})
	if err != nil {
		return nil, CeilingOut{}, err
	}

	out := CeilingOut{
		Source: source, TVSpec: a.Result.Spec, Width: a.Result.Width, Height: a.Result.Height,
		Pixels: a.Result.Pixels, Columns: a.Result.Columns, UniqueLines: a.Result.UniqueLines,
		Flat: CeilingRung{Rung: "flat", Model: a.Result.Flat.Model, RMSE: a.Result.Flat.RMSE, SumSq: a.Result.Flat.SumSq},
		Note: ceilingNote,
	}
	for _, r := range a.Result.Rungs {
		out.Rungs = append(out.Rungs, CeilingRung{Rung: string(r.Rung), Model: r.Model, RMSE: r.RMSE, SumSq: r.SumSq})
	}
	for _, d := range a.Result.Deltas {
		out.Deltas = append(out.Deltas, CeilingDelta{From: string(d.From), To: string(d.To), RMSEDrop: d.RMSEDrop, Question: d.Question})
	}

	if in.PNGPath != "" {
		rung := ceiling.Rung(strings.ToUpper(in.Render))
		if rung == "" {
			rung = ceiling.C1
		}
		pic, err := a.Render(rung)
		if err != nil {
			return nil, CeilingOut{}, err
		}
		f, err := os.Create(in.PNGPath)
		if err != nil {
			return nil, CeilingOut{}, err
		}
		if err := png.Encode(f, pic); err != nil {
			f.Close()
			return nil, CeilingOut{}, err
		}
		f.Close()
		out.PNGPath = in.PNGPath
	}
	return nil, out, nil
}

func registerCeilingTools(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{Name: "visual_ceiling", Description: "" +
		"The DENOMINATOR for a picture: given a target frame, the smallest error any 2600 kernel could reach " +
		"for it under each of three stated constraint sets, as a ladder. Returns per rung an rmse in RGB units " +
		"(0..255 per channel, 0 = the constraint set reproduces the target exactly) plus `flat` = the error of " +
		"one flat colour per line, the weakest picture the machine can draw, to normalise against. " +
		"RUNGS: C1 = playfield only, 2 colours per line (COLUBK+COLUPF) on the 40-column x 4-clock grid. " +
		"C2 = C1 plus one 8-clock object supplying a third colour (per-clock control inside the window; the " +
		"window is aligned to the column grid, which can only understate the machine). C3 = 2 colours per line " +
		"with NO column grid — NOT 2600-achievable, included only to isolate what the 4-clock grid itself costs. " +
		"Read the DELTAS, not the rungs: C1->C2 answers 'what would one sprite buy on this picture', C1->C3 " +
		"answers 'how much is the column grid costing here'. " +
		"WHAT THIS IS NOT. It is not a score for a kernel and it grades nothing: it never looks at a ROM, only at " +
		"a picture, so it cannot tell you whether your kernel is good — it tells you how much of the remaining " +
		"error is the hardware's. A ceiling is a property of (picture, constraint set), so a low C1 on a " +
		"sprite-drawn game says only that the playfield alone cannot draw that game, NOT that its kernel is bad. " +
		"C1 measures the PICTURE, not the mechanism: a single 8-pixel player that happens to sit on a 4-clock " +
		"boundary is expressible as playfield, so C1 can be 0 for a frame drawn with a sprite. " +
		"NOT VALIDATED: no rung emits a cartridge, so none of these bounds has been proven to fit inside 76 " +
		"cycles for THIS picture. C1's reachability was demonstrated once by hand (a generated cartridge proved " +
		"at 66 cycles and reproduced a Chopper Command C1 ceiling with 0 of 29440 pixels differing); C2 and C3 " +
		"have no such evidence, and C3 is known to be unreachable by design. " +
		"target: a .png, or omit it to grade the CURRENT emulator frame (load_rom + step_frame first). A .bin is " +
		"refused because loading it would disturb the machine you are working on. Does NOT advance the emulator. " +
		"png_path renders the chosen rung as a picture, which is how the bound becomes legible: on a landscape " +
		"the C1 ceiling comes back nearly intact while the actors collapse into 4-clock smears."}, handleCeiling)
}
