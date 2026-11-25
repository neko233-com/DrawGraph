package layout

import (
	"math"
	"sort"

	"github.com/neko233-com/DrawGraph/internal/geo"
	"github.com/neko233-com/DrawGraph/internal/graph"
	"github.com/neko233-com/DrawGraph/internal/habits"
)

func init() {
	RegisterStrategy("grid", formatGrid)
	RegisterStrategy("tree", formatTree)
	RegisterStrategy("radial", formatRadial)
	RegisterStrategy("swimlane", formatSwimlane)
	RegisterStrategy("compact", formatCompact)
}

func prepareNodesAndAdjacency(page *graph.Page, opts Options, rep *Report) ([]*graph.Node, map[string]*graph.Node, map[string][]string, map[string]int, habits.Preset) {
	opts = normalizeOptions(nil, opts)
	preset := habits.PresetByID(opts.Preset)
	if opts.HarmonizeStyles {
		rep.StyledNodes += harmonizeNodes(page, preset, opts.Dark)
	}
	if opts.NormalizeSizes {
		fitNodeSizes(page, opts.GridSize)
	}
	nodeMap := make(map[string]*graph.Node, len(page.Nodes))
	var nodes []*graph.Node
	for _, n := range page.Nodes {
		nodeMap[n.ID] = n
		if n.Kind == graph.KindContainer || n.Kind == graph.KindSwimlane {
			continue
		}
		nodes = append(nodes, n)
	}
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].ParentID != nodes[j].ParentID {
			return nodes[i].ParentID < nodes[j].ParentID
		}
		return nodes[i].ID < nodes[j].ID
	})
	adj := make(map[string][]string, len(nodes))
	inDeg := make(map[string]int, len(nodes))
	for _, n := range nodes {
		inDeg[n.ID] = 0
	}
	for _, e := range page.Edges {
		if e.SourceID == e.TargetID {
			continue
		}
		s, okS := nodeMap[e.SourceID]
		t, okT := nodeMap[e.TargetID]
		if !okS || !okT {
			continue
		}
		if s.Kind == graph.KindContainer || s.Kind == graph.KindSwimlane ||
			t.Kind == graph.KindContainer || t.Kind == graph.KindSwimlane {
			continue
		}
		adj[e.SourceID] = append(adj[e.SourceID], e.TargetID)
		inDeg[e.TargetID]++
	}
	return nodes, nodeMap, adj, inDeg, preset
}

// formatGrid arranges nodes into a crisp balanced matrix/grid (ideal for media walls, icon packs, service grids).
func formatGrid(doc *graph.Document, page *graph.Page, opts Options) Report {
	opts = normalizeOptions(doc, opts)
	rep := Report{Direction: opts.Direction, Preset: opts.Preset, Strategy: "grid", Layers: 1}
	nodes, _, _, _, preset := prepareNodesAndAdjacency(page, opts, &rep)
	if len(nodes) == 0 {
		return rep
	}

	cols := int(math.Ceil(math.Sqrt(float64(len(nodes)))))
	if cols < 1 {
		cols = 1
	}
	rows := (len(nodes) + cols - 1) / cols
	colW := make([]float64, cols)
	rowH := make([]float64, rows)
	for i, n := range nodes {
		r, c := i/cols, i%cols
		colW[c] = math.Max(colW[c], n.W)
		rowH[r] = math.Max(rowH[r], n.H)
	}

	colX := make([]float64, cols)
	curX := opts.StartX
	for c := 0; c < cols; c++ {
		colX[c] = curX
		curX += colW[c] + opts.GapX*0.75
	}
	rowY := make([]float64, rows)
	curY := opts.StartY
	for r := 0; r < rows; r++ {
		rowY[r] = curY
		curY += rowH[r] + opts.GapY*0.75
	}
	rep.Layers = rows

	for i, n := range nodes {
		if n.Locked {
			continue
		}
		r, c := i/cols, i%cols
		tx := geo.Snap(colX[c]+(colW[c]-n.W)/2, opts.GridSize)
		ty := geo.Snap(rowY[r]+(rowH[r]-n.H)/2, opts.GridSize)
		if n.X != tx || n.Y != ty {
			n.X, n.Y = tx, ty
			rep.MovedNodes++
		}
	}
	finishLayout(doc, page, opts, preset, &rep)
	return rep
}

