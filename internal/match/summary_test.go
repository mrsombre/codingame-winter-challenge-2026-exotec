package match

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSummarizeMatchesOmitsTimingTotalsAndRoundsAverage(t *testing.T) {
	results := []MatchResult{
		{
			Scores:            [2]int{17, 0},
			TimeToFirstAnswer: [2]time.Duration{111114 * time.Microsecond, 0},
		},
		{
			Scores:            [2]int{19, 0},
			TimeToFirstAnswer: [2]time.Duration{111046 * time.Microsecond, 0},
		},
	}

	summary := SummarizeMatches(results)

	score := summary.Get("score_p0")
	require.NotNil(t, score)
	require.NotNil(t, score.Total)
	assert.Equal(t, 36.0, *score.Total)
	assert.Equal(t, 18.0, score.Avg)

	timing := summary.Get("time_to_first_answer_p0")
	require.NotNil(t, timing)
	assert.Nil(t, timing.Total)
	assert.Equal(t, 111.08, timing.Avg)
}
