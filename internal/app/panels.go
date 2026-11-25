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
	"github.com/neko233-com/DrawGraph/internal/i18n"
	dglayout "github.com/neko233-com/DrawGraph/internal/layout"
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
		// Brand
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Box(c).Size(24, 24).Radius(6).Background(tk.Primary).Center().Children(func() {
				ui.Icon(c, lucide.Workflow()).FontSize(14).TextColor(tk.PrimaryForeground)
			})
			ui.Text(c, a.T("app.name")).FontSize(13).FontWeight(600).TextColor(tk.Text)
		})

		dividerV(c, tk)

		// File Actions
		if action.ActionIcon(c, lucide.FilePlus(), action.ActionIconProps{Size: theme.Small, Title: a.T("tb.new")}).Clicked() {
			a.NewBlankDocument()
		}
		if action.ActionIcon(c, lucide.FolderOpen(), action.ActionIconProps{Size: theme.Small, Title: a.T("tb.open")}).Clicked() {
			a.OpenFromDialog()
		}
		if action.ActionIcon(c, lucide.Save(), action.ActionIconProps{Size: theme.Small, Title: a.T("tb.save")}).Clicked() {
			a.SaveToCurrentOrDialog(false)
		}
		if iconButton(c, lucide.ImagePlus(), a.T("tb.media"), action.ButtonProps{Variant: action.Default, Size: theme.Small}).Clicked() {
			a.ImportMediaFromDialog()
		}
		if iconButton(c, lucide.FileCode(), a.T("tb.mermaid"), action.ButtonProps{Variant: action.Default, Size: theme.Small}).Clicked() {
			a.MermaidOpen = true
		}
		if action.Button(c, a.T("tb.exportSVG"), action.ButtonProps{Variant: action.Text, Size: theme.Small}).Clicked() {
			a.ExportDialog("svg")
		}
		if action.Button(c, a.T("tb.exportPNG"), action.ButtonProps{Variant: action.Text, Size: theme.Small}).Clicked() {
			a.ExportDialog("png")
		}
		if action.Button(c, a.T("tb.exportDrawio"), action.ButtonProps{Variant: action.Text, Size: theme.Small}).Clicked() {
			a.ExportDialog("drawio")
		}

		dividerV(c, tk)

		// History
		if action.ActionIcon(c, lucide.Undo(), action.ActionIconProps{Size: theme.Small, Title: a.T("tb.undo")}).Disabled(!a.Hist.CanUndo()).Clicked() {
			a.Undo()
		}
		if action.ActionIcon(c, lucide.Redo(), action.ActionIconProps{Size: theme.Small, Title: a.T("tb.redo")}).Disabled(!a.Hist.CanRedo()).Clicked() {
			a.Redo()
		}

		dividerV(c, tk)

		// Primary Canvas Tools
		a.toolButton(c, tk, ToolSelect, lucide.MousePointer2(), a.T("tb.toolSelect"))
		a.toolButton(c, tk, ToolHand, lucide.Hand(), a.T("tb.toolHand"))
		a.toolButton(c, tk, ToolConnect, lucide.ArrowUpRight(), a.T("tb.toolConnect"))

		dividerV(c, tk)

		// One-Click Elegant Format + Strategy & Direction quick switches!
		if iconButton(c, lucide.Sparkles(), a.T("tb.format"), action.ButtonProps{
			Variant: action.Primary,
			Size:    theme.Small,
		}).Clicked() {
			a.FormatOneClick("")
		}
		curStrat := a.LayoutStrategy
		if curStrat == "" {
			curStrat = "layered"
		}
		if action.Button(c, strings.ToUpper(curStrat[:1])+curStrat[1:], action.ButtonProps{
			Variant: action.Filled,
			Size:    theme.Small,
		}).Clicked() {
			all := dglayout.RegisteredStrategies()
			if len(all) > 0 {
				next := all[0]
				for i, s := range all {
					if s == curStrat {
						next = all[(i+1)%len(all)]
						break
					}
				}
				a.FormatWithStrategy(next, "")
			}
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

		// Align & Group actions when 2+ nodes selected
		if len(a.SelectedNodes) >= 2 {
			dividerV(c, tk)
			if action.ActionIcon(c, lucide.AlignStartVertical(), action.ActionIconProps{Size: theme.Small, Title: a.T("tb.alignLeft")}).Clicked() {
				a.AlignSelection(graph.AlignLeft)
			}
			if action.ActionIcon(c, lucide.AlignCenterVertical(), action.ActionIconProps{Size: theme.Small, Title: a.T("tb.alignCenterX")}).Clicked() {
				a.AlignSelection(graph.AlignCenterX)
			}
			if action.ActionIcon(c, lucide.AlignStartHorizontal(), action.ActionIconProps{Size: theme.Small, Title: a.T("tb.alignTop")}).Clicked() {
				a.AlignSelection(graph.AlignTop)
			}
			if action.ActionIcon(c, lucide.Group(), action.ActionIconProps{Size: theme.Small, Title: a.T("tb.group")}).Clicked() {
				a.GroupSelection()
			}
		}

		ui.Spacer(c)

		// Command Palette / Node Finder
		if action.ActionIcon(c, lucide.Search(), action.ActionIconProps{Size: theme.Small, Title: a.T("tb.search")}).Clicked() {
			a.PaletteQuery = ""
			a.PaletteOpen = true
		}

		// Zoom Controls
		if action.ActionIcon(c, lucide.ZoomOut(), action.ActionIconProps{Size: theme.Small, Title: a.T("tb.zoomOut")}).Clicked() {
			a.ZoomOut()
		}
		if action.Button(c, fmt.Sprintf("%d%%", int(a.Cam.Zoom*100)), action.ButtonProps{Variant: action.Text, Size: theme.Small}).Clicked() {
			a.ResetZoom()
		}
		if action.ActionIcon(c, lucide.ZoomIn(), action.ActionIconProps{Size: theme.Small, Title: a.T("tb.zoomIn")}).Clicked() {
			a.ZoomIn()
		}
		if action.ActionIcon(c, lucide.Maximize2(), action.ActionIconProps{Size: theme.Small, Title: a.T("tb.zoomFit")}).Clicked() {
			a.ZoomToFit()
		}

		dividerV(c, tk)

		// Grid, Theme, Quick Language Cycle, Plugin/Habit Drawers
		if action.ActionIcon(c, lucide.Grid3x3(), action.ActionIconProps{
			Variant: boolIconVariant(a.ShowGrid),
			Size:    theme.Small,
			Title:   a.T("tb.grid"),
		}).Clicked() {
			a.ShowGrid = !a.ShowGrid
			_ = a.Habits.Update(func(p *habits.Profile) { p.ShowGrid = a.ShowGrid })
		}
		themeIcon := lucide.Sun()
		if a.DarkActive {
			themeIcon = lucide.Moon()
		}
		if action.ActionIcon(c, themeIcon, action.ActionIconProps{Size: theme.Small, Title: a.T("tb.theme")}).Clicked() {
			if a.DarkActive {
				a.ThemeMode = "light"
			} else {
				a.ThemeMode = "dark"
			}
			_ = a.Habits.Update(func(p *habits.Profile) { p.ThemeMode = a.ThemeMode })
		}
		if action.ActionIcon(c, lucide.Languages(), action.ActionIconProps{Size: theme.Small, Title: a.T("drawer.language")}).Clicked() {
			cycleNextLanguage(a)
		}
		if iconButton(c, lucide.GitBranch(), a.T("tb.iconPlugins"), action.ButtonProps{
			Variant: action.Filled,
			Size:    theme.Small,
		}).Clicked() {
			a.PluginsOpen = true
		}
		if iconButton(c, lucide.SlidersHorizontal(), a.T("tb.habits"), action.ButtonProps{
			Variant: action.Filled,
			Size:    theme.Small,
		}).Clicked() {
			a.HabitsOpen = true
		}
	})
}