// formatTree computes symmetrical subtree-width spans so org charts, decision trees, and mindmaps never overlap.
func formatTree(doc *graph.Document, page *graph.Page, opts Options) Report {
	opts = normalizeOptions(doc, opts)
	rep := Report{Direction: opts.Direction, Preset: opts.Preset, Strategy: "tree"}
	nodes, nodeMap, adj, inDeg, preset := prepareNodesAndAdjacency(page, opts, &rep)
	if len(nodes) == 0 {
		return rep
	}

	horiz := isHorizontalDir(opts.Direction)
	children := make(map[string][]*graph.Node, len(nodes))
	depthOf := make(map[string]int, len(nodes))
	visited := make(map[string]bool, len(nodes))
	var roots []*graph.Node

	var rootsCand []*graph.Node
	for _, n := range nodes {
		if inDeg[n.ID] == 0 {
			rootsCand = append(rootsCand, n)
		}
	}
	for _, n := range nodes {
		rootsCand = append(rootsCand, n)
	}

	var queue []*graph.Node
	for _, seed := range rootsCand {
		if visited[seed.ID] {
			continue
		}
		visited[seed.ID] = true
		depthOf[seed.ID] = 0
		roots = append(roots, seed)
		queue = append(queue[:0], seed)
		for len(queue) > 0 {
			cur := queue[0]
			queue = queue[1:]
			for _, cid := range adj[cur.ID] {
				if visited[cid] {
					continue
				}
				cn := nodeMap[cid]
				if cn == nil {
					continue
				}
				visited[cid] = true
				depthOf[cid] = depthOf[cur.ID] + 1
				children[cur.ID] = append(children[cur.ID], cn)
				queue = append(queue, cn)
			}
		}
	}

	maxDepth := 0
	depthMaxSize := map[int]float64{}
	for _, n := range nodes {
		d := depthOf[n.ID]
		if d > maxDepth {
			maxDepth = d
		}
		mainSz := n.H
		if horiz {
			mainSz = n.W
		}
		depthMaxSize[d] = math.Max(depthMaxSize[d], mainSz)
	}
	rep.Layers = maxDepth + 1

	depthPos := make([]float64, maxDepth+1)
	curMain := opts.StartX
	gapMain := opts.GapX
	gapCross := opts.GapY
	if !horiz {
		curMain = opts.StartY
		gapMain = opts.GapY
		gapCross = opts.GapX
	}
	for d := 0; d <= maxDepth; d++ {
		depthPos[d] = curMain
		curMain += depthMaxSize[d] + gapMain
	}

	subtreeSpan := make(map[string]float64, len(nodes))
	var measureSubtree func(n *graph.Node) float64
	measureSubtree = func(n *graph.Node) float64 {
		selfCross := n.W
		if horiz {
			selfCross = n.H
		}
		ch := children[n.ID]
		if len(ch) == 0 {
			subtreeSpan[n.ID] = selfCross
			return selfCross
		}
		sum := 0.0
		for i, c := range ch {
			if i > 0 {
				sum += gapCross
			}
			sum += measureSubtree(c)
		}
		span := math.Max(selfCross, sum)
		subtreeSpan[n.ID] = span
		return span
	}
	for _, r := range roots {
		measureSubtree(r)
	}

	var assignSubtree func(n *graph.Node, crossStart float64)
	assignSubtree = func(n *graph.Node, crossStart float64) {
		d := depthOf[n.ID]
		span := subtreeSpan[n.ID]
		if !n.Locked {
			var tx, ty float64
			if horiz {
				tx = geo.Snap(depthPos[d]+(depthMaxSize[d]-n.W)/2, opts.GridSize)
				ty = geo.Snap(crossStart+(span-n.H)/2, opts.GridSize)
			} else {
				tx = geo.Snap(crossStart+(span-n.W)/2, opts.GridSize)
				ty = geo.Snap(depthPos[d]+(depthMaxSize[d]-n.H)/2, opts.GridSize)
			}
			if n.X != tx || n.Y != ty {
				n.X, n.Y = tx, ty
				rep.MovedNodes++
			}
		}
		ch := children[n.ID]
		if len(ch) == 0 {
			return
		}
		childSum := 0.0
		for i, c := range ch {
			if i > 0 {
				childSum += gapCross
			}
			childSum += subtreeSpan[c.ID]
		}
		cur := crossStart + (span-childSum)/2
		for _, c := range ch {
			assignSubtree(c, cur)
			cur += subtreeSpan[c.ID] + gapCross
		}
	}

	curCross := opts.StartY
	if !horiz {
		curCross = opts.StartX
	}
	for _, r := range roots {
		assignSubtree(r, curCross)
		curCross += subtreeSpan[r.ID] + gapCross*1.4
	}

	if opts.Direction == "RL" || opts.Direction == "BT" {
		mirrorLayoutDirection(nodes, opts)
	}
	finishLayout(doc, page, opts, preset, &rep)
	return rep
}

