package main

import "sort"

// Plan holds persistent data across turns.
type Plan struct {
	G          *Game
	PrevAssign [MaxPSn]int // previous turn's assigned apple cell (-1 = none)
}

// Init precomputes the surface graph from the current game state.
// Call once after the first Turn() so apples are available.
func (p *Plan) Init() {
	g := p.G
	if g == nil {
		return
	}

	for i := range p.PrevAssign {
		p.PrevAssign[i] = -1
	}

	if len(g.SurfAt) != g.NCells {
		g.SurfAt = make([]int, g.NCells)
	}
	for i := range g.SurfAt {
		g.SurfAt[i] = -1
	}
	g.Surfs = g.Surfs[:0]

	p.detectSurfaces()
	p.addAppleSurfaces()
	p.buildSurfaceLinks()
	p.buildAppleLinks()
	p.buildClusters()
}

func (p *Plan) detectSurfaces() {
	g := p.G
	for y := 0; y < g.H; y++ {
		inSurf := false
		var cur Surface
		for x := 0; x < g.W; x++ {
			idx := g.Idx(x, y)
			grounded := g.Cell[idx] && (y+1 >= g.H || !g.Cell[idx+g.Stride])
			if grounded {
				if !inSurf {
					cur = Surface{ID: len(g.Surfs), Y: y, Left: x, Right: x, Type: SurfSolid}
					inSurf = true
				} else {
					cur.Right = x
				}
			} else if inSurf {
				p.addSurface(cur)
				inSurf = false
			}
		}
		if inSurf {
			p.addSurface(cur)
		}
	}
}

func (p *Plan) addSurface(s Surface) {
	g := p.G
	s.ID = len(g.Surfs)
	s.Len = s.Right - s.Left + 1
	g.Surfs = append(g.Surfs, s)
	for x := s.Left; x <= s.Right; x++ {
		g.SurfAt[g.Idx(x, s.Y)] = s.ID
	}
}

func (p *Plan) addAppleSurfaces() {
	g := p.G
	for i := 0; i < g.ANum; i++ {
		ax, ay := g.XY(g.Ap[i])
		above := g.Idx(ax, ay-1)
		if ay <= 0 || !g.Cell[above] {
			continue
		}
		if sid := g.SurfAt[above]; sid >= 0 && g.Surfs[sid].Type == SurfSolid {
			continue
		}
		p.addSurface(Surface{Y: ay - 1, Left: ax, Right: ax, Type: SurfApple})
	}
}

const maxLinkDepth = 8      // max BFS steps from surface edge
const maxAppleLinkDepth = 7 // reject apple-link paths with Len >= 8

// surfBFS holds reusable buffers for surface link BFS.
type surfBFS struct {
	visited  []bool
	dist     []int
	parent   []int
	queue    []int
	surfBest map[int]SurfLink
}

func newSurfBFS(ncells int) *surfBFS {
	return &surfBFS{
		visited:  make([]bool, ncells),
		dist:     make([]int, ncells),
		parent:   make([]int, ncells),
		queue:    make([]int, 0, ncells),
		surfBest: make(map[int]SurfLink),
	}
}

// buildLinksFor runs BFS from the edges of surface si and populates its Links.
func (b *surfBFS) buildLinksFor(g *Game, si int) {
	s := &g.Surfs[si]

	edges := [2]int{g.Idx(s.Left, s.Y), -1}
	ne := 1
	if s.Right != s.Left {
		edges[1] = g.Idx(s.Right, s.Y)
		ne = 2
	}

	for k := range b.surfBest {
		delete(b.surfBest, k)
	}

	for ei := 0; ei < ne; ei++ {
		src := edges[ei]
		b.queue = b.queue[:0]

		for x := s.Left; x <= s.Right; x++ {
			b.visited[g.Idx(x, s.Y)] = true
		}
		b.visited[src] = false

		b.queue = append(b.queue, src)
		b.visited[src] = true
		b.dist[src] = 0
		b.parent[src] = -1

		edgeHits := make(map[int]int)
		var surfHits []int

		head := 0
		for head < len(b.queue) {
			cur := b.queue[head]
			head++
			if b.dist[cur] >= maxLinkDepth {
				continue
			}

			for d := 0; d < 4; d++ {
				nb := g.Nbm[cur][d]
				if nb < 0 || b.visited[nb] {
					continue
				}
				b.visited[nb] = true
				b.dist[nb] = b.dist[cur] + 1
				b.parent[nb] = cur

				tid := g.SurfAt[nb]
				if tid >= 0 && tid != si {
					surfHits = append(surfHits, nb)
					if _, already := edgeHits[tid]; !already {
						edgeHits[tid] = nb
					}
					continue
				}
				b.queue = append(b.queue, nb)
			}
		}

		for tid, landing := range edgeHits {
			d := b.dist[landing]
			if prev, ok := b.surfBest[tid]; ok && d >= prev.Len {
				continue
			}
			path := make([]int, d+1)
			p := landing
			for i := d; i >= 0; i-- {
				path[i] = p
				p = b.parent[p]
			}
			b.surfBest[tid] = SurfLink{
				To: tid, Landing: landing, Len: d, Path: path,
			}
		}

		for i := 0; i < len(b.queue); i++ {
			b.visited[b.queue[i]] = false
		}
		for _, cell := range surfHits {
			b.visited[cell] = false
		}
		for x := s.Left; x <= s.Right; x++ {
			b.visited[g.Idx(x, s.Y)] = false
		}
	}

	s.Links = make([]SurfLink, 0, len(b.surfBest))
	for _, link := range b.surfBest {
		s.Links = append(s.Links, link)
	}
	sort.Slice(s.Links, func(i, j int) bool {
		return s.Links[i].Len < s.Links[j].Len
	})
}

