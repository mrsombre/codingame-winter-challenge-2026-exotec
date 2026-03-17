// Package grid
package grid

// JavaRandom replicates java.util.Random (48-bit LCG) so that seeds from the
// Java referee produce identical sequences in Go.
type JavaRandom struct {
	seed int64
}

const (
	jrMultiplier = int64(0x5DEECE66D)
	jrAddend     = int64(0xB)
	jrMask       = int64((int64(1) << 48) - 1)
)

func NewJavaRandom(seed int64) *JavaRandom {
	return &JavaRandom{seed: (seed ^ jrMultiplier) & jrMask}
}

func (r *JavaRandom) next(bits int) int32 {
	r.seed = (r.seed*jrMultiplier + jrAddend) & jrMask
	return int32(uint64(r.seed) >> uint(48-bits))
}

// Float64 matches java.util.Random.nextDouble().
func (r *JavaRandom) Float64() float64 {
	hi := int64(r.next(26))
	lo := int64(r.next(27))
	return float64(hi<<27+lo) / float64(int64(1)<<53)
}

// Intn matches java.util.Random.nextInt(int).
func (r *JavaRandom) Intn(n int) int {
	bound := int32(n)
	m := bound - 1
	bits := r.next(31)
	if bound&m == 0 {
		return int(int64(bound) * int64(bits) >> 31)
	}
	val := bits % bound
	for bits-val+m < 0 {
		bits = r.next(31)
		val = bits % bound
	}
	return int(val)
}

// Shuffle matches the Fisher-Yates order used by Java's Collections.shuffle.
func (r *JavaRandom) Shuffle(n int, swap func(i, j int)) {
	for i := n - 1; i > 0; i-- {
		swap(i, r.Intn(i+1))
	}
}
