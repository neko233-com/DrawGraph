// Package habits persists cross-project user preferences, learned diagramming habits,
// favorite icons, recent files, and style presets so users never start from scratch.
package habits

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/neko233-com/DrawGraph/internal/graph"
)

// Preset defines a coherent light/dark-compatible diagram palette and geometry style.
type Preset struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	Description string             `json:"description"`
	LightNode   graph.NodeStyle    `json:"lightNode"`
	DarkNode    graph.NodeStyle    `json:"darkNode"`
	LightEdge   graph.EdgeStyle    `json:"lightEdge"`
	DarkEdge    graph.EdgeStyle    `json:"darkEdge"`
	Routing     graph.RoutingStyle `json:"routing"`
	AccentStops []string           `json:"accentStops"`
}

// BuiltinPresets returns curated visual presets inspired by Vercel Geist, draw.io, and Lobe UI.
func BuiltinPresets() []Preset {
	return []Preset{
		{
			ID:          "vercel-color",
			Name:        "Vercel Spectrum",
			Description: "Crisp Vercel Geist cards with semantic electric blue, cyan, violet, emerald, and pink accents.",
			Routing:     graph.RouteOrthogonal,
			LightNode: graph.NodeStyle{
				Fill: "#EFF6FF", Stroke: "#0070F3", StrokeWidth: 1.6, Radius: 10,
				FontSize: 13, TextColor: "#09090B", Shadow: true, Opacity: 1,
			},
			DarkNode: graph.NodeStyle{
				Fill: "#0F172A", Stroke: "#3B82F6", StrokeWidth: 1.6, Radius: 10,
				FontSize: 13, TextColor: "#FAFAFA", Shadow: false, Opacity: 1,
			},
			LightEdge:   graph.EdgeStyle{Stroke: "#0070F3", StrokeWidth: 1.75, FontSize: 12, TextColor: "#1E293B", Opacity: 1},
			DarkEdge:    graph.EdgeStyle{Stroke: "#38BDF8", StrokeWidth: 1.75, FontSize: 12, TextColor: "#F4F4F5", Opacity: 1},
			AccentStops: []string{"#EFF6FF", "#ECFEFF", "#F5F3FF", "#ECFDF5", "#FDF2F8", "#FEF3C7"},
		},
		{
			ID:          "lobe-clean",
			Name:        "Lobe Clean",
			Description: "Crisp modern SaaS boxes with subtle borders, 10px radius, and orthogonal connectors.",
			Routing:     graph.RouteOrthogonal,
			LightNode: graph.NodeStyle{
				Fill: "#FFFFFF", Stroke: "#CBD5E1", StrokeWidth: 1.5, Radius: 10,
				FontSize: 13, TextColor: "#0F172A", Shadow: true, Opacity: 1,
			},
			DarkNode: graph.NodeStyle{
				Fill: "#1E293B", Stroke: "#334155", StrokeWidth: 1.5, Radius: 10,
				FontSize: 13, TextColor: "#F8FAFC", Shadow: false, Opacity: 1,
			},
			LightEdge:   graph.EdgeStyle{Stroke: "#64748B", StrokeWidth: 1.75, FontSize: 12, TextColor: "#334155", Opacity: 1},
			DarkEdge:    graph.EdgeStyle{Stroke: "#94A3B8", StrokeWidth: 1.75, FontSize: 12, TextColor: "#E2E8F0", Opacity: 1},
			AccentStops: []string{"#EFF6FF", "#ECFDF5", "#FEF3C7", "#F5F3FF", "#FFF1F2"},
		},
		{
			ID:          "drawio-classic",
			Name:        "draw.io Classic",
			Description: "Familiar high-contrast diagram.net palette with crisp outlines and bold routing.",
			Routing:     graph.RouteOrthogonal,
			LightNode: graph.NodeStyle{
				Fill: "#DAE8FC", Stroke: "#6C8EBF", StrokeWidth: 1.5, Radius: 6,
				FontSize: 13, TextColor: "#1E293B", Shadow: false, Opacity: 1,
			},
			DarkNode: graph.NodeStyle{
				Fill: "#1E3A5F", Stroke: "#6C8EBF", StrokeWidth: 1.5, Radius: 6,
				FontSize: 13, TextColor: "#F1F5F9", Shadow: false, Opacity: 1,
			},
			LightEdge:   graph.EdgeStyle{Stroke: "#334155", StrokeWidth: 1.5, FontSize: 12, TextColor: "#0F172A", Opacity: 1},
			DarkEdge:    graph.EdgeStyle{Stroke: "#CBD5E1", StrokeWidth: 1.5, FontSize: 12, TextColor: "#F8FAFC", Opacity: 1},
			AccentStops: []string{"#DAE8FC", "#D5E8D4", "#FFE6CC", "#E1D5E7", "#F8CECC"},
		},
		{
			ID:          "blueprint",
			Name:        "Architecture Blueprint",
			Description: "Technical indigo-accented architecture cards with generous spacing.",
			Routing:     graph.RouteOrthogonal,
			LightNode: graph.NodeStyle{
				Fill: "#F8FAFF", Stroke: "#3B82F6", StrokeWidth: 1.75, Radius: 8,
				FontSize: 13, TextColor: "#1E3A8A", Shadow: true, Opacity: 1,
			},
			DarkNode: graph.NodeStyle{
				Fill: "#0F172A", Stroke: "#3B82F6", StrokeWidth: 1.75, Radius: 8,
				FontSize: 13, TextColor: "#DBEAFE", Shadow: false, Opacity: 1,
			},
			LightEdge:   graph.EdgeStyle{Stroke: "#2563EB", StrokeWidth: 1.75, FontSize: 12, TextColor: "#1E40AF", Opacity: 1},
			DarkEdge:    graph.EdgeStyle{Stroke: "#60A5FA", StrokeWidth: 1.75, FontSize: 12, TextColor: "#BFDBFE", Opacity: 1},
			AccentStops: []string{"#EFF6FF", "#DBEAFE", "#E0E7FF", "#F0F9FF"},
		},
		{
			ID:          "slate-mono",
			Name:        "Minimal Monochrome",
			Description: "Distraction-free grayscale cards ideal for documentation and dark mode.",
			Routing:     graph.RouteCurved,
			LightNode: graph.NodeStyle{
				Fill: "#FAFAFA", Stroke: "#18181B", StrokeWidth: 1.5, Radius: 8,
				FontSize: 13, TextColor: "#18181B", Shadow: false, Opacity: 1,
			},
			DarkNode: graph.NodeStyle{
				Fill: "#18181B", Stroke: "#A1A1AA", StrokeWidth: 1.5, Radius: 8,
				FontSize: 13, TextColor: "#FAFAFA", Shadow: false, Opacity: 1,
			},
			LightEdge:   graph.EdgeStyle{Stroke: "#3F3F46", StrokeWidth: 1.6, FontSize: 12, TextColor: "#27272A", Opacity: 1},
			DarkEdge:    graph.EdgeStyle{Stroke: "#D4D4D8", StrokeWidth: 1.6, FontSize: 12, TextColor: "#F4F4F5", Opacity: 1},
			AccentStops: []string{"#FAFAFA", "#F4F4F5", "#E4E4E7"},
		},
		{
			ID:          "vibrant-tech",
			Name:        "Vibrant Tech",
			Description: "Crisp high-clarity cards with semantic indigo strokes and generous radii.",
			Routing:     graph.RouteOrthogonal,
			LightNode: graph.NodeStyle{
				Fill: "#F8FAFF", Stroke: "#6366F1", StrokeWidth: 1.6, Radius: 12,
				FontSize: 13, TextColor: "#1E1B4B", Shadow: true, Opacity: 1,
			},
			DarkNode: graph.NodeStyle{
				Fill: "#1E1B4B", Stroke: "#818CF8", StrokeWidth: 1.6, Radius: 12,
				FontSize: 13, TextColor: "#EEF2FF", Shadow: false, Opacity: 1,
			},
			LightEdge:   graph.EdgeStyle{Stroke: "#4F46E5", StrokeWidth: 1.75, FontSize: 12, TextColor: "#312E81", Opacity: 1},
			DarkEdge:    graph.EdgeStyle{Stroke: "#A5B4FC", StrokeWidth: 1.75, FontSize: 12, TextColor: "#E0E7FF", Opacity: 1},
			AccentStops: []string{"#EEF2FF", "#ECFEFF", "#ECFDF5", "#FEF3C7", "#FCE7F3"},
		},
		{
			ID:          "warm-paper",
			Name:        "Warm Editorial",
			Description: "High-legibility warm paper tones crafted for product specs and design docs.",
			Routing:     graph.RouteCurved,
			LightNode: graph.NodeStyle{
				Fill: "#FFFDF7", Stroke: "#D97706", StrokeWidth: 1.5, Radius: 10,
				FontSize: 13, TextColor: "#292524", Shadow: true, Opacity: 1,
			},
			DarkNode: graph.NodeStyle{
				Fill: "#292524", Stroke: "#F59E0B", StrokeWidth: 1.5, Radius: 10,
				FontSize: 13, TextColor: "#FAF7F2", Shadow: false, Opacity: 1,
			},
			LightEdge:   graph.EdgeStyle{Stroke: "#78716C", StrokeWidth: 1.75, FontSize: 12, TextColor: "#44403C", Opacity: 1},
			DarkEdge:    graph.EdgeStyle{Stroke: "#D6D3D1", StrokeWidth: 1.75, FontSize: 12, TextColor: "#F5F5F4", Opacity: 1},
			AccentStops: []string{"#FEF3C7", "#FFEDD5", "#DCFCE7", "#E0F2FE"},
		},
	}
}