// formatRadial arranges connected components in concentric rings around the highest-degree hub.
func formatRadial(doc *graph.Document, page *graph.Page, opts Options) Report {
	opts = normalizeOptions(doc, opts)
	rep := Report{Direction: opts.Direction, Preset: opts.Preset, Strategy: "radial"}
	nodes, nodeMap, adj, _, preset := prepareNodesAndAdjacency(page, opts, &rep)
	if len(nodes) == 0 {
		return rep
	}

	undir := make(map[string][]string, len(nodes))
	for _, e := range page.Edges {
		if e.SourceID != e.TargetID && nodeMap[e.SourceID] != nil && nodeMap[e.TargetID] != nil {
			undir[e.SourceID] = append(undir[e.SourceID], e.TargetID)
			undir[e.TargetID] = append(undir[e.TargetID], e.SourceID)
		}
	}
	_ = adj

	hub := nodes[0]
	for _, n := range nodes[1:] {
		if len(undir[n.ID]) > len(undir[hub.ID]) {
			hub = n
		}
	}

	dist := map[string]int{hub.ID: 0}
	queue := []*graph.Node{hub}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, nb := range undir[cur.ID] {
			if _, ok := dist[nb]; ok {
				continue
			}
			nn := nodeMap[nb]
			if nn == nil {
				continue
			}
			dist[nb] = dist[cur.ID] + 1
			queue = append(queue, nn)
		}
	}

	maxRing := 0
	for _, n := range nodes {
		if _, ok := dist[n.ID]; !ok {
			dist[n.ID] = maxRing + 1
		}
		if dist[n.ID] > maxRing {
			maxRing = dist[n.ID]
		}
	}
	rings := make([][]*graph.Node, maxRing+1)
	for _, n := range nodes {
		r := dist[n.ID]
		rings[r] = append(rings[r], n)
	}
	rep.Layers = len(rings)

	ringRadii := make([]float64, len(rings))
	prevR := 0.0
	for r, ring := range rings {
		if r == 0 {
			continue
		}
		circumNeeded := 0.0
		maxDiag := 0.0
		for _, n := range ring {
			d := math.Hypot(n.W, n.H)
			maxDiag = math.Max(maxDiag, d)
			circumNeeded += d + opts.GapX*0.6
		}
		minR := circumNeeded / (2 * math.Pi)
		stepR := prevR + math.Max(220, maxDiag+opts.GapY)
		ringRadii[r] = math.Max(minR, stepR)
		prevR = ringRadii[r]
	}

	centerX := opts.StartX + prevR + 140
	centerY := opts.StartY + prevR + 100
	for r, ring := range rings {
		if r == 0 {
			for i, n := range ring {
				if n.Locked {
					continue
				}
				tx := geo.Snap(centerX-n.W/2+float64(i)*opts.GapX, opts.GridSize)
				ty := geo.Snap(centerY-n.H/2, opts.GridSize)
				if n.X != tx || n.Y != ty {
					n.X, n.Y = tx, ty
					rep.MovedNodes++
				}
			}
			continue
		}
		rad := ringRadii[r]
		step := (2 * math.Pi) / float64(len(ring))
		for i, n := range ring {
			if n.Locked {
				continue
			}
			ang := -math.Pi/2 + float64(i)*step
			cx := centerX + rad*math.Cos(ang)
			cy := centerY + rad*math.Sin(ang)
			tx := geo.Snap(cx-n.W/2, opts.GridSize)
			ty := geo.Snap(cy-n.H/2, opts.GridSize)
			if n.X != tx || n.Y != ty {
				n.X, n.Y = tx, ty
				rep.MovedNodes++
			}
		}
	}

	finishLayout(doc, page, opts, preset, &rep)
	return rep
}

