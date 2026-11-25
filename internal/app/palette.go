package app

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/action"
	"github.com/lobehub/lobe-mygo/pkg/display"
	"github.com/lobehub/lobe-mygo/pkg/feedback"
	"github.com/lobehub/lobe-mygo/pkg/field"
	"github.com/lobehub/lobe-mygo/pkg/layout"
	"github.com/lobehub/lobe-mygo/pkg/theme"

	"github.com/neko233-com/DrawGraph/internal/i18n"
	dglayout "github.com/neko233-com/DrawGraph/internal/layout"
)

func (a *App) paletteModal(c *ui.Context, tk *theme.Tokens) {
	if !a.PaletteOpen {
		return
	}
	feedback.Modal(c, &a.PaletteOpen, feedback.ModalProps{
		Title:    a.T("palette.title"),
		Width:    540,
		NoFooter: true,
	}, func() {
		ui.Column(c).Gap(10).Children(func() {
			sb := field.SearchBar(c, &a.PaletteQuery, field.SearchBarProps{
				Placeholder: a.T("palette.placeholder"),
				Size:        theme.Middle,
				Variant:     field.InputFilled,
			})
			sb.FillWidth()

			q := strings.ToLower(strings.TrimSpace(a.PaletteQuery))
			layout.ScrollArea(c, layout.ScrollAreaProps{Fade: true}, func() {
				ui.Column(c).Gap(6).Children(func() {
					// 1. Jump to matching nodes across any page on the infinite canvas!
					shownNodes := 0
					for _, pg := range a.Doc().Pages {
						for _, n := range pg.Nodes {
							if shownNodes >= 18 {
								break
							}
							hay := strings.ToLower(n.Label + " " + n.ID + " " + string(n.Kind) + " " + n.Sublabel)
							if q != "" && !strings.Contains(hay, q) {
								continue
							}
							shownNodes++
							pageID, nodeID := pg.ID, n.ID
							row := ui.ButtonBase(c).Label("Jump "+nodeID).FillWidth().Padding(8, 10).Radius(8).
								Row().AlignItems(ui.Center).Gap(8).Background(tk.FillTertiary)
							row.Children(func() {
								display.Tag(c, string(n.Kind), display.TagProps{Size: theme.Small})
								lbl := n.Label
								if lbl == "" {
									lbl = n.ID
								}
								ui.Text(c, lbl).FontSize(12).FontWeight(600).TextColor(tk.Text).SingleLine()
								ui.Spacer(c)
								ui.Text(c, fmt.Sprintf("%s · (%.0f, %.0f)", pg.Name, n.X, n.Y)).FontSize(11).TextColor(tk.TextTertiary)
							})
							if row.Clicked() {
								a.Doc().ActivePageID = pageID
								a.FocusNode(nodeID)
								a.PaletteOpen = false
							}
						}
					}

					// 2. Quick layout algorithms
					for _, strat := range dglayout.RegisteredStrategies() {
						label := a.T("strat." + strat)
						if label == "strat."+strat {
							label = strat
						}
						if q != "" && !strings.Contains(strings.ToLower(strat+" "+label+" layout format"), q) {
							continue
						}
						s := strat
						btn := ui.ButtonBase(c).Label("Run Layout "+s).FillWidth().Padding(8, 10).Radius(8).
							Row().AlignItems(ui.Center).Gap(8).Background(tk.BgContainer).
							BorderWidth(1).BorderColor(tk.BorderSecondary)
						btn.Children(func() {
							ui.Text(c, a.T("tb.format")+": "+label).FontSize(12).TextColor(tk.Text)
							ui.Spacer(c)
							ui.Text(c, s).FontSize(11).TextColor(tk.TextQuaternary)
						})
						if btn.Clicked() {
							a.FormatWithStrategy(s, "")
							a.PaletteOpen = false
						}
					}

					// 3. Language switches
					for _, loc := range i18n.SupportedLocales() {
						if q != "" && !strings.Contains(strings.ToLower(loc.Code+" "+loc.NativeName+" lang language 语言 语言"), q) {
							continue
						}
						code := loc.Code
						btn := ui.ButtonBase(c).Label("Lang "+code).FillWidth().Padding(8, 10).Radius(8).
							Row().AlignItems(ui.Center).Gap(8).Background(tk.BgContainer).
							BorderWidth(1).BorderColor(tk.BorderSecondary)
						btn.Children(func() {
							ui.Text(c, a.T("drawer.language")+": "+loc.NativeName).FontSize(12).TextColor(tk.Text)
							ui.Spacer(c)
							ui.Text(c, code).FontSize(11).TextColor(tk.TextQuaternary)
						})
						if btn.Clicked() {
							a.SetLanguage(code)
							a.PaletteOpen = false
						}
					}
				})
			}).Height(320)

			ui.Row(c).Justify(ui.End).Children(func() {
				if action.Button(c, a.T("palette.close"), action.ButtonProps{Variant: action.Default, Size: theme.Small}).Clicked() {
					a.PaletteOpen = false
				}
			})
		})
	})
}

func (a *App) mermaidModal(c *ui.Context, tk *theme.Tokens) {
	if !a.MermaidOpen {
		return
	}
	if feedback.Modal(c, &a.MermaidOpen, feedback.ModalProps{
		Title:  a.T("mermaid.title"),
		Width:  540,
		OkText: a.T("mermaid.importBtn"),
	}, func() {
		ui.Column(c).Gap(10).Children(func() {
			ta := field.TextArea(c, &a.MermaidText, field.TextAreaProps{
				Placeholder: a.T("mermaid.placeholder"),
				MinRows:     8,
				MaxRows:     12,
			})
			ta.FillWidth()
			ui.Row(c).Gap(6).Children(func() {
				if action.Button(c, "Cloud Arch", action.ButtonProps{Variant: action.Filled, Size: theme.Small}).Clicked() {
					a.MermaidText = "flowchart LR\n  subgraph VPC[Cloud VPC]\n    GW([API Gateway]) --> Auth{JWT Auth}\n    Auth -->|OK| Orders[Orders Service]\n    Orders --> DB[(Orders DB)]\n  end\n  Client((Browser)) --> GW"
				}
				if action.Button(c, "Decision Flow", action.ButtonProps{Variant: action.Filled, Size: theme.Small}).Clicked() {
					a.MermaidText = "flowchart TD\n  Start((Start)) --> Check{Valid Input?}\n  Check -->|Yes| Run[Execute Job]\n  Check -->|No| Err[Return 400]\n  Run --> Done((Done))"
				}
			})
		})
	}) {
		if err := a.ImportMermaidText(a.MermaidText); err != nil {
			a.notifyToast(feedback.ToastError, "Mermaid Error", err.Error())
		} else {
			a.MermaidOpen = false
		}
	}
}
