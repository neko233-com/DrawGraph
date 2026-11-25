package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/action"
	"github.com/lobehub/lobe-mygo/pkg/choice"
	"github.com/lobehub/lobe-mygo/pkg/display"
	"github.com/lobehub/lobe-mygo/pkg/feedback"
	"github.com/lobehub/lobe-mygo/pkg/field"
	"github.com/lobehub/lobe-mygo/pkg/layout"
	"github.com/lobehub/lobe-mygo/pkg/lucide"
	"github.com/lobehub/lobe-mygo/pkg/theme"

	"github.com/neko233-com/DrawGraph/internal/graph"
	"github.com/neko233-com/DrawGraph/internal/habits"
	"github.com/neko233-com/DrawGraph/internal/version"
)

func (a *App) toolbar(c *ui.Context, tk *theme.Tokens) {
	bar := c.TitleBar()
	h := float32(46)
	if bar.Height > h {
		h = bar.Height
	}
	ui.Row(c).Height(h).Shrink(0).Gap(6).AlignItems(ui.Center).
		Padding(0, bar.Right+12, 0, bar.Left+12).
		Background(tk.BgContainer).
		BorderWidth(0, 0, 1, 0).BorderColor(tk.BorderSecondary).
		DragWindow().Children(func() {
		// Brand + Document Title
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Box(c).Size(24, 24).Radius(6).Background(tk.Primary).Center().Children(func() {
				ui.Icon(c, lucide.Workflow()).FontSize(14).TextColor(tk.PrimaryForeground)
			})
			ui.Text(c, "DrawGraph").FontSize(13).FontWeight(600).TextColor(tk.Text)
		})

		dividerV(c, tk)

		// File Actions
		if action.ActionIcon(c, lucide.FilePlus(), action.ActionIconProps{Size: theme.Small, Title: "New Diagram"}).Clicked() {
			a.NewBlankDocument()
		}
		if action.ActionIcon(c, lucide.FolderOpen(), action.ActionIconProps{Size: theme.Small, Title: "Open (.drawgraph / .drawio)"}).Clicked() {
			a.OpenFromDialog()
		}
		if action.ActionIcon(c, lucide.Save(), action.ActionIconProps{Size: theme.Small, Title: "Save"}).Clicked() {
			a.SaveToCurrentOrDialog(false)
		}
		if iconButton(c, lucide.ImagePlus(), "Media", action.ButtonProps{Variant: action.Default, Size: theme.Small}).Clicked() {
			a.ImportMediaFromDialog()
		}
		if action.Button(c, "Export SVG", action.ButtonProps{Variant: action.Text, Size: theme.Small}).Clicked() {
			a.ExportDialog("svg")
		}
		if action.Button(c, "PNG", action.ButtonProps{Variant: action.Text, Size: theme.Small}).Clicked() {
			a.ExportDialog("png")
		}
		if action.Button(c, ".drawio", action.ButtonProps{Variant: action.Text, Size: theme.Small}).Clicked() {
			a.ExportDialog("drawio")
		}

		dividerV(c, tk)

		// History
		if action.ActionIcon(c, lucide.Undo(), action.ActionIconProps{Size: theme.Small, Title: "Undo"}).Disabled(!a.Hist.CanUndo()).Clicked() {
			a.Undo()
		}
		if action.ActionIcon(c, lucide.Redo(), action.ActionIconProps{Size: theme.Small, Title: "Redo"}).Disabled(!a.Hist.CanRedo()).Clicked() {
			a.Redo()
		}

		dividerV(c, tk)

		// Primary Canvas Tools
		a.toolButton(c, tk, ToolSelect, lucide.MousePointer2(), "Select (V)")
		a.toolButton(c, tk, ToolHand, lucide.Hand(), "Pan (H)")
		a.toolButton(c, tk, ToolConnect, lucide.ArrowUpRight(), "Connect (C)")

		dividerV(c, tk)

		// One-Click Elegant Format!
		if iconButton(c, lucide.Sparkles(), "Format", action.ButtonProps{
			Variant: action.Primary,
			Size:    theme.Small,
		}).Clicked() {
			a.FormatOneClick("")
		}
		pref := a.Habits.Snapshot()
		dirLabel := pref.PreferredDirection
		if action.Button(c, dirLabel, action.ButtonProps{
			Variant: action.Filled,
			Size:    theme.Small,
		}).Clicked() {
			nextDir := "TB"
			if dirLabel == "TB" {
				nextDir = "LR"
			}
			a.FormatOneClick(nextDir)
		}

		// Align actions when 2+ nodes selected
		if len(a.SelectedNodes) >= 2 {
			dividerV(c, tk)
			if action.ActionIcon(c, lucide.AlignStartVertical(), action.ActionIconProps{Size: theme.Small, Title: "Align Left"}).Clicked() {
				a.AlignSelection(graph.AlignLeft)
			}
			if action.ActionIcon(c, lucide.AlignCenterVertical(), action.ActionIconProps{Size: theme.Small, Title: "Align Center X"}).Clicked() {
				a.AlignSelection(graph.AlignCenterX)
			}
			if action.ActionIcon(c, lucide.AlignStartHorizontal(), action.ActionIconProps{Size: theme.Small, Title: "Align Top"}).Clicked() {
				a.AlignSelection(graph.AlignTop)
			}
		}

		ui.Spacer(c)

		// Zoom Controls
		if action.ActionIcon(c, lucide.ZoomOut(), action.ActionIconProps{Size: theme.Small, Title: "Zoom Out"}).Clicked() {
			a.ZoomOut()
		}
		if action.Button(c, fmt.Sprintf("%d%%", int(a.Cam.Zoom*100)), action.ButtonProps{Variant: action.Text, Size: theme.Small}).Clicked() {
			a.ResetZoom()
		}
		if action.ActionIcon(c, lucide.ZoomIn(), action.ActionIconProps{Size: theme.Small, Title: "Zoom In"}).Clicked() {
			a.ZoomIn()
		}
		if action.ActionIcon(c, lucide.Maximize2(), action.ActionIconProps{Size: theme.Small, Title: "Zoom to Fit"}).Clicked() {
			a.ZoomToFit()
		}

		dividerV(c, tk)

		// Grid & Theme & Plugin/Habit Drawers
		if action.ActionIcon(c, lucide.Grid3x3(), action.ActionIconProps{
			Variant: boolIconVariant(a.ShowGrid),
			Size:    theme.Small,
			Title:   "Toggle Grid",
		}).Clicked() {
			a.ShowGrid = !a.ShowGrid
			_ = a.Habits.Update(func(p *habits.Profile) { p.ShowGrid = a.ShowGrid })
		}
		themeIcon := lucide.Sun()
		if a.DarkActive {
			themeIcon = lucide.Moon()
		}
		if action.ActionIcon(c, themeIcon, action.ActionIconProps{Size: theme.Small, Title: "Toggle Theme"}).Clicked() {
			if a.DarkActive {
				a.ThemeMode = "light"
			} else {
				a.ThemeMode = "dark"
			}
			_ = a.Habits.Update(func(p *habits.Profile) { p.ThemeMode = a.ThemeMode })
		}
		if iconButton(c, lucide.GitBranch(), "Icon Plugins", action.ButtonProps{
			Variant: action.Filled,
			Size:    theme.Small,
		}).Clicked() {
			a.PluginsOpen = true
		}
		if iconButton(c, lucide.SlidersHorizontal(), "Habits", action.ButtonProps{
			Variant: action.Filled,
			Size:    theme.Small,
		}).Clicked() {
			a.HabitsOpen = true
		}
	})
}