func (p *Plan) buildSurfaceLinks() {
	g := p.G
	b := newSurfBFS(g.NCells)
	for si := 0; si < len(g.Surfs); si++ {
		b.buildLinksFor(g, si)
	}
	p.addFallLinks()
}

func upsertSurfaceLink(s *Surface, link SurfLink) {
	for i := range s.Links {
		if s.Links[i].To != link.To {
			continue
		}
		if s.Links[i].Len <= link.Len {
			return
		}
		s.Links[i] = link
		return
	}
	s.Links = append(s.Links, link)
}

// addFallLinks adds one-way "fall" links from solid surface edges.
// From each edge, step one cell off the surface horizontally, then fall
// straight down to the first surface. No depth limit.
func (p *Plan) addFallLinks() {
	g := p.G
	for si := range g.Surfs {
		s := &g.Surfs[si]
		if s.Type == SurfNone {
			continue
		}

		type edgeFall struct {
			edge int
			dx   int
		}
		var falls []edgeFall
		falls = append(falls, edgeFall{g.Idx(s.Left, s.Y), -1})
		if s.Right != s.Left {
			falls = append(falls, edgeFall{g.Idx(s.Right, s.Y), +1})
		} else {
			falls = append(falls, edgeFall{g.Idx(s.Left, s.Y), +1})
		}

		for _, f := range falls {
			ex, ey := g.XY(f.edge)
			offX := ex + f.dx
			offCell := g.Idx(offX, ey)

			if offCell < 0 || !g.Cell[offCell] {
				continue
			}
			if g.SurfAt[offCell] >= 0 {
				continue
			}

			path := []int{f.edge, offCell}
			for y := ey + 1; y < g.H; y++ {
				cell := g.Idx(offX, y)
				if !g.Cell[cell] {
					break
				}
				path = append(path, cell)
				tid := g.SurfAt[cell]
				if tid >= 0 && tid != si {
					upsertSurfaceLink(s, SurfLink{
						To:      tid,
						Landing: cell,
						Len:     len(path) - 1,
						Path:    path,
					})
					break
				}
			}
		}
		sort.Slice(s.Links, func(i, j int) bool {
			return s.Links[i].Len < s.Links[j].Len
		})
	}
}

// appleBFS holds reusable buffers for apple-to-surface BFS.
type appleBFS struct {
	visited []bool
	dist    []int
	parent  []int
	queue   []int
}

func newAppleBFS(ncells int) *appleBFS {
	return &appleBFS{
		visited: make([]bool, ncells),
		dist:    make([]int, ncells),
		parent:  make([]int, ncells),
		queue:   make([]int, 0, ncells),
	}
}

func upsertAppleLink(s *Surface, link AppleLink) {
	for i := range s.Apples {
		if s.Apples[i].Apple != link.Apple {
			continue
		}
		if s.Apples[i].Len <= link.Len {
			return
		}
		s.Apples[i] = link
		return
	}
	s.Apples = append(s.Apples, link)
}

func (b *appleBFS) addAppleLinks(g *Game, apple int) {
	b.queue = b.queue[:0]
	b.visited[apple] = true
	b.dist[apple] = 0
	b.parent[apple] = -1
	b.queue = append(b.queue, apple)

	found := make(map[int]AppleLink)
	head := 0
	for head < len(b.queue) {
		cur := b.queue[head]
		head++
		if b.dist[cur] >= maxAppleLinkDepth {
			continue
		}

		for d := 0; d < 4; d++ {
			nb := g.Nbm[cur][d]
			if nb < 0 || b.visited[nb] {
				continue
			}
			b.visited[nb] = true
			b.dist[nb] = b.dist[cur] + 1
			b.parent[nb] = cur
			b.queue = append(b.queue, nb)

			sid := g.SurfAt[nb]
			if sid < 0 || g.Surfs[sid].Type == SurfNone {
				continue
			}
			if _, ok := found[sid]; ok {
				continue
			}

			dist := b.dist[nb]
			path := make([]int, dist+1)
			p := nb
			for i := 0; i <= dist; i++ {
				path[i] = p
				p = b.parent[p]
			}
			found[sid] = AppleLink{
				Apple: apple,
				Start: nb,
				Len:   dist,
				Path:  path,
			}
		}
	}

	for _, cell := range b.queue {
		b.visited[cell] = false
	}

	if len(found) == 0 {
		return
	}

	for sid, link := range found {
		upsertAppleLink(&g.Surfs[sid], link)
	}
}

