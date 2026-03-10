package engine

type Referee struct {
	CommandManager *CommandManager
	Game           *Game
}

func NewReferee(game *Game, commandManager *CommandManager) *Referee {
	if game == nil {
		panic("game is required")
	}
	if commandManager == nil {
		commandManager = NewCommandManager()
	}
	return &Referee{
		CommandManager: commandManager,
		Game:           game,
	}
}

func (r *Referee) Init(players []*Player) {
	r.Game.Init(players)
}

func (r *Referee) ResetGameTurnData() {
	r.Game.ResetGameTurnData()
}

func (r *Referee) GlobalInfoFor(player *Player) []string {
	return SerializeGlobalInfoFor(player, r.Game)
}

func (r *Referee) FrameInfoFor(player *Player) []string {
	return SerializeFrameInfoFor(player, r.Game)
}

func (r *Referee) ParsePlayerOutputs(players []*Player) {
	for _, player := range players {
		if player.IsDeactivated() || r.Game.ShouldSkipPlayerTurn(player) {
			continue
		}
		_ = r.CommandManager.ParseCommands(player, player.GetOutputs())
	}
}

func (r *Referee) PerformGameUpdate(turn int) {
	r.Game.PerformGameUpdate(turn)
}

func (r *Referee) OnEnd() {
	r.Game.OnEnd()
}

func (r *Referee) EndGame() {
	r.Game.EndGame()
}

func (r *Referee) Ended() bool {
	return r.Game.Ended()
}

func (r *Referee) ShouldSkipPlayerTurn(player *Player) bool {
	return r.Game.ShouldSkipPlayerTurn(player)
}

func (r *Referee) ActivePlayers(players []*Player) int {
	active := 0
	for _, player := range players {
		if !player.IsDeactivated() {
			active++
		}
	}
	return active
}
