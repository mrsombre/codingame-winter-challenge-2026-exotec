package match

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	engine "codingame/internal/engine"
)

type LossReason string

const (
	LossReasonNone       LossReason = ""
	LossReasonScore      LossReason = "score"
	LossReasonTimeout    LossReason = "timeout"
	LossReasonBadCommand LossReason = "bad_command"
)

type MatchOptions struct {
	MaxTurns    int
	LeagueLevel int
	P0Bin       string
	P1Bin       string
	Debug       bool
	Timing      bool
}

type Runner struct {
	Options MatchOptions
}

func NewRunner(options MatchOptions) *Runner {
	if options.MaxTurns == 0 {
		options.MaxTurns = engine.MaxTurns
	}
	if options.LeagueLevel == 0 {
		options.LeagueLevel = 4
	}
	return &Runner{Options: options}
}

type MatchResult struct {
	ID                int
	Seed              int64
	Turns             int
	Scores            [2]int
	Losses            [2]int
	SegmentsLost      [2]int
	BotsLost          [2]int
	Winner            int
	LossReasons       [2]LossReason
	TimeToFirstAnswer [2]time.Duration
	TimeToTurnP99     [2]time.Duration
	TimeToTurnMax     [2]time.Duration
	BirdsPerPlayer    int
	MapWidth          int
	MapHeight         int
	Apples            int
	Swapped           bool
}

func (r MatchResult) SimulationID() int { return r.ID }

type Metric struct {
	Label string
	Value float64
}

func (r MatchResult) Metrics() []Metric {
	winsP0, winsP1, draws := 0.0, 0.0, 0.0
	switch r.Winner {
	case 0:
		winsP0 = 1.0
	case 1:
		winsP1 = 1.0
	default:
		draws = 1.0
	}
	return []Metric{
		{Label: "turns", Value: float64(r.Turns)},
		{Label: "wins_p0", Value: winsP0},
		{Label: "wins_p1", Value: winsP1},
		{Label: "draws", Value: draws},
		{Label: "loss_score_p0", Value: lossMetric(r.LossReasons[0], LossReasonScore)},
		{Label: "loss_score_p1", Value: lossMetric(r.LossReasons[1], LossReasonScore)},
		{Label: "loss_timeout_p0", Value: lossMetric(r.LossReasons[0], LossReasonTimeout)},
		{Label: "loss_timeout_p1", Value: lossMetric(r.LossReasons[1], LossReasonTimeout)},
		{Label: "loss_bad_command_p0", Value: lossMetric(r.LossReasons[0], LossReasonBadCommand)},
		{Label: "loss_bad_command_p1", Value: lossMetric(r.LossReasons[1], LossReasonBadCommand)},
		{Label: "score_p0", Value: float64(r.Scores[0])},
		{Label: "score_p1", Value: float64(r.Scores[1])},
		{Label: "segments_lost_p0", Value: float64(r.SegmentsLost[0])},
		{Label: "segments_lost_p1", Value: float64(r.SegmentsLost[1])},
		{Label: "bots_lost_p0", Value: float64(r.BotsLost[0])},
		{Label: "bots_lost_p1", Value: float64(r.BotsLost[1])},
		{Label: "time_to_first_answer_p0", Value: durationMillis(r.TimeToFirstAnswer[0])},
		{Label: "time_to_first_answer_p1", Value: durationMillis(r.TimeToFirstAnswer[1])},
		{Label: "time_to_turn_p99_p0", Value: durationMillis(r.TimeToTurnP99[0])},
		{Label: "time_to_turn_p99_p1", Value: durationMillis(r.TimeToTurnP99[1])},
		{Label: "time_to_turn_max_p0", Value: durationMillis(r.TimeToTurnMax[0])},
		{Label: "time_to_turn_max_p1", Value: durationMillis(r.TimeToTurnMax[1])},
	}
}