func (a *App) toolButton(c *ui.Context, tk *theme.Tokens, tool Tool, icon *ui.SVG, title string) {
	variant := action.ActionIconBorderless
	if a.Tool == tool {
		variant = action.ActionIconFilled
	}
	b := action.ActionIcon(c, icon, action.ActionIconProps{
		Variant: variant,
		Size:    theme.Small,
		Title:   title,
	})
	if a.Tool == tool {
		b.BorderWidth(1).BorderColor(tk.Primary.Alpha(0.45))
	}
	if b.Clicked() {
		a.Tool = tool
	}
}

func (a *App) sidebar(c *ui.Context, tk *theme.Tokens) {
	ui.Column(c).Width(258).Shrink(0).Padding(10).Gap(10).
		Background(tk.BgContainer).
		BorderWidth(0, 1, 0, 0).BorderColor(tk.BorderSecondary).Children(func() {
		sb := field.SearchBar(c, &a.IconSearch, field.SearchBarProps{
			Placeholder: "Search shapes & icons...",
			Size:        theme.Small,
			Variant:     field.InputFilled,
		})
		sb.FillWidth()

		layout.ScrollArea(c, layout.ScrollAreaProps{Fade: true}, func() {
			ui.Column(c).Gap(12).Children(func() {
				// Core draw.io Shapes
				ui.Text(c, "CORE SHAPES").FontSize(10).FontWeight(600).LetterSpacing(0.5).TextColor(tk.TextQuaternary)
				q := strings.ToLower(strings.TrimSpace(a.IconSearch))
				ui.Row(c).Gap(6).Wrap().Children(func() {
					for _, k := range graph.AllNodeKinds {
						name := string(k)
						if q != "" && !strings.Contains(name, q) {
							continue
						}
						btn := action.Button(c, name, action.ButtonProps{
							Variant: action.Default,
							Size:    theme.Small,
						})
						if btn.Clicked() {
							a.QuickInsertShape(k, "", "")
						}
					}
				})

				// Favorite Icons (remembered across projects)
				favs := a.Habits.Snapshot().FavoriteIcons
				if len(favs) > 0 && q == "" {
					ui.Text(c, "REMEMBERED FAVORITE ICONS").FontSize(10).FontWeight(600).LetterSpacing(0.5).TextColor(tk.TextQuaternary)
					ui.Row(c).Gap(6).Wrap().Children(func() {
						for _, ref := range favs {
							svg, ok := a.Icons.ParsedSVG(ref)
							if !ok {
								continue
							}
							if b := ui.ButtonBase(c).Label("Fav "+ref).Size(36, 36).Radius(8).
								Background(tk.FillTertiary).Center().Children(func() {
								ui.Icon(c, svg).FontSize(18).TextColor(tk.Text)
							}); b.Clicked() {
								a.insertOrAssignIcon(ref)
							}
						}
					})
				}

				// Pack Filter + Git Install Shortcut
				ui.Row(c).AlignItems(ui.Center).Children(func() {
					ui.Text(c, "ICON LIBRARY").FontSize(10).FontWeight(600).LetterSpacing(0.5).TextColor(tk.TextQuaternary)
					ui.Spacer(c)
					if action.Button(c, "+ Git Pack", action.ButtonProps{Variant: action.Text, Size: theme.Small}).Clicked() {
						a.PluginsOpen = true
					}
				})

				packOpts := []field.SelectOption{{Value: "", Label: "All Enabled Packs"}}
				for _, pk := range a.Icons.Packs() {
					if pk.Enabled {
						packOpts = append(packOpts, field.SelectOption{
							Value: pk.ID,
							Label: fmt.Sprintf("%s (%d)", pk.Name, len(pk.Icons)),
						})
					}
				}
				field.Select(c, &a.ActivePack, field.SelectProps{
					Options: packOpts,
					Size:    theme.Small,
				})

				hits := a.Icons.Search(a.IconSearch, a.ActivePack, "")
				ui.Row(c).Gap(6).Wrap().Children(func() {
					for i, ic := range hits {
						if i >= 80 {
							break
						}
						svg, ok := a.Icons.ParsedSVG(ic.Ref())
						if !ok {
							continue
						}
						cell := ui.ButtonBase(c).Label("Icon " + ic.Ref()).
							Width(72).Height(62).Radius(8).Column().Center().Gap(4).Padding(4).
							Background(tk.FillTertiary)
						cell.Children(func() {
							ui.Icon(c, svg).FontSize(18).TextColor(tk.Text)
							ui.Text(c, ic.Name).FontSize(10).TextColor(tk.TextSecondary).SingleLine().MaxWidth(64)
						})
						if cell.Clicked() {
							a.insertOrAssignIcon(ic.Ref())
						}
					}
				})
			})
		}).Grow(1).Basis(0).MinHeight(0)
	})
}

