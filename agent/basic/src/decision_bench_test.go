package main

import (
	"bufio"
	"os"
	"strings"
	"testing"

	"codingame/internal/engine"
)

var realStdout = os.Stdout

func TestMain(m *testing.M) {
	debug = false
	os.Exit(m.Run())
}

func suppressStdout() func() {
	devNull, _ := os.Open(os.DevNull)
	os.Stdout = devNull
	return func() {
		devNull.Close()
		os.Stdout = realStdout
	}
}

// testGameWithSeed creates a full game (grid + turn data) for arbitrary seed/league.
func testGameWithSeed(seed int64, league int) (*Game, *Plan, *Decision) {
	eg := engine.NewGame(seed, league)
	p0 := engine.NewPlayer(0)
	p1 := engine.NewPlayer(1)
	eg.Init([]*engine.Player{p0, p1})

	lines := engine.SerializeGlobalInfoFor(p0, eg)
	lines = append(lines, engine.SerializeFrameInfoFor(p0, eg)...)
	s := bufio.NewScanner(strings.NewReader(strings.Join(lines, "\n")))
	g := Init(s)
	g.Read(s)

	p := &Plan{g: g}
	p.Precompute()
	return g, p, &Decision{G: g, P: p}
}

// --- Benchmarks: individual phases ---

func BenchmarkPhaseBFS(b *testing.B) {
	_, _, d := testGameWithSeed(testSeed, 3)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.phaseBFS()
	}
}

func BenchmarkPhaseInfluence(b *testing.B) {
	_, _, d := testGameWithSeed(testSeed, 3)
	d.phaseBFS()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.phaseInfluence()
	}
}

func BenchmarkPhaseScoring(b *testing.B) {
	_, _, d := testGameWithSeed(testSeed, 3)
	d.phaseBFS()
	d.phaseInfluence()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.phaseScoring()
	}
}

func BenchmarkPhaseAssignment(b *testing.B) {
	_, _, d := testGameWithSeed(testSeed, 3)
	d.phaseBFS()
	d.phaseInfluence()
	d.phaseScoring()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.phaseAssignment()
	}
}

func BenchmarkPhaseSafety(b *testing.B) {
	_, _, d := testGameWithSeed(testSeed, 3)
	d.phaseBFS()
	d.phaseInfluence()
	d.phaseScoring()
	d.phaseAssignment()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.phaseSafety()
	}
}

func BenchmarkDecideFull(b *testing.B) {
	_, _, d := testGameWithSeed(testSeed, 3)
	restore := suppressStdout()
	defer restore()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.Decide()
	}
}

// --- Benchmarks: BFS internals ---

func BenchmarkBFSFindAll(b *testing.B) {
	g, p, _ := testGameWithSeed(testSeed, 3)
	body := g.Sn[0].Body
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.BFSFindAll(body)
	}
}

func BenchmarkScoreRollout(b *testing.B) {
	_, _, d := testGameWithSeed(testSeed, 3)
	d.phaseBFS()
	d.phaseInfluence()
	d.phaseScoring()
	d.phaseAssignment()

	// Build safeScore so we can construct candidate dirs.
	numMy := len(d.MySnakes)
	dirs := make([]int, numMy)
	copy(dirs, d.AssignedDir)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.scoreRollout(dirs)
	}
}

// --- Larger map benchmarks (legend league, bigger grid) ---

const largeSeed int64 = 42

func BenchmarkPhaseBFS_Large(b *testing.B) {
	_, _, d := testGameWithSeed(largeSeed, 0)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.phaseBFS()
	}
}

func BenchmarkPhaseSafety_Large(b *testing.B) {
	_, _, d := testGameWithSeed(largeSeed, 0)
	d.phaseBFS()
	d.phaseInfluence()
	d.phaseScoring()
	d.phaseAssignment()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.phaseSafety()
	}
}

func BenchmarkDecideFull_Large(b *testing.B) {
	_, _, d := testGameWithSeed(largeSeed, 0)
	restore := suppressStdout()
	defer restore()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.Decide()
	}
}

func BenchmarkBFSFindAll_Large(b *testing.B) {
	g, p, _ := testGameWithSeed(largeSeed, 0)
	body := g.Sn[0].Body
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.BFSFindAll(body)
	}
}

func BenchmarkScoreRollout_Large(b *testing.B) {
	_, _, d := testGameWithSeed(largeSeed, 0)
	d.phaseBFS()
	d.phaseInfluence()
	d.phaseScoring()
	d.phaseAssignment()

	numMy := len(d.MySnakes)
	dirs := make([]int, numMy)
	copy(dirs, d.AssignedDir)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.scoreRollout(dirs)
	}
}