// PresetByID finds a preset by id, falling back to lobe-clean.
func PresetByID(id string) Preset {
	for _, p := range BuiltinPresets() {
		if p.ID == id {
			return p
		}
	}
	return BuiltinPresets()[0]
}

// Profile holds all persisted user habits across projects.
type Profile struct {
	Version int `json:"version"`
	// AutoRemember updates defaults automatically as the user styles/formats diagrams.
	AutoRemember bool `json:"autoRemember"`

	// Visual & Style Habits
	ThemeMode          string             `json:"themeMode"`   // "auto", "light", "dark"
	StylePreset        string             `json:"stylePreset"` // e.g. "lobe-clean", "drawio-classic"
	DefaultNodeKind    graph.NodeKind     `json:"defaultNodeKind"`
	DefaultFill        string             `json:"defaultFill"`
	DefaultStroke      string             `json:"defaultStroke"`
	DefaultStrokeWidth float64            `json:"defaultStrokeWidth"`
	DefaultRadius      float64            `json:"defaultRadius"`
	DefaultFontSize    float64            `json:"defaultFontSize"`
	DefaultTextColor   string             `json:"defaultTextColor"`
	DefaultShadow      bool               `json:"defaultShadow"`
	DefaultRouting     graph.RoutingStyle `json:"defaultRouting"`
	DefaultArrow       graph.ArrowStyle   `json:"defaultArrow"`
	DefaultEdgeWidth   float64            `json:"defaultEdgeWidth"`

	// Layout & One-Click Format Habits
	PreferredDirection string  `json:"preferredDirection"` // "LR" or "TB"
	GapX               float64 `json:"gapX"`
	GapY               float64 `json:"gapY"`
	GridSize           float64 `json:"gridSize"`
	SnapToGrid         bool    `json:"snapToGrid"`
	ShowGrid           bool    `json:"showGrid"`
	NormalizeSizes     bool    `json:"normalizeSizes"`
	HarmonizePalette   bool    `json:"harmonizePalette"`
	ContainerPadding   float64 `json:"containerPadding"`
	DefaultMediaMax    float64 `json:"defaultMediaMax"` // Reasonable max canvas display side (DIPs) for imported HD images/videos

	// Workspace & Asset Habits
	SidebarOpen      bool           `json:"sidebarOpen"`
	InspectorOpen    bool           `json:"inspectorOpen"`
	MinimapOpen      bool           `json:"minimapOpen"`
	PinnedIconPacks  []string       `json:"pinnedIconPacks"`
	FavoriteIcons    []string       `json:"favoriteIcons"`
	IconUsage        map[string]int `json:"iconUsage,omitempty"`
	ShapeUsage       map[string]int `json:"shapeUsage,omitempty"`
	RecentFiles      []string       `json:"recentFiles,omitempty"`
	LastExportFormat string         `json:"lastExportFormat"`
	LastExportScale  float64        `json:"lastExportScale"`

	// Update Habits (Manual + Automatic Updates)
	AutoCheckUpdates    bool   `json:"autoCheckUpdates"`
	AutoDownloadUpdates bool   `json:"autoDownloadUpdates"`
	UpdateChannel       string `json:"updateChannel"`    // "stable" or "prerelease"
	UpdateCheckHours    int    `json:"updateCheckHours"` // default 24
	LastUpdateCheck     int64  `json:"lastUpdateCheck,omitempty"`
	SkippedVersion      string `json:"skippedVersion,omitempty"`

	// Localization & Interaction Habits
	Language        string `json:"language"`        // "auto", "en", "zh-CN", "zh-TW", "ja", "ko"
	DefaultStrategy string `json:"defaultStrategy"` // "layered", "tree", "grid", "radial", "swimlane", "compact"
	WheelMode       string `json:"wheelMode"`       // "pan" or "zoom"
	ShowPerfHUD     bool   `json:"showPerfHud"`

	UpdatedAt time.Time `json:"updatedAt"`
}

