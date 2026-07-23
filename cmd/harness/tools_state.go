package main

// State snapshots (save_state / restore_state) and the RAM-semantics probe
// (probe_ram_semantics). Kept in their own file so the tool surface can grow
// without everyone editing main.go.

import (
	"context"
	"fmt"
	"sort"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/kidsnz/atari2600-harness/internal/emu"
)

// slots は名前付きスナップショット置き場。MCP は逐次呼び出しなので mu で保護すれば足りる。
// ROM をロードし直したら（load_rom / assemble_and_load）中身は無効になるので破棄する。
var slots = map[string]*emu.State{}

// resetSlots は load_rom 側から呼ばれる（別の機械の状態を復元させない）。
func resetSlots() { slots = map[string]*emu.State{} }

// --- save_state ---

type SaveStateIn struct {
	Slot string `json:"slot,omitempty" jsonschema:"name to store the snapshot under (default \"default\"); saving again under the same name overwrites it"`
}
type SaveStateOut struct {
	Slot   string   `json:"slot"`
	Coords Coords   `json:"coords"`
	Slots  []string `json:"slots"` // 現在保持している全スロット名
}

func handleSaveState(ctx context.Context, req *mcp.CallToolRequest, in SaveStateIn) (*mcp.CallToolResult, SaveStateOut, error) {
	mu.Lock()
	defer mu.Unlock()

	e, err := get()
	if err != nil {
		return nil, SaveStateOut{}, err
	}
	name := in.Slot
	if name == "" {
		name = "default"
	}
	slots[name] = e.SaveState()
	return nil, SaveStateOut{Slot: name, Coords: coordsOf(e), Slots: slotNames()}, nil
}

// --- restore_state ---

type RestoreStateIn struct {
	Slot string `json:"slot,omitempty" jsonschema:"name passed to save_state (default \"default\")"`
}
type RestoreStateOut struct {
	Slot   string `json:"slot"`
	Coords Coords `json:"coords"`
}

func handleRestoreState(ctx context.Context, req *mcp.CallToolRequest, in RestoreStateIn) (*mcp.CallToolResult, RestoreStateOut, error) {
	mu.Lock()
	defer mu.Unlock()

	e, err := get()
	if err != nil {
		return nil, RestoreStateOut{}, err
	}
	name := in.Slot
	if name == "" {
		name = "default"
	}
	st, ok := slots[name]
	if !ok {
		return nil, RestoreStateOut{}, fmt.Errorf("restore_state: no slot %q (saved slots: %v)", name, slotNames())
	}
	if err := e.RestoreState(st); err != nil {
		return nil, RestoreStateOut{}, err
	}
	return nil, RestoreStateOut{Slot: name, Coords: coordsOf(e)}, nil
}

func slotNames() []string {
	names := make([]string, 0, len(slots))
	for k := range slots {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// --- probe_ram_semantics ---

type ProbeRAMIn struct {
	Addrs  []int `json:"addrs,omitempty" jsonschema:"RAM addresses to probe ($80-$FF as decimal or via peek-style ints); omit for the whole of $80-$FF"`
	Values []int `json:"values,omitempty" jsonschema:"values to write at each address; omit for 0,48,96,144,192,240"`
	Frames int   `json:"frames,omitempty" jsonschema:"frames to run after each poke before comparing (default 1)"`
	MinPix int   `json:"min_pixels,omitempty" jsonschema:"treat fewer changed pixels than this as no effect (default 1)"`
	Top    int   `json:"top,omitempty" jsonschema:"return only the N addresses with the largest effect (default: all that had any effect)"`
}
type ProbeRAMOut struct {
	Results  []emu.ProbeResult `json:"results"`
	Probed   int               `json:"addresses_probed"`
	Affected int               `json:"addresses_with_effect"`
	Frames   int               `json:"frames_per_probe"`
	Coords   Coords            `json:"coords"` // 呼び出し前と同じはず（非破壊）
}

func handleProbeRAM(ctx context.Context, req *mcp.CallToolRequest, in ProbeRAMIn) (*mcp.CallToolResult, ProbeRAMOut, error) {
	mu.Lock()
	defer mu.Unlock()

	e, err := get()
	if err != nil {
		return nil, ProbeRAMOut{}, err
	}

	opt := emu.ProbeOptions{Frames: in.Frames, MinPix: in.MinPix}
	for _, a := range in.Addrs {
		if a < 0 || a > 0xFFFF {
			return nil, ProbeRAMOut{}, fmt.Errorf("probe_ram_semantics: address %d out of range", a)
		}
		opt.Addrs = append(opt.Addrs, uint16(a))
	}
	for _, v := range in.Values {
		if v < 0 || v > 0xFF {
			return nil, ProbeRAMOut{}, fmt.Errorf("probe_ram_semantics: value %d is not a byte", v)
		}
		opt.Values = append(opt.Values, uint8(v))
	}

	res, err := e.ProbeRAMSemantics(opt)
	if err != nil {
		return nil, ProbeRAMOut{}, err
	}

	out := ProbeRAMOut{Probed: len(res), Frames: opt.Frames, Coords: coordsOf(e)}
	if out.Frames == 0 {
		out.Frames = 1
	}
	for _, r := range res {
		if r.Class != "none" {
			out.Affected++
		}
	}
	// 既定は「効果があったものだけ」を返す（128件×6サンプルは読むには多すぎる）。
	limit := in.Top
	if limit <= 0 {
		limit = out.Affected
	}
	for _, r := range res {
		if len(out.Results) >= limit {
			break
		}
		if r.Class == "none" && in.Top <= 0 {
			continue
		}
		out.Results = append(out.Results, r)
	}
	return nil, out, nil
}

// registerStateTools は main() から呼ばれる。
func registerStateTools(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{Name: "save_state", Description: "Snapshot the WHOLE machine (CPU/RAM/TIA/RIOT/cartridge/TV + the rendered framebuffer + the cycle counters) into a named slot. Pairs with restore_state to branch-search from one position: try N different inputs or N different RAM values from the SAME frame instead of replaying from load_rom each time. A slot can be restored any number of times. Does NOT advance the emulator. Not covered (append-only recorders, they do not rewind): video/audio digests, coverage, audio capture. Slots are dropped when a new ROM is loaded."}, handleSaveState)
	mcp.AddTool(server, &mcp.Tool{Name: "restore_state", Description: "Restore a slot saved by save_state — RAM, beam coords, cycle counters AND the on-screen frame all go back (the framebuffer is restored explicitly; Gopher2600's own Plumb leaves the renderer holding the diverged picture). Reusable: restore the same slot repeatedly to try alternative continuations."}, handleRestoreState)
	mcp.AddTool(server, &mcp.Tool{Name: "probe_ram_semantics", Description: "Discover what each RAM byte DOES to the picture, by experiment — for commercial ROMs where you have no source. For every address x every probe value: save, poke, run `frames` frames, diff the frame against the un-poked baseline, restore. Returns per address the changed-pixel count, bbox, per-value changed-region centroid, and a class derived from how that centroid travels: x_position / y_position (centroid moves >=8px on one axis and more than 2x the other) / appearance (it changes the picture but does not move it) / none. NON-DESTRUCTIVE: the machine is back exactly where it started when this returns. Set up the interesting game state first (in attract mode you mostly measure the title screen). Blind spot: a byte the kernel recomputes every frame before using it reads as `none` — this measures observable effect, not intent."}, handleProbeRAM)
}
