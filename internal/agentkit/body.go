package agentkit

const MaxBody = 80

type Body struct {
	Parts [MaxBody]Point
	Len   int
}

func NewBody(points []Point) Body {
	var b Body
	b.Set(points)
	return b
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
	return FacingPts(b.Parts[0], b.Parts[1])
}

func (b Body) Contains(p Point) bool {
	for i := 0; i < b.Len; i++ {
		if b.Parts[i] == p {
			return true
		}
	}
	return false
}

func (b *Body) Copy(other Body) {
	b.Len = other.Len
	copy(b.Parts[:b.Len], other.Parts[:other.Len])
}
