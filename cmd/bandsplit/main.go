// Command bandsplit cuts a recording into frequency bands and writes ONE self-contained page
// that plays each of them, so a person can point at the sound they mean.
//
//	ffmpeg -i track.mp3 -ac 1 -ar 44100 -c:a pcm_s16le track.wav
//	bandsplit -wav track.wav -out pick.html && open pick.html
//	bandsplit -files bass.wav,drums.wav,other.wav -out pick.html      # already-separated sources
//
// WHY IT EXISTS. Every other tool here answers a question about a sound that has already been
// identified. Identifying it is the step before, it is not a measurement, and it was the step
// that actually blocked the work: an author asked for "the most prominent melodic sound" to be
// reproduced and there was no way to establish which sound that was. Guessing produced a bass
// reproduction that was not the thing wanted. Splitting the record into bands and letting the
// author say "B and C" settled it in one exchange — and then the page that did it was thrown
// away, so the next work would have started by writing it again.
//
// WHAT TO WATCH FOR when reading the answer. A band is not an instrument. A part whose
// fundamental is 97 Hz has its second harmonic at 194 Hz, so it is audible in a 110-300 band
// with its fundamental missing — which is exactly what happened on the record this came from,
// and it is why the author's correct answer of "B" still pointed at a band that EXCLUDED the
// fundamental. Take the answer as "this is the part I mean" and then run cmd/f0check to find
// out where that part actually lives before measuring anything.
//
// TWO MODES, because the same page answered the question twice on the same job. `-wav` splits ONE
// file into frequency bands. `-files` takes SEVERAL files and puts them side by side, which is what
// you want once a separator has produced stems — and separating is usually the right next step,
// because a band is a slice of the mix and a stem is an instrument. On the job this comes from,
// the band page got the author to "B" and the stem page got him to "the bass stem", and only the
// second one was the sound he meant. The stem page was hand-written the first time; this is it.
//
// The page embeds decoded audio and needs no network. It is written for one listener with
// headphones, not for publishing: the excerpt is the source recording, so treat the output the
// way the source is treated.
package main

import (
	"encoding/base64"
	"encoding/binary"
	"flag"
	"fmt"
	"html"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/kidsnz/atari2600-harness/internal/audioingest"
)

type band struct {
	key, note string
	lo, hi    float64
}

// The default split. The boundaries are not arbitrary: 110 Hz is under most bass fundamentals
// and over almost no melodic ones; 300 and 800 bracket where a mid part's first harmonics sit;
// 2500 up is hats and air. F is the whole pitched range at once, which is what to reach for
// when the parts are hard to separate by ear one band at a time.
var defaults = []band{
	{"A", "bass region", 32, 110},
	{"B", "low mid", 110, 300},
	{"C", "mid", 300, 800},
	{"D", "high mid", 800, 2500},
	{"E", "highs — hats, cymbals, air", 2500, 14000},
	{"F", "everything but the bass and the highs", 110, 2500},
}

