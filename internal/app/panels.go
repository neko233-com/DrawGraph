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
	ui.Row(c).Height(h).Shrink(0).Gap(4).AlignItems(ui.Center).
		Padding(0, bar.Right+10, 0, bar.Left+10).
		Background(tk.BgContainer).
		BorderWidth(0, 0, 1, 0).BorderColor(tk.BorderSecondary).
		DragWindow().Children(func() {
		// Vercel Gradient Triangle Brand
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Box(c).Size(24, 24).Radius(7).LinearGradient(ui.LinearGradient{
				From:  vercelPink,
				To:    vercelBlue,
				Angle: 135,
				Oklab: true,
			}).Center().Children(func() {
				ui.Icon(c, lucide.Triangle()).FontSize(12).TextColor(ui.Hex("#FFFFFF"))
			})
			ui.Text(c, a.T("app.name")).FontSize(13).FontWeight(700).TextColor(tk.Text).SingleLine()
		})

		dividerV(c, tk)

		// File + Import Actions inside subtle pill frame
		ui.Row(c).Gap(2).Padding(2).Radius(8).Background(tk.FillQuaternary).Children(func() {
			if action.ActionIcon(c, lucide.FilePlus(), action.ActionIconProps{Size: theme.Small, Title: a.T("tb.new")}).Clicked() {
				a.NewBlankDocument()
			}
			if action.ActionIcon(c, lucide.FolderOpen(), action.ActionIconProps{Size: theme.Small, Title: a.T("tb.open")}).Clicked() {
				a.OpenFromDialog()
			}
			if action.ActionIcon(c, lucide.Save(), action.ActionIconProps{Size: theme.Small, Title: a.T("tb.save")}).Clicked() {
				a.SaveToCurrentOrDialog(false)
			}
			if action.ActionIcon(c, lucide.ImagePlus(), action.ActionIconProps{Size: theme.Small, Title: a.T("tb.media")}).Clicked() {
				a.ImportMediaFromDialog()
			}
			if action.ActionIcon(c, lucide.FileCode(), action.ActionIconProps{Size: theme.Small, Title: a.T("tb.mermaid")}).Clicked() {
				a.MermaidOpen = true
			}
		})

		// Color-coded Export Badges
		if vercelTintPill(c, tk, vercelEmerald, nil, a.T("tb.exportSVG"), false).Clicked() {
			a.ExportDialog("svg")
		}
		if vercelTintPill(c, tk, vercelBlue, nil, a.T("tb.exportPNG"), false).Clicked() {
			a.ExportDialog("png")
		}
		if vercelTintPill(c, tk, vercelViolet, nil, a.T("tb.exportDrawio"), false).Clicked() {
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

		// Segmented Canvas Tools Capsule
		ui.Row(c).Gap(2).Padding(2).Radius(8).Background(tk.FillQuaternary).BorderWidth(1).BorderColor(tk.BorderSecondary).Children(func() {
			a.toolButton(c, tk, ToolSelect, lucide.MousePointer2(), a.T("tb.toolSelect"))
			a.toolButton(c, tk, ToolHand, lucide.Hand(), a.T("tb.toolHand"))
			a.toolButton(c, tk, ToolConnect, lucide.ArrowUpRight(), a.T("tb.toolConnect"))
		})

		dividerV(c, tk)

		// Vercel Signature Gradient CTA: One-Click Format + Color-Coded Strategy Pill
		if vercelGradientButton(c, lucide.Sparkles(), a.T("tb.format"), true).Clicked() {
			a.FormatOneClick("")
		}
		curStrat := a.LayoutStrategy
		if curStrat == "" {
			curStrat = "layered"
		}
		sAccent := strategyAccent(curStrat)
		sLabel := a.T("strat." + curStrat)
		if sLabel == "strat."+curStrat {
			sLabel = strings.ToUpper(curStrat[:1]) + curStrat[1:]
		}
		if b := ui.ButtonBase(c).Label("Strategy "+curStrat).Height(26).Padding(0, 8).Radius(7).
			Row().AlignItems(ui.Center).Gap(5).
			Background(sAccent.Alpha(0.11)).BorderWidth(1).BorderColor(sAccent.Alpha(0.35)).
			Children(func() {
				ui.Box(c).Size(7, 7).Radius(4).Background(sAccent)
				ui.Text(c, sLabel).FontSize(11).FontWeight(600).TextColor(sAccent).SingleLine()
			}); b.Clicked() {
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
		if vercelTintPill(c, tk, vercelCyan, nil, dirLabel, false).Clicked() {
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

		// Command Palette Search
		if action.ActionIcon(c, lucide.Search(), action.ActionIconProps{Size: theme.Small, Title: a.T("tb.search")}).Clicked() {
			a.PaletteQuery = ""
			a.PaletteOpen = true
		}

		// Zoom Pill
		ui.Row(c).Gap(1).Padding(2).Radius(7).Background(tk.FillQuaternary).AlignItems(ui.Center).Children(func() {
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
		})

		// Grid, Theme, Shortcuts, Language, Plugins, Habits
		gridBtn := action.ActionIcon(c, lucide.Grid3x3(), action.ActionIconProps{
			Variant: boolIconVariant(a.ShowGrid),
			Size:    theme.Small,
			Title:   a.T("tb.grid"),
		})
		if a.ShowGrid {
			gridBtn.Background(vercelCyan.Alpha(0.14))
		}
		if gridBtn.Clicked() {
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

		if action.ActionIcon(c, lucide.Keyboard(), action.ActionIconProps{Size: theme.Small, Title: a.T("tb.shortcuts")}).Clicked() {
			a.ShortcutsOpen = true
		}

		if action.ActionIcon(c, lucide.Languages(), action.ActionIconProps{Size: theme.Small, Title: a.T("drawer.language")}).Clicked() {
			cycleNextLanguage(a)
		}

		if vercelTintPill(c, tk, vercelViolet, lucide.GitBranch(), a.T("tb.iconPlugins"), a.PluginsOpen).Clicked() {
			a.PluginsOpen = true
		}
		if vercelTintPill(c, tk, vercelBlue, lucide.SlidersHorizontal(), a.T("tb.habits"), a.HabitsOpen).Clicked() {
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
	if toolPill(c, tk, a.Tool == tool, icon, title).Clicked() {
		a.Tool = tool
	}
}

func (a *App) sidebar(c *ui.Context, tk *theme.Tokens) {
	ui.Column(c).Width(264).Shrink(0).Padding(10).Gap(10).
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
				// Categorized Color Shapes Grid
				ui.Row(c).AlignItems(ui.Center).Gap(6).Children(func() {
					ui.Box(c).Size(6, 6).Radius(3).Background(vercelBlue)
					ui.Text(c, a.T("side.coreShapes")).FontSize(10).FontWeight(700).LetterSpacing(0.5).TextColor(tk.TextSecondary)
				})
				q := strings.ToLower(strings.TrimSpace(a.IconSearch))
				ui.Row(c).Gap(6).Wrap().Children(func() {
					for _, k := range graph.AllNodeKinds {
						rawName := string(k)
						dispName := a.T("shape." + rawName)
						if dispName == "shape."+rawName {
							dispName = rawName
						}
						if q != "" && !strings.Contains(rawName, q) && !strings.Contains(strings.ToLower(dispName), q) {
							continue
						}
						accent := shapeCategoryColor(k)
						btn := ui.ButtonBase(c).Label(rawName).Height(26).Padding(0, 8).Radius(7).
							Row().AlignItems(ui.Center).Gap(5).
							Background(accent.Alpha(0.08)).BorderWidth(1).BorderColor(accent.Alpha(0.24)).
							Children(func() {
								ui.Box(c).Size(6, 6).Radius(3).Background(accent)
								ui.Text(c, dispName).FontSize(11).FontWeight(500).TextColor(tk.Text).SingleLine()
							})
						if btn.Clicked() {
							a.QuickInsertShape(k, "", "")
						}
					}
				})

				// Colorful Vercel Quick Templates Cards
				if q == "" {
					ui.Row(c).AlignItems(ui.Center).Gap(6).Children(func() {
						ui.Box(c).Size(6, 6).Radius(3).Background(vercelPink)
						ui.Text(c, a.T("side.templates")).FontSize(10).FontWeight(700).LetterSpacing(0.5).TextColor(tk.TextSecondary)
					})
					tpls := []struct {
						id, titleKey, subKey string
						accent               ui.Color
					}{
						{"arch", "tpl.arch", "tpl.archSub", vercelBlue},
						{"flow", "tpl.flow", "tpl.flowSub", vercelEmerald},
						{"microservices", "tpl.microservices", "tpl.microservicesSub", vercelViolet},
						{"org", "tpl.org", "tpl.orgSub", vercelAmber},
					}
					ui.Column(c).Gap(6).Children(func() {
						for i := 0; i < len(tpls); i += 2 {
							ui.Row(c).Gap(6).Children(func() {
								for j := i; j < i+2 && j < len(tpls); j++ {
									t := tpls[j]
									card := ui.ButtonBase(c).Label(t.id).Grow(1).Basis(0).Padding(8, 9).Radius(8).
										Column().Gap(2).
										Background(t.accent.Alpha(0.09)).BorderWidth(1).BorderColor(t.accent.Alpha(0.28)).
										Children(func() {
											ui.Row(c).AlignItems(ui.Center).Gap(5).Children(func() {
												ui.Box(c).Size(6, 6).Radius(3).Background(t.accent)
												ui.Text(c, a.T(t.titleKey)).FontSize(11).FontWeight(600).TextColor(t.accent).SingleLine()
											})
											sub := a.T(t.subKey)
											if sub != t.subKey {
												ui.Text(c, sub).FontSize(10).TextColor(tk.TextTertiary).SingleLine()
											}
										})
									if card.Clicked() {
										a.ApplyBuiltinTemplate(t.id)
									}
								}
							})
						}
					})
				}

				// Favorite Icons (remembered across projects) in Semantic Colors
				favs := a.Habits.Snapshot().FavoriteIcons
				if len(favs) > 0 && q == "" {
					ui.Row(c).AlignItems(ui.Center).Gap(6).Children(func() {
						ui.Box(c).Size(6, 6).Radius(3).Background(vercelCyan)
						ui.Text(c, a.T("side.favorites")).FontSize(10).FontWeight(700).LetterSpacing(0.5).TextColor(tk.TextSecondary)
					})
					ui.Row(c).Gap(6).Wrap().Children(func() {
						for _, ref := range favs {
							svg, ok := a.Icons.ParsedSVG(ref)
							if !ok {
								continue
							}
							col := iconRefColor(ref)
							if b := ui.ButtonBase(c).Label("Fav "+ref).Size(36, 36).Radius(8).
								Background(col.Alpha(0.12)).BorderWidth(1).BorderColor(col.Alpha(0.28)).
								Center().Children(func() {
								ui.Icon(c, svg).FontSize(18).TextColor(col)
							}); b.Clicked() {
								a.insertOrAssignIcon(ref)
							}
						}
					})
				}

				// Pack Filter + Git Install Shortcut
				ui.Row(c).AlignItems(ui.Center).Gap(6).Children(func() {
					ui.Box(c).Size(6, 6).Radius(3).Background(vercelViolet)
					ui.Text(c, a.T("side.iconLibrary")).FontSize(10).FontWeight(700).LetterSpacing(0.5).TextColor(tk.TextSecondary)
					ui.Spacer(c)
					if vercelTintPill(c, tk, vercelViolet, nil, a.T("side.addGitPack"), false).Clicked() {
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
						col := iconCategoryColor(ic)
						cell := ui.ButtonBase(c).Label("Icon " + ic.Ref()).
							Width(74).Height(64).Radius(9).Column().Center().Gap(4).Padding(4).
							Background(tk.BgLayout).BorderWidth(1).BorderColor(tk.BorderSecondary)
						cell.Children(func() {
							ui.Box(c).Size(28, 28).Radius(7).Background(col.Alpha(0.14)).Center().Children(func() {
								ui.Icon(c, svg).FontSize(16).TextColor(col)
							})
							ui.Text(c, ic.Name).FontSize(10).FontWeight(500).TextColor(tk.TextSecondary).SingleLine().MaxWidth(66)
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
			bg := tk.FillQuaternary
			border := tk.BorderSecondary
			txtCol := tk.TextSecondary
			if active {
				bg = vercelBlue.Alpha(0.12)
				border = vercelBlue.Alpha(0.45)
				txtCol = vercelBlue
			}
			tab := ui.ButtonBase(c).Label(p.Name).Height(24).Padding(0, 10).Radius(6).
				Row().AlignItems(ui.Center).Gap(5).Background(bg).BorderWidth(1).BorderColor(border).
				Children(func() {
					if active {
						ui.Box(c).Size(6, 6).Radius(3).Background(vercelBlue)
					}
					ui.Text(c, p.Name).FontSize(11).FontWeight(600).TextColor(txtCol).SingleLine()
				})
			if tab.Clicked() {
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

func inspectorHeader(c *ui.Context, tk *theme.Tokens, title string, accent ui.Color) {
	ui.Row(c).AlignItems(ui.Center).Gap(6).Children(func() {
		ui.Box(c).Size(8, 8).Radius(4).Background(accent)
		ui.Text(c, title).FontSize(10).FontWeight(700).LetterSpacing(0.6).TextColor(accent)
	})
}

func (a *App) nodeInspector(c *ui.Context, tk *theme.Tokens, n *graph.Node) {
	ui.Column(c).Gap(10).Children(func() {
		inspectorHeader(c, tk, a.T("insp.nodeTitle"), vercelBlue)

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
			lbl := a.T("shape." + string(k))
			if lbl == "shape."+string(k) {
				lbl = string(k)
			}
			kindOpts = append(kindOpts, field.SelectOption{Value: string(k), Label: lbl})
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
				for _, hex := range []string{"#FFFFFF", "#EFF6FF", "#ECFEFF", "#F5F3FF", "#ECFDF5", "#FDF2F8", "#FEF3C7", "#0F172A"} {
					sel := strings.EqualFold(n.Style.Fill, hex)
					bw := float32(1)
					bc := tk.Border
					if sel {
						bw = 2
						bc = vercelBlue
					}
					sw := ui.ButtonBase(c).Label("Fill "+hex).Size(22, 22).Radius(11).
						Background(ui.Hex(hex)).BorderWidth(bw).BorderColor(bc)
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
				for _, hex := range []string{"#0070F3", "#7928CA", "#FF0080", "#06B6D4", "#10B981", "#F59E0B", "#0F172A"} {
					sel := strings.EqualFold(n.Style.Stroke, hex)
					bw := float32(1)
					bc := tk.Border
					if sel {
						bw = 2
						bc = vercelBlue
					}
					sw := ui.ButtonBase(c).Label("Stroke "+hex).Size(22, 22).Radius(11).
						Background(ui.Hex(hex)).BorderWidth(bw).BorderColor(bc)
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
			if vercelTintPill(c, tk, vercelCyan, nil, a.T("insp.bringFront"), false).Clicked() {
				a.ReorderSelection(graph.ZOrderFront)
			}
			if vercelTintPill(c, tk, vercelIndigo, nil, a.T("insp.sendBack"), false).Clicked() {
				a.ReorderSelection(graph.ZOrderBack)
			}
			if vercelTintPill(c, tk, vercelBlue, nil, a.T("insp.duplicate"), false).Clicked() {
				a.DuplicateSelection()
			}
		})

		ui.Row(c).Gap(8).Children(func() {
			if vercelTintPill(c, tk, vercelEmerald, nil, a.T("insp.saveHabit"), false).Clicked() {
				a.Habits.RecordNodeHabit(n)
				a.notifyToast(feedback.ToastSuccess, a.T("toast.habitSaved"), a.T("toast.habitSavedDesc"))
			}
			if vercelTintPill(c, tk, vercelPink, nil, a.T("insp.delete"), false).Clicked() {
				a.DeleteSelection()
			}
		})
	})
}

func (a *App) edgeInspector(c *ui.Context, tk *theme.Tokens, e *graph.Edge) {
	ui.Column(c).Gap(10).Children(func() {
		inspectorHeader(c, tk, a.T("insp.edgeTitle"), vercelViolet)

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
				for _, hex := range []string{"#0070F3", "#7928CA", "#FF0080", "#06B6D4", "#10B981", "#F59E0B", "#64748B"} {
					sw := ui.ButtonBase(c).Label("Edge "+hex).Size(22, 22).Radius(11).
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
			if vercelTintPill(c, tk, vercelBlue, lucide.ArrowLeftRight(), a.T("insp.swapEnds"), false).Clicked() {
				a.Hist.Snapshot()
				e.SourceID, e.TargetID = e.TargetID, e.SourceID
				e.SourceAnchor, e.TargetAnchor = e.TargetAnchor, e.SourceAnchor
				a.Page().MarkDirty()
				a.notifyChanged()
			}
			if vercelTintPill(c, tk, vercelPink, nil, a.T("insp.deleteEdge"), false).Clicked() {
				a.DeleteSelection()
			}
		})
	})
}

func (a *App) documentInspector(c *ui.Context, tk *theme.Tokens) {
	ui.Column(c).Gap(12).Children(func() {
		inspectorHeader(c, tk, a.T("insp.docTitle"), vercelCyan)

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
			if vercelGradientButton(c, lucide.Sparkles(), a.T("insp.formatLR"), true).Clicked() {
				a.FormatOneClick("LR")
			}
			if vercelTintPill(c, tk, vercelCyan, nil, a.T("insp.formatTB"), false).Clicked() {
				a.FormatOneClick("TB")
			}
		})

		ui.Text(c, a.T("insp.stats", len(a.Page().Nodes), len(a.Page().Edges), len(a.Doc().Pages))).
			FontSize(11).TextColor(tk.TextTertiary)
	})
}

func (a *App) mediaInspectorSection(c *ui.Context, tk *theme.Tokens, n *graph.Node) {
	asset := a.Doc().Assets[n.Media.AssetID]
	ui.Column(c).Gap(8).Padding(10).Radius(9).Background(vercelEmerald.Alpha(0.07)).
		BorderWidth(1).BorderColor(vercelEmerald.Alpha(0.24)).Children(func() {
		inspectorHeader(c, tk, a.T("media.title"), vercelEmerald)
		if asset != nil && asset.NaturalW > 0 {
			meta := fmt.Sprintf("Source: %d × %d px", asset.NaturalW, asset.NaturalH)
			if asset.DurationSec > 0 {
				meta += fmt.Sprintf(" · %.1fs", asset.DurationSec)
			}
			ui.Text(c, meta).FontSize(11).TextColor(tk.TextSecondary)
			ui.Row(c).Gap(6).Children(func() {
				if vercelTintPill(c, tk, vercelEmerald, nil, a.T("media.fit"), false).Clicked() {
					a.ResizeMediaNode(n, false)
				}
				if vercelTintPill(c, tk, vercelBlue, nil, fmt.Sprintf("1:1 (%dx%d)", asset.NaturalW, asset.NaturalH), false).Clicked() {
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
				if vercelGradientButton(c, nil, lbl, true).Clicked() {
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
	ui.Row(c).Height(30).Shrink(0).Gap(10).AlignItems(ui.Center).Padding(0, 12).
		Background(tk.BgContainer).BorderWidth(1, 0, 0, 0).BorderColor(tk.BorderSecondary).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(5).Children(func() {
			ui.Box(c).Size(6, 6).Radius(3).Background(vercelBlue)
			ui.Text(c, a.T("status.tool", a.Tool)).FontSize(11).TextColor(tk.TextSecondary)
		})
		ui.Text(c, a.T("status.nodes", len(a.Page().Nodes))).FontSize(11).TextColor(tk.TextTertiary)
		ui.Text(c, a.T("status.edges", len(a.Page().Edges))).FontSize(11).TextColor(tk.TextTertiary)
		ui.Text(c, a.T("status.assets", len(a.Doc().Assets))).FontSize(11).TextColor(tk.TextTertiary)
		if a.LastStats.TotalNodes > 0 {
			ui.Text(c, a.T("status.visible", a.LastStats.VisibleNodes, a.LastStats.TotalNodes)).FontSize(11).TextColor(vercelCyan)
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
			ui.Row(c).AlignItems(ui.Center).Gap(5).Padding(2, 7).Radius(99).
				Background(vercelEmerald.Alpha(0.10)).BorderWidth(1).BorderColor(vercelEmerald.Alpha(0.28)).Children(func() {
				ui.Box(c).Size(6, 6).Radius(3).Background(vercelEmerald)
				ui.Text(c, a.T("status.habitOn")).FontSize(10).FontWeight(600).TextColor(vercelEmerald)
			})
		}
		ui.Spacer(c)
		if a.FilePath != "" {
			ui.Text(c, a.FilePath).FontSize(11).TextColor(tk.TextQuaternary).SingleLine().MaxWidth(240)
		}
		if b := ui.ButtonBase(c).Label("KeysHelp").Height(20).Padding(0, 7).Radius(99).
			Row().AlignItems(ui.Center).Gap(4).
			Background(vercelCyan.Alpha(0.10)).BorderWidth(1).BorderColor(vercelCyan.Alpha(0.30)).
			Children(func() {
				ui.Icon(c, lucide.Keyboard()).FontSize(11).TextColor(vercelCyan)
				ui.Text(c, "?").FontSize(10).FontWeight(700).TextColor(vercelCyan)
			}); b.Clicked() {
			a.ShortcutsOpen = true
		}
		if vercelTintPill(c, tk, vercelBlue, nil, a.statusUpdateLabel(), a.UpdateState == updateAvailable).Clicked() {
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
			vercelSpectrumBar(c, 3)
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
					if vercelGradientButton(c, lucide.GitBranch(), a.T("drawer.cloneGit"), true).
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
						display.Tag(c, fmt.Sprintf("%d icons", len(pack.Icons)), display.TagProps{Size: theme.Small, Color: display.TagColor("#7928CA")})
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
			vercelSpectrumBar(c, 3)
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
				if vercelGradientButton(c, lucide.Sparkles(), a.T("drawer.applyHabits"), false).Clicked() {
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

func dividerV(c *ui.Context, tk *theme.Tokens) {
	ui.Box(c).Width(1).Height(18).Background(tk.BorderSecondary).Margin(0, 2)
}

func boolIconVariant(active bool) action.ActionIconVariant {
	if active {
		return action.ActionIconFilled
	}
	return action.ActionIconBorderless
}
