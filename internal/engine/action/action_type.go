// Package action
// Source: source/src/main/java/com/codingame/game/action/ActionType.java
package action

import (
	"fmt"
	"regexp"
	"strconv"
)

type ActionType int

const (
	ActionTypeMoveUp ActionType = iota
	ActionTypeMoveDown
	ActionTypeMoveLeft
	ActionTypeMoveRight
	ActionTypeMark
	ActionTypeWait
)

type actionTypeSpec struct {
	pattern *regexp.Regexp
	apply   func(match []string, action *Action) error
}

var orderedActionTypes = []ActionType{
	ActionTypeMoveUp,
	ActionTypeMoveDown,
	ActionTypeMoveLeft,
	ActionTypeMoveRight,
	ActionTypeMark,
	ActionTypeWait,
}

var actionTypeSpecs = map[ActionType]actionTypeSpec{
	ActionTypeMoveUp: {
		pattern: regexp.MustCompile(`(?i)^(\d+) UP(?: ([^;]*))?$`),
		apply: func(match []string, action *Action) error {
			return setMoveAction(match, action, DirNorth)
		},
	},
	ActionTypeMoveDown: {
		pattern: regexp.MustCompile(`(?i)^(\d+) DOWN(?: ([^;]*))?$`),
		apply: func(match []string, action *Action) error {
			return setMoveAction(match, action, DirSouth)
		},
	},
	ActionTypeMoveLeft: {
		pattern: regexp.MustCompile(`(?i)^(\d+) LEFT(?: ([^;]*))?$`),
		apply: func(match []string, action *Action) error {
			return setMoveAction(match, action, DirWest)
		},
	},
	ActionTypeMoveRight: {
		pattern: regexp.MustCompile(`(?i)^(\d+) RIGHT(?: ([^;]*))?$`),
		apply: func(match []string, action *Action) error {
			return setMoveAction(match, action, DirEast)
		},
	},
	ActionTypeMark: {
		pattern: regexp.MustCompile(`(?i)^MARK (\d+) (\d+)$`),
		apply: func(match []string, action *Action) error {
			x, err := strconv.Atoi(match[1])
			if err != nil {
				return err
			}
			y, err := strconv.Atoi(match[2])
			if err != nil {
				return err
			}
			action.SetCoord(Coord{X: x, Y: y})
			return nil
		},
	},
	ActionTypeWait: {
		pattern: regexp.MustCompile(`(?i)^WAIT$`),
		apply:   func(_ []string, _ *Action) error { return nil },
	},
}

func AllActionTypes() []ActionType {
	return orderedActionTypes
}

func (a ActionType) Pattern() *regexp.Regexp {
	return actionTypeSpecs[a].pattern
}

func (a ActionType) Apply(match []string, action *Action) error {
	return actionTypeSpecs[a].apply(match, action)
}

func ParseAction(command string) (*Action, error) {
	for _, actionType := range AllActionTypes() {
		match := actionType.Pattern().FindStringSubmatch(command)
		if match == nil {
			continue
		}

		action := NewAction(actionType)
		if err := actionType.Apply(match, action); err != nil {
			return nil, err
		}
		if actionType == ActionTypeWait {
			return nil, nil
		}
		return action, nil
	}

	return nil, fmt.Errorf("invalid input: expected %s but got %q", "MESSAGE text", command)
}

func setMoveAction(match []string, action *Action, direction Direction) error {
	birdID, err := strconv.Atoi(match[1])
	if err != nil {
		return fmt.Errorf("invalid bird id: %w", err)
	}
	action.SetBirdID(birdID)
	action.SetDirection(direction)
	if len(match) > 2 {
		action.SetMessage(match[2])
	}
	return nil
}