func main() {
	wav := flag.String("wav", "", "16-bit PCM WAV to split into bands")
	files := flag.String("files", "", "instead: comma-separated WAVs to put side by side (stems from a separator)")
	out := flag.String("out", "", "page to write (required — there is deliberately no default)")
	from := flag.Float64("from", 0, "start of the excerpt, in seconds")
	dur := flag.Float64("dur", 8, "length of the excerpt, in seconds — the page embeds every band, so this sets its size")
	title := flag.String("title", "", "heading for the page (default: the wav's name)")
	bandsFlag := flag.String("bands", "", "override the split: name,lo,hi;name,lo,hi;...")
	flag.Parse()

	// -out has no default ON PURPOSE. It used to default to "bandsplit.html" in the working
	// directory, and this is a PUBLIC repository: running the tool from inside it writes a
	// multi-megabyte page with the source recording embedded straight into the working tree,
	// one `git add -A` away from publishing a client's unreleased master. The umbrella's rule
	// that other people's material never enters a repository is structural, and a convenient
	// default that quietly breaks it is worse than no default.
	if *out == "" {
		fmt.Fprintln(os.Stderr, "-out is required: this tool embeds the source audio in the page,")
		fmt.Fprintln(os.Stderr, "so where it lands is a decision and not a default. Write it OUTSIDE")
		fmt.Fprintln(os.Stderr, "any repository that must not carry the recording.")
		os.Exit(2)
	}
	if (*wav == "") == (*files == "") {
		fmt.Fprintln(os.Stderr, "give exactly one of -wav (split into bands) or -files (compare sources)")
		os.Exit(2)
	}
	if *files != "" {
		if err := compareFiles(strings.Split(*files, ","), *out, *from, *dur, *title); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		return
	}
	bands := defaults
	if *bandsFlag != "" {
		var err error
		if bands, err = parseBands(*bandsFlag); err != nil {
			fmt.Fprintf(os.Stderr, "-bands: %v\n", err)
			os.Exit(2)
		}
	}
	raw, err := os.ReadFile(*wav)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	samples, rate, err := audioingest.DecodeWAV(raw)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	i0, i1 := int(*from*float64(rate)), int((*from+*dur)*float64(rate))
	if i0 < 0 {
		i0 = 0
	}
	if i1 > len(samples) {
		i1 = len(samples)
	}
	if i0 >= i1 {
		fmt.Fprintf(os.Stderr, "-from %.2f +%.2fs is outside the file (%.2fs)\n",
			*from, *dur, float64(len(samples))/float64(rate))
		os.Exit(2)
	}
	clip := samples[i0:i1]

	head := *title
	if head == "" {
		head = *wav
	}
	var b strings.Builder
	fmt.Fprintf(&b, pageHead, html.EscapeString(head), html.EscapeString(head),
		*from, *from+float64(i1-i0)/float64(rate))

	// The unsplit excerpt first. Without it there is nothing to compare a band against, and
	// "which of these is the part" is a harder question than "which of these sounds like that".
	b.WriteString(row("—", "the excerpt, unsplit", 0, 0, dataURI(clip, rate)))
	for _, bd := range bands {
		f := audioingest.BandPass(clip, rate, bd.lo, bd.hi)
		normalise(f)
		b.WriteString(row(bd.key, bd.note, bd.lo, bd.hi, dataURI(f, rate)))
	}
	b.WriteString(pageTail)

	if err := os.WriteFile(*out, []byte(b.String()), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	fmt.Printf("wrote %s  (%d bands + the unsplit excerpt, %.1f s each, %.1f MB)\n",
		*out, len(bands), float64(i1-i0)/float64(rate), float64(b.Len())/(1<<20))
	fmt.Println("Open it, play the bands, and ask which one holds the part.")
	fmt.Println("Then run cmd/f0check on that part: a band can carry a harmonic of something")
	fmt.Println("whose fundamental is outside it, and that is the usual way this goes wrong.")
}

// normalise brings each band to the same peak. Bands differ in level by tens of dB — the bass
// region of a dance record can be 30 dB over the mid — and a listener asked to compare them
// at their natural levels is really being asked which is loudest.
func normalise(x []float64) {
	var peak float64
	for _, v := range x {
		if a := math.Abs(v); a > peak {
			peak = a
		}
	}
	if peak == 0 {
		return
	}
	k := 0.89 / peak // leave headroom so the 16-bit conversion cannot clip
	for i := range x {
		x[i] *= k
	}
}

func dataURI(x []float64, rate int) string {
	return "data:audio/wav;base64," + base64.StdEncoding.EncodeToString(encodeWAV(x, rate))
}

// encodeWAV writes 16-bit mono PCM. DecodeWAV reads the same format and nothing else, on
// purpose, so a page written here can be fed straight back into the tools that made it.
func encodeWAV(x []float64, rate int) []byte {
	n := len(x)
	buf := make([]byte, 44+2*n)
	copy(buf[0:], "RIFF")
	binary.LittleEndian.PutUint32(buf[4:], uint32(36+2*n))
	copy(buf[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(buf[16:], 16)
	binary.LittleEndian.PutUint16(buf[20:], 1)
	binary.LittleEndian.PutUint16(buf[22:], 1)
	binary.LittleEndian.PutUint32(buf[24:], uint32(rate))
	binary.LittleEndian.PutUint32(buf[28:], uint32(rate*2))
	binary.LittleEndian.PutUint16(buf[32:], 2)
	binary.LittleEndian.PutUint16(buf[34:], 16)
	copy(buf[36:], "data")
	binary.LittleEndian.PutUint32(buf[40:], uint32(2*n))
	for i, v := range x {
		s := v
		if s > 1 {
			s = 1
		} else if s < -1 {
			s = -1
		}
		binary.LittleEndian.PutUint16(buf[44+2*i:], uint16(int16(s*32767)))
	}
	return buf
}

func parseBands(s string) ([]band, error) {
	var out []band
	for _, part := range strings.Split(s, ";") {
		f := strings.Split(part, ",")
		if len(f) != 3 {
			return nil, fmt.Errorf("want name,lo,hi — got %q", part)
		}
		lo, e1 := strconv.ParseFloat(strings.TrimSpace(f[1]), 64)
		hi, e2 := strconv.ParseFloat(strings.TrimSpace(f[2]), 64)
		if e1 != nil || e2 != nil || lo <= 0 || hi <= lo {
			return nil, fmt.Errorf("want 0 < lo < hi — got %q", part)
		}
		out = append(out, band{strings.TrimSpace(f[0]), "", lo, hi})
	}
	return out, nil
}

func row(key, note string, lo, hi float64, uri string) string {
	rng := "the whole excerpt"
	if hi > 0 {
		rng = fmt.Sprintf("%.0f – %.0f Hz", lo, hi)
	}
	n := ""
	if note != "" {
		n = "<span>" + html.EscapeString(note) + "</span>"
	}
	return fmt.Sprintf(`<div class="row"><div class="k">%s</div>
 <div class="t"><b>%s</b>%s</div>
 <audio controls preload="none" src="%s"></audio></div>
`, html.EscapeString(key), rng, n, uri)
}

const pageHead = `<!doctype html><html lang="en"><head><meta charset="utf-8">
<title>%s — bands</title><meta name="robots" content="noindex, nofollow">
<meta name="viewport" content="width=device-width, initial-scale=1">
<style>:root{color-scheme:dark}
body{background:#0a0a0a;color:#ddd;font:14px/1.7 -apple-system,BlinkMacSystemFont,sans-serif;
margin:0;padding:30px 20px 60px}
h1{font-size:13px;letter-spacing:.2em;text-transform:uppercase;color:#eee;margin:0 0 4px}
header p{color:#666;font:11px ui-monospace,Menlo,monospace;margin:0 0 26px}
.row{display:flex;align-items:center;gap:16px;max-width:760px;margin:0 auto 12px;
border:1px solid #1e1e1e;border-radius:6px;padding:12px 16px}
.k{font:700 15px ui-monospace,Menlo,monospace;color:#e0a33a;width:24px;flex:none}
.t{flex:1;min-width:0}
.t b{display:block;font:12px ui-monospace,Menlo,monospace;color:#ddd;font-weight:600}
.t span{color:#777;font-size:11.5px}
audio{width:280px;flex:none}
footer{color:#555;font-size:11.5px;max-width:760px;margin:26px auto 0;line-height:1.8}
</style></head><body>
<header><h1>%s</h1><p>excerpt %.2f – %.2f s &middot; each band normalised to the same peak</p></header>
`

const pageTail = `<footer>Every band is brought to the same peak level, because bands differ by tens of
dB and comparing them at their natural levels asks which is loudest rather than which holds the
part. <b>A band is not an instrument:</b> a part whose fundamental is 97 Hz has its second
harmonic at 194 Hz and is audible in a 110–300 Hz band with its fundamental missing. Point at
the band that holds the part, then measure where that part actually lives.</footer>
</body></html>
`

// compareFiles is the -files mode: several sources, one page, all at the same peak. It is the
// same page as the band mode and for the same reason — the question "which of these is the
// sound you mean" is answered by pointing, not by describing.
func compareFiles(paths []string, out string, from, dur float64, title string) error {
	var rows []string
	head := title
	if head == "" {
		head = "sources"
	}
	var b strings.Builder
	var rate int
	for i, p := range paths {
		p = strings.TrimSpace(p)
		raw, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		samples, r, err := audioingest.DecodeWAV(raw)
		if err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		rate = r
		i0, i1 := int(from*float64(r)), int((from+dur)*float64(r))
		if i0 < 0 {
			i0 = 0
		}
		if i1 > len(samples) {
			i1 = len(samples)
		}
		if i0 >= i1 {
			return fmt.Errorf("%s: -from %.2f +%.2fs is outside the file", p, from, dur)
		}
		clip := append([]float64(nil), samples[i0:i1]...)
		// Level BEFORE normalising is a finding in itself: a stem 25 dB under the others is one
		// the separator found nothing for, and on the job this comes from that is exactly what
		// `other` was. Print it, then normalise so the comparison is about content.
		var peak, sum float64
		for _, v := range clip {
			if a := math.Abs(v); a > peak {
				peak = a
			}
			sum += v * v
		}
		rms := math.Sqrt(sum / float64(len(clip)))
		normalise(clip)
		note := fmt.Sprintf("%s &middot; %.1f dB before levelling", filepath.Base(p),
			20*math.Log10(rms/32768+1e-12))
		rows = append(rows, row(fmt.Sprint(i+1), note, 0, 0, dataURI(clip, r)))
	}
	fmt.Fprintf(&b, pageHead, html.EscapeString(head), html.EscapeString(head), from, from+dur)
	for _, r := range rows {
		b.WriteString(r)
	}
	b.WriteString(filesTail)
	if err := os.WriteFile(out, []byte(b.String()), 0o644); err != nil {
		return err
	}
	fmt.Printf("wrote %s  (%d sources, %.1f s each, %.1f MB, %d Hz)\n",
		out, len(paths), dur, float64(b.Len())/(1<<20), rate)
	fmt.Println("Open it and ask which one holds the part. A stem that is 20 dB under the")
	fmt.Println("others is one the separator found nothing for, not a quiet instrument.")
	return nil
}

const filesTail = `<footer>Every source is brought to the same peak, because comparing them at their
natural levels asks which is loudest rather than which holds the part &mdash; the level each one had
BEFORE levelling is printed next to it, and a source far under the others is usually one the
separator found nothing for. Point at the one that holds the part, then measure that file rather
than the mix.</footer>
</body></html>
`
