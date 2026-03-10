// Package engine
// Source: source/src/main/java/com/codingame/game/Player.java
package engine

import "fmt"

// Player represents one of the two players.
type Player struct {
	index              int
	birds              []*Bird
	marks              []Coord
	score              int
	nicknameToken      string
	deactivated        bool
	deactivationReason string
	timedOut           bool
	inputLines         []string
	outputs            []string
	outputError        error
	executeFn          func() error
}

func NewPlayer(index int) *Player {
	return &Player{
		index:      index,
		birds:      make([]*Bird, 0),
		marks:      make([]Coord, 0),
		inputLines: make([]string, 0),
		outputs:    make([]string, 0),
	}
}

func (p *Player) GetIndex() int                  { return p.index }
func (p *Player) GetBirds() []*Bird              { return p.birds }
func (p *Player) GetScore() int                  { return p.score }
func (p *Player) SetScore(score int)             { p.score = score }
func (p *Player) IsDeactivated() bool            { return p.deactivated }
func (p *Player) IsTimedOut() bool               { return p.timedOut }
func (p *Player) SetTimedOut(v bool)             { p.timedOut = v }
func (p *Player) GetExpectedOutputLines() int    { return 1 }
func (p *Player) GetOutputs() []string           { return p.outputs }
func (p *Player) SetOutputs(o []string)          { p.outputs = o }
func (p *Player) GetOutputError() error          { return p.outputError }
func (p *Player) SetExecuteFunc(fn func() error) { p.executeFn = fn }

func (p *Player) GetNicknameToken() string {
	if p.nicknameToken == "" {
		return fmt.Sprintf("Player %d", p.index)
	}
	return p.nicknameToken
}

func (p *Player) SetNicknameToken(t string) { p.nicknameToken = t }

func (p *Player) Deactivate(message string) {
	p.deactivated = true
	p.deactivationReason = message
}

func (p *Player) Reset() {
	for _, bird := range p.birds {
		bird.Direction = DirUnset
		bird.Message = ""
	}
	p.marks = p.marks[:0]
}

func (p *Player) GetBirdByID(id int) *Bird {
	for _, bird := range p.birds {
		if bird.ID == id {
			return bird
		}
	}
	return nil
}

func (p *Player) AddMark(c Coord) bool {
	if len(p.marks) < 4 {
		p.marks = append(p.marks, c)
		return true
	}
	return false
}

func (p *Player) SendInputLine(line string) {
	p.inputLines = append(p.inputLines, line)
}

func (p *Player) ConsumeInputLines() []string {
	lines := append([]string(nil), p.inputLines...)
	p.inputLines = p.inputLines[:0]
	return lines
}

func (p *Player) Execute() error {
	p.outputError = nil
	if p.executeFn != nil {
		p.outputError = p.executeFn()
		return p.outputError
	}
	return nil
}
