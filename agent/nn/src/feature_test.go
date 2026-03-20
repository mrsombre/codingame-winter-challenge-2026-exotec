package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fixtureCoord struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type fixtureSnake struct {
	ID    int            `json:"id"`
	Owner int            `json:"owner"`
	Body  []fixtureCoord `json:"body"`
}

type featureFixture struct {
	Row struct {
		Width     int            `json:"width"`
		Height    int            `json:"height"`
		Turn      int            `json:"turn"`
		Walls     []string       `json:"walls"`
		Apples    []fixtureCoord `json:"apples"`
		Snakes    []fixtureSnake `json:"snakes"`
		P0Command string         `json:"p0_command"`
		P1Command string         `json:"p1_command"`
	} `json:"row"`
	Features [][]float64 `json:"features"`
	Mask     []int       `json:"mask"`
	Label    int         `json:"label"`
	SnakeID  int         `json:"snake_id"`
}

func TestFeatureFixtureMatchesPython(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "agent", "nn", "train", "fixtures", "feature_fixture.json"))
	require.NoError(t, err)

	var fixture featureFixture
	require.NoError(t, json.Unmarshal(data, &fixture))

	d := decisionFromFixture(t, fixture)
	d.rebuildTurnMaps()
	d.collectMySnakes()
	targetSlot := -1
	for i, snIdx := range d.MySnakes {
		if d.G.Sn[snIdx].ID == fixture.SnakeID {
			targetSlot = i
			break
		}
	}
	require.GreaterOrEqual(t, targetSlot, 0)

	for dir := 0; dir < 4; dir++ {
		cand := d.simulateCandidate(d.MySnakes[targetSlot], dir)
		assert.Equal(t, fixture.Mask[dir] == 1, cand.Legal)
		if !cand.Legal {
			continue
		}
		require.Len(t, fixture.Features[dir], featureCount)
		for i := 0; i < featureCount; i++ {
			assert.InDelta(t, fixture.Features[dir][i], cand.Features[i], 1e-6)
		}
	}
}

func decisionFromFixture(t *testing.T, fixture featureFixture) *Decision {
	t.Helper()
	var init strings.Builder
	init.WriteString("0\n")
	init.WriteString(strconv.Itoa(fixture.Row.Width) + "\n")
	init.WriteString(strconv.Itoa(fixture.Row.Height) + "\n")
	for _, row := range fixture.Row.Walls {
		init.WriteString(row + "\n")
	}

	myIDs := []int{}
	opIDs := []int{}
	for _, sn := range fixture.Row.Snakes {
		if sn.Owner == 0 {
			myIDs = append(myIDs, sn.ID)
		} else {
			opIDs = append(opIDs, sn.ID)
		}
	}
	init.WriteString(strconv.Itoa(len(myIDs)) + "\n")
	for _, id := range myIDs {
		init.WriteString(strconv.Itoa(id) + "\n")
	}
	for _, id := range opIDs {
		init.WriteString(strconv.Itoa(id) + "\n")
	}

	var turn strings.Builder
	turn.WriteString(strconv.Itoa(len(fixture.Row.Apples)) + "\n")
	for _, ap := range fixture.Row.Apples {
		turn.WriteString(strconv.Itoa(ap.X) + " " + strconv.Itoa(ap.Y) + "\n")
	}
	turn.WriteString(strconv.Itoa(len(fixture.Row.Snakes)) + "\n")
	for _, sn := range fixture.Row.Snakes {
		parts := make([]string, 0, len(sn.Body))
		for _, part := range sn.Body {
			parts = append(parts, strconv.Itoa(part.X)+","+strconv.Itoa(part.Y))
		}
		turn.WriteString(strconv.Itoa(sn.ID) + " " + strings.Join(parts, ":") + "\n")
	}
	d := newTestDecision(t, init.String(), turn.String())
	d.G.TurnNum = fixture.Row.Turn
	d.rebuildTurnMaps()
	return d
}