// formatSwimlane packs Swimlane/Container lanes into uniform parallel columns/bands with layered flow inside each lane.
func formatSwimlane(doc *graph.Document, page *graph.Page, opts Options) Report {
	opts = normalizeOptions(doc, opts)
	rep := Report{Direction: opts.Direction, Preset: opts.Preset, Strategy: "swimlane"}
	nodes, nodeMap, adj, inDeg, preset := prepareNodesAndAdjacency(page, opts, &rep)
	if len(nodes) == 0 {
		return rep
	}

	var lanes []*graph.Node
	for _, n := range page.Nodes {
		if n.Kind == graph.KindSwimlane || n.Kind == graph.KindContainer {
			lanes = append(lanes, n)
		}
	}
	if len(lanes) == 0 {
		// Fall back cleanly to layered layout if page has no swimlane/container groups.
		opts.Strategy = "layered"
		return FormatPage(doc, page, opts)
	}
	sort.Slice(lanes, func(i, j int) bool { return lanes[i].ID < lanes[j].ID })

	rank := assignRanks(nodes, adj, inDeg)
	byParent := map[string][]*graph.Node{}
	for _, n := range nodes {
		byParent[n.ParentID] = append(byParent[n.ParentID], n)
	}

	horiz := isHorizontalDir(opts.Direction) // LR => horizontal swimlane rows stacked vertically
	laneOffset := opts.StartY
	if !horiz {
		laneOffset = opts.StartX
	}

	maxMainSpan := 320.0
	for _, lane := range lanes {
		members := byParent[lane.ID]
		if len(members) == 0 {
			continue
		}
		sort.Slice(members, func(i, j int) bool {
			if rank[members[i].ID] != rank[members[j].ID] {
				return rank[members[i].ID] < rank[members[j].ID]
			}
			return members[i].ID < members[j].ID
		})
		curMain := opts.StartX + opts.ContainerPadding
		if !horiz {
			curMain = opts.StartY + 42 + opts.ContainerPadding
		}
		maxCross := 0.0
		for _, m := range members {
			if horiz {
				maxCross = math.Max(maxCross, m.H)
			} else {
				maxCross = math.Max(maxCross, m.W)
			}
		}
		for _, m := range members {
			if m.Locked {
				continue
			}
			if horiz {
				tx := geo.Snap(curMain, opts.GridSize)
				ty := geo.Snap(laneOffset+42+opts.ContainerPadding+(maxCross-m.H)/2, opts.GridSize)
				if m.X != tx || m.Y != ty {
					m.X, m.Y = tx, ty
					rep.MovedNodes++
				}
				curMain += m.W + opts.GapX
			} else {
				tx := geo.Snap(laneOffset+opts.ContainerPadding+(maxCross-m.W)/2, opts.GridSize)
				ty := geo.Snap(curMain, opts.GridSize)
				if m.X != tx || m.Y != ty {
					m.X, m.Y = tx, ty
					rep.MovedNodes++
				}
				curMain += m.H + opts.GapY
			}
		}
		maxMainSpan = math.Max(maxMainSpan, curMain)
		if horiz {
			laneOffset += maxCross + 42 + opts.ContainerPadding*2 + 24
		} else {
			laneOffset += maxCross + opts.ContainerPadding*2 + 24
		}
	}

	// Place any unparented nodes neatly beside/below lanes.
	if loose := byParent[""]; len(loose) > 0 {
		curMain := opts.StartX
		if !horiz {
			curMain = opts.StartY
		}
		for _, m := range loose {
			if m.Locked {
				continue
			}
			if horiz {
				m.X = geo.Snap(curMain, opts.GridSize)
				m.Y = geo.Snap(laneOffset+20, opts.GridSize)
				curMain += m.W + opts.GapX
			} else {
				m.X = geo.Snap(laneOffset+20, opts.GridSize)
				m.Y = geo.Snap(curMain, opts.GridSize)
				curMain += m.H + opts.GapY
			}
			rep.MovedNodes++
		}
	}

	rep.Layers = len(lanes)
	_ = nodeMap
	finishLayout(doc, page, opts, preset, &rep)
	return rep
}

