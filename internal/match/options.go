package match

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type BatchOptions struct {
	Simulations   int
	Parallel      int
	Seed          int64
	SeedIncrement *int64
	OutputMatches bool
}

type ParsedArgs struct {
	BatchOptions
	P0Bin       string
	P1Bin       string
	MaxTurns    int
	LeagueLevel int
	Debug       bool
	Timing      bool
	Help        bool
}

func defaultBatchOptions() BatchOptions {
	return BatchOptions{
		Simulations: 1,
		Parallel:    runtime.NumCPU(),
		Seed:        time.Now().UnixNano(),
	}
}

func parseArgs(args []string) (ParsedArgs, error) {
	parsed := ParsedArgs{
		BatchOptions: defaultBatchOptions(),
		P1Bin:        filepath.Clean("./bin/opponent"),
		MaxTurns:     200,
		LeagueLevel:  4,
	}

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--simulations":
			i++
			if i >= len(args) {
				return ParsedArgs{}, fmt.Errorf("missing value for --simulations")
			}
			n, err := strconv.Atoi(args[i])
			if err != nil {
				return ParsedArgs{}, fmt.Errorf("invalid integer for --simulations: %s", args[i])
			}
			parsed.Simulations = n
		case "--parallel":
			i++
			if i >= len(args) {
				return ParsedArgs{}, fmt.Errorf("missing value for --parallel")
			}
			n, err := strconv.Atoi(args[i])
			if err != nil {
				return ParsedArgs{}, fmt.Errorf("invalid integer for --parallel: %s", args[i])
			}
			parsed.Parallel = n
		case "--seed":
			i++
			if i >= len(args) {
				return ParsedArgs{}, fmt.Errorf("missing value for --seed")
			}
			n, err := parseSeed(args[i])
			if err != nil {
				return ParsedArgs{}, fmt.Errorf("invalid integer for --seed: %s", args[i])
			}
			parsed.Seed = n
		case "--seedx":
			i++
			if i >= len(args) {
				return ParsedArgs{}, fmt.Errorf("missing value for --seedx")
			}
			n, err := parseSeed(args[i])
			if err != nil {
				return ParsedArgs{}, fmt.Errorf("invalid integer for --seedx: %s", args[i])
			}
			parsed.SeedIncrement = &n
		case "--output-matches":
			parsed.OutputMatches = true
		case "--debug":
			parsed.Debug = true
		case "--timing":
			parsed.Timing = true
		case "--max-turns":
			i++
			if i >= len(args) {
				return ParsedArgs{}, fmt.Errorf("missing value for --max-turns")
			}
			n, err := strconv.Atoi(args[i])
			if err != nil {
				return ParsedArgs{}, fmt.Errorf("invalid integer for --max-turns: %s", args[i])
			}
			parsed.MaxTurns = n
		case "--league-level":
			i++
			if i >= len(args) {
				return ParsedArgs{}, fmt.Errorf("missing value for --league-level")
			}
			n, err := strconv.Atoi(args[i])
			if err != nil {
				return ParsedArgs{}, fmt.Errorf("invalid integer for --league-level: %s", args[i])
			}
			parsed.LeagueLevel = n
		case "--p0-bin":
			i++
			if i >= len(args) {
				return ParsedArgs{}, fmt.Errorf("missing value for --p0-bin")
			}
			parsed.P0Bin = args[i]
		case "--p1-bin":
			i++
			if i >= len(args) {
				return ParsedArgs{}, fmt.Errorf("missing value for --p1-bin")
			}
			parsed.P1Bin = args[i]
		case "-h", "--help":
			parsed.Help = true
		default:
			return ParsedArgs{}, fmt.Errorf("unknown option: %s", args[i])
		}
	}

	if parsed.Simulations == 0 {
		return ParsedArgs{}, fmt.Errorf("--simulations must be >= 1")
	}
	if parsed.Parallel == 0 {
		return ParsedArgs{}, fmt.Errorf("--parallel must be >= 1")
	}
	if parsed.MaxTurns == 0 {
		return ParsedArgs{}, fmt.Errorf("--max-turns must be >= 1")
	}
	if parsed.LeagueLevel < 1 || parsed.LeagueLevel > 4 {
		return ParsedArgs{}, fmt.Errorf("--league-level must be between 1 and 4")
	}
	if parsed.SeedIncrement != nil && *parsed.SeedIncrement <= 0 {
		return ParsedArgs{}, fmt.Errorf("--seedx must be >= 1")
	}
	if !parsed.Help && parsed.P0Bin == "" {
		return ParsedArgs{}, fmt.Errorf("--p0-bin is required")
	}
	if parsed.Debug {
		parsed.Simulations = 1
		parsed.Parallel = 1
	}

	return parsed, nil
}

func usage() string {
	return strings.TrimSpace(`Usage: match [OPTIONS]

Options:
  --simulations <N>    Number of matches to run (default: 1)
  --parallel <N>       Number of worker threads (default: logical CPUs)
  --seed <N>           Base RNG seed (default: current time)
  --seedx <N>          Seed increment per match (seed_i = seed + i*N)
  --output-matches     Include per-match results in JSON output
  --debug              Force one match, fixed sides, print map/turn trace to stderr
  --max-turns <N>      Maximum turns per match (default: 200)
  --league-level <N>   Game league level 1..4 (default: 4)
  --p0-bin <PATH>      Run player 0 as an external stdin/stdout bot binary
  --p1-bin <PATH>      Run player 1 as an external stdin/stdout bot binary (default: ./bin/opponent)
  -h, --help           Show this help`)
}

func parseSeed(value string) (int64, error) {
	raw := strings.TrimPrefix(value, "seed=")
	return strconv.ParseInt(raw, 10, 64)
}