func (p *Plan) buildAppleLinks() {
	g := p.G
	for i := range g.Surfs {
		g.Surfs[i].Apples = g.Surfs[i].Apples[:0]
	}

	b := newAppleBFS(g.NCells)
	for i := 0; i < g.ANum; i++ {
		b.addAppleLinks(g, g.Ap[i])
	}

	for i := range g.Surfs {
		s := &g.Surfs[i]
		sort.Slice(s.Apples, func(a, b int) bool {
			if s.Apples[a].Len != s.Apples[b].Len {
				return s.Apples[a].Len < s.Apples[b].Len
			}
			return s.Apples[a].Apple < s.Apples[b].Apple
		})
	}
}

// Turn marks eaten apple surfaces as SurfNone.
func (p *Plan) Turn() {
	g := p.G
	if g == nil {
		return
	}

	appleAt := make([]bool, g.NCells)
	for i := 0; i < g.ANum; i++ {
		appleAt[g.Ap[i]] = true
	}

	for i := range g.Surfs {
		s := &g.Surfs[i]
		if s.Type != SurfApple {
			continue
		}
		appleCell := g.Idx(s.Left, s.Y+1)
		if !appleAt[appleCell] {
			s.Type = SurfNone
		}
	}
}

const clusterMaxDist = 3 // max Manhattan distance to cluster two apples

func (p *Plan) buildClusters() {
	g := p.G
	if g.ANum == 0 {
		g.Clusters = g.Clusters[:0]
		return
	}

	// Union-Find
	parent := make([]int, g.ANum)
	rank := make([]int, g.ANum)
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(x int) int {
		if parent[x] != x {
			parent[x] = find(parent[x])
		}
		return parent[x]
	}
	union := func(a, b int) {
		ra, rb := find(a), find(b)
		if ra == rb {
			return
		}
		if rank[ra] < rank[rb] {
			ra, rb = rb, ra
		}
		parent[rb] = ra
		if rank[ra] == rank[rb] {
			rank[ra]++
		}
	}

	// Union apple pairs within Manhattan distance threshold
	for i := 0; i < g.ANum; i++ {
		for j := i + 1; j < g.ANum; j++ {
			if g.Manhattan(g.Ap[i], g.Ap[j]) <= clusterMaxDist {
				union(i, j)
			}
		}
	}

	// Merge singletons into closest larger cluster
	// Count members per root
	rootSize := make(map[int]int)
	for i := 0; i < g.ANum; i++ {
		rootSize[find(i)]++
	}
	for i := 0; i < g.ANum; i++ {
		if rootSize[find(i)] > 1 {
			continue
		}
		// Singleton: find closest apple in a bigger cluster
		bestJ := -1
		bestDist := 1<<30
		for j := 0; j < g.ANum; j++ {
			if find(j) == find(i) || rootSize[find(j)] <= 1 {
				continue
			}
			d := g.Manhattan(g.Ap[i], g.Ap[j])
			if d < bestDist {
				bestDist = d
				bestJ = j
			}
		}
		if bestJ >= 0 {
			union(i, bestJ)
			// Update root sizes after merge
			newRoot := find(i)
			rootSize[newRoot] = rootSize[find(i)]
			if rootSize[newRoot] == 0 {
				rootSize[newRoot] = 2
			}
		}
	}

	// Assign cluster IDs and build Clusters
	rootToID := make(map[int]int)
	g.Clusters = g.Clusters[:0]
	for i := 0; i < g.ANum; i++ {
		r := find(i)
		if _, ok := rootToID[r]; !ok {
			rootToID[r] = len(g.Clusters)
			g.Clusters = append(g.Clusters, Constellation{ID: len(g.Clusters)})
		}
	}
	for i := 0; i < g.ANum; i++ {
		cid := rootToID[find(i)]
		cl := &g.Clusters[cid]
		cl.Apples = append(cl.Apples, g.Ap[i])
	}
	for i := range g.Clusters {
		g.Clusters[i].Size = len(g.Clusters[i].Apples)
	}

	// Build ClusterAt
	if len(g.ClusterAt) != g.NCells {
		g.ClusterAt = make([]int, g.NCells)
	}
	for i := range g.ClusterAt {
		g.ClusterAt[i] = -1
	}
	for i := 0; i < g.ANum; i++ {
		g.ClusterAt[g.Ap[i]] = rootToID[find(i)]
	}
}

// BuildSurfaceGraph rebuilds the precomputed surface graph using the current apples.
func (g *Game) BuildSurfaceGraph() {
	(&Plan{G: g}).Init()
}

// UpdateAppleSurfaces marks eaten apple surfaces as SurfNone.
func (g *Game) UpdateAppleSurfaces() {
	(&Plan{G: g}).Turn()
}
