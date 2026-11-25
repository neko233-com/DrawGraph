// Package layout implements one-click elegant diagram formatting: layered Sugiyama-style
// DAG layout, crossing minimization, container auto-fitting, smart edge routing, grid snapping,
// and user-habit-guided visual harmonization.
package layout

import (
	"math"
	"slices"
	"sort"
	"strings"

	"github.com/neko233-com/DrawGraph/internal/geo"
	"github.com/neko233-com/DrawGraph/internal/graph"
	"github.com/neko233-com/DrawGraph/internal/habits"
)

// Options configures one-click formatting behavior.
type Options struct {
	Direction        string             `json:"direction"` // "LR" or "TB"
	GapX             float64            `json:"gapX"`
	GapY             float64            `json:"gapY"`
	GridSize         float64            `json:"gridSize"`
	ContainerPadding float64            `json:"containerPadding"`
	Routing          graph.RoutingStyle `json:"routing"`
	Preset           string             `json:"preset"`
	Dark             bool               `json:"dark"`
	NormalizeSizes   bool               `json:"normalizeSizes"`
	HarmonizeStyles  bool               `json:"harmonizeStyles"`
	ClearWaypoints   bool               `json:"clearWaypoints"`
	StartX           float64            `json:"startX"`
	StartY           float64            `json:"startY"`
	Strategy         string             `json:"strategy,omitempty"` // "layered" (default) or custom registered strategy
}

// Strategy implements a pluggable diagram formatting/layout algorithm.
type Strategy func(doc *graph.Document, page *graph.Page, opts Options) Report

var customStrategies = map[string]Strategy{}

// RegisterStrategy registers an extensible layout strategy by name (e.g. "grid", "radial").
func RegisterStrategy(name string, fn Strategy) {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" || fn == nil {
		return
	}
	customStrategies[name] = fn
}

