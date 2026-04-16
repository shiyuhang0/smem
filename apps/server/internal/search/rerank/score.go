package rerank

import (
	"time"

	"smem/apps/server/internal/domain/memory"
)

func Score(item memory.Memory, targetType memory.Type, targetKinds []string) float64 {
	score := 0.0
	if targetType != "" && item.Type == targetType {
		score += 2
	}
	for _, kind := range targetKinds {
		for _, itemKind := range item.Kinds {
			if kind == itemKind {
				score += 1.5
				break
			}
		}
	}
	score += float64(item.StoreCount) * 0.2
	ageHours := time.Since(item.UpdatedAt).Hours()
	if ageHours < 0 {
		ageHours = 0
	}
	score += 1.0 / (1.0 + ageHours/24.0)
	return score
}