func cycleNextLanguage(a *App) {
	locs := i18n.SupportedLocales()
	cur := i18n.Resolve(a.Language)
	next := "en"
	for i, l := range locs {
		if l.Code == cur {
			idx := (i + 1) % len(locs)
			if locs[idx].Code == "auto" {
				idx = (idx + 1) % len(locs)
			}
			next = locs[idx].Code
			break
		}
	}
	a.SetLanguage(next)
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
			Placeholder: a.T("side.searchPlaceholder"),
			Size:        theme.Small,
			Variant:     field.InputFilled,
		})
		sb.FillWidth()

		layout.ScrollArea(c, layout.ScrollAreaProps{Fade: true}, func() {
			ui.Column(c).Gap(12).Children(func() {
				// Core draw.io Shapes
				ui.Text(c, a.T("side.coreShapes")).FontSize(10).FontWeight(600).LetterSpacing(0.5).TextColor(tk.TextQuaternary)
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

				// Quick Architecture / Flow Templates
				if q == "" {
					ui.Text(c, a.T("side.templates")).FontSize(10).FontWeight(600).LetterSpacing(0.5).TextColor(tk.TextQuaternary)
					ui.Row(c).Gap(6).Wrap().Children(func() {
						for _, tpl := range []struct{ id, label string }{
							{"arch", "Cloud Arch"},
							{"flow", "Decision Flow"},
							{"microservices", "Microservices"},
							{"org", "Org Tree"},
						} {
							t := tpl
							if action.Button(c, t.label, action.ButtonProps{Variant: action.Filled, Size: theme.Small}).Clicked() {
								a.ApplyBuiltinTemplate(t.id)
							}
						}
					})
				}

				// Favorite Icons (remembered across projects)
				favs := a.Habits.Snapshot().FavoriteIcons
				if len(favs) > 0 && q == "" {
					ui.Text(c, a.T("side.favorites")).FontSize(10).FontWeight(600).LetterSpacing(0.5).TextColor(tk.TextQuaternary)
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
					ui.Text(c, a.T("side.iconLibrary")).FontSize(10).FontWeight(600).LetterSpacing(0.5).TextColor(tk.TextQuaternary)
					ui.Spacer(c)
					if action.Button(c, a.T("side.addGitPack"), action.ButtonProps{Variant: action.Text, Size: theme.Small}).Clicked() {
						a.PluginsOpen = true
					}
				})

				packOpts := []field.SelectOption{{Value: "", Label: a.T("side.allPacks")}}
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
		if action.ActionIcon(c, lucide.Plus(), action.ActionIconProps{Size: theme.Small, Title: a.T("page.add")}).Clicked() {
			a.Hist.Snapshot()
			_, _ = a.Doc().AddPage("", "", true)
			a.SelectedNodes = map[string]bool{}
			a.SelectedEdges = map[string]bool{}
			a.notifyChanged()
		}
		ui.Spacer(c)
		if action.ActionIcon(c, lucide.Copy(), action.ActionIconProps{Size: theme.Small, Title: a.T("page.duplicate")}).Clicked() {
			a.DuplicateActivePage()
		}
		if len(a.Doc().Pages) > 1 {
			if action.ActionIcon(c, lucide.Trash2(), action.ActionIconProps{Size: theme.Small, Title: a.T("page.delete")}).Clicked() {
				a.DeleteActivePage()
			}
		}
	})
}

