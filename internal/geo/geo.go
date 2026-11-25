// Package geo provides 2D vector, bounding-box, hit-test, and edge-routing geometry primitives.
package geo

import (
	"math"
)

// Vec is a 2D point or vector in canvas/world coordinates.
type Vec struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// V constructs a Vec.
func V(x, y float64) Vec { return Vec{X: x, Y: y} }

func (a Vec) Add(b Vec) Vec       { return Vec{a.X + b.X, a.Y + b.Y} }
func (a Vec) Sub(b Vec) Vec       { return Vec{a.X - b.X, a.Y - b.Y} }
func (a Vec) Scale(s float64) Vec { return Vec{a.X * s, a.Y * s} }
func (a Vec) Dot(b Vec) float64   { return a.X*b.X + a.Y*b.Y }
func (a Vec) Len() float64        { return math.Hypot(a.X, a.Y) }

func (a Vec) Norm() Vec {
	l := a.Len()
	if l == 0 {
		return Vec{}
	}
	return Vec{a.X / l, a.Y / l}
}

func (a Vec) Dist(b Vec) float64 { return a.Sub(b).Len() }

// Box is an axis-aligned rectangle in canvas coordinates.
type Box struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	W float64 `json:"w"`
	H float64 `json:"h"`
}

func B(x, y, w, h float64) Box { return Box{X: x, Y: y, W: w, H: h} }

func (b Box) MaxX() float64 { return b.X + b.W }
func (b Box) MaxY() float64 { return b.Y + b.H }
func (b Box) Center() Vec   { return Vec{b.X + b.W/2, b.Y + b.H/2} }

func (b Box) Expand(pad float64) Box {
	return Box{X: b.X - pad, Y: b.Y - pad, W: b.W + pad*2, H: b.H + pad*2}
}

func (b Box) Contains(p Vec) bool {
	return p.X >= b.X && p.X <= b.MaxX() && p.Y >= b.Y && p.Y <= b.MaxY()
}

func (b Box) Overlaps(o Box) bool {
	return b.X < o.MaxX() && b.MaxX() > o.X && b.Y < o.MaxY() && b.MaxY() > o.Y
}

// Union returns the smallest box enclosing both b and o.
func (b Box) Union(o Box) Box {
	if b.W <= 0 && b.H <= 0 {
		return o
	}
	if o.W <= 0 && o.H <= 0 {
		return b
	}
	minX := math.Min(b.X, o.X)
	minY := math.Min(b.Y, o.Y)
	maxX := math.Max(b.MaxX(), o.MaxX())
	maxY := math.Max(b.MaxY(), o.MaxY())
	return Box{X: minX, Y: minY, W: maxX - minX, H: maxY - minY}
}

// Normalized returns a box with non-negative W and H.
func (b Box) Normalized() Box {
	if b.W < 0 {
		b.X += b.W
		b.W = -b.W
	}
	if b.H < 0 {
		b.Y += b.H
		b.H = -b.H
	}
	return b
}

// AnchorSide names a side port on a node rectangle.
type AnchorSide string

const (
	AnchorAuto   AnchorSide = "auto"
	AnchorTop    AnchorSide = "top"
	AnchorRight  AnchorSide = "right"
	AnchorBottom AnchorSide = "bottom"
	AnchorLeft   AnchorSide = "left"
)

// AnchorPoint returns the connection point and outward unit normal on box b for side.
// When side is AnchorAuto, it selects the side facing toward targetCenter.
func AnchorPoint(b Box, side AnchorSide, targetCenter Vec) (pt Vec, dir Vec) {
	if side == "" || side == AnchorAuto {
		c := b.Center()
		dx := targetCenter.X - c.X
		dy := targetCenter.Y - c.Y
		w := math.Max(b.W, 1)
		h := math.Max(b.H, 1)
		// Normalize by aspect ratio so rectangular nodes pick the true intersecting edge.
		if math.Abs(dx)/w >= math.Abs(dy)/h {
			if dx >= 0 {
				side = AnchorRight
			} else {
				side = AnchorLeft
			}
		} else {
			if dy >= 0 {
				side = AnchorBottom
			} else {
				side = AnchorTop
			}
		}
	}
	switch side {
	case AnchorTop:
		return Vec{b.X + b.W/2, b.Y}, Vec{0, -1}
	case AnchorRight:
		return Vec{b.MaxX(), b.Y + b.H/2}, Vec{1, 0}
	case AnchorBottom:
		return Vec{b.X + b.W/2, b.MaxY()}, Vec{0, 1}
	case AnchorLeft:
		return Vec{b.X, b.Y + b.H/2}, Vec{-1, 0}
	default:
		return b.Center(), Vec{1, 0}
	}
}

// Snap rounds v to the nearest grid step when step > 0.
func Snap(v, step float64) float64 {
	if step <= 0 {
		return v
	}
	return math.Round(v/step) * step
}

// SnapVec snaps both coordinates of p to step.
func SnapVec(p Vec, step float64) Vec {
	return Vec{Snap(p.X, step), Snap(p.Y, step)}
}

