// Package metamorphic checks a metamorphic relation between two scenario runs:
// instead of an oracle for either run's output, it asserts a *relation* between
// them (e.g. "a longer run never scores less than its shorter prefix"). This
// addresses the oracle problem for emergent behaviour. docs/testing-playbook.md.
// Source: Chen/Cheung/Yiu 1998; Segura et al. survey 2016.
package metamorphic

import (
	"fmt"

	"github.com/kidsnz/atari2600-harness/internal/scenario"
)

// Outcome は metamorphic 評価の結果。
type Outcome struct {
	AVal, BVal int64
	Rel        string
	Pass       bool
	Desc       string
}

// runMetric は scenario を走らせ field の終了時値を返す（field を Metrics に確実に含める）。
func runMetric(path, field string) (int64, error) {
	s, err := scenario.Load(path)
	if err != nil {
		return 0, err
	}
	s.Metrics = append(s.Metrics, field)
	res, err := scenario.Run(s, false)
	if err != nil {
		return 0, err
	}
	v, ok := res.Metrics[field]
	if !ok {
		return 0, fmt.Errorf("%s: metric %q not captured", path, field)
	}
	return v, nil
}

func relPass(a int64, rel string, b int64) (bool, error) {
	switch rel {
	case "==":
		return a == b, nil
	case "!=":
		return a != b, nil
	case "<":
		return a < b, nil
	case "<=":
		return a <= b, nil
	case ">":
		return a > b, nil
	case ">=":
		return a >= b, nil
	default:
		return false, fmt.Errorf("unknown relation %q (want == != < <= > >=)", rel)
	}
}

// Eval は A.field <rel> B.field を評価する（A,B は scenario ファイルパス）。
func Eval(aPath, bPath, field, rel string) (Outcome, error) {
	av, err := runMetric(aPath, field)
	if err != nil {
		return Outcome{}, fmt.Errorf("run A: %w", err)
	}
	bv, err := runMetric(bPath, field)
	if err != nil {
		return Outcome{}, fmt.Errorf("run B: %w", err)
	}
	ok, err := relPass(av, rel, bv)
	if err != nil {
		return Outcome{}, err
	}
	return Outcome{
		AVal: av, BVal: bv, Rel: rel, Pass: ok,
		Desc: fmt.Sprintf("A.%s %s B.%s  (%d %s %d)", field, rel, field, av, rel, bv),
	}, nil
}