func (a *App) insertOrAssignIcon(ref string) {
	if sel := a.singleSelectedNode(); sel != nil {
		a.Hist.Snapshot()
		sel.Icon = graph.ParseIconRef(ref)
		a.Habits.RecordIconUse(ref)
		a.notifyChanged()
		return
	}
	parts := strings.SplitN(ref, "/", 2)
	title := ref
	if len(parts) == 2 {
		title = parts[1]
	}
	a.QuickInsertShape(graph.KindRounded, title, ref)
}

func (a *App) pageBar(c *ui.Context, tk *theme.Tokens) {
	ui.Row(c).Height(34).Shrink(0).Gap(6).AlignItems(ui.Center).Padding(0, 10).
		Background(tk.BgContainer).BorderWidth(1, 0, 0, 0).BorderColor(tk.BorderSecondary).Children(func() {
		for _, p := range a.Doc().Pages {
			active := p.ID == a.Doc().ActivePageID
			variant := action.Text
			if active {
				variant = action.Filled
			}
			if action.Button(c, p.Name, action.ButtonProps{Variant: variant, Size: theme.Small}).Clicked() {
				a.Doc().ActivePageID = p.ID
				a.SelectedNodes = map[string]bool{}
				a.SelectedEdges = map[string]bool{}
				a.ZoomToFit()
			}
		}
		if action.ActionIcon(c, lucide.Plus(), action.ActionIconProps{Size: theme.Small, Title: "Add Page"}).Clicked() {
			a.Hist.Snapshot()
			_, _ = a.Doc().AddPage("", "", true)
			a.SelectedNodes = map[string]bool{}
			a.SelectedEdges = map[string]bool{}
			a.notifyChanged()
		}
	})
}

func (a *App) inspector(c *ui.Context, tk *theme.Tokens) {
	ui.Column(c).Width(268).Shrink(0).Padding(12).Gap(12).
		Background(tk.BgContainer).
		BorderWidth(0, 0, 0, 1).BorderColor(tk.BorderSecondary).Children(func() {
		layout.ScrollArea(c, layout.ScrollAreaProps{Fade: true}, func() {
			if n := a.singleSelectedNode(); n != nil {
				a.nodeInspector(c, tk, n)
			} else if e := a.singleSelectedEdge(); e != nil {
				a.edgeInspector(c, tk, e)
			} else {
				a.documentInspector(c, tk)
			}
		}).Grow(1).Basis(0).MinHeight(0)
	})
}

