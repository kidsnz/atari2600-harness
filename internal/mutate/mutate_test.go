package mutate

import "testing"

// TestEvalRandomKillsSome checks that injecting random faults into smoke.bin gets at least some
// of them caught by the smoke scenario (ram.0x80==66 plus 262 lines) (kill rate > 0), and that
// the same seed is deterministic (reproducible).
func TestEvalRandomKillsSome(t *testing.T) {
	t.Chdir("../..")
	scen := []string{"roms/litmus/scenarios/smoke.json"}
	rs1, err := EvalRandom("roms/litmus/smoke.bin", 30, 1, scen)
	if err != nil {
		t.Fatal(err)
	}
	if KillRate(rs1) <= 0 {
		t.Fatalf("expected the suite to catch at least one mutation, kill rate = 0")
	}
	// Determinism: same seed, same result.
	rs2, err := EvalRandom("roms/litmus/smoke.bin", 30, 1, scen)
	if err != nil {
		t.Fatal(err)
	}
	if KillRate(rs1) != KillRate(rs2) {
		t.Fatalf("mutation not deterministic: %.3f vs %.3f", KillRate(rs1), KillRate(rs2))
	}
}

// TestCoveredKillRateIsHonest is the VV-11 part-2 core: restricting mutations to
// executed code yields a higher, honest kill rate than the naive all-bytes
// version. smoke.bin is a 4K ROM that is mostly unexecuted padding — naive
// mutation wastes most of its budget on dead bytes that can never be killed,
// while covered mutation targets the live kernel. The covered rate must exceed
// the naive rate (and be deterministic).
func TestCoveredKillRateIsHonest(t *testing.T) {
	t.Chdir("../..")
	scen := []string{"roms/litmus/scenarios/smoke.json"}

	naive, err := EvalRandom("roms/litmus/smoke.bin", 60, 7, scen)
	if err != nil {
		t.Fatal(err)
	}
	cov1, err := EvalRandomCovered("roms/litmus/smoke.bin", 60, 7, scen, 2)
	if err != nil {
		t.Fatal(err)
	}
	naiveRate, coveredRate := KillRate(naive), KillRate(cov1)
	t.Logf("naive kill-rate=%.3f  covered kill-rate=%.3f", naiveRate, coveredRate)

	if coveredRate <= naiveRate {
		t.Fatalf("covered kill-rate %.3f should exceed naive %.3f (dead-code dilution not removed)",
			coveredRate, naiveRate)
	}
	if coveredRate <= 0 {
		t.Fatal("covered kill-rate is 0 — every live-code mutation survived (vacuous)")
	}
	// determinism
	cov2, err := EvalRandomCovered("roms/litmus/smoke.bin", 60, 7, scen, 2)
	if err != nil {
		t.Fatal(err)
	}
	if KillRate(cov1) != KillRate(cov2) {
		t.Fatalf("covered mutation not deterministic: %.3f vs %.3f", KillRate(cov1), KillRate(cov2))
	}
}