func (a *App) inspector(c *ui.Context, tk *theme.Tokens) {
	ui.Column(c).Width(274).Shrink(0).Padding(12).Gap(12).
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
		ui.Text(c, a.T("insp.nodeTitle")).FontSize(10).FontWeight(600).LetterSpacing(0.5).TextColor(tk.TextQuaternary)

		label := n.Label
		ui.Column(c).Gap(4).Children(func() {
			ui.Text(c, a.T("insp.label")).FontSize(11).TextColor(tk.TextTertiary)
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
			ui.Text(c, a.T("insp.sublabel")).FontSize(11).TextColor(tk.TextTertiary)
			inp := field.Input(c, &sub, field.InputProps{Size: theme.Small, Placeholder: a.T("insp.sublabelPlaceholder")})
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
			ui.Text(c, a.T("insp.shape")).FontSize(11).TextColor(tk.TextTertiary)
			field.Select(c, &kindStr, field.SelectProps{Options: kindOpts, Size: theme.Small})
			if kindStr != string(n.Kind) {
				a.Hist.Snapshot()
				n.Kind = graph.NodeKind(kindStr)
				a.Page().MarkDirty()
				a.Habits.RecordNodeHabit(n)
				a.notifyChanged()
			}
		})

		ui.Column(c).Gap(4).Children(func() {
			ui.Text(c, a.T("insp.fillColor")).FontSize(11).TextColor(tk.TextTertiary)
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
			ui.Text(c, a.T("insp.strokeColor")).FontSize(11).TextColor(tk.TextTertiary)
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

		dash := string(n.Style.Dash)
		if dash == "" {
			dash = string(graph.DashSolid)
		}
		ui.Column(c).Gap(4).Children(func() {
			ui.Text(c, a.T("insp.dash")).FontSize(11).TextColor(tk.TextTertiary)
			field.Select(c, &dash, field.SelectProps{
				Options: []field.SelectOption{
					{Value: string(graph.DashSolid), Label: a.T("insp.dashSolid")},
					{Value: string(graph.DashDashed), Label: a.T("insp.dashDashed")},
					{Value: string(graph.DashDotted), Label: a.T("insp.dashDotted")},
				},
				Size: theme.Small,
			})
			if dash != string(n.Style.Dash) {
				a.Hist.Snapshot()
				n.Style.Dash = graph.DashStyle(dash)
				a.notifyChanged()
			}
		})

		ui.Row(c).Gap(8).Children(func() {
			rad := n.Style.Radius
			ui.Column(c).Grow(1).Basis(0).Gap(4).Children(func() {
				ui.Text(c, a.T("insp.radius")).FontSize(11).TextColor(tk.TextTertiary)
				field.InputNumber(c, &rad, field.InputNumberProps{Min: 0, Max: 48, Step: 2, Size: theme.Small})
				if rad != n.Style.Radius {
					a.Hist.Snapshot()
					n.Style.Radius = rad
					a.Habits.RecordNodeHabit(n)
					a.notifyChanged()
				}
			})
			fs := n.Style.FontSize
			ui.Column(c).Grow(1).Basis(0).Gap(4).Children(func() {
				ui.Text(c, a.T("insp.fontSize")).FontSize(11).TextColor(tk.TextTertiary)
				field.InputNumber(c, &fs, field.InputNumberProps{Min: 8, Max: 36, Step: 1, Size: theme.Small})
				if fs != n.Style.FontSize {
					a.Hist.Snapshot()
					n.Style.FontSize = fs
					a.Habits.RecordNodeHabit(n)
					a.notifyChanged()
				}
			})
		})

		shadow := n.Style.Shadow
		if choice.Checkbox(c, &shadow, a.T("insp.dropShadow"), choice.CheckboxProps{}).Changed() {
			a.Hist.Snapshot()
			n.Style.Shadow = shadow
			a.Habits.RecordNodeHabit(n)
			a.notifyChanged()
		}

		locked := n.Locked
		if choice.Checkbox(c, &locked, a.T("insp.locked"), choice.CheckboxProps{}).Changed() {
			a.Hist.Snapshot()
			n.Locked = locked
			a.notifyChanged()
		}

		if n.Icon.Valid() {
			ui.Row(c).AlignItems(ui.Center).Gap(6).Children(func() {
				ui.Text(c, "Icon: "+n.Icon.String()).FontSize(11).TextColor(tk.TextSecondary).SingleLine()
				ui.Spacer(c)
				if action.Button(c, a.T("insp.clear"), action.ButtonProps{Variant: action.Text, Size: theme.Small}).Clicked() {
					a.Hist.Snapshot()
					n.Icon = nil
					a.notifyChanged()
				}
			})
		}

		if (n.Kind == graph.KindImage || n.Kind == graph.KindVideo) && n.Media != nil {
			a.mediaInspectorSection(c, tk, n)
		}

		ui.Row(c).Gap(6).Children(func() {
			if action.Button(c, a.T("insp.bringFront"), action.ButtonProps{Variant: action.Default, Size: theme.Small}).Clicked() {
				a.ReorderSelection(graph.ZOrderFront)
			}
			if action.Button(c, a.T("insp.sendBack"), action.ButtonProps{Variant: action.Default, Size: theme.Small}).Clicked() {
				a.ReorderSelection(graph.ZOrderBack)
			}
			if action.Button(c, a.T("insp.duplicate"), action.ButtonProps{Variant: action.Default, Size: theme.Small}).Clicked() {
				a.DuplicateSelection()
			}
		})

		ui.Row(c).Gap(8).Children(func() {
			if action.Button(c, a.T("insp.saveHabit"), action.ButtonProps{Variant: action.Filled, Size: theme.Small}).Clicked() {
				a.Habits.RecordNodeHabit(n)
				a.notifyToast(feedback.ToastSuccess, a.T("toast.habitSaved"), a.T("toast.habitSavedDesc"))
			}
			if action.Button(c, a.T("insp.delete"), action.ButtonProps{Variant: action.Default, Size: theme.Small}).TextColor(tk.Error).Clicked() {
				a.DeleteSelection()
			}
		})
	})
}