func (a *App) nodeInspector(c *ui.Context, tk *theme.Tokens, n *graph.Node) {
	ui.Column(c).Gap(10).Children(func() {
		ui.Text(c, "NODE PROPERTIES").FontSize(10).FontWeight(600).LetterSpacing(0.5).TextColor(tk.TextQuaternary)

		label := n.Label
		ui.Column(c).Gap(4).Children(func() {
			ui.Text(c, "Label").FontSize(11).TextColor(tk.TextTertiary)
			inp := field.Input(c, &label, field.InputProps{Size: theme.Small})
			inp.FillWidth()
			if label != n.Label {
				a.Hist.Snapshot()
				n.Label = label
				a.notifyChanged()
			}
		})

		sub := n.Sublabel
		ui.Column(c).Gap(4).Children(func() {
			ui.Text(c, "Sublabel").FontSize(11).TextColor(tk.TextTertiary)
			inp := field.Input(c, &sub, field.InputProps{Size: theme.Small, Placeholder: "Optional subtitle"})
			inp.FillWidth()
			if sub != n.Sublabel {
				a.Hist.Snapshot()
				n.Sublabel = sub
				a.notifyChanged()
			}
		})

		kindStr := string(n.Kind)
		kindOpts := make([]field.SelectOption, 0, len(graph.AllNodeKinds))
		for _, k := range graph.AllNodeKinds {
			kindOpts = append(kindOpts, field.SelectOption{Value: string(k), Label: string(k)})
		}
		ui.Column(c).Gap(4).Children(func() {
			ui.Text(c, "Shape").FontSize(11).TextColor(tk.TextTertiary)
			field.Select(c, &kindStr, field.SelectProps{Options: kindOpts, Size: theme.Small})
			if kindStr != string(n.Kind) {
				a.Hist.Snapshot()
				n.Kind = graph.NodeKind(kindStr)
				a.Habits.RecordNodeHabit(n)
				a.notifyChanged()
			}
		})

		ui.Column(c).Gap(4).Children(func() {
			ui.Text(c, "Fill Color").FontSize(11).TextColor(tk.TextTertiary)
			ui.Row(c).Gap(6).Wrap().Children(func() {
				for _, hex := range []string{"#FFFFFF", "#EFF6FF", "#ECFDF5", "#FEF3C7", "#F5F3FF", "#FFF1F2", "#1E293B"} {
					sw := ui.ButtonBase(c).Label("Fill "+hex).Size(22, 22).Radius(6).
						Background(ui.Hex(hex)).BorderWidth(1).BorderColor(tk.Border)
					if sw.Clicked() {
						a.Hist.Snapshot()
						n.Style.Fill = hex
						a.Habits.RecordNodeHabit(n)
						a.notifyChanged()
					}
				}
			})
		})

		ui.Column(c).Gap(4).Children(func() {
			ui.Text(c, "Stroke Color").FontSize(11).TextColor(tk.TextTertiary)
			ui.Row(c).Gap(6).Wrap().Children(func() {
				for _, hex := range []string{"#CBD5E1", "#3B82F6", "#10B981", "#F59E0B", "#8B5CF6", "#EF4444", "#0F172A"} {
					sw := ui.ButtonBase(c).Label("Stroke "+hex).Size(22, 22).Radius(6).
						Background(ui.Hex(hex)).BorderWidth(1).BorderColor(tk.Border)
					if sw.Clicked() {
						a.Hist.Snapshot()
						n.Style.Stroke = hex
						a.Habits.RecordNodeHabit(n)
						a.notifyChanged()
					}
				}
			})
		})

		shadow := n.Style.Shadow
		if choice.Checkbox(c, &shadow, "Drop shadow", choice.CheckboxProps{}).Changed() {
			a.Hist.Snapshot()
			n.Style.Shadow = shadow
			a.Habits.RecordNodeHabit(n)
			a.notifyChanged()
		}

		if n.Icon.Valid() {
			ui.Row(c).AlignItems(ui.Center).Gap(6).Children(func() {
				ui.Text(c, "Icon: "+n.Icon.String()).FontSize(11).TextColor(tk.TextSecondary).SingleLine()
				ui.Spacer(c)
				if action.Button(c, "Clear", action.ButtonProps{Variant: action.Text, Size: theme.Small}).Clicked() {
					a.Hist.Snapshot()
					n.Icon = nil
					a.notifyChanged()
				}
			})
		}

		if (n.Kind == graph.KindImage || n.Kind == graph.KindVideo) && n.Media != nil {
			a.mediaInspectorSection(c, tk, n)
		}

		ui.Row(c).Gap(8).Children(func() {
			if action.Button(c, "Save as Habit", action.ButtonProps{Variant: action.Filled, Size: theme.Small}).Clicked() {
				a.Habits.RecordNodeHabit(n)
				a.notifyToast(feedback.ToastSuccess, "Habit Saved", "Default style updated across all projects")
			}
			if action.Button(c, "Delete", action.ButtonProps{Variant: action.Default, Size: theme.Small}).TextColor(tk.Error).Clicked() {
				a.DeleteSelection()
			}
		})
	})
}