// DefaultProfile returns sensible starter habits.
func DefaultProfile() *Profile {
	return &Profile{
		Version:             1,
		AutoRemember:        true,
		Language:            "auto",
		ThemeMode:           "auto",
		StylePreset:         "vercel-color",
		DefaultNodeKind:     graph.KindRounded,
		DefaultFill:         "#EFF6FF",
		DefaultStroke:       "#0070F3",
		DefaultStrokeWidth:  1.6,
		DefaultRadius:       10,
		DefaultFontSize:     13,
		DefaultTextColor:    "#09090B",
		DefaultShadow:       true,
		DefaultRouting:      graph.RouteOrthogonal,
		DefaultArrow:        graph.ArrowClassic,
		DefaultEdgeWidth:    1.75,
		PreferredDirection:  "LR",
		DefaultStrategy:     "layered",
		WheelMode:           "pan",
		ShowPerfHUD:         true,
		GapX:                76,
		GapY:                56,
		GridSize:            20,
		SnapToGrid:          true,
		ShowGrid:            true,
		NormalizeSizes:      true,
		HarmonizePalette:    true,
		ContainerPadding:    36,
		DefaultMediaMax:     480,
		SidebarOpen:         true,
		InspectorOpen:       true,
		MinimapOpen:         true,
		PinnedIconPacks:     []string{"builtin"},
		FavoriteIcons:       []string{"builtin/gateway", "builtin/server", "builtin/database", "builtin/cache", "builtin/queue", "builtin/bot"},
		IconUsage:           map[string]int{},
		ShapeUsage:          map[string]int{},
		LastExportFormat:    "svg",
		LastExportScale:     2,
		AutoCheckUpdates:    true,
		AutoDownloadUpdates: false,
		UpdateChannel:       "stable",
		UpdateCheckHours:    24,
		UpdatedAt:           time.Now().UTC(),
	}
}

