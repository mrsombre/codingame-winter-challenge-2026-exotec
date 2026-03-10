package engine

type Bird struct {
	ID        int
	Body      []Coord
	Owner     *Player
	Alive     bool
	Direction Direction
	Message   string
}

func NewBird(id int, owner *Player) *Bird {
	return &Bird{
		ID:        id,
		Body:      make([]Coord, 0),
		Owner:     owner,
		Alive:     true,
		Direction: DirUnset,
	}
}

func (b *Bird) HeadPos() Coord {
	return b.Body[0]
}

func (b *Bird) GetFacing() Direction {
	if len(b.Body) < 2 {
		return DirUnset
	}
	return DirectionFromCoord(Coord{
		X: b.Body[0].X - b.Body[1].X,
		Y: b.Body[0].Y - b.Body[1].Y,
	})
}

func (b *Bird) IsAlive() bool {
	return b.Alive
}

func (b *Bird) SetMessage(msg string) {
	b.Message = msg
	if len(b.Message) > 48 {
		b.Message = b.Message[:46] + "..."
	}
}

func (b *Bird) HasMessage() bool {
	return b.Message != ""
}

func (b *Bird) BodyContains(c Coord) bool {
	for _, part := range b.Body {
		if part == c {
			return true
		}
	}
	return false
}
