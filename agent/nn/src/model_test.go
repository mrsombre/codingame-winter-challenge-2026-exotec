package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUnpackInt4RoundTrip(t *testing.T) {
	values := []int8{-8, -3, 0, 6, 7}
	packed := packTestInt4(values)
	got := unpackInt4(packed, len(values))
	assert.Equal(t, values, got)
}

func TestLoadModelIsStable(t *testing.T) {
	m := LoadModel()
	assert.Equal(t, modelBlobBase64 != "", m.Trained)
	features := make([]float32, featureCount)
	score := m.Score(features)
	assert.True(t, score == score)
}

func packTestInt4(values []int8) []byte {
	padded := append([]int8(nil), values...)
	if len(padded)%2 == 1 {
		padded = append(padded, 0)
	}
	data := make([]byte, 0, len(padded)/2)
	for i := 0; i < len(padded); i += 2 {
		lo := byte(int(padded[i]) & 0x0F)
		hi := byte(int(padded[i+1]) & 0x0F)
		data = append(data, lo|(hi<<4))
	}
	return data
}