// Store loads, mutates, and atomically persists Profile across projects.
type Store struct {
	mu      sync.RWMutex
	path    string
	profile *Profile
}

// DefaultPath returns the cross-project habits JSON path.
func DefaultPath() string {
	if home := strings.TrimSpace(os.Getenv("DRAWGRAPH_HOME")); home != "" {
		return filepath.Join(home, "habits.json")
	}
	base, err := os.UserConfigDir()
	if err != nil || base == "" {
		base, _ = os.UserHomeDir()
		return filepath.Join(base, ".drawgraph", "habits.json")
	}
	return filepath.Join(base, "DrawGraph", "habits.json")
}

// OpenStore loads existing persisted habits or initializes DefaultProfile.
func OpenStore(path string) (*Store, error) {
	if strings.TrimSpace(path) == "" {
		path = DefaultPath()
	}
	s := &Store{path: path, profile: DefaultProfile()}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	var p Profile
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	mergeMissingDefaults(&p)
	s.profile = &p
	return s, nil
}

// Path returns the backing file path.
func (s *Store) Path() string { return s.path }

// Snapshot returns a copy of the current Profile.
func (s *Store) Snapshot() *Profile {
	s.mu.RLock()
	defer s.mu.RUnlock()
	raw, _ := json.Marshal(s.profile)
	var cp Profile
	_ = json.Unmarshal(raw, &cp)
	mergeMissingDefaults(&cp)
	return &cp
}

