package game

const MaxBody = 80

type Body struct {
	Parts [MaxBody + 1]Point
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

func (b Body) Slice() []Point {
	return b.Parts[:b.Len]
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

// --- Free functions for Body (not methods, to avoid bundling) ----------------

func BodyHead(b *Body) (Point, bool) {
	if b.Len == 0 {
		return Point{}, false
	}
	return b.Parts[0], true
}

func BodyTail(b *Body) (Point, bool) {
	if b.Len == 0 {
		return Point{}, false
	}
	return b.Parts[b.Len-1], true
}

func BodyReset(b *Body) {
	b.Len = 0
}

func BodyCopy(dst, src *Body) {
	dst.Len = src.Len
	copy(dst.Parts[:dst.Len], src.Parts[:src.Len])
}

// BodyFacing returns the facing direction for a body slice.
// Returns DirUp (not DirNone) for bodies shorter than 2 parts,
// so callers like LegalDirs always receive a valid direction.
func BodyFacing(body []Point) Direction {
	if len(body) < 2 {
		return DirUp
	}
	return FacingPts(body[0], body[1])
}
