package engine

type Action struct {
	actionType ActionType
	direction  Direction
	birdID     int
	coord      Coord
	hasBirdID  bool
	hasDir     bool
	hasCoord   bool
	message    string
}

func NewAction(actionType ActionType) *Action {
	return &Action{
		actionType: actionType,
		direction:  DirUnset,
	}
}

func (a *Action) GetType() ActionType {
	return a.actionType
}

func (a *Action) GetMessage() string {
	return a.message
}

func (a *Action) SetMessage(message string) {
	a.message = message
}

func (a *Action) IsMove() bool {
	return a.hasDir
}

func (a *Action) IsMark() bool {
	return a.hasCoord
}

func (a *Action) GetBirdID() int {
	return a.birdID
}

func (a *Action) SetBirdID(birdID int) {
	a.birdID = birdID
	a.hasBirdID = true
}

func (a *Action) GetDirection() Direction {
	return a.direction
}

func (a *Action) SetDirection(direction Direction) {
	a.direction = direction
	a.hasDir = true
}

func (a *Action) SetCoord(coord Coord) {
	a.coord = coord
	a.hasCoord = true
}

func (a *Action) GetCoord() Coord {
	return a.coord
}
