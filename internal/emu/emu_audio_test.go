package emu

import "testing"

// TestReadAudio は TIA 音声レジスタの実測読みが ROM の既知書込みと一致することを裏取りする。
// litmus_audio は ch0=(AUDC0=$0C,AUDF0=$14,AUDV0=$0A) / ch1=(AUDC1=$04,AUDF1=$1F,AUDV1=$08) を設定する。
func TestReadAudio(t *testing.T) {
	e, err := New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM("../../roms/litmus/litmus_audio.bin"); err != nil {
		t.Fatal(err)
	}
	if err := e.RunFrames(2); err != nil {
		t.Fatal(err)
	}

	got := e.ReadAudio()
	want := AudioState{
		Channel0: AudioChannel{Control: 0x0C, Freq: 0x14, Volume: 0x0A},
		Channel1: AudioChannel{Control: 0x04, Freq: 0x1F, Volume: 0x08},
	}
	if got != want {
		t.Fatalf("ReadAudio = %+v, want %+v", got, want)
	}
}
