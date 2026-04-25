package scoring

import "sort"

func RRF(rankings [][]string) []string {
	k := adaptiveRRFK(rankings)
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

func adaptiveRRFK(rankings [][]string) float64 {
	maxRankingSize := 0
	for _, ranking := range rankings {
		if len(ranking) > maxRankingSize {
			maxRankingSize = len(ranking)
		}
	}
	k := float64(maxRankingSize * 2)
	if k < 10 {
		return 10
	}
	if k > 60 {
		return 60
	}
	return k
}