func (a *App) edgeInspector(c *ui.Context, tk *theme.Tokens, e *graph.Edge) {
	ui.Column(c).Gap(10).Children(func() {
		ui.Text(c, a.T("insp.edgeTitle")).FontSize(10).FontWeight(600).LetterSpacing(0.5).TextColor(tk.TextQuaternary)

		label := e.Label
		ui.Column(c).Gap(4).Children(func() {
			ui.Text(c, a.T("insp.label")).FontSize(11).TextColor(tk.TextTertiary)
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
			ui.Text(c, a.T("insp.routing")).FontSize(11).TextColor(tk.TextTertiary)
			field.Select(c, &routing, field.SelectProps{
				Options: []field.SelectOption{
					{Value: "orthogonal", Label: a.T("insp.routeOrthogonal")},
					{Value: "curved", Label: a.T("insp.routeCurved")},
					{Value: "straight", Label: a.T("insp.routeStraight")},
				},
				Size: theme.Small,
			})
			if routing != string(e.Routing) {
				a.Hist.Snapshot()
				e.Routing = graph.RoutingStyle(routing)
				a.Page().MarkDirty()
				a.Habits.RecordEdgeHabit(e)
				a.notifyChanged()
			}
		})

		arrowOpts := []field.SelectOption{
			{Value: "arrow", Label: a.T("insp.arrowClassic")},
			{Value: "open", Label: a.T("insp.arrowOpen")},
			{Value: "diamond", Label: a.T("insp.arrowDiamond")},
			{Value: "circle", Label: a.T("insp.arrowCircle")},
			{Value: "none", Label: a.T("insp.arrowNone")},
		}

		sArrow := string(e.StartArrow)
		if sArrow == "" {
			sArrow = "none"
		}
		ui.Column(c).Gap(4).Children(func() {
			ui.Text(c, a.T("insp.startArrow")).FontSize(11).TextColor(tk.TextTertiary)
			field.Select(c, &sArrow, field.SelectProps{Options: arrowOpts, Size: theme.Small})
			if sArrow != string(e.StartArrow) {
				a.Hist.Snapshot()
				e.StartArrow = graph.ArrowStyle(sArrow)
				a.notifyChanged()
			}
		})

		arrow := string(e.EndArrow)
		ui.Column(c).Gap(4).Children(func() {
			ui.Text(c, a.T("insp.endArrow")).FontSize(11).TextColor(tk.TextTertiary)
			field.Select(c, &arrow, field.SelectProps{Options: arrowOpts, Size: theme.Small})
			if arrow != string(e.EndArrow) {
				a.Hist.Snapshot()
				e.EndArrow = graph.ArrowStyle(arrow)
				a.Habits.RecordEdgeHabit(e)
				a.notifyChanged()
			}
		})

		dash := string(e.Style.Dash)
		if dash == "" {
			dash = string(graph.DashSolid)
		}
		ui.Column(c).Gap(4).Children(func() {
			ui.Text(c, a.T("insp.dash")).FontSize(11).TextColor(tk.TextTertiary)
			field.Select(c, &dash, field.SelectProps{
				Options: []field.SelectOption{
					{Value: string(graph.DashSolid), Label: a.T("insp.dashSolid")},
					{Value: string(graph.DashDashed), Label: a.T("insp.dashDashed")},
					{Value: string(graph.DashDotted), Label: a.T("insp.dashDotted")},
				},
				Size: theme.Small,
			})
			if dash != string(e.Style.Dash) {
				a.Hist.Snapshot()
				e.Style.Dash = graph.DashStyle(dash)
				a.notifyChanged()
			}
		})

		ui.Column(c).Gap(4).Children(func() {
			ui.Text(c, a.T("insp.strokeColor")).FontSize(11).TextColor(tk.TextTertiary)
			ui.Row(c).Gap(6).Wrap().Children(func() {
				for _, hex := range []string{"#64748B", "#2563EB", "#10B981", "#F59E0B", "#8B5CF6", "#EF4444"} {
					sw := ui.ButtonBase(c).Label("Edge "+hex).Size(22, 22).Radius(6).
						Background(ui.Hex(hex)).BorderWidth(1).BorderColor(tk.Border)
					if sw.Clicked() {
						a.Hist.Snapshot()
						e.Style.Stroke = hex
						a.notifyChanged()
					}
				}
			})
		})

		ui.Row(c).Gap(8).Children(func() {
			if iconButton(c, lucide.ArrowLeftRight(), a.T("insp.swapEnds"), action.ButtonProps{Variant: action.Filled, Size: theme.Small}).Clicked() {
				a.Hist.Snapshot()
				e.SourceID, e.TargetID = e.TargetID, e.SourceID
				e.SourceAnchor, e.TargetAnchor = e.TargetAnchor, e.SourceAnchor
				a.Page().MarkDirty()
				a.notifyChanged()
			}
			if action.Button(c, a.T("insp.deleteEdge"), action.ButtonProps{Variant: action.Default, Size: theme.Small}).TextColor(tk.Error).Clicked() {
				a.DeleteSelection()
			}
		})
	})
}

