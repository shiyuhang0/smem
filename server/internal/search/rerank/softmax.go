package rerank

import "math"

func Softmax(scores []float64, temperature float64) []float64 {
	if temperature <= 0 {
		temperature = 1
	}
	maxScore := scores[0]
	for _, score := range scores[1:] {
		if score > maxScore {
			maxScore = score
		}
	}
	exps := make([]float64, len(scores))
	total := 0.0
	for i, score := range scores {
		exps[i] = math.Exp((score - maxScore) / temperature)
		total += exps[i]
	}
	for i := range exps {
		exps[i] /= total
	}
	return exps
}
