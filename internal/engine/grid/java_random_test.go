// Package grid
package grid

import (
	"fmt"
	"math"
	"testing"
)

// Known values from java.util.Random(0):
//
//	new Random(0).nextInt()    = -1155484576
//	new Random(0).nextDouble() = 0.730967787376657
//	new Random(0).nextInt(10)  = 0
func TestJavaRandomKnownValues(t *testing.T) {
	r := NewJavaRandom(0)
	gotInt := int32(r.next(32))
	if gotInt != -1155484576 {
		t.Errorf("nextInt() got %d, want -1155484576", gotInt)
	}

	r2 := NewJavaRandom(0)
	gotDouble := r2.Float64()
	want := 0.730967787376657
	if gotDouble != want {
		t.Errorf("nextDouble() got %.15f, want %.15f", gotDouble, want)
	}

	r3 := NewJavaRandom(0)
	gotBounded := r3.Intn(10)
	if gotBounded != 0 {
		t.Errorf("nextInt(10) got %d, want 0", gotBounded)
	}
}

// nextLong matches java.util.Random.nextLong().
func nextLong(r *JavaRandom) int64 {
	return int64(r.next(32))<<32 + int64(r.next(32))
}

// TestJavaRandomDebugSequence prints the first few values with advance=1
// to understand what the grid maker sees.
func TestJavaRandomDebugSequence(t *testing.T) {
	matchSeed := int64(-1755827269105404700)

	// Try 0..5 advance steps and print first 3 Float64 values (height, b, first row)
	for advance := 0; advance <= 5; advance++ {
		r := NewJavaRandom(matchSeed)
		for i := 0; i < advance; i++ {
			r.next(32)
		}
		f1 := r.Float64() // height factor
		f2 := r.Float64() // b factor
		h := 10 + int(math.Round(math.Pow(f1, 2)*14)) // bronze skew
		b := 5 + f2*10
		t.Logf("advance=%d: height_factor=%.6f height=%d b_factor=%.6f b=%.2f",
			advance, f1, h, f2, b)
	}

	// Also print for advance=1 with Float64 followed by full Make output summary
	t.Log("---")
	r := NewJavaRandom(matchSeed)
	r.next(32)
	f1 := r.Float64()
	f2 := r.Float64()
	t.Logf("advance=1: f1=%.10f f2=%.10f b=%.4f", f1, f2, 5+f2*10)
	_ = fmt.Sprintf // keep import
}

// TestJavaRandomAdvanceProbe tries advancing the RNG by N steps before running
// GridMaker to find the (N, leagueLevel) combination that produces 18×10.
func TestJavaRandomAdvanceProbe(t *testing.T) {
	matchSeed := int64(-1755827269105404700)
	skews := map[string]float64{"legend(0)": 0.3, "gold(3)": 0.8, "silver(2)": 1.0, "bronze(1)": 2.0}

	for n := 0; n <= 10; n++ {
		for name, skew := range skews {
			r := NewJavaRandom(matchSeed)
			for i := 0; i < n; i++ {
				r.next(32)
			}
			f := r.Float64()
			h := 10 + int(math.Round(math.Pow(f, skew)*14))
			w := int(math.Round(float64(h) * 1.8))
			if w%2 != 0 {
				w++
			}
			if w == 18 && h == 10 {
				t.Logf("MATCH: advance=%d, %s → %dx%d (first float=%.6f)", n, name, w, h, f)
			}
		}
	}
}
