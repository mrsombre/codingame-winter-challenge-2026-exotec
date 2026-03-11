package agentkit

// MaxBody matches the practical fixed-capacity style used by the arena-focused agent.
const MaxBody = 80

// Body is an arena-friendly snake body representation.
//
// Current codebase patterns:
// - engine and agent/basic keep bodies as slices
// - agent/genetic keeps a fixed array plus body length
//
// This example follows the fixed-array layout so helpers can avoid per-turn allocations.
type Body struct {
	Parts [MaxBody]Point
	Len   int
}

func NewBody(points []Point) Body {
	var body Body
	body.Set(points)
	return body
}

func (b *Body) Set(points []Point) {
	if len(points) > MaxBody {
		panic("body too large")
	}
	b.Len = len(points)
	copy(b.Parts[:b.Len], points)
}

func (b *Body) Reset() {
	b.Len = 0
}

func (b Body) Slice() []Point {
	return b.Parts[:b.Len]
}

func (b Body) Head() (Point, bool) {
	if b.Len == 0 {
		return Point{}, false
	}
	return b.Parts[0], true
}

func (b Body) Tail() (Point, bool) {
	if b.Len == 0 {
		return Point{}, false
	}
	return b.Parts[b.Len-1], true
}

func (b Body) Facing() Direction {
	if b.Len < 2 {
		return DirNone
	}
	return FacingFromPoints(b.Parts[0], b.Parts[1])
}

func (b Body) Contains(p Point) bool {
	for i := 0; i < b.Len; i++ {
		if b.Parts[i] == p {
			return true
		}
	}
	return false
}

func (b *Body) CopyFrom(other Body) {
	b.Len = other.Len
	copy(b.Parts[:b.Len], other.Parts[:other.Len])
}
