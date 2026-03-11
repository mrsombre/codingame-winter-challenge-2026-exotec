package match

import (
	"errors"
	"fmt"
	"os"
	"strings"

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
	ID             int
	Seed           int64
	Turns          int
	Scores         [2]int
	Losses         [2]int
	Winner         int
	LossReasons    [2]LossReason
	BirdsPerPlayer int
	MapWidth       int
	MapHeight      int
	Apples         int
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
		{Label: "losses_p0", Value: float64(r.Losses[0])},
		{Label: "losses_p1", Value: float64(r.Losses[1])},
	}
}

func (r MatchResult) RenderMatch() string {
	return fmt.Sprintf(
		`{"id":%d,"seed":%d,"turns":%d,"winner":%d,"loss_reason_p0":%q,"loss_reason_p1":%q,"score_p0":%d,"score_p1":%d,"losses_p0":%d,"losses_p1":%d,"birds_per_player":%d,"map_width":%d,"map_height":%d,"apples":%d}`,
		r.ID, r.Seed, r.Turns, r.Winner,
		r.LossReasons[0], r.LossReasons[1],
		r.Scores[0], r.Scores[1],
		r.Losses[0], r.Losses[1],
		r.BirdsPerPlayer, r.MapWidth, r.MapHeight, r.Apples,
	)
}

func (runner *Runner) RunMatch(simulationID int, seed int64) MatchResult {
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

	cleanup, err := attachCommandPlayers(runner.Options, players)
	if err != nil {
		panic(err)
	}
	defer cleanup()

	for _, player := range players {
		for _, line := range referee.GlobalInfoFor(player) {
			player.SendInputLine(line)
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
			for _, line := range referee.FrameInfoFor(player) {
				player.SendInputLine(line)
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

	return buildMatchResult(simulationID, seed, turn, referee.Game, players)
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

		var timeoutErr *timeoutError
		if errors.As(err, &timeoutErr) {
			player.Deactivate("Timeout!")
			player.SetTimedOut(true)
			continue
		}

		player.Deactivate(err.Error())
	}

	referee.ParsePlayerOutputs(players)
}

func attachCommandPlayers(options MatchOptions, players []*engine.Player) (func(), error) {
	controllers := make([]*commandPlayer, 0, len(players))
	bins := []string{options.P0Bin, options.P1Bin}

	for i, path := range bins {
		cp, err := newCommandPlayer(players[i], path)
		if err != nil {
			for _, controller := range controllers {
				_ = controller.Close()
			}
			return nil, fmt.Errorf("failed to start player %d session: %w", i, err)
		}
		players[i].SetExecuteFunc(cp.Execute)
		controllers = append(controllers, cp)
	}

	return func() {
		for _, controller := range controllers {
			_ = controller.Close()
		}
	}, nil
}

func buildMatchResult(simulationID int, seed int64, turns int, game *engine.Game, players []*engine.Player) MatchResult {
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

	return MatchResult{
		ID:             simulationID,
		Seed:           seed,
		Turns:          turns,
		Scores:         [2]int{players[0].GetScore(), players[1].GetScore()},
		Losses:         game.Losses,
		Winner:         winner,
		LossReasons:    [2]LossReason{lossReasonFor(players[0], winner, 0), lossReasonFor(players[1], winner, 1)},
		BirdsPerPlayer: birdsPerPlayer,
		MapWidth:       game.Grid.Width,
		MapHeight:      game.Grid.Height,
		Apples:         len(game.Grid.Apples),
	}
}

func lossMetric(actual, expected LossReason) float64 {
	if actual == expected {
		return 1.0
	}
	return 0.0
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
