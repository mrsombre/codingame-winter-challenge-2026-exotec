package src

const (
	MaxBots    = 8   // total snakes both players
	MaxPerSide = 4   // max snakes per player
	MaxBody    = 256 // max body parts per snake
)

// Snake holds one snake's current body as flat cell indices, head-first.
type Snake struct {
	ID    int
	Owner int // 0 = mine, 1 = enemy
	Body  [MaxBody]int
	Len   int
	Alive bool
}

// Head returns the head cell index.
func (s *Snake) Head() int { return s.Body[0] }

// State holds full game state: immutable init data + mutable per-turn data.
type State struct {
	G *Grid // immutable, set once

	// Init data (immutable after init)
	ID     int             // my player id (0 or 1)
	MyIDs  [MaxPerSide]int // my snake IDs
	MyN    int
	OppIDs [MaxPerSide]int // enemy snake IDs
	OppN   int

	// Turn data (overwritten each turn)
	Snakes [MaxBots]Snake
	SnakeN int           // alive snakes this turn
	Apples [MaxCells]int // flat cell indices
	AppleN int
}

// IsMyID returns true if id belongs to my snakes.
func (st *State) IsMyID(id int) bool {
	for i := 0; i < st.MyN; i++ {
		if st.MyIDs[i] == id {
			return true
		}
	}
	return false
}

// SetApple stores an apple at position x,y.
func (st *State) SetApple(i, x, y int) {
	st.Apples[i] = st.G.Idx(x, y)
}

// SetSnake parses a body string and stores the snake at slot i.
func (st *State) SetSnake(i, id int, body string) {
	s := &st.Snakes[i]
	s.ID = id
	s.Alive = true
	if st.IsMyID(id) {
		s.Owner = 0
	} else {
		s.Owner = 1
	}
	s.Len = ParseBody(body, &s.Body, st.G)
}

// ParseBody parses "x,y:x,y:x,y" into flat indices, returns body length.
func ParseBody(s string, dst *[MaxBody]int, g *Grid) int {
	n := 0
	i := 0
	for i < len(s) && n < MaxBody {
		x := 0
		for i < len(s) && s[i] != ',' {
			x = x*10 + int(s[i]-'0')
			i++
		}
		i++ // skip ','
		y := 0
		for i < len(s) && s[i] != ':' {
			y = y*10 + int(s[i]-'0')
			i++
		}
		i++ // skip ':'
		dst[n] = g.Idx(x, y)
		n++
	}
	return n
}