// Save writes the current Profile atomically to disk.
func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveLocked()
}

// Update applies mutator fn to the profile and persists immediately.
func (s *Store) Update(fn func(p *Profile)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	fn(s.profile)
	mergeMissingDefaults(s.profile)
	s.profile.UpdatedAt = time.Now().UTC()
	return s.saveLocked()
}

// Reset restores DefaultProfile and saves it.
func (s *Store) Reset() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.profile = DefaultProfile()
	return s.saveLocked()
}

// ApplyToDocument configures a newly created document with the user's persisted defaults.
func (s *Store) ApplyToDocument(doc *graph.Document) {
	if doc == nil {
		return
	}
	p := s.Snapshot()
	doc.GridSize = p.GridSize
	doc.SnapToGrid = p.SnapToGrid
	doc.ThemeMode = p.ThemeMode
	doc.DefaultRouting = p.DefaultRouting
	doc.StylePreset = p.StylePreset
}

// NewNodeFromHabits constructs a node seeded with the user's learned defaults.
func (s *Store) NewNodeFromHabits(kind graph.NodeKind, label string, x, y float64) *graph.Node {
	p := s.Snapshot()
	if kind == "" {
		kind = p.DefaultNodeKind
	}
	w, h := 160.0, 72.0
	switch kind {
	case graph.KindContainer, graph.KindSwimlane:
		w, h = 360, 220
	case graph.KindDiamond:
		w, h = 140, 88
	case graph.KindEllipse:
		w, h = 140, 80
	case graph.KindIcon:
		w, h = 96, 96
	case graph.KindText:
		w, h = 160, 40
	}
	return &graph.Node{
		Kind:  kind,
		X:     x,
		Y:     y,
		W:     w,
		H:     h,
		Label: label,
		Style: graph.NodeStyle{
			Fill:        p.DefaultFill,
			Stroke:      p.DefaultStroke,
			StrokeWidth: p.DefaultStrokeWidth,
			Radius:      p.DefaultRadius,
			Shadow:      p.DefaultShadow,
			Opacity:     1,
			FontSize:    p.DefaultFontSize,
			TextColor:   p.DefaultTextColor,
			Align:       "center",
			VAlign:      "middle",
		},
	}
}

