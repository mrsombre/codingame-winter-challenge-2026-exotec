package grid

import (
	"crypto/sha1"
	"encoding/binary"
)

const sha1DigestSize = 20

// SHA1PRNG mirrors the seeded SHA1PRNG path used by CodinGame's Java engine:
// SecureRandom.getInstance("SHA1PRNG"); random.setSeed(seed).
type SHA1PRNG struct {
	state     []byte
	remainder []byte
	remCount  int
}

func NewSHA1PRNG(seed int64) *SHA1PRNG {
	r := &SHA1PRNG{}
	r.setSeed(longToLittleEndian(seed))
	return r
}

func (r *SHA1PRNG) Float64() float64 {
	hi := int64(r.next(26))
	lo := int64(r.next(27))
	return float64(hi<<27+lo) / float64(int64(1)<<53)
}

func (r *SHA1PRNG) Intn(n int) int {
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

func (r *SHA1PRNG) Shuffle(n int, swap func(i, j int)) {
	for i := n - 1; i > 0; i-- {
		swap(i, r.Intn(i+1))
	}
}

func (r *SHA1PRNG) setSeed(seed []byte) {
	if len(r.state) != 0 {
		buf := make([]byte, 0, len(r.state)+len(seed))
		buf = append(buf, r.state...)
		buf = append(buf, seed...)
		sum := sha1.Sum(buf)
		r.state = sum[:]
	} else {
		sum := sha1.Sum(seed)
		r.state = sum[:]
	}
	r.remCount = 0
}

func (r *SHA1PRNG) next(numBits int) int32 {
	numBytes := (numBits + 7) / 8
	buf := make([]byte, numBytes)
	r.nextBytes(buf)

	next := 0
	for _, b := range buf {
		next = (next << 8) + int(b)
	}
	return int32(uint32(next) >> uint(numBytes*8-numBits))
}

func (r *SHA1PRNG) nextBytes(result []byte) {
	index := 0
	output := r.remainder

	if len(r.state) == 0 {
		// CodinGame always provides an explicit arena seed. Keep the zero-state
		// path deterministic rather than falling back to host entropy.
		r.setSeed(make([]byte, 8))
	}

	if r.remCount > 0 {
		todo := min(len(result)-index, sha1DigestSize-r.remCount)
		rpos := r.remCount
		for i := 0; i < todo; i++ {
			result[index+i] = output[rpos]
			output[rpos] = 0
			rpos++
		}
		r.remCount += todo
		index += todo
	}

	for index < len(result) {
		sum := sha1.Sum(r.state)
		output = make([]byte, sha1DigestSize)
		copy(output, sum[:])
		updateState(r.state, output)

		todo := min(len(result)-index, sha1DigestSize)
		for i := 0; i < todo; i++ {
			result[index] = output[i]
			output[i] = 0
			index++
		}
		r.remCount += todo
	}

	r.remainder = output
	r.remCount %= sha1DigestSize
}

func updateState(state, output []byte) {
	last := 1
	changed := false

	for i := range state {
		v := int(int8(state[i])) + int(int8(output[i])) + last
		t := byte(v)
		if state[i] != t {
			changed = true
		}
		state[i] = t
		last = v >> 8
	}

	if !changed {
		state[0]++
	}
}

func longToLittleEndian(seed int64) []byte {
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, uint64(seed))
	return buf
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
