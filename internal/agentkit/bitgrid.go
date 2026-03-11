package agentkit

// BitGrid is a compact occupancy grid for arena-sized maps.
type BitGrid struct {
	Width  int
	Height int
	bits   []uint64
}

func NewBitGrid(width, height int) BitGrid {
	cells := width * height
	return BitGrid{
		Width:  width,
		Height: height,
		bits:   make([]uint64, (cells+63)/64),
	}
}

func (g BitGrid) InBounds(p Point) bool {
	return p.X >= 0 && p.X < g.Width && p.Y >= 0 && p.Y < g.Height
}

func (g *BitGrid) Reset() {
	for i := range g.bits {
		g.bits[i] = 0
	}
}

func (g BitGrid) Has(p Point) bool {
	idx, ok := g.cellIdx(p)
	if !ok {
		return false
	}
	return g.bits[idx/64]&(uint64(1)<<uint(idx%64)) != 0
}

func (g *BitGrid) Set(p Point) {
	idx, ok := g.cellIdx(p)
	if !ok {
		return
	}
	g.bits[idx/64] |= uint64(1) << uint(idx%64)
}

func (g *BitGrid) Clear(p Point) {
	idx, ok := g.cellIdx(p)
	if !ok {
		return
	}
	g.bits[idx/64] &^= uint64(1) << uint(idx%64)
}

func (g BitGrid) cellIdx(p Point) (int, bool) {
	if !g.InBounds(p) {
		return 0, false
	}
	return p.Y*g.Width + p.X, true
}