// NewEdgeFromHabits constructs an edge seeded with the user's learned defaults.
func (s *Store) NewEdgeFromHabits(srcID, dstID, label string) *graph.Edge {
	p := s.Snapshot()
	return &graph.Edge{
		SourceID: srcID,
		TargetID: dstID,
		Label:    label,
		Routing:  p.DefaultRouting,
		EndArrow: p.DefaultArrow,
		Style: graph.EdgeStyle{
			StrokeWidth: p.DefaultEdgeWidth,
			FontSize:    max(11, p.DefaultFontSize-1),
			Opacity:     1,
		},
	}
}

// RecordNodeHabit remembers user shape & styling habits when AutoRemember is enabled.
func (s *Store) RecordNodeHabit(n *graph.Node) {
	if n == nil {
		return
	}
	_ = s.Update(func(p *Profile) {
		if !p.AutoRemember {
			return
		}
		if n.Kind != "" && n.Kind != graph.KindContainer && n.Kind != graph.KindSwimlane && n.Kind != graph.KindText {
			p.ShapeUsage[string(n.Kind)]++
			p.DefaultNodeKind = n.Kind
		}
		if n.Style.Fill != "" {
			p.DefaultFill = n.Style.Fill
		}
		if n.Style.Stroke != "" {
			p.DefaultStroke = n.Style.Stroke
		}
		if n.Style.StrokeWidth > 0 {
			p.DefaultStrokeWidth = n.Style.StrokeWidth
		}
		if n.Style.Radius >= 0 {
			p.DefaultRadius = n.Style.Radius
		}
		if n.Style.FontSize > 0 {
			p.DefaultFontSize = n.Style.FontSize
		}
		if n.Style.TextColor != "" {
			p.DefaultTextColor = n.Style.TextColor
		}
		p.DefaultShadow = n.Style.Shadow
		if n.Icon.Valid() {
			s.recordIconLocked(p, n.Icon.String())
		}
	})
}

// RecordEdgeHabit remembers user connector routing and arrow preferences.
func (s *Store) RecordEdgeHabit(e *graph.Edge) {
	if e == nil {
		return
	}
	_ = s.Update(func(p *Profile) {
		if !p.AutoRemember {
			return
		}
		if e.Routing != "" {
			p.DefaultRouting = e.Routing
		}
		if e.EndArrow != "" {
			p.DefaultArrow = e.EndArrow
		}
		if e.Style.StrokeWidth > 0 {
			p.DefaultEdgeWidth = e.Style.StrokeWidth
		}
	})
}

// RecordIconUse increments usage count for iconRef ("packId/name") and updates FavoriteIcons.
func (s *Store) RecordIconUse(iconRef string) {
	if strings.TrimSpace(iconRef) == "" {
		return
	}
	_ = s.Update(func(p *Profile) {
		s.recordIconLocked(p, iconRef)
	})
}

// RecordRecentFile prepends filePath to RecentFiles (deduplicated, capped at 12).
func (s *Store) RecordRecentFile(filePath string) {
	filePath = strings.TrimSpace(filePath)
	if filePath == "" {
		return
	}
	_ = s.Update(func(p *Profile) {
		next := []string{filePath}
		for _, prev := range p.RecentFiles {
			if prev != filePath {
				next = append(next, prev)
			}
			if len(next) >= 12 {
				break
			}
		}
		p.RecentFiles = next
	})
}

// RecordFormatHabit remembers the user's preferred layout direction, spacing, and preset.
func (s *Store) RecordFormatHabit(direction string, routing graph.RoutingStyle, preset string, gapX, gapY, grid float64) {
	_ = s.Update(func(p *Profile) {
		if !p.AutoRemember {
			return
		}
		if direction == "LR" || direction == "TB" {
			p.PreferredDirection = direction
		}
		if routing != "" {
			p.DefaultRouting = routing
		}
		if preset != "" {
			p.StylePreset = preset
		}
		if gapX > 0 {
			p.GapX = gapX
		}
		if gapY > 0 {
			p.GapY = gapY
		}
		if grid > 0 {
			p.GridSize = grid
		}
	})
}

