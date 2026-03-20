package match

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseArgsParsesSignedSeedAndLeagueLevel(t *testing.T) {
	got, err := parseArgs([]string{
		"--p0-bin", "./bin/p0",
		"--seed", "-1755827269105404700",
		"--seedx", "7",
		"--league-level", "1",
		"--max-turns", "123",
		"--simulations", "4",
		"--parallel", "2",
		"--output-matches",
	})
	require.NoError(t, err)
	require.NotNil(t, got.SeedIncrement)
	assert.Equal(t, int64(-1755827269105404700), got.Seed)
	assert.Equal(t, int64(7), *got.SeedIncrement)
	assert.Equal(t, 1, got.LeagueLevel)
	assert.Equal(t, 123, got.MaxTurns)
	assert.Equal(t, 4, got.Simulations)
	assert.Equal(t, 2, got.Parallel)
	assert.True(t, got.OutputMatches)
	assert.Equal(t, filepath.Clean("./bin/opponent"), got.P1Bin)
}

func TestParseArgsAcceptsSeedPrefix(t *testing.T) {
	got, err := parseArgs([]string{
		"--p0-bin", "./bin/p0",
		"--seed", "seed=1001",
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1001), got.Seed)
}

func TestParseArgsParsesTraceAndNoSwap(t *testing.T) {
	got, err := parseArgs([]string{
		"--p0-bin", "./bin/p0",
		"--trace-out", "./tmp/trace.jsonl.gz",
		"--no-swap",
	})
	require.NoError(t, err)
	assert.Equal(t, "./tmp/trace.jsonl.gz", got.TraceOut)
	assert.True(t, got.NoSwap)
}

func TestParseArgsRejectsInvalidLeagueLevel(t *testing.T) {
	_, err := parseArgs([]string{
		"--p0-bin", "./bin/p0",
		"--league-level", "5",
	})
	require.Error(t, err)
	assert.EqualError(t, err, "--league-level must be between 1 and 4")
}

func TestParseArgsRejectsNonPositiveSeedIncrement(t *testing.T) {
	testCases := []string{"0", "-1"}
	for _, value := range testCases {
		t.Run(value, func(t *testing.T) {
			_, err := parseArgs([]string{
				"--p0-bin", "./bin/p0",
				"--seedx", value,
			})
			require.Error(t, err)
			assert.EqualError(t, err, "--seedx must be >= 1")
		})
	}
}

func TestParseArgsRequiresTracePath(t *testing.T) {
	_, err := parseArgs([]string{
		"--p0-bin", "./bin/p0",
		"--trace-out",
	})
	require.Error(t, err)
	assert.EqualError(t, err, "missing value for --trace-out")
}
