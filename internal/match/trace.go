package match

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	engine "codingame/internal/engine"
)

type traceCoord struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type traceSnake struct {
	ID    int          `json:"id"`
	Owner int          `json:"owner"`
	Body  []traceCoord `json:"body"`
}

type TraceTurn struct {
	MatchID   int          `json:"match_id"`
	Seed      int64        `json:"seed"`
	Turn      int          `json:"turn"`
	Winner    int          `json:"winner"`
	Width     int          `json:"width,omitempty"`
	Height    int          `json:"height,omitempty"`
	Walls     []string     `json:"walls,omitempty"`
	Apples    []traceCoord `json:"apples"`
	Snakes    []traceSnake `json:"snakes"`
	P0Command string       `json:"p0_command,omitempty"`
	P1Command string       `json:"p1_command,omitempty"`
	Swapped   bool         `json:"swapped,omitempty"`
}

type TraceWriter struct {
	mu   sync.Mutex
	file *os.File
	gz   *gzip.Writer
	enc  *json.Encoder
}

func NewTraceWriter(path string) (*TraceWriter, error) {
	if path == "" {
		return nil, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create trace directory: %w", err)
	}
	file, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("create trace file: %w", err)
	}
	gz := gzip.NewWriter(file)
	return &TraceWriter{
		file: file,
		gz:   gz,
		enc:  json.NewEncoder(gz),
	}, nil
}

func (w *TraceWriter) WriteMatch(rows []TraceTurn) error {
	if w == nil || len(rows) == 0 {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, row := range rows {
		if err := w.enc.Encode(row); err != nil {
			return err
		}
	}
	return nil
}

func (w *TraceWriter) Close() error {
	if w == nil {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	var firstErr error
	if w.gz != nil {
		if err := w.gz.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		w.gz = nil
	}
	if w.file != nil {
		if err := w.file.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		w.file = nil
	}
	return firstErr
}

func snapshotTraceTurn(matchID int, seed int64, turn int, game *engine.Game, players []*engine.Player, swapped bool) TraceTurn {
	row := TraceTurn{
		MatchID: matchID,
		Seed:    seed,
		Turn:    turn,
		Apples:  make([]traceCoord, 0, len(game.Grid.Apples)),
		Snakes:  make([]traceSnake, 0, len(game.LiveBirds())),
		Swapped: swapped,
	}
	for _, apple := range game.Grid.Apples {
		row.Apples = append(row.Apples, traceCoord{X: apple.X, Y: apple.Y})
	}
	for _, bird := range game.LiveBirds() {
		body := make([]traceCoord, 0, len(bird.Body))
		for _, part := range bird.Body {
			body = append(body, traceCoord{X: part.X, Y: part.Y})
		}
		row.Snakes = append(row.Snakes, traceSnake{
			ID:    bird.ID,
			Owner: bird.Owner.GetIndex(),
			Body:  body,
		})
	}
	if len(players) > 0 {
		row.P0Command = strings.TrimSpace(strings.Join(players[0].GetOutputs(), " "))
	}
	if len(players) > 1 {
		row.P1Command = strings.TrimSpace(strings.Join(players[1].GetOutputs(), " "))
	}
	return row
}

func addTraceMap(row *TraceTurn, game *engine.Game) {
	row.Width = game.Grid.Width
	row.Height = game.Grid.Height
	row.Walls = make([]string, 0, game.Grid.Height)
	for y := 0; y < game.Grid.Height; y++ {
		var line strings.Builder
		for x := 0; x < game.Grid.Width; x++ {
			if game.Grid.GetXY(x, y).Type == engine.TileWall {
				line.WriteByte('#')
			} else {
				line.WriteByte('.')
			}
		}
		row.Walls = append(row.Walls, line.String())
	}
}