func (a *App) edgeInspector(c *ui.Context, tk *theme.Tokens, e *graph.Edge) {
	ui.Column(c).Gap(10).Children(func() {
		ui.Text(c, "CONNECTOR PROPERTIES").FontSize(10).FontWeight(600).LetterSpacing(0.5).TextColor(tk.TextQuaternary)

		label := e.Label
		ui.Column(c).Gap(4).Children(func() {
			ui.Text(c, "Label").FontSize(11).TextColor(tk.TextTertiary)
			inp := field.Input(c, &label, field.InputProps{Size: theme.Small})
			inp.FillWidth()
			if label != e.Label {
				a.Hist.Snapshot()
				e.Label = label
				a.notifyChanged()
			}
		})

		routing := string(e.Routing)
		ui.Column(c).Gap(4).Children(func() {
			ui.Text(c, "Routing").FontSize(11).TextColor(tk.TextTertiary)
			field.Select(c, &routing, field.SelectProps{
				Options: []field.SelectOption{
					{Value: "orthogonal", Label: "Orthogonal (draw.io)"},
					{Value: "curved", Label: "Curved Bezier"},
					{Value: "straight", Label: "Straight"},
				},
				Size: theme.Small,
			})
			if routing != string(e.Routing) {
				a.Hist.Snapshot()
				e.Routing = graph.RoutingStyle(routing)
				a.Habits.RecordEdgeHabit(e)
				a.notifyChanged()
			}
		})

		arrow := string(e.EndArrow)
		ui.Column(c).Gap(4).Children(func() {
			ui.Text(c, "Arrowhead").FontSize(11).TextColor(tk.TextTertiary)
			field.Select(c, &arrow, field.SelectProps{
				Options: []field.SelectOption{
					{Value: "arrow", Label: "Classic Arrow"},
					{Value: "open", Label: "Open Arrow"},
					{Value: "diamond", Label: "Diamond"},
					{Value: "circle", Label: "Circle"},
					{Value: "none", Label: "None"},
				},
				Size: theme.Small,
			})
			if arrow != string(e.EndArrow) {
				a.Hist.Snapshot()
				e.EndArrow = graph.ArrowStyle(arrow)
				a.Habits.RecordEdgeHabit(e)
				a.notifyChanged()
			}
		})

		if action.Button(c, "Delete Connector", action.ButtonProps{Variant: action.Default, Size: theme.Small}).TextColor(tk.Error).Clicked() {
			a.DeleteSelection()
		}
	})
}

func (a *App) documentInspector(c *ui.Context, tk *theme.Tokens) {
	ui.Column(c).Gap(12).Children(func() {
		ui.Text(c, "CANVAS & ONE-CLICK STYLE").FontSize(10).FontWeight(600).LetterSpacing(0.5).TextColor(tk.TextQuaternary)

		title := a.Doc().Title
		ui.Column(c).Gap(4).Children(func() {
			ui.Text(c, "Diagram Title").FontSize(11).TextColor(tk.TextTertiary)
			inp := field.Input(c, &title, field.InputProps{Size: theme.Small})
			inp.FillWidth()
			if title != a.Doc().Title {
				a.Doc().Title = title
				a.notifyChanged()
			}
		})

		snap := a.Doc().SnapToGrid
		if choice.Checkbox(c, &snap, "Snap to grid", choice.CheckboxProps{}).Changed() {
			a.Doc().SnapToGrid = snap
			_ = a.Habits.Update(func(p *habits.Profile) { p.SnapToGrid = snap })
		}

		preset := a.Doc().StylePreset
		if preset == "" {
			preset = a.Habits.Snapshot().StylePreset
		}
		var presetOpts []field.SelectOption
		for _, pr := range habits.BuiltinPresets() {
			presetOpts = append(presetOpts, field.SelectOption{Value: pr.ID, Label: pr.Name})
		}
		ui.Column(c).Gap(4).Children(func() {
			ui.Text(c, "Style Preset").FontSize(11).TextColor(tk.TextTertiary)
			field.Select(c, &preset, field.SelectProps{Options: presetOpts, Size: theme.Small})
			if preset != a.Doc().StylePreset {
				a.Doc().StylePreset = preset
				_ = a.Habits.Update(func(p *habits.Profile) { p.StylePreset = preset })
				a.FormatOneClick("")
			}
		})

		ui.Row(c).Gap(6).Children(func() {
			if iconButton(c, lucide.Sparkles(), "Format LR", action.ButtonProps{Variant: action.Primary, Size: theme.Small}).Clicked() {
				a.FormatOneClick("LR")
			}
			if action.Button(c, "Format TB", action.ButtonProps{Variant: action.Default, Size: theme.Small}).Clicked() {
				a.FormatOneClick("TB")
			}
		})

		ui.Text(c, fmt.Sprintf("Nodes: %d  ·  Edges: %d  ·  Pages: %d",
			len(a.Page().Nodes), len(a.Page().Edges), len(a.Doc().Pages))).
			FontSize(11).TextColor(tk.TextTertiary)
	})
}

