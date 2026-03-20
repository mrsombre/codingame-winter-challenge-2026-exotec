package match

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	engine "codingame/internal/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSnapshotTraceTurnIncludesStateAndCommands(t *testing.T) {
	game := engine.NewGame(1, 1)
	players := []*engine.Player{
		engine.NewPlayer(0),
		engine.NewPlayer(1),
	}
	game.Init(players)
	game.Grid.Apples = []engine.Coord{{X: 2, Y: 3}}
	players[0].SetOutputs([]string{"0 RIGHT;1 DOWN"})
	players[1].SetOutputs([]string{"4 LEFT"})

	row := snapshotTraceTurn(7, 99, 3, game, players, false)
	addTraceMap(&row, game)

	require.Len(t, row.Apples, 1)
	assert.Equal(t, traceCoord{X: 2, Y: 3}, row.Apples[0])
	require.NotEmpty(t, row.Snakes)
	assert.Equal(t, "0 RIGHT;1 DOWN", row.P0Command)
	assert.Equal(t, "4 LEFT", row.P1Command)
	assert.Equal(t, game.Grid.Width, row.Width)
	assert.Equal(t, game.Grid.Height, row.Height)
	require.Len(t, row.Walls, game.Grid.Height)
	assert.Contains(t, row.Walls[game.Grid.Height-1], "#")
}

func TestTraceWriterWritesGzipJSONL(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "trace.jsonl.gz")

	writer, err := NewTraceWriter(path)
	require.NoError(t, err)
	require.NotNil(t, writer)

	rows := []TraceTurn{
		{MatchID: 1, Seed: 2, Turn: 0, Winner: 0, Width: 3, Height: 2, Walls: []string{"...", "..#"}},
		{MatchID: 1, Seed: 2, Turn: 1, Winner: 0, P0Command: "0 RIGHT"},
	}
	require.NoError(t, writer.WriteMatch(rows))
	require.NoError(t, writer.Close())

	file, err := os.Open(path)
	require.NoError(t, err)
	defer file.Close()

	gz, err := gzip.NewReader(file)
	require.NoError(t, err)
	defer gz.Close()

	dec := json.NewDecoder(gz)
	var got []TraceTurn
	for dec.More() {
		var row TraceTurn
		require.NoError(t, dec.Decode(&row))
		got = append(got, row)
	}
	require.Len(t, got, 2)
	assert.Equal(t, rows[0].Walls, got[0].Walls)
	assert.Equal(t, rows[1].P0Command, got[1].P0Command)
}