func (s *Store) recordIconLocked(p *Profile, iconRef string) {
	if p.IconUsage == nil {
		p.IconUsage = map[string]int{}
	}
	p.IconUsage[iconRef]++
	type kv struct {
		ref   string
		count int
	}
	var ranked []kv
	for k, v := range p.IconUsage {
		ranked = append(ranked, kv{k, v})
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].count != ranked[j].count {
			return ranked[i].count > ranked[j].count
		}
		return ranked[i].ref < ranked[j].ref
	})
	favs := make([]string, 0, 12)
	for _, item := range ranked {
		favs = append(favs, item.ref)
		if len(favs) >= 12 {
			break
		}
	}
	for _, old := range p.FavoriteIcons {
		if len(favs) >= 12 {
			break
		}
		found := false
		for _, f := range favs {
			if f == old {
				found = true
				break
			}
		}
		if !found {
			favs = append(favs, old)
		}
	}
	p.FavoriteIcons = favs
}

func (s *Store) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s.profile, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func mergeMissingDefaults(p *Profile) {
	def := DefaultProfile()
	if p.Version <= 0 {
		p.Version = def.Version
	}
	if p.ThemeMode == "" {
		p.ThemeMode = def.ThemeMode
	}
	if p.StylePreset == "" {
		p.StylePreset = def.StylePreset
	}
	if p.DefaultNodeKind == "" {
		p.DefaultNodeKind = def.DefaultNodeKind
	}
	if p.DefaultFill == "" {
		p.DefaultFill = def.DefaultFill
	}
	if p.DefaultStroke == "" {
		p.DefaultStroke = def.DefaultStroke
	}
	if p.DefaultStrokeWidth <= 0 {
		p.DefaultStrokeWidth = def.DefaultStrokeWidth
	}
	if p.DefaultFontSize <= 0 {
		p.DefaultFontSize = def.DefaultFontSize
	}
	if p.DefaultTextColor == "" {
		p.DefaultTextColor = def.DefaultTextColor
	}
	if p.DefaultRouting == "" {
		p.DefaultRouting = def.DefaultRouting
	}
	if p.DefaultArrow == "" {
		p.DefaultArrow = def.DefaultArrow
	}
	if p.DefaultEdgeWidth <= 0 {
		p.DefaultEdgeWidth = def.DefaultEdgeWidth
	}
	if p.PreferredDirection != "LR" && p.PreferredDirection != "TB" {
		p.PreferredDirection = def.PreferredDirection
	}
	if p.GapX <= 0 {
		p.GapX = def.GapX
	}
	if p.GapY <= 0 {
		p.GapY = def.GapY
	}
	if p.GridSize <= 0 {
		p.GridSize = def.GridSize
	}
	if p.ContainerPadding <= 0 {
		p.ContainerPadding = def.ContainerPadding
	}
	if p.DefaultMediaMax <= 0 {
		p.DefaultMediaMax = def.DefaultMediaMax
	}
	if p.IconUsage == nil {
		p.IconUsage = map[string]int{}
	}
	if p.ShapeUsage == nil {
		p.ShapeUsage = map[string]int{}
	}
	if len(p.PinnedIconPacks) == 0 {
		p.PinnedIconPacks = def.PinnedIconPacks
	}
	if p.LastExportFormat == "" {
		p.LastExportFormat = def.LastExportFormat
	}
	if p.LastExportScale <= 0 {
		p.LastExportScale = def.LastExportScale
	}
	if p.UpdateChannel != "stable" && p.UpdateChannel != "prerelease" {
		p.UpdateChannel = def.UpdateChannel
	}
	if p.UpdateCheckHours <= 0 {
		p.UpdateCheckHours = def.UpdateCheckHours
	}
	if p.Language == "" {
		p.Language = def.Language
	}
	if p.DefaultStrategy == "" {
		p.DefaultStrategy = def.DefaultStrategy
	}
	if p.WheelMode != "zoom" && p.WheelMode != "pan" {
		p.WheelMode = def.WheelMode
	}
}
