package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func hasSurfaceLink(s Surface, to int) bool {
	for _, link := range s.Links {
		if link.To == to {
			return true
		}
	}
	return false
}

func hasReachApple(reach []ReachInfo, apple int) bool {
	for _, ri := range reach {
		if ri.Apple == apple {
			return true
		}
	}
	return false
}

func TestPlanInitAcceptanceSurfaceAndAppleLinks(t *testing.T) {
	g := testGridInput([]string{
		".......",
		".......",
		".......",
		".#.....",
		".......",
		"#######",
	})
	g.Ap = []int{g.Idx(3, 2), g.Idx(2, 4), g.Idx(4, 4)}
	g.ANum = len(g.Ap)

	p := &Plan{G: g}
	p.Init()

	groundSID := g.SurfAt[g.Idx(0, 4)]
	assert.True(t, groundSID >= 0, "solid ground surface at y=4")

	sid31 := g.SurfAt[g.Idx(3, 1)]
	sid23 := g.SurfAt[g.Idx(2, 3)]
	sid43 := g.SurfAt[g.Idx(4, 3)]
	assert.True(t, sid31 >= 0, "apple surface at (3,1)")
	assert.True(t, sid23 >= 0, "apple surface at (2,3)")
	assert.True(t, sid43 >= 0, "apple surface at (4,3)")

	assert.Equal(t, SurfApple, g.Surfs[sid31].Type)
	assert.Equal(t, SurfApple, g.Surfs[sid23].Type)
	assert.Equal(t, SurfApple, g.Surfs[sid43].Type)

	assert.True(t, hasSurfaceLink(g.Surfs[groundSID], sid31), "ground should traverse to apple surface at (3,1)")

	link31, ok := findAppleLink(g.Surfs[groundSID], g.Idx(3, 2))
	assert.True(t, ok, "ground should have apple-eat link to (3,2)")
	assert.Equal(t, 2, link31.Len)

	link23, ok := findAppleLink(g.Surfs[groundSID], g.Idx(2, 4))
	assert.True(t, ok, "ground should have apple-eat link to (2,4)")
	assert.Equal(t, 1, link23.Len)

	linkAppleSurf, ok := findAppleLink(g.Surfs[sid31], g.Idx(3, 2))
	assert.True(t, ok, "apple surface should eat its own apple")
	assert.Equal(t, 1, linkAppleSurf.Len)
}

func TestPlanUpdateAppleSurfacesMarksSurfNoneAndRemovesReach(t *testing.T) {
	g := testGridInput([]string{
		".......",
		".......",
		".......",
		".#.....",
		".......",
		"#######",
	})
	removedApple := g.Idx(3, 2)
	keptApple := g.Idx(2, 4)
	g.Ap = []int{removedApple, keptApple}
	g.ANum = len(g.Ap)

	p := &Plan{G: g}
	p.Init()

	removedSID := g.SurfAt[g.Idx(3, 1)]
	groundSID := g.SurfAt[g.Idx(0, 4)]
	assert.True(t, removedSID >= 0, "removed apple surface exists after init")
	assert.True(t, hasSurfaceLink(g.Surfs[groundSID], removedSID), "ground should link to removable apple surface")

	sn := &Snake{
		ID:    0,
		Owner: 0,
		Alive: true,
		Body:  []int{g.Idx(1, 4), g.Idx(0, 4), g.Idx(-1, 4)},
		Len:   3,
	}
	reachBefore := surfaceReach(g, sn, true)
	assert.True(t, hasReachApple(reachBefore, removedApple), "removed apple should be reachable before update")
	assert.True(t, hasReachApple(reachBefore, keptApple), "kept apple should be reachable before update")

	g.Ap = []int{keptApple}
	g.ANum = len(g.Ap)
	p.UpdateAppleSurfaces()

	assert.Equal(t, SurfNone, g.Surfs[removedSID].Type, "removed apple surface should become SurfNone")

	reachAfter := surfaceReach(g, sn, true)
	assert.False(t, hasReachApple(reachAfter, removedApple), "removed apple should no longer be reachable for eating")
	assert.True(t, hasReachApple(reachAfter, keptApple), "remaining apple should stay reachable")
}

func TestPlanInitStackedAppleSurfaces(t *testing.T) {
	g := testGridInput([]string{
		".......",
		".......",
		".......",
		".......",
		".......",
		"#######",
	})
	g.Ap = []int{g.Idx(3, 3), g.Idx(3, 2)}
	g.ANum = len(g.Ap)

	p := &Plan{G: g}
	p.Init()

	sid32 := g.SurfAt[g.Idx(3, 2)]
	sid31 := g.SurfAt[g.Idx(3, 1)]
	assert.True(t, sid32 >= 0, "apple surface at (3,2)")
	assert.True(t, sid31 >= 0, "apple surface at (3,1)")
	assert.Equal(t, SurfApple, g.Surfs[sid32].Type)
	assert.Equal(t, SurfApple, g.Surfs[sid31].Type)
	assert.NotEqual(t, sid32, sid31, "stacked apples should create separate surfaces")
}