func (a *App) documentInspector(c *ui.Context, tk *theme.Tokens) {
	ui.Column(c).Gap(12).Children(func() {
		ui.Text(c, a.T("insp.docTitle")).FontSize(10).FontWeight(600).LetterSpacing(0.5).TextColor(tk.TextQuaternary)

		title := a.Doc().Title
		ui.Column(c).Gap(4).Children(func() {
			ui.Text(c, a.T("insp.diagramTitle")).FontSize(11).TextColor(tk.TextTertiary)
			inp := field.Input(c, &title, field.InputProps{Size: theme.Small})
			inp.FillWidth()
			if title != a.Doc().Title {
				a.Doc().Title = title
				a.notifyChanged()
			}
		})

		pgName := a.Page().Name
		ui.Column(c).Gap(4).Children(func() {
			ui.Text(c, a.T("insp.pageName")).FontSize(11).TextColor(tk.TextTertiary)
			inp := field.Input(c, &pgName, field.InputProps{Size: theme.Small})
			inp.FillWidth()
			if pgName != a.Page().Name && strings.TrimSpace(pgName) != "" {
				a.RenameActivePage(pgName)
			}
		})

		snap := a.Doc().SnapToGrid
		if choice.Checkbox(c, &snap, a.T("insp.snapToGrid"), choice.CheckboxProps{}).Changed() {
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
			ui.Text(c, a.T("insp.stylePreset")).FontSize(11).TextColor(tk.TextTertiary)
			field.Select(c, &preset, field.SelectProps{Options: presetOpts, Size: theme.Small})
			if preset != a.Doc().StylePreset {
				a.Doc().StylePreset = preset
				_ = a.Habits.Update(func(p *habits.Profile) { p.StylePreset = preset })
				a.FormatOneClick("")
			}
		})

		strat := a.LayoutStrategy
		if strat == "" {
			strat = "layered"
		}
		var stratOpts []field.SelectOption
		for _, st := range dglayout.RegisteredStrategies() {
			lbl := a.T("strat." + st)
			if lbl == "strat."+st {
				lbl = st
			}
			stratOpts = append(stratOpts, field.SelectOption{Value: st, Label: lbl})
		}
		ui.Column(c).Gap(4).Children(func() {
			ui.Text(c, a.T("insp.layoutStrategy")).FontSize(11).TextColor(tk.TextTertiary)
			field.Select(c, &strat, field.SelectProps{Options: stratOpts, Size: theme.Small})
			if strat != a.LayoutStrategy {
				a.FormatWithStrategy(strat, "")
			}
		})

		ui.Row(c).Gap(6).Children(func() {
			if iconButton(c, lucide.Sparkles(), a.T("insp.formatLR"), action.ButtonProps{Variant: action.Primary, Size: theme.Small}).Clicked() {
				a.FormatOneClick("LR")
			}
			if action.Button(c, a.T("insp.formatTB"), action.ButtonProps{Variant: action.Default, Size: theme.Small}).Clicked() {
				a.FormatOneClick("TB")
			}
		})

		ui.Text(c, a.T("insp.stats", len(a.Page().Nodes), len(a.Page().Edges), len(a.Doc().Pages))).
			FontSize(11).TextColor(tk.TextTertiary)
	})
}

