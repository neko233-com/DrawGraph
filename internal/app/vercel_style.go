package app

import (
	"runtime"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/action"
	"github.com/lobehub/lobe-mygo/pkg/theme"

	"github.com/neko233-com/DrawGraph/internal/graph"
	"github.com/neko233-com/DrawGraph/internal/icons"
)

// Vercel Geist Spectrum signature colors (light/dark adaptive accents).
var (
	vercelPink    = ui.Hex("#FF0080")
	vercelViolet  = ui.Hex("#7928CA")
	vercelBlue    = ui.Hex("#0070F3")
	vercelCyan    = ui.Hex("#06B6D4")
	vercelEmerald = ui.Hex("#10B981")
	vercelAmber   = ui.Hex("#F59E0B")
	vercelIndigo  = ui.Hex("#6366F1")
)

var vercelSpectrumStops = []ui.GradientStop{
	{Color: vercelPink, At: 0.0},
	{Color: vercelViolet, At: 0.25},
	{Color: vercelBlue, At: 0.50},
	{Color: vercelCyan, At: 0.75},
	{Color: vercelEmerald, At: 1.0},
}

func vercelSpectrumBar(c *ui.Context, h float32) {
	ui.Box(c).FillWidth().Height(h).Shrink(0).LinearGradient(ui.LinearGradient{
		Angle: 90,
		Oklab: true,
		Stops: vercelSpectrumStops,
	})
}

func vercelGradientButton(c *ui.Context, svg *ui.SVG, label string, small bool) ui.Element {
	h := float32(28)
	px := float32(10)
	fs := float32(12)
	if !small {
		h = 34
		px = 14
		fs = 13
	}
	b := ui.ButtonBase(c).Label(label).Height(h).Padding(0, px).Radius(8).
		Row().AlignItems(ui.Center).Gap(6).
		LinearGradient(ui.LinearGradient{
			From:  vercelBlue,
			To:    vercelViolet,
			Angle: 125,
			Oklab: true,
		})
	b.Children(func() {
		fg := ui.Hex("#FFFFFF")
		if svg != nil {
			ui.Icon(c, svg).FontSize(fs + 1).TextColor(fg)
		}
		ui.Text(c, label).FontSize(fs).FontWeight(600).TextColor(fg).SingleLine()
	})
	return b
}

func vercelTintPill(c *ui.Context, tk *theme.Tokens, accent ui.Color, svg *ui.SVG, label string, active bool) ui.Element {
	bg := accent.Alpha(0.10)
	border := accent.Alpha(0.28)
	if active {
		bg = accent.Alpha(0.18)
		border = accent.Alpha(0.75)
	}
	b := ui.ButtonBase(c).Label(label).Height(26).Padding(0, 8).Radius(7).
		Row().AlignItems(ui.Center).Gap(5).
		Background(bg).BorderWidth(1).BorderColor(border)
	b.Children(func() {
		if svg != nil {
			ui.Icon(c, svg).FontSize(13).TextColor(accent)
		}
		if label != "" {
			ui.Text(c, label).FontSize(11).FontWeight(600).TextColor(accent).SingleLine()
		}
	})
	return b
}

func kbdBadge(c *ui.Context, tk *theme.Tokens, keyText string, accent ui.Color) {
	ui.Box(c).Padding(2, 6).Radius(5).
		Background(accent.Alpha(0.10)).
		BorderWidth(1).BorderColor(accent.Alpha(0.32)).
		Children(func() {
			ui.Text(c, keyText).FontSize(10).FontWeight(600).TextColor(accent).SingleLine()
		})
}

func platformMod() string {
	if runtime.GOOS == "darwin" {
		return "⌘"
	}
	return "Ctrl"
}

func strategyAccent(s string) ui.Color {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "tree":
		return vercelEmerald
	case "grid":
		return vercelCyan
	case "radial":
		return vercelViolet
	case "swimlane":
		return vercelAmber
	case "compact":
		return vercelPink
	default: // "layered"
		return vercelBlue
	}
}

func shapeCategoryColor(kind graph.NodeKind) ui.Color {
	switch kind {
	case graph.KindCylinder:
		return vercelEmerald
	case graph.KindCloud, graph.KindIcon:
		return vercelCyan
	case graph.KindDiamond, graph.KindNote, graph.KindText:
		return vercelAmber
	case graph.KindContainer, graph.KindSwimlane:
		return vercelViolet
	case graph.KindImage, graph.KindVideo, graph.KindActor, graph.KindEllipse:
		return vercelPink
	default:
		return vercelBlue
	}
}

func iconCategoryColor(ic icons.IconEntry) ui.Color {
	key := strings.ToLower(ic.Name + " " + ic.Category)
	switch {
	case strings.Contains(key, "db"), strings.Contains(key, "database"), strings.Contains(key, "storage"), strings.Contains(key, "cache"):
		return vercelEmerald
	case strings.Contains(key, "bot"), strings.Contains(key, "brain"), strings.Contains(key, "ai"), strings.Contains(key, "lambda"), strings.Contains(key, "pipeline"):
		return vercelViolet
	case strings.Contains(key, "gateway"), strings.Contains(key, "cloud"), strings.Contains(key, "load"), strings.Contains(key, "webhook"):
		return vercelCyan
	case strings.Contains(key, "lock"), strings.Contains(key, "firewall"), strings.Contains(key, "monitor"), strings.Contains(key, "queue"):
		return vercelAmber
	case strings.Contains(key, "user"), strings.Contains(key, "browser"), strings.Contains(key, "mobile"):
		return vercelPink
	default:
		return vercelBlue
	}
}

func iconRefColor(ref string) ui.Color {
	return iconCategoryColor(icons.IconEntry{Name: ref})
}

func toolPill(c *ui.Context, tk *theme.Tokens, active bool, svg *ui.SVG, title string) ui.Element {
	variant := action.ActionIconBorderless
	if active {
		variant = action.ActionIconFilled
	}
	b := action.ActionIcon(c, svg, action.ActionIconProps{
		Variant: variant,
		Size:    theme.Small,
		Title:   title,
	})
	if active {
		b.Background(vercelBlue.Alpha(0.14)).BorderWidth(1).BorderColor(vercelBlue.Alpha(0.65))
	}
	return b
}
