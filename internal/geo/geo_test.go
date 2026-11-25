package geo_test

import (
	"math"
	"testing"

	"github.com/neko233-com/DrawGraph/internal/geo"
)

func TestAnchorPointAutoSelection(t *testing.T) {
	b := geo.B(100, 100, 120, 60) // center (160, 130)
	pt, dir := geo.AnchorPoint(b, geo.AnchorAuto, geo.V(400, 130))
	if pt != geo.V(220, 130) || dir != geo.V(1, 0) {
		t.Fatalf("right target got pt=%v dir=%v", pt, dir)
	}
	pt, dir = geo.AnchorPoint(b, geo.AnchorAuto, geo.V(160, 300))
	if pt != geo.V(160, 160) || dir != geo.V(0, 1) {
		t.Fatalf("bottom target got pt=%v dir=%v", pt, dir)
	}
}

func TestOrthogonalRouteIsOrthogonalAndSimplified(t *testing.T) {
	src := geo.B(0, 0, 120, 60)
	dst := geo.B(240, 100, 120, 60)
	pts := geo.OrthogonalRoute(src, dst, geo.AnchorRight, geo.AnchorLeft, 24)
	if len(pts) != 4 {
		t.Fatalf("expected 4 Z-bend points, got %d (%v)", len(pts), pts)
	}
	for i := 0; i < len(pts)-1; i++ {
		dx := math.Abs(pts[i+1].X - pts[i].X)
		dy := math.Abs(pts[i+1].Y - pts[i].Y)
		if dx > 0.01 && dy > 0.01 {
			t.Fatalf("segment %d->%d not axis-aligned: %v -> %v", i, i+1, pts[i], pts[i+1])
		}
	}
}

func TestPolylineDistAndMidpoint(t *testing.T) {
	pts := []geo.Vec{geo.V(0, 0), geo.V(100, 0), geo.V(100, 100)}
	mid := geo.PolylineMidpoint(pts)
	if math.Abs(mid.X-100) > 0.1 || math.Abs(mid.Y-0) > 0.1 {
		t.Fatalf("midpoint = %v, want (100,0)", mid)
	}
	if d := geo.DistToPolyline(geo.V(50, 5), pts); math.Abs(d-5) > 0.01 {
		t.Fatalf("dist = %v, want 5", d)
	}
}
