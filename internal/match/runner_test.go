package match

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMatchResultMetricsIncludeSegmentAndBotLosses(t *testing.T) {
	result := MatchResult{
		Winner:            1,
		Scores:            [2]int{17, 23},
		Losses:            [2]int{5, 2},
		SegmentsLost:      [2]int{5, 2},
		BotsLost:          [2]int{2, 1},
		LossReasons:       [2]LossReason{LossReasonScore, LossReasonNone},
		TimeToFirstAnswer: [2]time.Duration{1200 * time.Millisecond, 800 * time.Millisecond},
		TimeToTurnP99:     [2]time.Duration{49 * time.Millisecond, 62 * time.Millisecond},
		TimeToTurnMax:     [2]time.Duration{51 * time.Millisecond, 70 * time.Millisecond},
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
	_, hasLossesP0 := metricMap["losses_p0"]
	_, hasLossesP1 := metricMap["losses_p1"]
	assert.False(t, hasLossesP0)
	assert.False(t, hasLossesP1)
	assert.Equal(t, 1200.0, metricMap["time_to_first_answer_p0"])
	assert.Equal(t, 800.0, metricMap["time_to_first_answer_p1"])
	assert.Equal(t, 49.0, metricMap["time_to_turn_p99_p0"])
	assert.Equal(t, 62.0, metricMap["time_to_turn_p99_p1"])
	assert.Equal(t, 51.0, metricMap["time_to_turn_max_p0"])
	assert.Equal(t, 70.0, metricMap["time_to_turn_max_p1"])
}

func TestMatchResultRenderMatchIncludesSegmentAndBotLosses(t *testing.T) {
	result := MatchResult{
		ID:                7,
		Seed:              123,
		Turns:             42,
		Winner:            0,
		Scores:            [2]int{30, 27},
		Losses:            [2]int{4, 6},
		SegmentsLost:      [2]int{4, 6},
		BotsLost:          [2]int{1, 2},
		LossReasons:       [2]LossReason{LossReasonNone, LossReasonScore},
		TimeToFirstAnswer: [2]time.Duration{1400 * time.Millisecond, 900 * time.Millisecond},
		TimeToTurnP99:     [2]time.Duration{48 * time.Millisecond, 61 * time.Millisecond},
		TimeToTurnMax:     [2]time.Duration{50 * time.Millisecond, 75 * time.Millisecond},
		BirdsPerPlayer:    4,
		MapWidth:          40,
		MapHeight:         22,
		Apples:            3,
	}

	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.RenderMatch()), &got))

	assert.Equal(t, float64(4), got["segments_lost_p0"])
	assert.Equal(t, float64(6), got["segments_lost_p1"])
	assert.Equal(t, float64(1), got["bots_lost_p0"])
	assert.Equal(t, float64(2), got["bots_lost_p1"])
	_, hasLossesP0 := got["losses_p0"]
	_, hasLossesP1 := got["losses_p1"]
	assert.False(t, hasLossesP0)
	assert.False(t, hasLossesP1)
	assert.Equal(t, 1400.0, got["time_to_first_answer_p0"])
	assert.Equal(t, 900.0, got["time_to_first_answer_p1"])
	assert.Equal(t, 48.0, got["time_to_turn_p99_p0"])
	assert.Equal(t, 61.0, got["time_to_turn_p99_p1"])
	assert.Equal(t, 50.0, got["time_to_turn_max_p0"])
	assert.Equal(t, 75.0, got["time_to_turn_max_p1"])
}
