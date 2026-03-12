package match

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMatchResultMetricsIncludeSegmentAndBotLosses(t *testing.T) {
	result := MatchResult{
		Winner:       1,
		Scores:       [2]int{17, 23},
		Losses:       [2]int{5, 2},
		SegmentsLost: [2]int{5, 2},
		BotsLost:     [2]int{2, 1},
		LossReasons:  [2]LossReason{LossReasonScore, LossReasonNone},
	}

	metrics := result.Metrics()
	metricMap := make(map[string]float64, len(metrics))
	for _, metric := range metrics {
		metricMap[metric.Label] = metric.Value
	}

	assert.Equal(t, 5.0, metricMap["segments_lost_p0"])
	assert.Equal(t, 2.0, metricMap["segments_lost_p1"])
	assert.Equal(t, 2.0, metricMap["bots_lost_p0"])
	assert.Equal(t, 1.0, metricMap["bots_lost_p1"])
	assert.Equal(t, 5.0, metricMap["losses_p0"])
	assert.Equal(t, 2.0, metricMap["losses_p1"])
}

func TestMatchResultRenderMatchIncludesSegmentAndBotLosses(t *testing.T) {
	result := MatchResult{
		ID:             7,
		Seed:           123,
		Turns:          42,
		Winner:         0,
		Scores:         [2]int{30, 27},
		Losses:         [2]int{4, 6},
		SegmentsLost:   [2]int{4, 6},
		BotsLost:       [2]int{1, 2},
		LossReasons:    [2]LossReason{LossReasonNone, LossReasonScore},
		BirdsPerPlayer: 4,
		MapWidth:       40,
		MapHeight:      22,
		Apples:         3,
	}

	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.RenderMatch()), &got))

	assert.Equal(t, float64(4), got["segments_lost_p0"])
	assert.Equal(t, float64(6), got["segments_lost_p1"])
	assert.Equal(t, float64(1), got["bots_lost_p0"])
	assert.Equal(t, float64(2), got["bots_lost_p1"])
	assert.Equal(t, float64(4), got["losses_p0"])
	assert.Equal(t, float64(6), got["losses_p1"])
}
