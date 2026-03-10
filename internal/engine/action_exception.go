package engine

type ActionException struct {
	message string
}

func NewActionException(message string) *ActionException {
	return &ActionException{message: message}
}

func (e *ActionException) Error() string {
	return e.message
}