func (a *App) mediaInspectorSection(c *ui.Context, tk *theme.Tokens, n *graph.Node) {
	asset := a.Doc().Assets[n.Media.AssetID]
	ui.Column(c).Gap(8).Padding(10).Radius(8).Background(tk.FillTertiary).Children(func() {
		ui.Text(c, "HD MEDIA & RESOLUTION").FontSize(10).FontWeight(600).LetterSpacing(0.5).TextColor(tk.TextQuaternary)
		if asset != nil && asset.NaturalW > 0 {
			meta := fmt.Sprintf("Source: %d × %d px", asset.NaturalW, asset.NaturalH)
			if asset.DurationSec > 0 {
				meta += fmt.Sprintf(" · %.1fs", asset.DurationSec)
			}
			ui.Text(c, meta).FontSize(11).TextColor(tk.TextSecondary)
			ui.Row(c).Gap(6).Children(func() {
				if action.Button(c, "HD Fit", action.ButtonProps{Variant: action.Default, Size: theme.Small}).Clicked() {
					a.ResizeMediaNode(n, false)
				}
				if action.Button(c, fmt.Sprintf("1:1 (%dx%d)", asset.NaturalW, asset.NaturalH), action.ButtonProps{Variant: action.Default, Size: theme.Small}).Clicked() {
					a.ResizeMediaNode(n, true)
				}
			})
		}

		fit := n.Media.Fit
		if fit == "" {
			fit = "contain"
		}
		ui.Column(c).Gap(4).Children(func() {
			ui.Text(c, "Scale Mode").FontSize(11).TextColor(tk.TextTertiary)
			field.Select(c, &fit, field.SelectProps{
				Options: []field.SelectOption{
					{Value: "contain", Label: "Contain (Full Aspect)"},
					{Value: "cover", Label: "Cover (Crop Fill)"},
					{Value: "fill", Label: "Stretch Fill"},
				},
				Size: theme.Small,
			})
			if fit != n.Media.Fit {
				a.Hist.Snapshot()
				n.Media.Fit = fit
				a.notifyChanged()
			}
		})

		showBar := n.Media.ShowBar
		if choice.Checkbox(c, &showBar, "Show caption footer bar", choice.CheckboxProps{}).Changed() {
			a.Hist.Snapshot()
			n.Media.ShowBar = showBar
			a.notifyChanged()
		}

		if n.Kind == graph.KindVideo {
			if vp := a.ensureVideoPlayer(n); vp != nil && vp.player != nil {
				lbl := "Pause Video"
				if !vp.player.Playing() {
					lbl = "Play Video"
				}
				if action.Button(c, lbl, action.ButtonProps{Variant: action.Primary, Size: theme.Small}).Clicked() {
					if vp.player.Playing() {
						vp.player.Pause()
					} else {
						vp.player.Play()
					}
				}
			}
			loop := n.Media.Loop
			if choice.Checkbox(c, &loop, "Loop video", choice.CheckboxProps{}).Changed() {
				n.Media.Loop = loop
				if vp := a.videoPlayers[n.ID]; vp != nil && vp.player != nil {
					vp.player.SetLoop(loop)
				}
			}
			muted := n.Media.Muted
			if choice.Checkbox(c, &muted, "Muted", choice.CheckboxProps{}).Changed() {
				n.Media.Muted = muted
				if vp := a.videoPlayers[n.ID]; vp != nil && vp.player != nil {
					vp.player.SetMuted(muted)
				}
			}
		}
	})
}

func (a *App) statusBar(c *ui.Context, tk *theme.Tokens) {
	ui.Row(c).Height(28).Shrink(0).Gap(12).AlignItems(ui.Center).Padding(0, 12).
		Background(tk.BgContainer).BorderWidth(1, 0, 0, 0).BorderColor(tk.BorderSecondary).Children(func() {
		ui.Text(c, fmt.Sprintf("Tool: %s", a.Tool)).FontSize(11).TextColor(tk.TextTertiary)
		ui.Text(c, fmt.Sprintf("Nodes: %d", len(a.Page().Nodes))).FontSize(11).TextColor(tk.TextTertiary)
		ui.Text(c, fmt.Sprintf("Edges: %d", len(a.Page().Edges))).FontSize(11).TextColor(tk.TextTertiary)
		ui.Text(c, fmt.Sprintf("Assets: %d", len(a.Doc().Assets))).FontSize(11).TextColor(tk.TextTertiary)
		cx, cy := -a.Cam.X, -a.Cam.Y
		if a.CanvasRect.W > 0 && a.Cam.Zoom > 0 {
			cx += float64(a.CanvasRect.W) / (2 * a.Cam.Zoom)
			cy += float64(a.CanvasRect.H) / (2 * a.Cam.Zoom)
		}
		ui.Text(c, fmt.Sprintf("Canvas (%.0f, %.0f) · %d%%", cx, cy, int(a.Cam.Zoom*100))).FontSize(11).TextColor(tk.TextQuaternary)
		if action.Button(c, "Origin", action.ButtonProps{Variant: action.Text, Size: theme.Small}).Clicked() {
			a.ResetOrigin()
		}
		if a.Habits.Snapshot().AutoRemember {
			ui.Text(c, "Habit Memory: ON").FontSize(11).TextColor(tk.Success)
		}
		ui.Spacer(c)
		if a.FilePath != "" {
			ui.Text(c, a.FilePath).FontSize(11).TextColor(tk.TextQuaternary).SingleLine().MaxWidth(340)
		} else {
			ui.Text(c, "DrawGraph "+version.Version).FontSize(11).TextColor(tk.TextQuaternary)
		}
	})
}

