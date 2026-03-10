package match

import "testing"

func TestDeriveSeedKeepsBaseSeedForOffsetZero(t *testing.T) {
	base := int64(-1755827269105404700)
	if got := deriveSeed(base, 0, nil); got != base {
		t.Fatalf("deriveSeed(%d, 0, nil) = %d, want %d", base, got, base)
	}
}

func TestDeriveSeedUsesSignedIncrementSequence(t *testing.T) {
	base := int64(-5)
	increment := int64(7)

	testCases := []struct {
		offset uint64
		want   int64
	}{
		{offset: 0, want: -5},
		{offset: 1, want: 2},
		{offset: 2, want: 9},
	}

	for _, tc := range testCases {
		if got := deriveSeed(base, tc.offset, &increment); got != tc.want {
			t.Fatalf("deriveSeed(%d, %d, %d) = %d, want %d", base, tc.offset, increment, got, tc.want)
		}
	}
}

func TestMixSeedIsDeterministicForSignedBase(t *testing.T) {
	base := int64(-1755827269105404700)
	gotA := mixSeed(base, 3)
	gotB := mixSeed(base, 3)
	if gotA != gotB {
		t.Fatalf("mixSeed returned different values for same input: %d vs %d", gotA, gotB)
	}
	if gotA == base {
		t.Fatalf("mixSeed(%d, 3) = %d, want value different from base", base, gotA)
	}
}
