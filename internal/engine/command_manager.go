// Package engine
// Source: source/src/main/java/com/codingame/game/CommandManager.java
package engine

import (
	"errors"
	"strings"
)

type CommandManager struct{}

func NewCommandManager() *CommandManager {
	return &CommandManager{}
}

func (m *CommandManager) ParseCommands(player *Player, lines []string) error {
	if len(lines) == 0 {
		return nil
	}

	commands := strings.Split(lines[0], ";")
	limit := 30

	for _, rawCommand := range commands {
		if limit <= 0 {
			return nil
		}
		limit--

		command := strings.TrimSpace(rawCommand)
		action, err := ParseAction(command)
		if err != nil {
			m.DeactivatePlayer(player, err.Error())
			player.SetScore(-1)
			return err
		}
		if action == nil {
			continue
		}

		if action.IsMove() {
			if err := m.applyMove(player, action); err != nil {
				continue
			}
			continue
		}

		if action.IsMark() {
			_ = player.AddMark(action.GetCoord())
		}
	}

	return nil
}

func (m *CommandManager) applyMove(player *Player, action *Action) error {
	bird := player.GetBirdByID(action.GetBirdID())
	if bird == nil {
		return errors.New("bird not found")
	}
	if !bird.Alive {
		return errors.New("bird is dead")
	}
	if bird.Direction != DirUnset {
		return errors.New("bird has already been given a move")
	}
	if bird.GetFacing().Opposite() == action.GetDirection() {
		return errors.New("bird cannot move backwards")
	}

	bird.Direction = action.GetDirection()
	if action.GetMessage() != "" {
		bird.SetMessage(action.GetMessage())
	}
	return nil
}

func (m *CommandManager) DeactivatePlayer(player *Player, message string) {
	player.Deactivate(escapeHTMLEntities(message))
}

func ParseCommands(player *Player, lines []string, _ *Game) {
	_ = NewCommandManager().ParseCommands(player, lines)
}

func escapeHTMLEntities(message string) string {
	message = strings.ReplaceAll(message, "&lt;", "<")
	return strings.ReplaceAll(message, "&gt;", ">")
}