func (a *App) mediaInspectorSection(c *ui.Context, tk *theme.Tokens, n *graph.Node) {
	asset := a.Doc().Assets[n.Media.AssetID]
	ui.Column(c).Gap(8).Padding(10).Radius(8).Background(tk.FillTertiary).Children(func() {
		ui.Text(c, a.T("media.title")).FontSize(10).FontWeight(600).LetterSpacing(0.5).TextColor(tk.TextQuaternary)
		if asset != nil && asset.NaturalW > 0 {
			meta := fmt.Sprintf("Source: %d × %d px", asset.NaturalW, asset.NaturalH)
			if asset.DurationSec > 0 {
				meta += fmt.Sprintf(" · %.1fs", asset.DurationSec)
			}
			ui.Text(c, meta).FontSize(11).TextColor(tk.TextSecondary)
			ui.Row(c).Gap(6).Children(func() {
				if action.Button(c, a.T("media.fit"), action.ButtonProps{Variant: action.Default, Size: theme.Small}).Clicked() {
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
			ui.Text(c, a.T("media.scaleMode")).FontSize(11).TextColor(tk.TextTertiary)
			field.Select(c, &fit, field.SelectProps{
				Options: []field.SelectOption{
					{Value: "contain", Label: a.T("media.contain")},
					{Value: "cover", Label: a.T("media.cover")},
					{Value: "fill", Label: a.T("media.stretch")},
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
		if choice.Checkbox(c, &showBar, a.T("media.showBar"), choice.CheckboxProps{}).Changed() {
			a.Hist.Snapshot()
			n.Media.ShowBar = showBar
			a.notifyChanged()
		}

		if n.Kind == graph.KindVideo {
			if vp := a.ensureVideoPlayer(n); vp != nil && vp.player != nil {
				lbl := a.T("media.pause")
				if !vp.player.Playing() {
					lbl = a.T("media.play")
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
			if choice.Checkbox(c, &loop, a.T("media.loop"), choice.CheckboxProps{}).Changed() {
				n.Media.Loop = loop
				if vp := a.videoPlayers[n.ID]; vp != nil && vp.player != nil {
					vp.player.SetLoop(loop)
				}
			}
			muted := n.Media.Muted
			if choice.Checkbox(c, &muted, a.T("media.muted"), choice.CheckboxProps{}).Changed() {
				n.Media.Muted = muted
				if vp := a.videoPlayers[n.ID]; vp != nil && vp.player != nil {
					vp.player.SetMuted(muted)
				}
			}
		}
	})
}

func (a *App) statusBar(c *ui.Context, tk *theme.Tokens) {
	ui.Row(c).Height(28).Shrink(0).Gap(10).AlignItems(ui.Center).Padding(0, 12).
		Background(tk.BgContainer).BorderWidth(1, 0, 0, 0).BorderColor(tk.BorderSecondary).Children(func() {
		ui.Text(c, a.T("status.tool", a.Tool)).FontSize(11).TextColor(tk.TextTertiary)
		ui.Text(c, a.T("status.nodes", len(a.Page().Nodes))).FontSize(11).TextColor(tk.TextTertiary)
		ui.Text(c, a.T("status.edges", len(a.Page().Edges))).FontSize(11).TextColor(tk.TextTertiary)
		ui.Text(c, a.T("status.assets", len(a.Doc().Assets))).FontSize(11).TextColor(tk.TextTertiary)
		if a.LastStats.TotalNodes > 0 {
			ui.Text(c, a.T("status.visible", a.LastStats.VisibleNodes, a.LastStats.TotalNodes)).FontSize(11).TextColor(tk.TextQuaternary)
		}
		cx, cy := -a.Cam.X, -a.Cam.Y
		if a.CanvasRect.W > 0 && a.Cam.Zoom > 0 {
			cx += float64(a.CanvasRect.W) / (2 * a.Cam.Zoom)
			cy += float64(a.CanvasRect.H) / (2 * a.Cam.Zoom)
		}
		ui.Text(c, fmt.Sprintf("(%.0f, %.0f) · %d%%", cx, cy, int(a.Cam.Zoom*100))).FontSize(11).TextColor(tk.TextQuaternary)
		if action.Button(c, a.T("status.origin"), action.ButtonProps{Variant: action.Text, Size: theme.Small}).Clicked() {
			a.ResetOrigin()
		}
		if a.Habits.Snapshot().AutoRemember {
			ui.Text(c, a.T("status.habitOn")).FontSize(11).TextColor(tk.Success)
		}
		ui.Spacer(c)
		if a.FilePath != "" {
			ui.Text(c, a.FilePath).FontSize(11).TextColor(tk.TextQuaternary).SingleLine().MaxWidth(260)
		}
		if action.Button(c, a.statusUpdateLabel(), action.ButtonProps{
			Variant: action.Text,
			Size:    theme.Small,
		}).Clicked() {
			a.UpdateOpen = true
		}
	})
}

func (a *App) pluginsDrawer(c *ui.Context, tk *theme.Tokens) {
	feedback.Drawer(c, &a.PluginsOpen, feedback.DrawerProps{
		Placement: feedback.Right,
		Size:      460,
		Title:     a.T("drawer.pluginsTitle"),
	}, func() {
		ui.Column(c).Gap(14).Children(func() {
			ui.Text(c, a.T("drawer.pluginsDesc")).
				FontSize(12).LineHeight(1.5).TextColor(tk.TextSecondary)

			layout.Block(c, layout.BlockProps{Variant: layout.BlockFilled}).Padding(14).Gap(10).Children(func() {
				ui.Text(c, a.T("drawer.installGit")).FontSize(13).FontWeight(600).TextColor(tk.Text)
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
					if iconButton(c, lucide.GitBranch(), a.T("drawer.cloneGit"), action.ButtonProps{Variant: action.Primary, Size: theme.Small}).
						Disabled(a.PluginBusy || strings.TrimSpace(a.GitURL) == "").Clicked() {
						a.InstallGitIconPack()
					}
					if action.Button(c, a.T("drawer.importFolder"), action.ButtonProps{Variant: action.Default, Size: theme.Small}).Clicked() {
						a.ImportIconFolderDialog()
					}
				})
				if a.PluginStatus != "" {
					ui.Text(c, a.PluginStatus).FontSize(11).TextColor(tk.TextSecondary)
				}
			})

			ui.Text(c, a.T("drawer.installedPacks")).FontSize(13).FontWeight(600).TextColor(tk.Text)
			for _, pk := range a.Icons.Packs() {
				pack := pk
				layout.Block(c, layout.BlockProps{Variant: layout.BlockOutlined}).Padding(10).Gap(6).Children(func() {
					ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
						ui.Text(c, pack.Name).FontSize(13).FontWeight(600).TextColor(tk.Text)
						display.Tag(c, fmt.Sprintf("%d icons", len(pack.Icons)), display.TagProps{Size: theme.Small})
						ui.Spacer(c)
						if !pack.Builtin {
							if pack.SourceKind == "git" {
								if action.Button(c, a.T("drawer.sync"), action.ButtonProps{Variant: action.Text, Size: theme.Small}).Clicked() {
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
							if action.Button(c, a.T("drawer.remove"), action.ButtonProps{Variant: action.Text, Size: theme.Small}).TextColor(tk.Error).Clicked() {
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
		Title:     a.T("drawer.habitsTitle"),
	}, func() {
		p := a.Habits.Snapshot()
		ui.Column(c).Gap(12).Children(func() {
			ui.Text(c, a.T("drawer.habitsDesc", a.Habits.Path())).
				FontSize(12).LineHeight(1.5).TextColor(tk.TextSecondary)

			auto := p.AutoRemember
			if choice.Checkbox(c, &auto, a.T("drawer.autoRemember"), choice.CheckboxProps{}).Changed() {
				_ = a.Habits.Update(func(pr *habits.Profile) { pr.AutoRemember = auto })
			}

			lang := a.Language
			var langOpts []field.SelectOption
			for _, loc := range i18n.SupportedLocales() {
				langOpts = append(langOpts, field.SelectOption{Value: loc.Code, Label: loc.NativeName})
			}
			ui.Column(c).Gap(4).Children(func() {
				ui.Text(c, a.T("drawer.language")).FontSize(11).TextColor(tk.TextTertiary)
				field.Select(c, &lang, field.SelectProps{Options: langOpts, Size: theme.Middle})
				if lang != a.Language {
					a.SetLanguage(lang)
				}
			})

			dir := p.PreferredDirection
			ui.Column(c).Gap(4).Children(func() {
				ui.Text(c, a.T("drawer.prefDir")).FontSize(11).TextColor(tk.TextTertiary)
				field.Select(c, &dir, field.SelectProps{
					Options: []field.SelectOption{
						{Value: "LR", Label: "Left -> Right (LR)"},
						{Value: "TB", Label: "Top -> Bottom (TB)"},
						{Value: "RL", Label: "Right -> Left (RL)"},
						{Value: "BT", Label: "Bottom -> Top (BT)"},
					},
					Size: theme.Middle,
				})
				if dir != p.PreferredDirection {
					_ = a.Habits.Update(func(pr *habits.Profile) { pr.PreferredDirection = dir })
				}
			})

			defStrat := p.DefaultStrategy
			var stratOpts []field.SelectOption
			for _, st := range dglayout.RegisteredStrategies() {
				lbl := a.T("strat." + st)
				if lbl == "strat."+st {
					lbl = st
				}
				stratOpts = append(stratOpts, field.SelectOption{Value: st, Label: lbl})
			}
			ui.Column(c).Gap(4).Children(func() {
				ui.Text(c, a.T("drawer.prefStrategy")).FontSize(11).TextColor(tk.TextTertiary)
				field.Select(c, &defStrat, field.SelectProps{Options: stratOpts, Size: theme.Middle})
				if defStrat != p.DefaultStrategy {
					a.LayoutStrategy = defStrat
					_ = a.Habits.Update(func(pr *habits.Profile) { pr.DefaultStrategy = defStrat })
				}
			})

			routing := string(p.DefaultRouting)
			ui.Column(c).Gap(4).Children(func() {
				ui.Text(c, a.T("drawer.defaultRouting")).FontSize(11).TextColor(tk.TextTertiary)
				field.Select(c, &routing, field.SelectProps{
					Options: []field.SelectOption{
						{Value: "orthogonal", Label: a.T("insp.routeOrthogonal")},
						{Value: "curved", Label: a.T("insp.routeCurved")},
						{Value: "straight", Label: a.T("insp.routeStraight")},
					},
					Size: theme.Middle,
				})
				if routing != string(p.DefaultRouting) {
					_ = a.Habits.Update(func(pr *habits.Profile) { pr.DefaultRouting = graph.RoutingStyle(routing) })
				}
			})

			wheel := a.WheelMode
			if wheel == "" {
				wheel = p.WheelMode
			}
			ui.Column(c).Gap(4).Children(func() {
				ui.Text(c, a.T("drawer.wheelMode")).FontSize(11).TextColor(tk.TextTertiary)
				field.Select(c, &wheel, field.SelectProps{
					Options: []field.SelectOption{
						{Value: "pan", Label: a.T("wheel.pan")},
						{Value: "zoom", Label: a.T("wheel.zoom")},
					},
					Size: theme.Middle,
				})
				if wheel != a.WheelMode {
					a.WheelMode = wheel
					_ = a.Habits.Update(func(pr *habits.Profile) { pr.WheelMode = wheel })
				}
			})

			ui.Row(c).Gap(10).Children(func() {
				gx := p.GapX
				ui.Column(c).Grow(1).Basis(0).Gap(4).Children(func() {
					ui.Text(c, a.T("drawer.gapX")).FontSize(11).TextColor(tk.TextTertiary)
					field.InputNumber(c, &gx, field.InputNumberProps{Min: 20, Max: 240, Step: 4, Size: theme.Small})
					if gx != p.GapX {
						_ = a.Habits.Update(func(pr *habits.Profile) { pr.GapX = gx })
					}
				})
				gy := p.GapY
				ui.Column(c).Grow(1).Basis(0).Gap(4).Children(func() {
					ui.Text(c, a.T("drawer.gapY")).FontSize(11).TextColor(tk.TextTertiary)
					field.InputNumber(c, &gy, field.InputNumberProps{Min: 20, Max: 240, Step: 4, Size: theme.Small})
					if gy != p.GapY {
						_ = a.Habits.Update(func(pr *habits.Profile) { pr.GapY = gy })
					}
				})
			})

			mediaMax := p.DefaultMediaMax
			ui.Column(c).Gap(4).Children(func() {
				ui.Text(c, a.T("drawer.mediaMax")).FontSize(11).TextColor(tk.TextTertiary)
				field.InputNumber(c, &mediaMax, field.InputNumberProps{Min: 160, Max: 1920, Step: 40, Size: theme.Small})
				if mediaMax != p.DefaultMediaMax {
					_ = a.Habits.Update(func(pr *habits.Profile) { pr.DefaultMediaMax = mediaMax })
				}
			})

			ui.Row(c).Gap(8).Children(func() {
				if iconButton(c, lucide.Sparkles(), a.T("drawer.applyHabits"), action.ButtonProps{Variant: action.Primary, Size: theme.Middle}).Clicked() {
					a.Habits.ApplyToDocument(a.Doc())
					a.FormatOneClick("")
					a.HabitsOpen = false
				}
				if action.Button(c, a.T("drawer.resetDefaults"), action.ButtonProps{Variant: action.Default, Size: theme.Middle}).TextColor(tk.Error).Clicked() {
					_ = a.Habits.Reset()
					a.Language = a.Habits.Snapshot().Language
					a.WheelMode = a.Habits.Snapshot().WheelMode
					a.LayoutStrategy = a.Habits.Snapshot().DefaultStrategy
					a.notifyToast(feedback.ToastInfo, a.T("toast.habitsReset"), a.T("toast.habitsResetDesc"))
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
