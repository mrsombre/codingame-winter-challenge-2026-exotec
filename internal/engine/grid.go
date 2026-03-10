package engine

var adjacency4 = []Coord{{0, -1}, {1, 0}, {0, 1}, {-1, 0}}
var adjacency8 = []Coord{
	{0, -1}, {1, 0}, {0, 1}, {-1, 0},
	{-1, -1}, {1, 1}, {1, -1}, {-1, 1},
}

type Grid struct {
	Width  int
	Height int
	cells  map[Coord]*Tile
	Spawns []Coord
	Apples []Coord
}

func NewGrid(width, height int) *Grid {
	g := &Grid{
		Width:  width,
		Height: height,
		cells:  make(map[Coord]*Tile),
		Spawns: make([]Coord, 0),
		Apples: make([]Coord, 0),
	}
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			c := Coord{x, y}
			g.cells[c] = NewTile(c)
		}
	}
	return g
}

func (g *Grid) Get(c Coord) *Tile {
	if t, ok := g.cells[c]; ok {
		return t
	}
	return NoTile
}

func (g *Grid) GetXY(x, y int) *Tile {
	return g.Get(Coord{x, y})
}

func (g *Grid) Opposite(c Coord) Coord {
	return Coord{g.Width - c.X - 1, c.Y}
}

func (g *Grid) GetNeighbours(pos Coord, adj []Coord) []Coord {
	result := make([]Coord, 0, len(adj))
	for _, delta := range adj {
		n := pos.Add(delta)
		if g.Get(n).IsValid() {
			result = append(result, n)
		}
	}
	return result
}

func (g *Grid) GetNeighbours4(pos Coord) []Coord {
	return g.GetNeighbours(pos, adjacency4)
}

func (g *Grid) DetectAirPockets() []map[Coord]bool {
	var islands []map[Coord]bool
	computed := make(map[Coord]bool)

	for y := 0; y < g.Height; y++ {
		for x := 0; x < g.Width; x++ {
			p := Coord{x, y}
			if g.Get(p).Type == TileWall {
				computed[p] = true
				continue
			}
			if computed[p] {
				continue
			}
			island := make(map[Coord]bool)
			fifo := []Coord{p}
			computed[p] = true
			for len(fifo) > 0 {
				e := fifo[0]
				fifo = fifo[1:]
				for _, delta := range adjacency4 {
					n := e.Add(delta)
					cell := g.Get(n)
					if cell.IsValid() && !computed[n] && cell.Type != TileWall {
						fifo = append(fifo, n)
						computed[n] = true
					}
				}
				island[e] = true
			}
			islands = append(islands, island)
		}
	}
	return islands
}

func (g *Grid) DetectSpawnIslands() []map[Coord]bool {
	var islands []map[Coord]bool
	computed := make(map[Coord]bool)
	spawnsSet := make(map[Coord]bool)
	for _, s := range g.Spawns {
		spawnsSet[s] = true
	}

	for _, p := range g.Spawns {
		if computed[p] {
			continue
		}
		island := make(map[Coord]bool)
		fifo := []Coord{p}
		computed[p] = true
		for len(fifo) > 0 {
			e := fifo[0]
			fifo = fifo[1:]
			for _, delta := range adjacency4 {
				n := e.Add(delta)
				cell := g.Get(n)
				if cell.IsValid() && !computed[n] && spawnsSet[n] {
					fifo = append(fifo, n)
					computed[n] = true
				}
			}
			island[e] = true
		}
		islands = append(islands, island)
	}
	return islands
}

func (g *Grid) DetectLowestIsland() []Coord {
	start := Coord{0, g.Height - 1}
	if g.Get(start).Type != TileWall {
		return nil
	}
	computed := make(map[Coord]bool)
	fifo := []Coord{start}
	computed[start] = true
	lowest := []Coord{start}
	for len(fifo) > 0 {
		e := fifo[0]
		fifo = fifo[1:]
		for _, delta := range adjacency4 {
			n := e.Add(delta)
			cell := g.Get(n)
			if cell.IsValid() && !computed[n] && cell.Type == TileWall {
				fifo = append(fifo, n)
				computed[n] = true
				lowest = append(lowest, n)
			}
		}
	}
	return lowest
}

func (g *Grid) RemoveApple(c Coord) {
	for i, a := range g.Apples {
		if a == c {
			g.Apples = append(g.Apples[:i], g.Apples[i+1:]...)
			return
		}
	}
}

func (g *Grid) HasApple(c Coord) bool {
	for _, a := range g.Apples {
		if a == c {
			return true
		}
	}
	return false
}

func (g *Grid) Coords() []Coord {
	coords := make([]Coord, 0, len(g.cells))
	for y := 0; y < g.Height; y++ {
		for x := 0; x < g.Width; x++ {
			coords = append(coords, Coord{x, y})
		}
	}
	return coords
}
