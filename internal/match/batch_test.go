package match

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDeriveSeedKeepsBaseSeedForOffsetZero(t *testing.T) {
	base := int64(-1755827269105404700)
	assert.Equal(t, base, deriveSeed(base, 0, nil))
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
		assert.Equalf(t, tc.want, deriveSeed(base, tc.offset, &increment), "deriveSeed(%d, %d, %d)", base, tc.offset, increment)
	}
}

func TestMixSeedIsDeterministicForSignedBase(t *testing.T) {
	base := int64(-1755827269105404700)
	gotA := mixSeed(base, 3)
	gotB := mixSeed(base, 3)
	assert.Equal(t, gotA, gotB)
	assert.NotEqual(t, base, gotA)
}
