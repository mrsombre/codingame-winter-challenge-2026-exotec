package engine

type GameException struct {
	message string
}

func NewGameException(message string) *GameException {
	return &GameException{message: message}
}

func (e *GameException) Error() string {
	return e.message
}
