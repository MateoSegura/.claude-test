package analysis

import (
	"math"
	"testing"

	claude "github.com/MateoSegura/claudesdk-go"
	"github.com/MateoSegura/.claude-test/internal/metrics"
)

const tolerance = 0.001

func approxEqual(a, b float64) bool {
	return math.Abs(a-b) < tolerance
}

func TestMean(t *testing.T) {
	tests := []struct {
		name   string
		values []float64
		want   float64
	}{
		{"typical", []float64{2, 4, 6, 8, 10}, 6.0},
		{"empty", nil, 0.0},
		{"single", []float64{42}, 42.0},
		{"negative and positive", []float64{-3, -1, 1, 3}, 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Mean(tt.values)
			if !approxEqual(got, tt.want) {
				t.Errorf("Mean(%v) = %v, want %v", tt.values, got, tt.want)
			}
		})
	}
}

func TestStdDev(t *testing.T) {
	tests := []struct {
		name   string
		values []float64
		want   float64
	}{
		{"typical", []float64{2, 4, 4, 4, 5, 5, 7, 9}, 2.0},
		{"empty", nil, 0.0},
		{"single", []float64{5}, 0.0},
		{"all same", []float64{3, 3, 3}, 0.0},
		{"two values", []float64{0, 10}, 5.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StdDev(tt.values)
			if !approxEqual(got, tt.want) {
				t.Errorf("StdDev(%v) = %v, want %v", tt.values, got, tt.want)
			}
		})
	}
}

func TestCI95(t *testing.T) {
	tests := []struct {
		name   string
		mean   float64
		stddev float64
		n      int
		want   [2]float64
	}{
		{"typical", 6.0, 2.0, 25, [2]float64{5.216, 6.784}},
		{"zero stddev", 10.0, 0.0, 5, [2]float64{10.0, 10.0}},
		{"n=1", 10.0, 3.0, 1, [2]float64{10.0, 10.0}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CI95(tt.mean, tt.stddev, tt.n)
			if !approxEqual(got[0], tt.want[0]) || !approxEqual(got[1], tt.want[1]) {
				t.Errorf("CI95(%v, %v, %v) = %v, want %v", tt.mean, tt.stddev, tt.n, got, tt.want)
			}
		})
	}
}

func TestZTestTwoProportions(t *testing.T) {
	t.Run("significant difference", func(t *testing.T) {
		p := ZTestTwoProportions(0.8, 50, 0.6, 50)
		if p >= 0.05 {
			t.Errorf("expected p < 0.05, got %v", p)
		}
	})

	t.Run("equal proportions", func(t *testing.T) {
		p := ZTestTwoProportions(0.5, 10, 0.5, 10)
		if !approxEqual(p, 1.0) {
			t.Errorf("expected p == 1.0, got %v", p)
		}
	})

	t.Run("all successes", func(t *testing.T) {
		p := ZTestTwoProportions(1.0, 5, 1.0, 5)
		if p != 1.0 {
			t.Errorf("expected p == 1.0, got %v", p)
		}
	})

	t.Run("all failures", func(t *testing.T) {
		p := ZTestTwoProportions(0.0, 5, 0.0, 5)
		if p != 1.0 {
			t.Errorf("expected p == 1.0, got %v", p)
		}
	})

	t.Run("extreme difference", func(t *testing.T) {
		p := ZTestTwoProportions(1.0, 10, 0.0, 10)
		if p >= 0.001 {
			t.Errorf("expected p < 0.001, got %v", p)
		}
	})
}