func (r MatchResult) RenderMatch() string {
	payload := struct {
		ID                  int        `json:"id"`
		Seed                int64      `json:"seed"`
		Turns               int        `json:"turns"`
		Winner              int        `json:"winner"`
		LossReasonP0        LossReason `json:"loss_reason_p0"`
		LossReasonP1        LossReason `json:"loss_reason_p1"`
		ScoreP0             int        `json:"score_p0"`
		ScoreP1             int        `json:"score_p1"`
		SegmentsLostP0      int        `json:"segments_lost_p0"`
		SegmentsLostP1      int        `json:"segments_lost_p1"`
		BotsLostP0          int        `json:"bots_lost_p0"`
		BotsLostP1          int        `json:"bots_lost_p1"`
		TimeToFirstAnswerP0 float64    `json:"time_to_first_answer_p0"`
		TimeToFirstAnswerP1 float64    `json:"time_to_first_answer_p1"`
		TimeToTurnP99P0     float64    `json:"time_to_turn_p99_p0"`
		TimeToTurnP99P1     float64    `json:"time_to_turn_p99_p1"`
		TimeToTurnMaxP0     float64    `json:"time_to_turn_max_p0"`
		TimeToTurnMaxP1     float64    `json:"time_to_turn_max_p1"`
		BirdsPerPlayer      int        `json:"birds_per_player"`
		MapWidth            int        `json:"map_width"`
		MapHeight           int        `json:"map_height"`
		Apples              int        `json:"apples"`
	}{
		ID:                  r.ID,
		Seed:                r.Seed,
		Turns:               r.Turns,
		Winner:              r.Winner,
		LossReasonP0:        r.LossReasons[0],
		LossReasonP1:        r.LossReasons[1],
		ScoreP0:             r.Scores[0],
		ScoreP1:             r.Scores[1],
		SegmentsLostP0:      r.SegmentsLost[0],
		SegmentsLostP1:      r.SegmentsLost[1],
		BotsLostP0:          r.BotsLost[0],
		BotsLostP1:          r.BotsLost[1],
		TimeToFirstAnswerP0: durationMillis(r.TimeToFirstAnswer[0]),
		TimeToFirstAnswerP1: durationMillis(r.TimeToFirstAnswer[1]),
		TimeToTurnP99P0:     durationMillis(r.TimeToTurnP99[0]),
		TimeToTurnP99P1:     durationMillis(r.TimeToTurnP99[1]),
		TimeToTurnMaxP0:     durationMillis(r.TimeToTurnMax[0]),
		TimeToTurnMaxP1:     durationMillis(r.TimeToTurnMax[1]),
		BirdsPerPlayer:      r.BirdsPerPlayer,
		MapWidth:            r.MapWidth,
		MapHeight:           r.MapHeight,
		Apples:              r.Apples,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	return string(data)
}

func (runner *Runner) RunMatch(simulationID int, seed int64) MatchResult {
	// Swap sides unless in debug mode. Uses seed bit for deterministic ~50/50 split.
	swapSides := !runner.Options.Debug && seed%2 != 0

	players := []*engine.Player{
		engine.NewPlayer(0),
		engine.NewPlayer(1),
	}
	players[0].SetNicknameToken("Player 0")
	players[1].SetNicknameToken("Player 1")

	game := engine.NewGame(seed, runner.Options.LeagueLevel)
	referee := engine.NewReferee(game, engine.NewCommandManager())
	referee.Init(players)
	if runner.Options.Debug {
		printDebugMap(seed, game)
	}

	matchOptions := runner.Options
	if swapSides {
		matchOptions.P0Bin, matchOptions.P1Bin = matchOptions.P1Bin, matchOptions.P0Bin
	}

	controllers, cleanup, err := attachCommandPlayers(matchOptions, players)
	if err != nil {
		panic(err)
	}
	defer cleanup()

	for _, player := range players {
		lines := referee.GlobalInfoFor(player)
		for _, line := range lines {
			player.SendInputLine(line)
		}
		if runner.Options.Debug {
			fmt.Fprintf(os.Stderr, "--- p%d global input ---\n", player.GetIndex())
			for _, line := range lines {
				fmt.Fprintln(os.Stderr, line)
			}
		}
	}

	maxTurns := runner.Options.MaxTurns
	turn := 0
	for turn = 0; !referee.Ended() && turn < maxTurns; turn++ {
		referee.ResetGameTurnData()
		if runner.Options.Debug {
			printDebugTurnState("start", turn, game)
		}

		for _, player := range players {
			if player.IsDeactivated() || referee.ShouldSkipPlayerTurn(player) {
				continue
			}
			lines := referee.FrameInfoFor(player)
			for _, line := range lines {
				player.SendInputLine(line)
			}
			if runner.Options.Debug {
				fmt.Fprintf(os.Stderr, "--- turn %d p%d input ---\n", turn, player.GetIndex())
				for _, line := range lines {
					fmt.Fprintln(os.Stderr, line)
				}
			}
			_ = player.Execute()
			if runner.Options.Debug {
				fmt.Fprintf(os.Stderr, "turn %d p%d output: %s\n", turn, player.GetIndex(), strings.Join(player.GetOutputs(), " | "))
			}
		}

		handlePlayerCommands(players, referee)
		if referee.ActivePlayers(players) < 2 {
			referee.EndGame()
			break
		}

		referee.PerformGameUpdate(turn)
		if runner.Options.Debug {
			printDebugTurnState("after", turn, game)
		}
	}

	if !referee.Ended() {
		referee.EndGame()
	}
	referee.OnEnd()

	result := buildMatchResult(simulationID, seed, turn, referee.Game, players, controllers)
	if swapSides {
		result = swapMatchSides(result)
		result.Swapped = true
	}
	return result
}

func handlePlayerCommands(players []*engine.Player, referee *engine.Referee) {
	for _, player := range players {
		if player.IsDeactivated() {
			continue
		}
		err := player.GetOutputError()
		if err == nil {
			continue
		}

		player.Deactivate(err.Error())
	}

	referee.ParsePlayerOutputs(players)
}

func attachCommandPlayers(options MatchOptions, players []*engine.Player) ([]*commandPlayer, func(), error) {
	controllers := make([]*commandPlayer, 0, len(players))
	bins := []string{options.P0Bin, options.P1Bin}

	for i, path := range bins {
		cp, err := newCommandPlayer(players[i], path)
		if err != nil {
			for _, controller := range controllers {
				_ = controller.Close()
			}
			return nil, nil, fmt.Errorf("failed to start player %d session: %w", i, err)
		}
		cp.playerIdx = i
		cp.timing = options.Timing
		players[i].SetExecuteFunc(cp.Execute)
		controllers = append(controllers, cp)
	}

	return controllers, func() {
		for _, controller := range controllers {
			_ = controller.Close()
		}
	}, nil
}

func buildMatchResult(simulationID int, seed int64, turns int, game *engine.Game, players []*engine.Player, controllers []*commandPlayer) MatchResult {
	winner := -1
	if players[0].GetScore() > players[1].GetScore() {
		winner = 0
	} else if players[1].GetScore() > players[0].GetScore() {
		winner = 1
	}

	birdsPerPlayer := 0
	if len(players) > 0 {
		birdsPerPlayer = len(players[0].GetBirds())
	}

	botsLost := [2]int{}
	for i, player := range players {
		botsLost[i] = len(player.GetBirds()) - liveBirdCount(player)
	}

	firstAnswer := [2]time.Duration{}
	turnP99 := [2]time.Duration{}
	turnMax := [2]time.Duration{}
	for i, controller := range controllers {
		stats := controller.TimingStats()
		firstAnswer[i] = stats.FirstAnswer
		turnP99[i] = stats.TurnP99
		turnMax[i] = stats.TurnMax
	}

	return MatchResult{
		ID:                simulationID,
		Seed:              seed,
		Turns:             turns,
		Scores:            [2]int{players[0].GetScore(), players[1].GetScore()},
		Losses:            game.Losses,
		SegmentsLost:      game.Losses,
		BotsLost:          botsLost,
		Winner:            winner,
		LossReasons:       [2]LossReason{lossReasonFor(players[0], winner, 0), lossReasonFor(players[1], winner, 1)},
		TimeToFirstAnswer: firstAnswer,
		TimeToTurnP99:     turnP99,
		TimeToTurnMax:     turnMax,
		BirdsPerPlayer:    birdsPerPlayer,
		MapWidth:          game.Grid.Width,
		MapHeight:         game.Grid.Height,
		Apples:            len(game.Grid.Apples),
	}
}

func durationMillis(value time.Duration) float64 {
	return round2(float64(value) / float64(time.Millisecond))
}

func round2(value float64) float64 {
	return math.Round(value*100) / 100
}

func lossMetric(actual, expected LossReason) float64 {
	if actual == expected {
		return 1.0
	}
	return 0.0
}

func liveBirdCount(player *engine.Player) int {
	count := 0
	for _, bird := range player.GetBirds() {
		if bird.Alive {
			count++
		}
	}
	return count
}

// swapMatchSides flips all p0/p1 fields so that results always refer to
// the original --p0-bin / --p1-bin regardless of engine-side assignment.
func swapMatchSides(r MatchResult) MatchResult {
	r.Scores[0], r.Scores[1] = r.Scores[1], r.Scores[0]
	r.Losses[0], r.Losses[1] = r.Losses[1], r.Losses[0]
	r.SegmentsLost[0], r.SegmentsLost[1] = r.SegmentsLost[1], r.SegmentsLost[0]
	r.BotsLost[0], r.BotsLost[1] = r.BotsLost[1], r.BotsLost[0]
	r.LossReasons[0], r.LossReasons[1] = r.LossReasons[1], r.LossReasons[0]
	r.TimeToFirstAnswer[0], r.TimeToFirstAnswer[1] = r.TimeToFirstAnswer[1], r.TimeToFirstAnswer[0]
	r.TimeToTurnP99[0], r.TimeToTurnP99[1] = r.TimeToTurnP99[1], r.TimeToTurnP99[0]
	r.TimeToTurnMax[0], r.TimeToTurnMax[1] = r.TimeToTurnMax[1], r.TimeToTurnMax[0]
	switch r.Winner {
	case 0:
		r.Winner = 1
	case 1:
		r.Winner = 0
	}
	return r
}

func lossReasonFor(player *engine.Player, winner, playerIndex int) LossReason {
	if player.IsTimedOut() {
		return LossReasonTimeout
	}
	if player.IsDeactivated() {
		return LossReasonBadCommand
	}
	if winner >= 0 && winner != playerIndex {
		return LossReasonScore
	}
	return LossReasonNone
}

func printDebugMap(seed int64, game *engine.Game) {
	fmt.Fprintf(os.Stderr, "debug seed=%d map=%dx%d apples=%d\n", seed, game.Grid.Width, game.Grid.Height, len(game.Grid.Apples))
	for y := 0; y < game.Grid.Height; y++ {
		var row strings.Builder
		for x := 0; x < game.Grid.Width; x++ {
			if game.Grid.GetXY(x, y).Type == engine.TileWall {
				row.WriteByte('#')
			} else {
				row.WriteByte('.')
			}
		}
		fmt.Fprintln(os.Stderr, row.String())
	}
	fmt.Fprintln(os.Stderr, "initial birds:")
	for _, bird := range game.AllBirds() {
		fmt.Fprintf(os.Stderr, "  p%d bird %d: %s\n", bird.Owner.GetIndex(), bird.ID, debugBody(bird.Body))
	}
	fmt.Fprintln(os.Stderr, "initial apples:")
	for _, apple := range game.Grid.Apples {
		fmt.Fprintf(os.Stderr, "  %d %d\n", apple.X, apple.Y)
	}
}

func printDebugTurnState(label string, turn int, game *engine.Game) {
	fmt.Fprintf(os.Stderr, "turn %d %s\n", turn, label)
	fmt.Fprintf(os.Stderr, "  apples (%d): %s\n", len(game.Grid.Apples), debugCoords(game.Grid.Apples))
	for _, bird := range game.AllBirds() {
		fmt.Fprintf(os.Stderr, "  p%d bird %d alive=%v body=%s\n", bird.Owner.GetIndex(), bird.ID, bird.Alive, debugBody(bird.Body))
	}
}

func debugBody(body []engine.Coord) string {
	parts := make([]string, len(body))
	for i, c := range body {
		parts[i] = fmt.Sprintf("%d,%d", c.X, c.Y)
	}
	return strings.Join(parts, ":")
}

func debugCoords(coords []engine.Coord) string {
	if len(coords) == 0 {
		return "-"
	}
	parts := make([]string, len(coords))
	for i, c := range coords {
		parts[i] = fmt.Sprintf("%d,%d", c.X, c.Y)
	}
	return strings.Join(parts, " ")
}
