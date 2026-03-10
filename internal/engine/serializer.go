package engine

import (
	"fmt"
	"strings"
)

// SerializeGlobalInfoFor returns the initialization lines sent to a player.
func SerializeGlobalInfoFor(player *Player, game *Game) []string {
	var lines []string

	lines = append(lines, fmt.Sprintf("%d", player.GetIndex()))
	lines = append(lines, fmt.Sprintf("%d", game.Grid.Width))
	lines = append(lines, fmt.Sprintf("%d", game.Grid.Height))

	for y := 0; y < game.Grid.Height; y++ {
		var row strings.Builder
		for x := 0; x < game.Grid.Width; x++ {
			if game.Grid.GetXY(x, y).Type == TileWall {
				row.WriteByte('#')
			} else {
				row.WriteByte('.')
			}
		}
		lines = append(lines, row.String())
	}

	// Birds per player (same for both)
	lines = append(lines, fmt.Sprintf("%d", len(game.Players[0].birds)))

	// Player's own bird IDs
	for _, b := range player.birds {
		lines = append(lines, fmt.Sprintf("%d", b.ID))
	}

	// Opponent's bird IDs
	opp := game.Players[1-player.GetIndex()]
	for _, b := range opp.birds {
		lines = append(lines, fmt.Sprintf("%d", b.ID))
	}

	return lines
}

// SerializeFrameInfoFor returns the per-turn input lines sent to a player.
func SerializeFrameInfoFor(player *Player, game *Game) []string {
	var lines []string

	// Power sources (apples)
	lines = append(lines, fmt.Sprintf("%d", len(game.Grid.Apples)))
	for _, c := range game.Grid.Apples {
		lines = append(lines, c.IntString())
	}

	// Live birds
	liveBirds := game.LiveBirds()
	lines = append(lines, fmt.Sprintf("%d", len(liveBirds)))
	for _, b := range liveBirds {
		parts := make([]string, len(b.Body))
		for i, c := range b.Body {
			parts[i] = fmt.Sprintf("%d,%d", c.X, c.Y)
		}
		body := strings.Join(parts, ":")
		lines = append(lines, fmt.Sprintf("%d %s", b.ID, body))
	}

	return lines
}
