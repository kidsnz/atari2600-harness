// Package ramtrace records and describes a ROM's RAM as a time series.
//
// It is the measurement half of behavioural reproduction. Where vismatch asks
// "does it look the same?" and behavmatch's trajectory diff asks "does it move
// the same?", ramtrace asks the prior question: what is this ROM's state, and how
// does each byte of it change from frame to frame under a known input?
//
// Everything here is ROM-blind by construction — the input is a .bin and an input
// script, never a symbol map or a disassembly. That ordering is the point: an
// observation made before reading anyone's source is evidence; the same
// observation made afterwards is a confirmation of what you already believed.
package ramtrace

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/kidsnz/atari2600-harness/internal/behavmatch"
	"github.com/kidsnz/atari2600-harness/internal/emu"
	"github.com/kidsnz/atari2600-harness/internal/version"
)

// Provenance records where an artifact came from. Every file this package writes
// carries one: a measurement whose origin is unknown cannot be re-derived, and a
// number nobody can re-derive is not evidence.
type Provenance struct {
	Tool           string `json:"tool"`
	HarnessVersion string `json:"harness_version"`
	Engine         string `json:"engine"`
	ROM            string `json:"rom"`
	ROMMD5         string `json:"rom_md5"`
	Spec           string `json:"spec"`
	Scenario       string `json:"scenario"`
	Warmup         int    `json:"warmup"`
	GeneratedAt    string `json:"generated_at"`
}

// FileMD5 hashes a file — the ROM identity that pins every measurement to the
// exact bytes it was taken from.
func FileMD5(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := md5.Sum(b)
	return hex.EncodeToString(sum[:]), nil
}

// NewProvenance builds a provenance block for a recording.
func NewProvenance(tool, rom, spec, scenario string, warmup int) (Provenance, error) {
	sum, err := FileMD5(rom)
	if err != nil {
		return Provenance{}, err
	}
	return Provenance{
		Tool:           tool,
		HarnessVersion: version.Harness,
		Engine:         "gopher2600",
		ROM:            rom,
		ROMMD5:         sum,
		Spec:           spec,
		Scenario:       scenario,
		Warmup:         warmup,
		GeneratedAt:    time.Now().UTC().Format(time.RFC3339),
	}, nil
}

// ObjectSample is one TIA object's position for one frame.
type ObjectSample struct {
	Name    string `json:"name"`
	X       int    `json:"x"`
	YTop    int    `json:"y_top,omitempty"`
	Height  int    `json:"height,omitempty"`
	Present bool   `json:"present"`
}

// FrameSample is one frame of the recording.
type FrameSample struct {
	Frame      int                     `json:"frame"`
	RAM        string                  `json:"ram"` // 256 hex chars, $80..$FF
	Changed    []string                `json:"changed"`
	Inputs     behavmatch.InputState   `json:"inputs"`
	Collisions map[string]bool         `json:"collisions,omitempty"`
	SP         string                  `json:"sp"`
	Objects    map[string]ObjectSample `json:"objects,omitempty"`
}

// Doc is the recorded artifact.
type Doc struct {
	Provenance Provenance    `json:"provenance"`
	Scenario   string        `json:"scenario"`
	Frames     int           `json:"frames"`
	Samples    []FrameSample `json:"samples"`
}

var objName = [...]string{"P0", "M0", "P1", "M1", "BL"}

// collisionMap flattens the latch struct, keeping only the latched pairs. An
// empty map on every frame of a play trace is the evidence needed before claiming
// a game does not use hardware collision detection.
func collisionMap(c emu.Collisions) map[string]bool {
	out := map[string]bool{}
	for name, v := range map[string]bool{
		"p0_p1": c.P0P1, "m0_m1": c.M0M1,
		"m0_p0": c.M0P0, "m0_p1": c.M0P1, "m1_p0": c.M1P0, "m1_p1": c.M1P1,
		"p0_pf": c.P0PF, "p0_bl": c.P0BL, "p1_pf": c.P1PF, "p1_bl": c.P1BL,
		"m0_pf": c.M0PF, "m0_bl": c.M0BL, "m1_pf": c.M1PF, "m1_bl": c.M1BL,
		"bl_pf": c.BLPF,
	} {
		if v {
			out[name] = true
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// FromTrace converts a recorded trace into the serialisable document, computing
// the per-frame changed-address set.
func FromTrace(prov Provenance, tr *behavmatch.Trace, objects []int) *Doc {
	d := &Doc{Provenance: prov, Scenario: tr.Scenario, Frames: len(tr.Samples)}
	for i, s := range tr.Samples {
		fs := FrameSample{
			Frame:      i,
			RAM:        hex.EncodeToString(s.AllRAM[:]),
			Inputs:     s.Inputs,
			Collisions: collisionMap(s.Coll),
			SP:         fmt.Sprintf("$%02X", s.SP),
		}
		if i > 0 {
			prev := tr.Samples[i-1].AllRAM
			for j := 0; j < emu.RAMSize; j++ {
				if s.AllRAM[j] != prev[j] {
					fs.Changed = append(fs.Changed, fmt.Sprintf("$%02X", emu.RAMBase+j))
				}
			}
		}
		if len(objects) > 0 {
			fs.Objects = map[string]ObjectSample{}
			for _, idx := range objects {
				if idx < 0 || idx >= len(objName) {
					continue
				}
				fs.Objects[objName[idx]] = ObjectSample{
					Name: objName[idx], X: s.X[idx], YTop: s.YTop[idx],
					Height: s.Height[idx], Present: s.Present[idx],
				}
			}
		}
		d.Samples = append(d.Samples, fs)
	}
	return d
}

// WriteJSON writes v as indented JSON to path (or stdout when path is empty).
func WriteJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	if path == "" {
		_, err = os.Stdout.Write(b)
		return err
	}
	return os.WriteFile(path, b, 0o644)
}
