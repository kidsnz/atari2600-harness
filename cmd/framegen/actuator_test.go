package main

import "testing"

// THE ZONE ACTUATOR'S REAL MAP, measured on the machine at every input it accepts. Not a
// spot check: 198 of 198 points -- the actuator's whole domain -- taken by emitting the
// exact block framegen emits
// (sta WSYNC | n x nop | sta.w RESP0 | lda #nib | sta HMP0 | sta WSYNC | sta HMOVE) and
// decomposing the scanline to see where P0 actually drew.
//
// It exists because the arithmetic says the map has slope 1 everywhere and it does not.
// zoneCoarseFine splits the input on "one nop is six colour clocks", and 6*(in/6) + in%6
// is exactly in -- but below ten nops the RESxx strobe lands at CPU cycle 2n+3 <= 21,
// inside HBLANK, and an object cannot be placed further left. THE FIRST SIXTY INPUTS ALL
// LAND IN THE SAME SIX PIXELS.
//
// The calibration seeds every object at input 40, dead centre of that flat region, and its
// update rule assumes slope 1. That is the whole of the recorded "z1P0 want 9, read
// 3 -> 7 -> 9": the first correction is computed from a reading that carries the entire
// saturation error, and is wasted.
func TestTheZoneActuatorSaturatesBelowSixtyAndTheTableSaysWhere(t *testing.T) {
	want := []int{3,4,5,6,7,8,3,4,5,6,7,8,3,4,5,6,7,8,3,4,5,6,7,8,3,4,5,6,7,8,3,4,5,6,7,8,3,4,5,6,7,8,3,4,5,6,7,8,3,4,5,6,7,8,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20,21,22,23,24,25,26,27,28,29,30,31,32,33,34,35,36,37,38,39,40,41,42,43,44,45,46,47,48,49,50,51,52,53,54,55,56,57,58,59,60,61,62,63,64,65,66,67,68,69,70,71,72,73,74,75,76,77,78,79,80,81,82,83,84,85,86,87,88,89,90,91,92,93,94,95,96,97,98,99,100,101,102,103,104,105,106,107,108,109,110,111,112,113,114,115,116,117,118,119,120,121,122,123,124,125,126,127,128,129,130,131,132,133,134,135,136,137,138,139,140,141,142,143,144,145,146}
	if len(want) != zoneInputMax+1 {
		t.Fatalf("the golden has %d entries for %d inputs", len(want), zoneInputMax+1)
	}
	for in, w := range want {
		if got := zoneReadFor(in); got != w {
			t.Errorf("input %d: zoneReadFor says %d, the machine drew at %d", in, got, w)
		}
	}
	// the finding itself, asserted rather than left in the table
	if want[0] != 3 || want[5] != 8 || want[6] != 3 {
		t.Fatalf("the bottom of the sweep reads %d %d %d -- the flat region IS the finding "+
			"and this test is worthless without it", want[0], want[5], want[6])
	}
	if want[59] > 8 {
		t.Errorf("input 59 reads %d; measured the flat region runs to 59", want[59])
	}
	if want[60] != 9 {
		t.Errorf("input 60 reads %d, want 9 -- ten nops is where the strobe first clears "+
			"HBLANK and the coarse term starts working", want[60])
	}
}

// The inverse, which is what the calibration should seed from. It must never return an
// input inside the flat region, or the seed reintroduces the wasted iteration it exists to
// remove.
func TestTheSeedNeverLandsInTheFlatRegion(t *testing.T) {
	for x := -20; x <= 160; x++ {
		in := zoneInputFor(x)
		if in < 60 {
			t.Fatalf("x=%d seeds input %d, inside the flat region", x, in)
		}
		if in > zoneInputMax {
			t.Fatalf("x=%d seeds input %d, past the actuator ceiling %d", x, in, zoneInputMax)
		}
		if x >= 9 && x <= zoneInputMax-51 && zoneReadFor(in) != x {
			t.Errorf("x=%d seeds input %d which reads %d -- the inverse is not an inverse",
				x, in, zoneReadFor(in))
		}
	}
}