// formatCompact gently eliminates overlapping nodes while preserving hand-drawn relative layout.
func formatCompact(doc *graph.Document, page *graph.Page, opts Options) Report {
	opts = normalizeOptions(doc, opts)
	rep := Report{Direction: opts.Direction, Preset: opts.Preset, Strategy: "compact", Layers: 1}
	nodes, _, _, _, preset := prepareNodesAndAdjacency(page, opts, &rep)
	if len(nodes) == 0 {
		return rep
	}

	moved := make(map[string]bool, len(nodes))
	pad := math.Max(16, opts.GridSize)
	for iter := 0; iter < 40; iter++ {
		any := false
		for i := 0; i < len(nodes); i++ {
			a := nodes[i]
			for j := i + 1; j < len(nodes); j++ {
				b := nodes[j]
				if a.ParentID != b.ParentID {
					continue
				}
				ab := a.Box().Expand(pad / 2)
				bb := b.Box().Expand(pad / 2)
				if !ab.Overlaps(bb) {
					continue
				}
				any = true
				ox := math.Min(ab.MaxX(), bb.MaxX()) - math.Max(ab.X, bb.X)
				oy := math.Min(ab.MaxY(), bb.MaxY()) - math.Max(ab.Y, bb.Y)
				var sx, sy float64
				if ox <= oy {
					sx = ox/2 + opts.GridSize*0.5
					if a.X+a.W/2 <= b.X+b.W/2 {
						sx = -sx
					}
				} else {
					sy = oy/2 + opts.GridSize*0.5
					if a.Y+a.H/2 <= b.Y+b.H/2 {
						sy = -sy
					}
				}
				if !a.Locked {
					a.X = geo.Snap(a.X+sx, opts.GridSize)
					a.Y = geo.Snap(a.Y+sy, opts.GridSize)
					moved[a.ID] = true
				}
				if !b.Locked {
					b.X = geo.Snap(b.X-sx, opts.GridSize)
					b.Y = geo.Snap(b.Y-sy, opts.GridSize)
					moved[b.ID] = true
				}
			}
		}
		if !any {
			break
		}
	}
	rep.MovedNodes = len(moved)
	finishLayout(doc, page, opts, preset, &rep)
	return rep
}