func (a *App) pluginsDrawer(c *ui.Context, tk *theme.Tokens) {
	feedback.Drawer(c, &a.PluginsOpen, feedback.DrawerProps{
		Placement: feedback.Right,
		Size:      460,
		Title:     "Icon Plugins (Git & Local SVG)",
	}, func() {
		ui.Column(c).Gap(14).Children(func() {
			ui.Text(c, "Extend DrawGraph with any Git icon repository or local SVG folder. Installed packs stay available across every diagram.").
				FontSize(12).LineHeight(1.5).TextColor(tk.TextSecondary)

			layout.Block(c, layout.BlockProps{Variant: layout.BlockFilled}).Padding(14).Gap(10).Children(func() {
				ui.Text(c, "Install from Git URL").FontSize(13).FontWeight(600).TextColor(tk.Text)
				inp := field.Input(c, &a.GitURL, field.InputProps{
					Placeholder: "https://github.com/owner/icon-pack.git",
					Size:        theme.Middle,
				})
				inp.FillWidth()
				ui.Row(c).Gap(8).Children(func() {
					r1 := field.Input(c, &a.GitRef, field.InputProps{Placeholder: "Branch / tag (optional)", Size: theme.Small})
					r1.Grow(1).Basis(0)
					r2 := field.Input(c, &a.GitSubdir, field.InputProps{Placeholder: "Subdir (e.g. icons)", Size: theme.Small})
					r2.Grow(1).Basis(0)
				})
				ui.Row(c).Gap(8).Children(func() {
					if iconButton(c, lucide.GitBranch(), "Clone Git Pack", action.ButtonProps{Variant: action.Primary, Size: theme.Small}).
						Disabled(a.PluginBusy || strings.TrimSpace(a.GitURL) == "").Clicked() {
						a.InstallGitIconPack()
					}
					if action.Button(c, "Import Local Folder…", action.ButtonProps{Variant: action.Default, Size: theme.Small}).Clicked() {
						a.ImportIconFolderDialog()
					}
				})
				if a.PluginStatus != "" {
					ui.Text(c, a.PluginStatus).FontSize(11).TextColor(tk.TextSecondary)
				}
			})

			ui.Text(c, "Installed Icon Packs").FontSize(13).FontWeight(600).TextColor(tk.Text)
			for _, pk := range a.Icons.Packs() {
				pack := pk
				layout.Block(c, layout.BlockProps{Variant: layout.BlockOutlined}).Padding(10).Gap(6).Children(func() {
					ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
						ui.Text(c, pack.Name).FontSize(13).FontWeight(600).TextColor(tk.Text)
						display.Tag(c, fmt.Sprintf("%d icons", len(pack.Icons)), display.TagProps{Size: theme.Small})
						ui.Spacer(c)
						if !pack.Builtin {
							if pack.SourceKind == "git" {
								if action.Button(c, "Sync", action.ButtonProps{Variant: action.Text, Size: theme.Small}).Clicked() {
									go func(id string) {
										ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
										defer cancel()
										up, err := a.Icons.SyncPack(ctx, id)
										a.Host.Update(func() {
											if err != nil {
												a.notifyToast(feedback.ToastError, "Sync Failed", err.Error())
											} else {
												a.notifyToast(feedback.ToastSuccess, "Synced "+up.ID, fmt.Sprintf("%d icons", len(up.Icons)))
											}
										})
									}(pack.ID)
								}
							}
							if action.Button(c, "Remove", action.ButtonProps{Variant: action.Text, Size: theme.Small}).TextColor(tk.Error).Clicked() {
								_ = a.Icons.RemovePack(pack.ID)
								if a.ActivePack == pack.ID {
									a.ActivePack = ""
								}
							}
						}
					})
					ui.Text(c, pack.Source).FontSize(11).TextColor(tk.TextTertiary).SingleLine()
				})
			}
		})
	})
}

