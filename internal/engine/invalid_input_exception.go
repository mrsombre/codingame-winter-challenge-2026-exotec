package engine

import "fmt"

type InvalidInputException struct {
	message string
}

func NewInvalidInputException(expected, got string) *InvalidInputException {
	return &InvalidInputException{
		message: fmt.Sprintf("Invalid Input: Expected %s but got '%s'", expected, got),
	}
}

func NewInvalidInputExceptionWithError(prefix, expected, got string) *InvalidInputException {
	return &InvalidInputException{
		message: fmt.Sprintf("%s: Expected %s but got '%s'", prefix, expected, got),
	}
}

func (e *InvalidInputException) Error() string {
	return e.message
}
