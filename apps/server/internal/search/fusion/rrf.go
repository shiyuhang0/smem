package fusion

import "sort"

func RRF(rankings [][]string, k float64) []string {
	scores := map[string]float64{}
	for _, ranking := range rankings {
		for i, id := range ranking {
			scores[id] += 1.0 / (k + float64(i+1))
		}
	}
	ids := make([]string, 0, len(scores))
	for id := range scores {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		if scores[ids[i]] == scores[ids[j]] {
			return ids[i] < ids[j]
		}
		return scores[ids[i]] > scores[ids[j]]
	})
	return ids
}
