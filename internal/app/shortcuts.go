package app

import (
	"github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/action"
	"github.com/lobehub/lobe-mygo/pkg/feedback"
	"github.com/lobehub/lobe-mygo/pkg/layout"
	"github.com/lobehub/lobe-mygo/pkg/theme"
)

type shortcutEntry struct {
	Key   string
	Label string
	Color ui.Color
}

func (a *App) shortcutsModal(c *ui.Context, tk *theme.Tokens) {
	if !a.ShortcutsOpen {
		return
	}
	mod := platformMod()
	feedback.Modal(c, &a.ShortcutsOpen, feedback.ModalProps{
		Title:    a.T("keys.title"),
		Width:    660,
		NoFooter: true,
	}, func() {
		ui.Column(c).Gap(12).Children(func() {
			vercelSpectrumBar(c, 3)
			ui.Text(c, a.T("keys.subtitle")).FontSize(12).TextColor(tk.TextSecondary)

			renderGroup := func(title string, accent ui.Color, items []shortcutEntry) {
				layout.Block(c, layout.BlockProps{Variant: layout.BlockFilled}).Padding(10, 12).Gap(8).
					BorderWidth(1).BorderColor(accent.Alpha(0.24)).Children(func() {
					ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
						ui.Box(c).Size(8, 8).Radius(4).Background(accent)
						ui.Text(c, title).FontSize(11).FontWeight(700).TextColor(accent)
					})
					ui.Column(c).Gap(6).Children(func() {
						for _, it := range items {
							item := it
							ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
								ui.Text(c, item.Label).FontSize(12).TextColor(tk.Text)
								ui.Spacer(c)
								kbdBadge(c, tk, item.Key, item.Color)
							})
						}
					})
				})
			}

			layout.ScrollArea(c, layout.ScrollAreaProps{Fade: true}, func() {
				ui.Row(c).Gap(10).AlignItems(ui.Stretch).Children(func() {
					ui.Column(c).Grow(1).Basis(0).Gap(10).Children(func() {
						renderGroup(a.T("keys.catTools"), vercelBlue, []shortcutEntry{
							{Key: "V", Label: a.T("keys.select"), Color: vercelBlue},
							{Key: "H / Space", Label: a.T("keys.hand"), Color: vercelCyan},
							{Key: "C", Label: a.T("keys.connect"), Color: vercelViolet},
							{Key: "R · O · D · T", Label: a.T("keys.shapes"), Color: vercelPink},
							{Key: "F2 / DblClick", Label: a.T("keys.editText"), Color: vercelEmerald},
						})
						renderGroup(a.T("keys.catEdit"), vercelViolet, []shortcutEntry{
							{Key: mod + "+C / X / V", Label: a.T("keys.copyPaste"), Color: vercelViolet},
							{Key: mod + "+D", Label: a.T("keys.duplicate"), Color: vercelBlue},
							{Key: mod + "+G / ⇧G", Label: a.T("keys.group"), Color: vercelCyan},
							{Key: mod + "+] / [", Label: a.T("keys.zorder"), Color: vercelAmber},
							{Key: mod + "+L", Label: a.T("keys.lock"), Color: vercelPink},
						})
					})
					ui.Column(c).Grow(1).Basis(0).Gap(10).Children(func() {
						renderGroup(a.T("keys.catLayout"), vercelEmerald, []shortcutEntry{
							{Key: mod + "+Shift+F", Label: a.T("keys.format"), Color: vercelBlue},
							{Key: "Alt+1..6", Label: a.T("keys.strategies"), Color: vercelEmerald},
							{Key: mod + "+= / - / 0 / F", Label: a.T("keys.zoom"), Color: vercelCyan},
							{Key: "Shift+2", Label: a.T("keys.zoomSel"), Color: vercelViolet},
							{Key: "G", Label: a.T("keys.grid"), Color: vercelAmber},
						})
						renderGroup(a.T("keys.catPanels"), vercelPink, []shortcutEntry{
							{Key: mod + "+K / F", Label: a.T("keys.palette"), Color: vercelPink},
							{Key: mod + "+B / \\ / M", Label: a.T("keys.sidebars"), Color: vercelBlue},
							{Key: mod + "+Shift+M", Label: a.T("mermaid.title"), Color: vercelEmerald},
							{Key: mod + "+/ or ?", Label: a.T("keys.shortcutsModal"), Color: vercelCyan},
						})
					})
				})
			}).Height(360)

			ui.Row(c).Justify(ui.End).Children(func() {
				if action.Button(c, a.T("palette.close"), action.ButtonProps{Variant: action.Default, Size: theme.Small}).Clicked() {
					a.ShortcutsOpen = false
				}
			})
		})
	})
}