// OrthogonalRoute computes a clean draw.io-style orthogonal polyline from srcBox to dstBox.
// It offsets endpoints outside the boxes by stub distance and avoids collinear overlaps.
func OrthogonalRoute(srcBox, dstBox Box, srcSide, dstSide AnchorSide, stub float64) []Vec {
	if stub <= 0 {
		stub = 24
	}
	p0, d0 := AnchorPoint(srcBox, srcSide, dstBox.Center())
	p1, d1 := AnchorPoint(dstBox, dstSide, srcBox.Center())

	s0 := p0.Add(d0.Scale(stub))
	s1 := p1.Add(d1.Scale(stub))

	pts := []Vec{p0}

	sameHoriz := math.Abs(d0.X) > 0.5 && math.Abs(d1.X) > 0.5
	sameVert := math.Abs(d0.Y) > 0.5 && math.Abs(d1.Y) > 0.5

	switch {
	case sameHoriz && d0.X*d1.X < 0 && (d0.X*(p1.X-p0.X) > 0):
		// Facing horizontally toward each other: Z-bend at midpoint X.
		midX := (s0.X + s1.X) / 2
		pts = append(pts, Vec{midX, p0.Y}, Vec{midX, p1.Y})
	case sameVert && d0.Y*d1.Y < 0 && (d0.Y*(p1.Y-p0.Y) > 0):
		// Facing vertically toward each other: Z-bend at midpoint Y.
		midY := (s0.Y + s1.Y) / 2
		pts = append(pts, Vec{p0.X, midY}, Vec{p1.X, midY})
	case math.Abs(d0.X) > 0.5 && math.Abs(d1.Y) > 0.5:
		// Horizontal exit, vertical entry: single L-corner if cleanly aligned, else dogleg.
		if d0.X*(s1.X-p0.X) > 0 && d1.Y*(p0.Y-s1.Y) > 0 {
			pts = append(pts, Vec{p1.X, p0.Y})
		} else {
			pts = append(pts, s0, Vec{s0.X, s1.Y}, s1)
		}
	case math.Abs(d0.Y) > 0.5 && math.Abs(d1.X) > 0.5:
		// Vertical exit, horizontal entry.
		if d0.Y*(s1.Y-p0.Y) > 0 && d1.X*(p0.X-s1.X) > 0 {
			pts = append(pts, Vec{p0.X, p1.Y})
		} else {
			pts = append(pts, s0, Vec{s1.X, s0.Y}, s1)
		}
	default:
		// U-turn or back-to-back ports: route cleanly outside both boxes.
		if math.Abs(d0.X) > 0.5 {
			bypassY := math.Min(srcBox.Y, dstBox.Y) - stub
			if p0.Y > srcBox.Center().Y && p1.Y > dstBox.Center().Y {
				bypassY = math.Max(srcBox.MaxY(), dstBox.MaxY()) + stub
			}
			pts = append(pts, s0, Vec{s0.X, bypassY}, Vec{s1.X, bypassY}, s1)
		} else {
			bypassX := math.Min(srcBox.X, dstBox.X) - stub
			if p0.X > srcBox.Center().X && p1.X > dstBox.Center().X {
				bypassX = math.Max(srcBox.MaxX(), dstBox.MaxX()) + stub
			}
			pts = append(pts, s0, Vec{bypassX, s0.Y}, Vec{bypassX, s1.Y}, s1)
		}
	}

	pts = append(pts, p1)
	return SimplifyOrthogonal(pts)
}

// SimplifyOrthogonal removes consecutive duplicate and redundant collinear vertices.
func SimplifyOrthogonal(pts []Vec) []Vec {
	if len(pts) <= 2 {
		return pts
	}
	dedup := make([]Vec, 0, len(pts))
	for _, p := range pts {
		if len(dedup) == 0 || dedup[len(dedup)-1].Dist(p) > 0.5 {
			dedup = append(dedup, p)
		}
	}
	if len(dedup) <= 2 {
		return dedup
	}
	out := []Vec{dedup[0]}
	for i := 1; i < len(dedup)-1; i++ {
		a, b, c := out[len(out)-1], dedup[i], dedup[i+1]
		colX := math.Abs(a.X-b.X) < 0.5 && math.Abs(b.X-c.X) < 0.5
		colY := math.Abs(a.Y-b.Y) < 0.5 && math.Abs(b.Y-c.Y) < 0.5
		if colX || colY {
			continue
		}
		out = append(out, b)
	}
	out = append(out, dedup[len(dedup)-1])
	return out
}

// DistToSegment returns the Euclidean distance from point p to segment ab.
func DistToSegment(p, a, b Vec) float64 {
	ab := b.Sub(a)
	l2 := ab.Dot(ab)
	if l2 == 0 {
		return p.Dist(a)
	}
	t := p.Sub(a).Dot(ab) / l2
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}
	proj := a.Add(ab.Scale(t))
	return p.Dist(proj)
}

// DistToPolyline returns the minimum distance from point p to any segment in pts.
func DistToPolyline(p Vec, pts []Vec) float64 {
	if len(pts) == 0 {
		return math.Inf(1)
	}
	if len(pts) == 1 {
		return p.Dist(pts[0])
	}
	best := math.Inf(1)
	for i := 0; i < len(pts)-1; i++ {
		if d := DistToSegment(p, pts[i], pts[i+1]); d < best {
			best = d
		}
	}
	return best
}

// PolylineMidpoint returns the point halfway along the total arc length of pts.
func PolylineMidpoint(pts []Vec) Vec {
	if len(pts) == 0 {
		return Vec{}
	}
	if len(pts) == 1 {
		return pts[0]
	}
	total := 0.0
	for i := 0; i < len(pts)-1; i++ {
		total += pts[i].Dist(pts[i+1])
	}
	if total == 0 {
		return pts[0]
	}
	half := total / 2
	acc := 0.0
	for i := 0; i < len(pts)-1; i++ {
		seg := pts[i].Dist(pts[i+1])
		if acc+seg >= half && seg > 0 {
			t := (half - acc) / seg
			return pts[i].Add(pts[i+1].Sub(pts[i]).Scale(t))
		}
		acc += seg
	}
	return pts[len(pts)-1]
}
