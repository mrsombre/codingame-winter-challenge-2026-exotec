package engine

type TileType int

const (
	TileEmpty TileType = 0
	TileWall  TileType = 1
)

type Tile struct {
	Type  TileType
	Coord Coord
	valid bool
}

var NoTile = &Tile{
	Type:  -1,
	Coord: Coord{X: -1, Y: -1},
	valid: false,
}

func NewTile(coord Coord) *Tile {
	return &Tile{
		Type:  TileEmpty,
		Coord: coord,
		valid: true,
	}
}

func (t *Tile) IsValid() bool {
	return t != nil && t.valid
}

func (t *Tile) SetType(tileType TileType) {
	if t == nil || !t.valid {
		return
	}
	t.Type = tileType
}

func (t *Tile) Clear() {
	t.SetType(TileEmpty)
}

func (t *Tile) IsAccessible() bool {
	return true
}
