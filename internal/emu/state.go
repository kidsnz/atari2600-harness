package emu

import (
	"fmt"
	"image"

	"github.com/jetsetilly/gopher2600/hardware"
	"github.com/jetsetilly/gopher2600/hardware/television"
	"github.com/jetsetilly/gopher2600/hardware/television/frameinfo"
	"github.com/jetsetilly/gopher2600/rewind"
)

// State is a snapshot of "the whole machine at this instant". SaveState makes one,
// RestoreState goes back to it. The use is branch search = trying N different inputs / RAM
// values from the same position and comparing the outcomes (no re-run from load_rom every
// time). The same State can be restored any number of times (Plumb re-snapshots internally
// on its side).
//
// The contents are three layers:
//   - vcs / tv : Gopher2600's hardware.State / television.State (CPU, RAM, TIA, RIOT, cart, TV)
//   - pix      : capture's latest frame image (required, for the reason below)
//   - counters : the accumulated CPU cycles this wrapper holds (where read_cycles comes from)
//
// ★ Why pix is held (measured 2026-07-23, sandbox/experiments/monet-frogger/monet_anim.bin):
// television.Plumb only swaps tv.state and **never touches the PixelRenderer at all**
// (Gopher2600 hardware/television/television.go). Measured, capture.Reset() is indeed not
// called on restore, and the framebuffer right after a restore **still holds the stale
// picture that was drawn down the branch** (its hash does not match the one at save time and
// does match the one at branch time). So without restoring pix you get the hard-to-reproduce
// lie of "the RAM went back but get_screen alone shows the future". Restoring it here makes
// a restore consistent all the way to the screen.
//
// Not covered (the honest limits; every one of them is a recorder that only ever accumulates,
// not machine state):
//   - the chained hashes of EnableVideoDigest / EnableAudioDigest (do not rewind)
//   - EnableCoverage's PC/branch coverage (does not rewind = stays cumulative)
//   - EnableAudioCapture's raw sample stream (does not rewind)
type State struct {
	vcs *hardware.State
	tv  *television.State

	pix       []uint8           // an independent copy of capture.img.Pix
	frameInfo frameinfo.Current // where the crop rectangle comes from
	cropRect  image.Rectangle   // rectangle for re-making cropImg on restore

	cpuCycles     int64
	cycleMark     int64
	paddlePlugged [2]bool
}

// Coords is the TV coordinate the State points at (frame/scanline/clock). Used to identify
// the point at which it was saved.
func (s *State) Coords() (frame, scanline, clock int) {
	c := s.tv.GetCoords()
	return c.Frame, c.Scanline, c.Clock
}

// SaveState saves the whole of the current machine state. It does not advance the emulator
// (no side effects).
func (e *Emu) SaveState() *State {
	pix := make([]uint8, len(e.cap.img.Pix))
	copy(pix, e.cap.img.Pix)

	cropRect := e.cap.frameInfo.Crop()
	if e.cap.cropImg != nil {
		cropRect = e.cap.cropImg.Bounds()
	}

	return &State{
		vcs:           e.VCS.Snapshot(),
		tv:            e.VCS.TV.Snapshot(),
		pix:           pix,
		frameInfo:     e.cap.frameInfo,
		cropRect:      cropRect,
		cpuCycles:     e.cpuCycles,
		cycleMark:     e.cycleMark,
		paddlePlugged: e.paddlePlugged,
	}
}

// RestoreState goes back to a state made by SaveState. The same State can be gone back to
// any number of times.
func (e *Emu) RestoreState(s *State) error {
	if s == nil {
		return fmt.Errorf("restore state: nil state")
	}
	if len(s.pix) != len(e.cap.img.Pix) {
		return fmt.Errorf("restore state: framebuffer size mismatch (%d != %d)", len(s.pix), len(e.cap.img.Pix))
	}

	rewind.Plumb(e.VCS, &rewind.State{VCS: s.vcs, TV: s.tv}, false)

	// ★ Restore the framebuffer (see the comment above). cropImg is a SubImage that shares
	// img.Pix, so writing img.Pix back restores it at the same time. Only the rectangle has to
	// be re-made, together with frameInfo.
	copy(e.cap.img.Pix, s.pix)
	e.cap.frameInfo = s.frameInfo
	e.cap.cropImg = e.cap.img.SubImage(s.cropRect).(*image.RGBA)

	e.cpuCycles = s.cpuCycles
	e.cycleMark = s.cycleMark
	e.paddlePlugged = s.paddlePlugged
	return nil
}
