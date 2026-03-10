package match

import (
	"encoding/json"
	"fmt"
	"io"
)

type runnerOutput struct {
	P0Bin   string            `json:"p0_bin"`
	P1Bin   string            `json:"p1_bin"`
	Runner  runnerMetadata    `json:"runner"`
	Summary MatchSummary      `json:"summary"`
	Matches []json.RawMessage `json:"matches,omitempty"`
}

type runnerMetadata struct {
	Simulations   int     `json:"simulations"`
	Parallel      int     `json:"parallel"`
	Seed          uint64  `json:"seed"`
	SeedIncrement *uint64 `json:"seed_increment,omitempty"`
	OutputMatches bool    `json:"output_matches"`
	MaxTurns      int     `json:"max_turns"`
}

func Run(args []string, stdout io.Writer) error {
	parsed, err := parseArgs(args)
	if err != nil {
		return err
	}

	if parsed.Help {
		_, err = fmt.Fprintln(stdout, usage())
		return err
	}

	runner := NewRunner(MatchOptions{
		MaxTurns: parsed.MaxTurns,
		P0Bin:    parsed.P0Bin,
		P1Bin:    parsed.P1Bin,
	})
	results := runMatches(parsed.BatchOptions, runner.RunMatch)

	out := runnerOutput{
		P0Bin: parsed.P0Bin,
		P1Bin: parsed.P1Bin,
		Runner: runnerMetadata{
			Simulations:   parsed.Simulations,
			Parallel:      parsed.Parallel,
			Seed:          parsed.Seed,
			SeedIncrement: parsed.SeedIncrement,
			OutputMatches: parsed.OutputMatches,
			MaxTurns:      parsed.MaxTurns,
		},
		Summary: SummarizeMatches(results),
	}
	if parsed.OutputMatches {
		out.Matches = make([]json.RawMessage, 0, len(results))
		for _, result := range results {
			out.Matches = append(out.Matches, json.RawMessage(result.RenderMatch()))
		}
	}

	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
