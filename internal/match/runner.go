package match

import (
	"errors"
	"fmt"

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
	MaxTurns int
	P0Bin    string
	P1Bin    string
}

type Runner struct {
	Options MatchOptions
}

func NewRunner(options MatchOptions) *Runner {
	if options.MaxTurns == 0 {
		options.MaxTurns = engine.MaxTurns
	}
	return &Runner{Options: options}
}

type MatchResult struct {
	ID             int
	Seed           uint64
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

func (runner *Runner) RunMatch(simulationID int, seed uint64) MatchResult {
	players := []*engine.Player{
		engine.NewPlayer(0),
		engine.NewPlayer(1),
	}
	players[0].SetNicknameToken("Player 0")
	players[1].SetNicknameToken("Player 1")

	game := engine.NewGame(int64(seed), 4)
	referee := engine.NewReferee(game, engine.NewCommandManager())
	referee.Init(players)

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

		for _, player := range players {
			if player.IsDeactivated() || referee.ShouldSkipPlayerTurn(player) {
				continue
			}
			for _, line := range referee.FrameInfoFor(player) {
				player.SendInputLine(line)
			}
			_ = player.Execute()
		}

		handlePlayerCommands(players, referee)
		if referee.ActivePlayers(players) < 2 {
			referee.EndGame()
			break
		}

		referee.PerformGameUpdate(turn)
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

func buildMatchResult(simulationID int, seed uint64, turns int, game *engine.Game, players []*engine.Player) MatchResult {
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
