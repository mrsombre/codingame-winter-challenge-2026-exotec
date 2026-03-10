package match

import (
	"path/filepath"
	"testing"
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
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}

	if got.Seed != -1755827269105404700 {
		t.Fatalf("Seed = %d, want %d", got.Seed, int64(-1755827269105404700))
	}
	if got.SeedIncrement == nil || *got.SeedIncrement != 7 {
		t.Fatalf("SeedIncrement = %v, want 7", got.SeedIncrement)
	}
	if got.LeagueLevel != 1 {
		t.Fatalf("LeagueLevel = %d, want 1", got.LeagueLevel)
	}
	if got.MaxTurns != 123 {
		t.Fatalf("MaxTurns = %d, want 123", got.MaxTurns)
	}
	if got.Simulations != 4 {
		t.Fatalf("Simulations = %d, want 4", got.Simulations)
	}
	if got.Parallel != 2 {
		t.Fatalf("Parallel = %d, want 2", got.Parallel)
	}
	if !got.OutputMatches {
		t.Fatalf("OutputMatches = false, want true")
	}
	if got.P1Bin != filepath.Clean("./bin/opponent") {
		t.Fatalf("P1Bin = %q, want %q", got.P1Bin, filepath.Clean("./bin/opponent"))
	}
}

func TestParseArgsAcceptsSeedPrefix(t *testing.T) {
	got, err := parseArgs([]string{
		"--p0-bin", "./bin/p0",
		"--seed", "seed=1001",
	})
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}
	if got.Seed != 1001 {
		t.Fatalf("Seed = %d, want 1001", got.Seed)
	}
}

func TestParseArgsRejectsInvalidLeagueLevel(t *testing.T) {
	_, err := parseArgs([]string{
		"--p0-bin", "./bin/p0",
		"--league-level", "5",
	})
	if err == nil {
		t.Fatal("parseArgs returned nil error, want invalid league-level error")
	}
	if err.Error() != "--league-level must be between 1 and 4" {
		t.Fatalf("error = %q, want %q", err.Error(), "--league-level must be between 1 and 4")
	}
}

func TestParseArgsRejectsNonPositiveSeedIncrement(t *testing.T) {
	testCases := []string{"0", "-1"}
	for _, value := range testCases {
		t.Run(value, func(t *testing.T) {
			_, err := parseArgs([]string{
				"--p0-bin", "./bin/p0",
				"--seedx", value,
			})
			if err == nil {
				t.Fatal("parseArgs returned nil error, want invalid seedx error")
			}
			if err.Error() != "--seedx must be >= 1" {
				t.Fatalf("error = %q, want %q", err.Error(), "--seedx must be >= 1")
			}
		})
	}
}
