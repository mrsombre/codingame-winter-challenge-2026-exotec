package main

import (
	"encoding/base64"
)

const (
	featureCount = 16
	hidden1Count = 32
)

type Model struct {
	Trained bool
	W1      []float32
	B1      []float32
	W2      []float32
	B2      []float32
}

func LoadModel() *Model {
	m := &Model{
		W1: make([]float32, featureCount*hidden1Count),
		B1: make([]float32, hidden1Count),
		W2: make([]float32, hidden1Count),
		B2: make([]float32, 1),
	}
	if modelBlobBase64 == "" {
		return m
	}

	raw, err := base64.StdEncoding.DecodeString(modelBlobBase64)
	if err != nil {
		return m
	}
	total := featureCount*hidden1Count + hidden1Count + hidden1Count + 1
	values := unpackInt4(raw, total)
	offset := 0

	readInto(m.W1, values[offset:], modelTensorScales[0])
	offset += len(m.W1)
	readInto(m.B1, values[offset:], modelTensorScales[1])
	offset += len(m.B1)
	readInto(m.W2, values[offset:], modelTensorScales[2])
	offset += len(m.W2)
	readInto(m.B2, values[offset:], modelTensorScales[3])
	m.Trained = true
	return m
}

func readInto(dst []float32, src []int8, scale float32) {
	for i := range dst {
		dst[i] = float32(src[i]) * scale
	}
}

func unpackInt4(data []byte, count int) []int8 {
	out := make([]int8, 0, count)
	for _, b := range data {
		lo := int8(b & 0x0F)
		hi := int8((b >> 4) & 0x0F)
		if lo >= 8 {
			lo -= 16
		}
		if hi >= 8 {
			hi -= 16
		}
		out = append(out, lo)
		if len(out) == count {
			return out
		}
		out = append(out, hi)
		if len(out) == count {
			return out
		}
	}
	return out
}

func (m *Model) Score(features []float32) float32 {
	if !m.Trained {
		return 0
	}
	h1 := make([]float32, hidden1Count)
	for j := 0; j < hidden1Count; j++ {
		sum := m.B1[j]
		for i := 0; i < featureCount; i++ {
			sum += features[i] * m.W1[i*hidden1Count+j]
		}
		if sum > 0 {
			h1[j] = sum
		}
	}

	sum := m.B2[0]
	for i := 0; i < hidden1Count; i++ {
		sum += h1[i] * m.W2[i]
	}
	return sum
}
