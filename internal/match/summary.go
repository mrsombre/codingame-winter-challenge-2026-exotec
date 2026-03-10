package match

import (
	"sort"
)

type MetricSummary struct {
	Label string  `json:"label"`
	Total float64 `json:"total"`
	Avg   float64 `json:"avg"`
}

type MatchSummary struct {
	Simulations int             `json:"simulations"`
	Metrics     []MetricSummary `json:"metrics"`
}

func (s *MatchSummary) Get(label string) *MetricSummary {
	for i := range s.Metrics {
		if s.Metrics[i].Label == label {
			return &s.Metrics[i]
		}
	}
	return nil
}

func SummarizeMatches(results []MatchResult) MatchSummary {
	if len(results) == 0 {
		return MatchSummary{}
	}

	firstMetrics := results[0].Metrics()
	totals := make([]float64, len(firstMetrics))
	for _, result := range results {
		for i, metric := range result.Metrics() {
			totals[i] += metric.Value
		}
	}

	metrics := make([]MetricSummary, len(firstMetrics))
	for i, metric := range firstMetrics {
		metrics[i] = MetricSummary{
			Label: metric.Label,
			Total: totals[i],
			Avg:   totals[i] / float64(len(results)),
		}
	}

	return MatchSummary{
		Simulations: len(results),
		Metrics:     metrics,
	}
}

func FindWorstLosses(results []MatchResult, limit int) []int {
	type lossEntry struct {
		idx    int
		margin float64
	}

	var losses []lossEntry
	for idx, result := range results {
		metrics := result.Metrics()
		var wonByP1 bool
		var scoreP0, scoreP1 float64
		for _, metric := range metrics {
			switch metric.Label {
			case "wins_p1":
				wonByP1 = metric.Value == 1
			case "score_p0":
				scoreP0 = metric.Value
			case "score_p1":
				scoreP1 = metric.Value
			}
		}
		if wonByP1 {
			losses = append(losses, lossEntry{idx: idx, margin: scoreP0 - scoreP1})
		}
	}

	sort.Slice(losses, func(i, j int) bool {
		return losses[i].margin < losses[j].margin
	})

	if limit > len(losses) {
		limit = len(losses)
	}
	indices := make([]int, limit)
	for i := 0; i < limit; i++ {
		indices[i] = losses[i].idx
	}
	return indices
}