func (a *App) habitsDrawer(c *ui.Context, tk *theme.Tokens) {
	feedback.Drawer(c, &a.HabitsOpen, feedback.DrawerProps{
		Placement: feedback.Right,
		Size:      440,
		Title:     "Persistent User Habits (Global)",
	}, func() {
		p := a.Habits.Snapshot()
		ui.Column(c).Gap(14).Children(func() {
			ui.Text(c, "Saved globally to "+a.Habits.Path()+". New projects and One-Click Format inherit these defaults automatically.").
				FontSize(12).LineHeight(1.5).TextColor(tk.TextSecondary)

			auto := p.AutoRemember
			if choice.Checkbox(c, &auto, "Auto-remember style, icon, and layout habits across projects", choice.CheckboxProps{}).Changed() {
				_ = a.Habits.Update(func(pr *habits.Profile) { pr.AutoRemember = auto })
			}

			dir := p.PreferredDirection
			ui.Column(c).Gap(4).Children(func() {
				ui.Text(c, "Preferred Layout Direction").FontSize(11).TextColor(tk.TextTertiary)
				field.Select(c, &dir, field.SelectProps{
					Options: []field.SelectOption{
						{Value: "LR", Label: "Left -> Right (LR)"},
						{Value: "TB", Label: "Top -> Bottom (TB)"},
					},
					Size: theme.Middle,
				})
				if dir != p.PreferredDirection {
					_ = a.Habits.Update(func(pr *habits.Profile) { pr.PreferredDirection = dir })
				}
			})

			routing := string(p.DefaultRouting)
			ui.Column(c).Gap(4).Children(func() {
				ui.Text(c, "Default Connector Routing").FontSize(11).TextColor(tk.TextTertiary)
				field.Select(c, &routing, field.SelectProps{
					Options: []field.SelectOption{
						{Value: "orthogonal", Label: "Orthogonal (draw.io)"},
						{Value: "curved", Label: "Curved Bezier"},
						{Value: "straight", Label: "Straight"},
					},
					Size: theme.Middle,
				})
				if routing != string(p.DefaultRouting) {
					_ = a.Habits.Update(func(pr *habits.Profile) { pr.DefaultRouting = graph.RoutingStyle(routing) })
				}
			})

			ui.Row(c).Gap(10).Children(func() {
				gx := p.GapX
				ui.Column(c).Grow(1).Basis(0).Gap(4).Children(func() {
					ui.Text(c, "Horizontal Gap").FontSize(11).TextColor(tk.TextTertiary)
					field.InputNumber(c, &gx, field.InputNumberProps{Min: 20, Max: 240, Step: 4, Size: theme.Small})
					if gx != p.GapX {
						_ = a.Habits.Update(func(pr *habits.Profile) { pr.GapX = gx })
					}
				})
				gy := p.GapY
				ui.Column(c).Grow(1).Basis(0).Gap(4).Children(func() {
					ui.Text(c, "Vertical Gap").FontSize(11).TextColor(tk.TextTertiary)
					field.InputNumber(c, &gy, field.InputNumberProps{Min: 20, Max: 240, Step: 4, Size: theme.Small})
					if gy != p.GapY {
						_ = a.Habits.Update(func(pr *habits.Profile) { pr.GapY = gy })
					}
				})
			})

			mediaMax := p.DefaultMediaMax
			ui.Column(c).Gap(4).Children(func() {
				ui.Text(c, "Imported HD Image/Video Initial Max Size (DIPs)").FontSize(11).TextColor(tk.TextTertiary)
				field.InputNumber(c, &mediaMax, field.InputNumberProps{Min: 160, Max: 1920, Step: 40, Size: theme.Small})
				if mediaMax != p.DefaultMediaMax {
					_ = a.Habits.Update(func(pr *habits.Profile) { pr.DefaultMediaMax = mediaMax })
				}
			})

			ui.Row(c).Gap(8).Children(func() {
				if iconButton(c, lucide.Sparkles(), "Apply Habits Now", action.ButtonProps{Variant: action.Primary, Size: theme.Middle}).Clicked() {
					a.Habits.ApplyToDocument(a.Doc())
					a.FormatOneClick("")
					a.HabitsOpen = false
				}
				if action.Button(c, "Reset Defaults", action.ButtonProps{Variant: action.Default, Size: theme.Middle}).TextColor(tk.Error).Clicked() {
					_ = a.Habits.Reset()
					a.notifyToast(feedback.ToastInfo, "Habits Reset", "Restored factory defaults")
				}
			})
		})
	})
}

func iconButton(c *ui.Context, svg *ui.SVG, label string, p action.ButtonProps) ui.Element {
	b := action.StyleButton(c, ui.ButtonBase(c), "", p)
	if label != "" {
		b.Label(label)
	}
	b.Row().AlignItems(ui.Center).Children(func() {
		if svg != nil {
			ui.Icon(c, svg).FontSize(14)
		}
		if label != "" {
			ui.Text(c, label).SingleLine()
		}
	})
	return b
}

func dividerV(c *ui.Context, tk *theme.Tokens) {
	ui.Box(c).Width(1).Height(18).Background(tk.BorderSecondary).Margin(0, 2)
}

func boolIconVariant(active bool) action.ActionIconVariant {
	if active {
		return action.ActionIconFilled
	}
	return action.ActionIconBorderless
}
