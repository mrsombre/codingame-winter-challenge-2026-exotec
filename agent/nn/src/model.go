package main

import (
	"encoding/base64"
	"math"
)

const (
	featureCount = 96
	hidden1Count = 160
	hidden2Count = 96
)

type Model struct {
	Trained bool
	W1      []float32
	B1      []float32
	W2      []float32
	B2      []float32
	W3      []float32
	B3      []float32
}

func LoadModel() *Model {
	m := &Model{
		W1: make([]float32, featureCount*hidden1Count),
		B1: make([]float32, hidden1Count),
		W2: make([]float32, hidden1Count*hidden2Count),
		B2: make([]float32, hidden2Count),
		W3: make([]float32, hidden2Count),
		B3: make([]float32, 1),
	}
	if modelBlobBase64 == "" {
		return m
	}

	raw, err := base64.StdEncoding.DecodeString(modelBlobBase64)
	if err != nil {
		return m
	}
	total := featureCount*hidden1Count + hidden1Count + hidden1Count*hidden2Count + hidden2Count + hidden2Count + 1
	values := unpackInt4(raw, total)
	offset := 0

	readInto(m.W1, values[offset:], modelTensorScales[0])
	offset += len(m.W1)
	readInto(m.B1, values[offset:], modelTensorScales[1])
	offset += len(m.B1)
	readInto(m.W2, values[offset:], modelTensorScales[2])
	offset += len(m.W2)
	readInto(m.B2, values[offset:], modelTensorScales[3])
	offset += len(m.B2)
	readInto(m.W3, values[offset:], modelTensorScales[4])
	offset += len(m.W3)
	readInto(m.B3, values[offset:], modelTensorScales[5])
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
		base := j
		for i := 0; i < featureCount; i++ {
			sum += features[i] * m.W1[i*hidden1Count+base]
		}
		h1[j] = relu(sum)
	}

	h2 := make([]float32, hidden2Count)
	for j := 0; j < hidden2Count; j++ {
		sum := m.B2[j]
		for i := 0; i < hidden1Count; i++ {
			sum += h1[i] * m.W2[i*hidden2Count+j]
		}
		h2[j] = relu(sum)
	}

	sum := m.B3[0]
	for i := 0; i < hidden2Count; i++ {
		sum += h2[i] * m.W3[i]
	}
	return sum
}

func heuristicScore(features []float32) float32 {
	targetDist := features[84]
	enemyDist := features[85]
	supported := features[86]
	fall := features[87]
	eating := features[88]
	flood := features[89]
	safeMoves := features[90]
	wallAdj := features[91]
	headOnRisk := features[92]
	headOnWin := features[93]
	raceDelta := features[94]
	blockedAdj := features[95]
	return 4*eating + 2.5*flood + 1.5*safeMoves + supported + 0.2*enemyDist + 0.75*headOnWin - 1.2*fall - 0.8*targetDist - 1.5*wallAdj - 1.2*blockedAdj - 2.5*headOnRisk - 0.4*raceDelta
}

func relu(v float32) float32 {
	return float32(math.Max(0, float64(v)))
}