func TestTTestPaired(t *testing.T) {
	t.Run("clear improvement", func(t *testing.T) {
		a := []float64{85, 90, 88, 92, 87}
		b := []float64{80, 82, 84, 85, 81}
		p := TTestPaired(a, b)
		if p >= 0.01 {
			t.Errorf("expected p < 0.01, got %v", p)
		}
	})

	t.Run("identical values", func(t *testing.T) {
		a := []float64{5, 5, 5}
		b := []float64{5, 5, 5}
		p := TTestPaired(a, b)
		if p != 1.0 {
			t.Errorf("expected p == 1.0, got %v", p)
		}
	})

	t.Run("mismatched lengths", func(t *testing.T) {
		a := []float64{1, 2}
		b := []float64{1, 2, 3}
		p := TTestPaired(a, b)
		if p != 1.0 {
			t.Errorf("expected p == 1.0, got %v", p)
		}
	})
}

func makeResult(success bool, score, cost float64, turns int, durMS int64, inTok, outTok int) *metrics.RunResult {
	return &metrics.RunResult{
		Success:    success,
		Score:      score,
		CostUSD:    cost,
		NumTurns:   turns,
		DurationMS: durMS,
		Usage: &claude.Usage{
			InputTokens:  inTok,
			OutputTokens: outTok,
		},
	}
}

func TestComputeStats(t *testing.T) {
	t.Run("mixed results", func(t *testing.T) {
		results := []*metrics.RunResult{
			makeResult(true, 0.9, 0.30, 10, 3000, 300, 200),
			makeResult(true, 0.8, 0.25, 8, 2500, 250, 150),
			makeResult(true, 0.7, 0.20, 7, 2000, 200, 100),
			makeResult(true, 0.5, 0.15, 9, 2500, 200, 150),
			makeResult(false, 0.5, 0.10, 7, 1500, 200, 75),
		}

		s := ComputeStats(results)

		if s.N != 5 {
			t.Errorf("N = %d, want 5", s.N)
		}
		if !approxEqual(s.SuccessRate, 0.8) {
			t.Errorf("SuccessRate = %v, want 0.8", s.SuccessRate)
		}
		if !approxEqual(s.ScoreMean, 0.68) {
			t.Errorf("ScoreMean = %v, want 0.68", s.ScoreMean)
		}
		if !approxEqual(s.CostMean, 0.20) {
			t.Errorf("CostMean = %v, want 0.20", s.CostMean)
		}
		if !approxEqual(s.TurnsMean, 8.2) {
			t.Errorf("TurnsMean = %v, want 8.2", s.TurnsMean)
		}
		if !approxEqual(s.DurationMean, 2300) {
			t.Errorf("DurationMean = %v, want 2300", s.DurationMean)
		}
		if !approxEqual(s.TokensMean.Input, 230) {
			t.Errorf("TokensMean.Input = %v, want 230", s.TokensMean.Input)
		}
		if !approxEqual(s.TokensMean.Output, 135) {
			t.Errorf("TokensMean.Output = %v, want 135", s.TokensMean.Output)
		}
	})

	t.Run("empty", func(t *testing.T) {
		s := ComputeStats(nil)
		if s.N != 0 {
			t.Errorf("N = %d, want 0", s.N)
		}
		if s.SuccessRate != 0 {
			t.Errorf("SuccessRate = %v, want 0", s.SuccessRate)
		}
	})

	t.Run("nil usage entries", func(t *testing.T) {
		results := []*metrics.RunResult{
			makeResult(true, 0.9, 0.20, 5, 1000, 200, 100),
			{
				Success:    true,
				Score:      0.8,
				CostUSD:    0.15,
				NumTurns:   4,
				DurationMS: 800,
				Usage:      nil,
			},
			makeResult(true, 0.7, 0.10, 6, 1200, 300, 200),
		}

		s := ComputeStats(results)

		if s.N != 3 {
			t.Errorf("N = %d, want 3", s.N)
		}
		// Token means should be from the 2 non-nil entries only.
		if !approxEqual(s.TokensMean.Input, 250) {
			t.Errorf("TokensMean.Input = %v, want 250", s.TokensMean.Input)
		}
		if !approxEqual(s.TokensMean.Output, 150) {
			t.Errorf("TokensMean.Output = %v, want 150", s.TokensMean.Output)
		}
	})
}