// RegisteredStrategies returns built-in and registered layout strategy names.
func RegisteredStrategies() []string {
	out := []string{"layered"}
	for k := range customStrategies {
		if k != "layered" {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

// OptionsFromHabits seeds formatting options from the user's persisted Profile.
func OptionsFromHabits(p *habits.Profile, dark bool) Options {
	if p == nil {
		p = habits.DefaultProfile()
	}
	return Options{
		Direction:        p.PreferredDirection,
		GapX:             p.GapX,
		GapY:             p.GapY,
		GridSize:         p.GridSize,
		ContainerPadding: p.ContainerPadding,
		Routing:          p.DefaultRouting,
		Preset:           p.StylePreset,
		Dark:             dark,
		NormalizeSizes:   p.NormalizeSizes,
		HarmonizeStyles:  p.HarmonizePalette,
		ClearWaypoints:   true,
		StartX:           80,
		StartY:           80,
	}
}

// Report summarizes changes made by FormatPage.
type Report struct {
	MovedNodes        int    `json:"movedNodes"`
	ReroutedEdges     int    `json:"reroutedEdges"`
	ResizedContainers int    `json:"resizedContainers"`
	StyledNodes       int    `json:"styledNodes"`
	Layers            int    `json:"layers"`
	Direction         string `json:"direction"`
	Preset            string `json:"preset"`
}

// FormatPage performs one-click elegant layout, edge routing, container wrapping, and styling.
func FormatPage(doc *graph.Document, page *graph.Page, opts Options) Report {
	if page == nil {
		return Report{}
	}
	if strat := strings.ToLower(strings.TrimSpace(opts.Strategy)); strat != "" && strat != "layered" {
		if fn, ok := customStrategies[strat]; ok {
			return fn(doc, page, opts)
		}
	}
	var rep Report
	if opts.Direction != "LR" && opts.Direction != "TB" {
		opts.Direction = "LR"
	}
	if opts.GapX <= 0 {
		opts.GapX = 76
	}
	if opts.GapY <= 0 {
		opts.GapY = 56
	}
	if opts.GridSize <= 0 {
		if doc != nil && doc.GridSize > 0 {
			opts.GridSize = doc.GridSize
		} else {
			opts.GridSize = 20
		}
	}
	if opts.ContainerPadding <= 0 {
		opts.ContainerPadding = 36
	}
	if opts.StartX == 0 {
		opts.StartX = 80
	}
	if opts.StartY == 0 {
		opts.StartY = 80
	}
	rep.Direction = opts.Direction
	rep.Preset = opts.Preset

	// 1. Harmonize typography, sizes, and palette before computing layer bounds.
	preset := habits.PresetByID(opts.Preset)
	if opts.HarmonizeStyles {
		rep.StyledNodes += harmonizeNodes(page, preset, opts.Dark)
	}
	if opts.NormalizeSizes {
		fitNodeSizes(page, opts.GridSize)
	}

	// 2. Partition layoutable (non-container) nodes.
	var layoutNodes []*graph.Node
	nodeMap := map[string]*graph.Node{}
	for _, n := range page.Nodes {
		nodeMap[n.ID] = n
		if n.Kind == graph.KindContainer || n.Kind == graph.KindSwimlane {
			continue
		}
		layoutNodes = append(layoutNodes, n)
	}
	if len(layoutNodes) == 0 {
		return rep
	}
	sort.Slice(layoutNodes, func(i, j int) bool { return layoutNodes[i].ID < layoutNodes[j].ID })

	// Build adjacency among layoutNodes (ignoring self-loops and missing ends).
	adj := map[string][]string{}
	rev := map[string][]string{}
	inDeg := map[string]int{}
	for _, n := range layoutNodes {
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
		rev[e.TargetID] = append(rev[e.TargetID], e.SourceID)
		inDeg[e.TargetID]++
	}

	// 3. Longest-path layer assignment with cycle resilience.
	rank := assignRanks(layoutNodes, adj, inDeg)
	maxRank := 0
	for _, r := range rank {
		if r > maxRank {
			maxRank = r
		}
	}
	rep.Layers = maxRank + 1

	layers := make([][]*graph.Node, maxRank+1)
	for _, n := range layoutNodes {
		r := rank[n.ID]
		layers[r] = append(layers[r], n)
	}

	// 4. Crossing minimization using 4 forward/backward barycenter sweeps.
	orderIndex := map[string]int{}
	updateOrderIndex := func() {
		for _, layer := range layers {
			for idx, n := range layer {
				orderIndex[n.ID] = idx
			}
		}
	}
	// Initial deterministic sort within each layer by ParentID then ID.
	for r := range layers {
		sort.Slice(layers[r], func(i, j int) bool {
			if layers[r][i].ParentID != layers[r][j].ParentID {
				return layers[r][i].ParentID < layers[r][j].ParentID
			}
			return layers[r][i].ID < layers[r][j].ID
		})
	}
	updateOrderIndex()

	for sweep := 0; sweep < 4; sweep++ {
		for r := 1; r < len(layers); r++ {
			barySortLayer(layers[r], rev, orderIndex)
			updateOrderIndex()
		}
		for r := len(layers) - 2; r >= 0; r-- {
			barySortLayer(layers[r], adj, orderIndex)
			updateOrderIndex()
		}
	}

	// 5. Assign coordinates centered along the cross axis and snapped to GridSize.
	layerDepths := make([]float64, len(layers))
	layerCrossSpans := make([]float64, len(layers))
	maxCrossSpan := 0.0
	for r, layer := range layers {
		var depth, cross float64
		for i, n := range layer {
			if opts.Direction == "LR" {
				depth = math.Max(depth, n.W)
				cross += n.H
				if i > 0 {
					cross += opts.GapY
				}
			} else {
				depth = math.Max(depth, n.H)
				cross += n.W
				if i > 0 {
					cross += opts.GapX
				}
			}
		}
		layerDepths[r] = depth
		layerCrossSpans[r] = cross
		maxCrossSpan = math.Max(maxCrossSpan, cross)
	}

	// Extra gap when adjacent siblings belong to different containers so containers never collide.
	cursorMain := opts.StartX
	if opts.Direction == "TB" {
		cursorMain = opts.StartY
	}
	for r, layer := range layers {
		crossStart := opts.StartY + (maxCrossSpan-layerCrossSpans[r])/2
		if opts.Direction == "TB" {
			crossStart = opts.StartX + (maxCrossSpan-layerCrossSpans[r])/2
		}
		curCross := crossStart
		for i, n := range layer {
			if i > 0 && layer[i-1].ParentID != n.ParentID && (layer[i-1].ParentID != "" || n.ParentID != "") {
				curCross += opts.ContainerPadding * 0.75
			}
			if n.Locked {
				continue
			}
			var targetX, targetY float64
			if opts.Direction == "LR" {
				targetX = geo.Snap(cursorMain+(layerDepths[r]-n.W)/2, opts.GridSize)
				targetY = geo.Snap(curCross, opts.GridSize)
				curCross += n.H + opts.GapY
			} else {
				targetX = geo.Snap(curCross, opts.GridSize)
				targetY = geo.Snap(cursorMain+(layerDepths[r]-n.H)/2, opts.GridSize)
				curCross += n.W + opts.GapX
			}
			if n.X != targetX || n.Y != targetY {
				n.X = targetX
				n.Y = targetY
				rep.MovedNodes++
			}
		}
		if opts.Direction == "LR" {
			cursorMain += layerDepths[r] + opts.GapX
		} else {
			cursorMain += layerDepths[r] + opts.GapY
		}
	}

	// 6. Auto-fit container and swimlane bounds around their children.
	for _, n := range page.Nodes {
		if n.Kind != graph.KindContainer && n.Kind != graph.KindSwimlane {
			continue
		}
		var childBox geo.Box
		hasChild := false
		for _, ch := range page.Nodes {
			if ch.ParentID == n.ID {
				if !hasChild {
					childBox = ch.Box()
					hasChild = true
				} else {
					childBox = childBox.Union(ch.Box())
				}
			}
		}
		if !hasChild {
			continue
		}
		headerPad := 28.0
		if n.Kind == graph.KindSwimlane {
			headerPad = 42.0
		}
		nx := geo.Snap(childBox.X-opts.ContainerPadding, opts.GridSize)
		ny := geo.Snap(childBox.Y-opts.ContainerPadding-headerPad, opts.GridSize)
		nw := geo.Snap(childBox.W+opts.ContainerPadding*2, opts.GridSize)
		nh := geo.Snap(childBox.H+opts.ContainerPadding*2+headerPad, opts.GridSize)
		if n.X != nx || n.Y != ny || n.W != nw || n.H != nh {
			n.X, n.Y, n.W, n.H = nx, ny, nw, nh
			rep.ResizedContainers++
		}
	}

	// 7. Re-route edges cleanly using optimal anchors for Direction.
	for _, e := range page.Edges {
		if opts.Routing != "" {
			e.Routing = opts.Routing
		}
		if opts.ClearWaypoints {
			e.Waypoints = nil
		}
		src := nodeMap[e.SourceID]
		dst := nodeMap[e.TargetID]
		if src == nil || dst == nil {
			continue
		}
		if opts.Direction == "LR" {
			if src.X+src.W <= dst.X+10 {
				e.SourceAnchor = geo.AnchorRight
				e.TargetAnchor = geo.AnchorLeft
			} else if dst.X+dst.W <= src.X+10 {
				e.SourceAnchor = geo.AnchorLeft
				e.TargetAnchor = geo.AnchorRight
			} else {
				e.SourceAnchor = geo.AnchorAuto
				e.TargetAnchor = geo.AnchorAuto
			}
		} else {
			if src.Y+src.H <= dst.Y+10 {
				e.SourceAnchor = geo.AnchorBottom
				e.TargetAnchor = geo.AnchorTop
			} else if dst.Y+dst.H <= src.Y+10 {
				e.SourceAnchor = geo.AnchorTop
				e.TargetAnchor = geo.AnchorBottom
			} else {
				e.SourceAnchor = geo.AnchorAuto
				e.TargetAnchor = geo.AnchorAuto
			}
		}
		if opts.HarmonizeStyles {
			edgeSt := preset.LightEdge
			if opts.Dark {
				edgeSt = preset.DarkEdge
			}
			e.Style.Stroke = edgeSt.Stroke
			e.Style.StrokeWidth = edgeSt.StrokeWidth
			e.Style.FontSize = edgeSt.FontSize
			e.Style.TextColor = edgeSt.TextColor
		}
		rep.ReroutedEdges++
	}

	if doc != nil {
		doc.DefaultRouting = opts.Routing
		doc.StylePreset = opts.Preset
		doc.GridSize = opts.GridSize
		doc.Touch()
	}
	return rep
}

func harmonizeNodes(page *graph.Page, preset habits.Preset, dark bool) int {
	base := preset.LightNode
	if dark {
		base = preset.DarkNode
	}
	changed := 0
	for _, n := range page.Nodes {
		if n.Kind == graph.KindText {
			n.Style.TextColor = base.TextColor
			if n.Style.FontSize <= 0 {
				n.Style.FontSize = base.FontSize
			}
			continue
		}
		if n.Kind == graph.KindImage || n.Kind == graph.KindVideo {
			if n.Style.Stroke == "" {
				n.Style.Stroke = base.Stroke
			}
			n.Style.StrokeWidth = base.StrokeWidth
			if n.Style.Radius == 0 {
				n.Style.Radius = base.Radius
			}
			n.Style.Shadow = base.Shadow
			changed++
			continue
		}
		if n.Kind == graph.KindContainer || n.Kind == graph.KindSwimlane {
			if n.Style.Stroke == "" {
				n.Style.Stroke = base.Stroke
			}
			n.Style.StrokeWidth = base.StrokeWidth
			if n.Style.Radius == 0 {
				n.Style.Radius = base.Radius
			}
			if n.Style.TextColor == "" {
				n.Style.TextColor = base.TextColor
			}
			changed++
			continue
		}
		// Keep explicit semantic fills if already set unless blank/default, while harmonizing border, radius, shadow & text.
		if n.Style.Fill == "" {
			n.Style.Fill = base.Fill
		}
		n.Style.Stroke = base.Stroke
		n.Style.StrokeWidth = base.StrokeWidth
		if n.Kind == graph.KindRounded || n.Kind == graph.KindIcon {
			n.Style.Radius = base.Radius
		}
		n.Style.Shadow = base.Shadow
		n.Style.FontSize = base.FontSize
		n.Style.TextColor = base.TextColor
		changed++
	}
	return changed
}

func fitNodeSizes(page *graph.Page, grid float64) {
	for _, n := range page.Nodes {
		if n.Kind == graph.KindContainer || n.Kind == graph.KindSwimlane || n.Kind == graph.KindText || n.Kind == graph.KindImage || n.Kind == graph.KindVideo || n.Locked {
			continue
		}
		lines := strings.Split(n.Label, "\n")
		maxChars := 0
		for _, line := range lines {
			if l := len([]rune(strings.TrimSpace(line))); l > maxChars {
				maxChars = l
			}
		}
		fontSize := n.Style.FontSize
		if fontSize <= 0 {
			fontSize = 13
		}
		minW := float64(maxChars)*fontSize*0.62 + 40
		if n.Icon.Valid() && n.Kind != graph.KindIcon {
			minW += 28
		}
		if minW < 140 && n.Kind != graph.KindIcon {
			minW = 140
		}
		if n.W < minW {
			n.W = geo.Snap(math.Ceil(minW/grid)*grid, grid)
		}
		minH := float64(max(1, len(lines)))*(fontSize*1.45) + 32
		if n.Icon.Valid() && minH < 64 {
			minH = 64
		}
		if n.H < minH && n.Kind != graph.KindIcon {
			n.H = geo.Snap(math.Ceil(minH/grid)*grid, grid)
		}
		// Snap existing sizes gently to grid.
		n.W = math.Max(grid*2, geo.Snap(n.W, grid))
		n.H = math.Max(grid*2, geo.Snap(n.H, grid))
	}
}

func assignRanks(nodes []*graph.Node, adj map[string][]string, origIn map[string]int) map[string]int {
	in := make(map[string]int, len(origIn))
	for k, v := range origIn {
		in[k] = v
	}
	rank := make(map[string]int, len(nodes))
	var queue []string
	for _, n := range nodes {
		if in[n.ID] == 0 {
			queue = append(queue, n.ID)
		}
	}
	visited := map[string]bool{}
	for len(visited) < len(nodes) {
		if len(queue) == 0 {
			// Cycle detected: pick lowest-in-degree unvisited node deterministically.
			var best string
			bestIn := math.MaxInt
			for _, n := range nodes {
				if visited[n.ID] {
					continue
				}
				if in[n.ID] < bestIn {
					bestIn = in[n.ID]
					best = n.ID
				}
			}
			if best == "" {
				break
			}
			queue = append(queue, best)
		}
		sort.Strings(queue)
		cur := queue[0]
		queue = queue[1:]
		if visited[cur] {
			continue
		}
		visited[cur] = true
		for _, nxt := range adj[cur] {
			if visited[nxt] {
				continue
			}
			if rank[nxt] < rank[cur]+1 {
				rank[nxt] = rank[cur] + 1
			}
			in[nxt]--
			if in[nxt] <= 0 {
				queue = append(queue, nxt)
			}
		}
	}
	return rank
}

func barySortLayer(layer []*graph.Node, neighbors map[string][]string, orderIndex map[string]int) {
	if len(layer) <= 1 {
		return
	}
	scores := make(map[string]float64, len(layer))
	for idx, n := range layer {
		nbs := neighbors[n.ID]
		if len(nbs) == 0 {
			scores[n.ID] = float64(idx)
			continue
		}
		sum := 0.0
		for _, nb := range nbs {
			sum += float64(orderIndex[nb])
		}
		scores[n.ID] = sum / float64(len(nbs))
	}
	slices.SortStableFunc(layer, func(a, b *graph.Node) int {
		sa, sb := scores[a.ID], scores[b.ID]
		if sa < sb {
			return -1
		}
		if sa > sb {
			return 1
		}
		return strings.Compare(a.ID, b.ID)
	})
}
