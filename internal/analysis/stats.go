package analysis

import (
	"math"

	"github.com/MateoSegura/.claude-test/internal/metrics"
)

// Mean returns the arithmetic mean of values. Returns 0.0 for empty slices.
func Mean(values []float64) float64 {
	if len(values) == 0 {
		return 0.0
	}
	var sum float64
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

// StdDev returns the population standard deviation (divides by N).
// Returns 0.0 for slices with fewer than 2 elements.
func StdDev(values []float64) float64 {
	n := len(values)
	if n < 2 {
		return 0.0
	}
	mean := Mean(values)
	var sumSq float64
	for _, v := range values {
		d := v - mean
		sumSq += d * d
	}
	return math.Sqrt(sumSq / float64(n))
}

// CI95 returns the 95% confidence interval [lower, upper] using z=1.96.
// Returns [mean, mean] when n < 2 or stddev == 0.
func CI95(mean, stddev float64, n int) [2]float64 {
	if n < 2 || stddev == 0 {
		return [2]float64{mean, mean}
	}
	margin := 1.96 * stddev / math.Sqrt(float64(n))
	return [2]float64{mean - margin, mean + margin}
}

// ZTestTwoProportions performs a two-proportion z-test and returns the
// two-tailed p-value. Uses pooled proportion. Returns 1.0 when the pooled
// proportion is 0 or 1.
func ZTestTwoProportions(p1, n1, p2, n2 float64) float64 {
	pooled := (p1*n1 + p2*n2) / (n1 + n2)
	if pooled == 0 || pooled == 1 {
		return 1.0
	}
	se := math.Sqrt(pooled * (1 - pooled) * (1/n1 + 1/n2))
	z := (p1 - p2) / se
	p := math.Erfc(math.Abs(z) / math.Sqrt2)
	return p
}

// betaI computes the regularized incomplete beta function I_x(a, b) using
// the continued fraction expansion (Lentz's algorithm).
func betaI(a, b, x float64) float64 {
	if x == 0 || x == 1 {
		return x
	}

	// Use the symmetry relation when x > (a+1)/(a+b+2) for convergence.
	if x > (a+1)/(a+b+2) {
		return 1.0 - betaI(b, a, 1.0-x)
	}

	lnBeta := lgammaDiff(a, b)
	front := math.Exp(math.Log(x)*a+math.Log(1-x)*b-lnBeta) / a

	// Lentz's algorithm for the continued fraction.
	const maxIter = 200
	const epsilon = 1e-14
	const tiny = 1e-30

	f := 1.0
	c := 1.0
	d := 1.0 - (a+b)*x/(a+1)
	if math.Abs(d) < tiny {
		d = tiny
	}
	d = 1.0 / d
	f = d

	for m := 1; m <= maxIter; m++ {
		mf := float64(m)

		// Even step: d_{2m}
		num := mf * (b - mf) * x / ((a + 2*mf - 1) * (a + 2*mf))
		d = 1.0 + num*d
		if math.Abs(d) < tiny {
			d = tiny
		}
		c = 1.0 + num/c
		if math.Abs(c) < tiny {
			c = tiny
		}
		d = 1.0 / d
		f *= d * c

		// Odd step: d_{2m+1}
		num = -(a + mf) * (a + b + mf) * x / ((a + 2*mf) * (a + 2*mf + 1))
		d = 1.0 + num*d
		if math.Abs(d) < tiny {
			d = tiny
		}
		c = 1.0 + num/c
		if math.Abs(c) < tiny {
			c = tiny
		}
		d = 1.0 / d
		delta := d * c
		f *= delta

		if math.Abs(delta-1.0) < epsilon {
			break
		}
	}

	return front * f
}

// lgammaDiff computes log(Beta(a,b)) = lgamma(a) + lgamma(b) - lgamma(a+b).
func lgammaDiff(a, b float64) float64 {
	la, _ := math.Lgamma(a)
	lb, _ := math.Lgamma(b)
	lab, _ := math.Lgamma(a + b)
	return la + lb - lab
}

// TTestPaired performs a paired t-test and returns the two-tailed p-value.
// Requires len(a) == len(b) and len >= 2; returns 1.0 otherwise.
// Returns 1.0 when all differences are zero.
func TTestPaired(a, b []float64) float64 {
	if len(a) != len(b) || len(a) < 2 {
		return 1.0
	}

	diffs := make([]float64, len(a))
	for i := range a {
		diffs[i] = a[i] - b[i]
	}

	meanDiff := Mean(diffs)
	n := float64(len(diffs))

	// Sample standard deviation (N-1 denominator).
	var sumSq float64
	for _, d := range diffs {
		sumSq += (d - meanDiff) * (d - meanDiff)
	}
	sampleSD := math.Sqrt(sumSq / (n - 1))

	if sampleSD == 0 {
		return 1.0
	}

	t := meanDiff / (sampleSD / math.Sqrt(n))

	// Use regularized incomplete beta for t-distribution CDF.
	df := n - 1
	x := df / (df + t*t)
	p := betaI(df/2, 0.5, x)
	return p
}

// AttemptStats holds aggregated statistics for a set of benchmark run results.
type AttemptStats struct {
	N            int        `json:"n"`
	SuccessRate  float64    `json:"success_rate"`
	ScoreMean    float64    `json:"score_mean"`
	ScoreStdDev  float64    `json:"score_std_dev"`
	ScoreCI95    [2]float64 `json:"score_ci95"`
	CostMean     float64    `json:"cost_mean"`
	CostStdDev   float64    `json:"cost_std_dev"`
	TurnsMean    float64    `json:"turns_mean"`
	DurationMean float64    `json:"duration_mean"`
	TokensMean   struct {
		Input  float64 `json:"input"`
		Output float64 `json:"output"`
	} `json:"tokens_mean"`
}

// ComputeStats calculates aggregate statistics from a slice of RunResults.
func ComputeStats(results []*metrics.RunResult) AttemptStats {
	if len(results) == 0 {
		return AttemptStats{}
	}

	n := len(results)
	var stats AttemptStats
	stats.N = n

	var successCount int
	scores := make([]float64, n)
	costs := make([]float64, n)
	turns := make([]float64, n)
	durations := make([]float64, n)

	for i, r := range results {
		if r.Success {
			successCount++
		}
		scores[i] = r.Score
		costs[i] = r.CostUSD
		turns[i] = float64(r.NumTurns)
		durations[i] = float64(r.DurationMS)
	}

	stats.SuccessRate = float64(successCount) / float64(n)
	stats.ScoreMean = Mean(scores)
	stats.ScoreStdDev = StdDev(scores)
	stats.ScoreCI95 = CI95(stats.ScoreMean, stats.ScoreStdDev, n)
	stats.CostMean = Mean(costs)
	stats.CostStdDev = StdDev(costs)
	stats.TurnsMean = Mean(turns)
	stats.DurationMean = Mean(durations)

	// Token means from non-nil Usage entries only.
	var inputTokens, outputTokens []float64
	for _, r := range results {
		if r.Usage != nil {
			inputTokens = append(inputTokens, float64(r.Usage.InputTokens))
			outputTokens = append(outputTokens, float64(r.Usage.OutputTokens))
		}
	}
	stats.TokensMean.Input = Mean(inputTokens)
	stats.TokensMean.Output = Mean(outputTokens)

	return stats
}
