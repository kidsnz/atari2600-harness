package cpudiff

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// FindP6502 locates the built p6502step harness. It honours $P6502STEP, then
// searches upward from the working directory for bin/p6502step (so it resolves
// whether invoked from the repo root or from internal/cpudiff during tests).
// Returns "" if not found — callers gate on this (build it with
// scripts/install_perfect6502.sh).
func FindP6502() string {
	if p := os.Getenv("P6502STEP"); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		cand := filepath.Join(dir, "bin", "p6502step")
		if fi, err := os.Stat(cand); err == nil && !fi.IsDir() {
			return cand
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// P6502Available reports whether the perfect6502 harness binary is present.
func P6502Available() bool { return FindP6502() != "" }

// serialize encodes one vector as a p6502step stdin line:
//
//	A X Y S P N  addr0 val0 ... addr(N-1) val(N-1)
func serialize(v Vector) string {
	var b strings.Builder
	// all tokens are hex (p6502step.c parses every field with strtol base 16),
	// including the poke count N.
	fmt.Fprintf(&b, "%02X %02X %02X %02X %02X %X", v.A, v.X, v.Y, v.S, v.P, len(v.Mem))
	for addr, val := range v.Mem {
		fmt.Fprintf(&b, " %04X %02X", addr, val)
	}
	return b.String()
}

// parseResult decodes one p6502step stdout line.
func parseResult(line string) (Result, error) {
	f := strings.Fields(line)
	if len(f) < 9 {
		return Result{}, fmt.Errorf("short p6502step line: %q", line)
	}
	h8 := func(s string) byte { n, _ := strconv.ParseUint(s, 16, 8); return byte(n) }
	h16 := func(s string) uint16 { n, _ := strconv.ParseUint(s, 16, 16); return uint16(n) }
	r := Result{
		Status: f[0],
		A:      h8(f[1]),
		X:      h8(f[2]),
		Y:      h8(f[3]),
		S:      h8(f[4]),
		P:      h8(f[5]),
		PC:     h16(f[6]),
		Writes: map[uint16]byte{},
	}
	cyc, err := strconv.Atoi(f[7])
	if err != nil {
		return Result{}, fmt.Errorf("bad cycles in %q", line)
	}
	r.Cycles = cyc
	nw, err := strconv.Atoi(f[8])
	if err != nil {
		return Result{}, fmt.Errorf("bad nwrites in %q", line)
	}
	idx := 9
	for i := 0; i < nw && idx+1 < len(f); i++ {
		r.Writes[h16(f[idx])] = h8(f[idx+1])
		idx += 2
	}
	return r, nil
}

// RunP6502Batch runs every vector through a single p6502step process and returns
// the results in order. exe is the harness path (see FindP6502).
func RunP6502Batch(exe string, vs []Vector) ([]Result, error) {
	cmd := exec.Command(exe)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	writeErr := make(chan error, 1)
	go func() {
		w := bufio.NewWriter(stdin)
		for _, v := range vs {
			if _, err := fmt.Fprintln(w, serialize(v)); err != nil {
				writeErr <- err
				return
			}
		}
		writeErr <- w.Flush()
		stdin.Close()
	}()

	var out []Result
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 0, 1<<20), 1<<20)
	for sc.Scan() {
		r, err := parseResult(sc.Text())
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if err := <-writeErr; err != nil {
		return nil, err
	}
	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("p6502step exited: %w", err)
	}
	if len(out) != len(vs) {
		return nil, fmt.Errorf("p6502step returned %d results for %d vectors", len(out), len(vs))
	}
	return out, nil
}
