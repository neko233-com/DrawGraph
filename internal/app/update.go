package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/action"
	"github.com/lobehub/lobe-mygo/pkg/choice"
	"github.com/lobehub/lobe-mygo/pkg/feedback"
	"github.com/lobehub/lobe-mygo/pkg/field"
	"github.com/lobehub/lobe-mygo/pkg/layout"
	"github.com/lobehub/lobe-mygo/pkg/lucide"
	"github.com/lobehub/lobe-mygo/pkg/theme"

	"github.com/neko233-com/DrawGraph/internal/habits"
	"github.com/neko233-com/DrawGraph/internal/update"
	"github.com/neko233-com/DrawGraph/internal/version"
)

type updateState string

const (
	updateIdle        updateState = ""
	updateChecking    updateState = "checking"
	updateAvailable   updateState = "available"
	updateDownloading updateState = "downloading"
	updateReady       updateState = "ready"
	updateUptodate    updateState = "uptodate"
	updateError       updateState = "error"
)

// StartUpdateSchedule schedules an automatic startup release check when AutoCheckUpdates is enabled.
func (a *App) StartUpdateSchedule() {
	if a.Host == nil || a.Host.Window() == nil {
		return
	}
	prof := a.Habits.Snapshot()
	if !prof.AutoCheckUpdates || version.IsDev() {
		return
	}
	interval := time.Duration(max(1, prof.UpdateCheckHours)) * time.Hour
	if prof.LastUpdateCheck > 0 && time.Since(time.Unix(prof.LastUpdateCheck, 0)) < interval {
		return
	}
	time.AfterFunc(3*time.Second, func() {
		a.CheckUpdates(false)
	})
}

// CheckUpdates checks GitHub Releases for a newer version for the current OS/arch.
func (a *App) CheckUpdates(manual bool) {
	if a.UpdateState == updateChecking {
		return
	}
	a.UpdateState = updateChecking
	a.UpdateErr = ""
	prof := a.Habits.Snapshot()
	channel := prof.UpdateChannel

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		rel, err := update.Check(ctx, update.CheckOptions{
			Current: version.Version,
			Channel: channel,
		})
		a.Host.Update(func() {
			if err != nil {
				a.UpdateState = updateError
				a.UpdateErr = err.Error()
				if manual {
					a.notifyToast(feedback.ToastError, "Update Check Failed", err.Error())
				}
				return
			}
			_ = a.Habits.Update(func(p *habits.Profile) {
				p.LastUpdateCheck = time.Now().Unix()
			})
			if rel == nil {
				a.UpdateState = updateUptodate
				a.UpdateRelease = nil
				if manual {
					a.notifyToast(feedback.ToastSuccess, "Up to Date", "DrawGraph "+version.String()+" is the latest version")
				}
				return
			}
			a.UpdateState = updateAvailable
			a.UpdateRelease = rel
			a.UpdateErr = ""
			if !manual && prof.SkippedVersion == rel.Version {
				return
			}
			a.notifyToast(feedback.ToastInfo, "Update Available", "DrawGraph v"+rel.Version+" is ready")
			if a.Host != nil && a.Host.Window() != nil {
				n := mygo.NewNotification(mygo.NotificationOptions{
					ID:       "drawgraph-update",
					Title:    "DrawGraph v" + rel.Version + " Available",
					Body:     "Click to review release notes and install.",
					Subtitle: "DrawGraph",
				})
				n.OnClick(func() {
					a.Host.Update(func() {
						a.UpdateOpen = true
						if w := a.Host.Window(); w != nil {
							w.Show()
							w.Focus()
						}
					})
				})
				go func() { _ = n.Show() }()
			}
			if prof.AutoDownloadUpdates {
				a.DownloadUpdate()
			}
		})
	}()
}

// DownloadUpdate downloads and SHA-256-verifies the platform installer for UpdateRelease.
func (a *App) DownloadUpdate() {
	if a.UpdateRelease == nil {
		a.CheckUpdates(true)
		return
	}
	if a.UpdateState == updateDownloading || a.UpdateState == updateReady {
		return
	}
	rel := a.UpdateRelease
	a.UpdateState = updateDownloading
	a.UpdateProgress = 0
	a.UpdateErr = ""

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		var lastReport time.Time
		path, err := update.Download(ctx, rel, update.DefaultCacheDir(), func(done, total int64) {
			if total <= 0 {
				return
			}
			now := time.Now()
			if done < total && now.Sub(lastReport) < 120*time.Millisecond {
				return
			}
			lastReport = now
			pct := float64(done) * 100 / float64(total)
			a.Host.Update(func() {
				a.UpdateProgress = pct
			})
		})
		a.Host.Update(func() {
			if err != nil {
				a.UpdateState = updateError
				a.UpdateErr = err.Error()
				a.notifyToast(feedback.ToastError, "Update Download Failed", err.Error())
				return
			}
			a.UpdatePath = path
			a.UpdateProgress = 100
			a.UpdateState = updateReady
			a.notifyToast(feedback.ToastSuccess, "Update Ready", "DrawGraph v"+rel.Version+" downloaded & verified")
		})
	}()
}

// InstallUpdate installs the downloaded verified MSI/DMG and exits the running app so files can swap cleanly.
func (a *App) InstallUpdate() {
	if a.UpdatePath == "" {
		a.DownloadUpdate()
		return
	}
	if err := update.Apply(a.UpdatePath, update.ApplyOptions{Relaunch: true}); err != nil {
		a.UpdateState = updateError
		a.UpdateErr = err.Error()
		a.notifyToast(feedback.ToastError, "Install Failed", err.Error())
		return
	}
	a.UpdateQuitRequested = true
	if a.Host != nil && a.Host.Window() != nil {
		mygo.App.Quit()
	}
}

func (a *App) statusUpdateLabel() string {
	switch a.UpdateState {
	case updateChecking:
		return a.T("update.checking")
	case updateAvailable:
		if a.UpdateRelease != nil {
			return "Update v" + a.UpdateRelease.Version
		}
		return a.T("update.available")
	case updateDownloading:
		return fmt.Sprintf("Downloading %.0f%%", a.UpdateProgress)
	case updateReady:
		return a.T("update.restart")
	case updateUptodate:
		return a.T("update.uptodate")
	case updateError:
		return a.T("update.error")
	default:
		return version.String()
	}
}

func (a *App) updateDrawer(c *ui.Context, tk *theme.Tokens) {
	feedback.Drawer(c, &a.UpdateOpen, feedback.DrawerProps{
		Placement: feedback.Right,
		Size:      440,
		Title:     a.T("drawer.updatesTitle"),
	}, func() {
		prof := a.Habits.Snapshot()
		ui.Column(c).Gap(14).Children(func() {
			layout.Block(c, layout.BlockProps{Variant: layout.BlockFilled}).Padding(14).Gap(8).Children(func() {
				ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
					ui.Text(c, "DrawGraph "+version.String()).FontSize(15).FontWeight(600).TextColor(tk.Text)
					ui.Spacer(c)
					ui.Text(c, "Commit: "+version.Commit).FontSize(11).TextColor(tk.TextTertiary)
				})
				if a.UpdateState != "" {
					col := tk.TextSecondary
					switch a.UpdateState {
					case updateAvailable, updateReady:
						col = tk.Primary
					case updateUptodate:
						col = tk.Success
					case updateError:
						col = tk.Error
					}
					ui.Text(c, "Status: "+a.statusUpdateLabel()).FontSize(12).TextColor(col)
				}
				if a.UpdateState == updateDownloading {
					feedback.Progress(c, float32(a.UpdateProgress), feedback.ProgressProps{
						Size:     theme.Small,
						Status:   feedback.ProgressActive,
						ShowInfo: true,
					})
				}
				if a.UpdateErr != "" {
					ui.Text(c, a.UpdateErr).FontSize(11).TextColor(tk.Error).MaxLines(3)
				}
				if a.UpdateRelease != nil {
					ui.Text(c, fmt.Sprintf("Latest: v%s (%s)", a.UpdateRelease.Version, a.UpdateRelease.AssetName)).
						FontSize(12).FontWeight(600).TextColor(tk.Primary)
					if strings.TrimSpace(a.UpdateRelease.Notes) != "" {
						ui.Text(c, a.UpdateRelease.Notes).FontSize(11).LineHeight(1.5).TextColor(tk.TextSecondary).MaxLines(6)
					}
				}
				ui.Row(c).Gap(8).Children(func() {
					if iconButton(c, lucide.RefreshCw(), a.T("drawer.checkNow"), action.ButtonProps{
						Variant: action.Default,
						Size:    theme.Small,
					}).Disabled(a.UpdateState == updateChecking || a.UpdateState == updateDownloading).Clicked() {
						a.CheckUpdates(true)
					}
					if a.UpdateState == updateAvailable {
						if action.Button(c, a.T("drawer.downloadUpdate"), action.ButtonProps{
							Variant: action.Primary,
							Size:    theme.Small,
						}).Clicked() {
							a.DownloadUpdate()
						}
					}
					if a.UpdateState == updateReady {
						if action.Button(c, a.T("drawer.installRestart"), action.ButtonProps{
							Variant: action.Primary,
							Size:    theme.Small,
						}).Clicked() {
							a.InstallUpdate()
						}
					}
				})
			})

			layout.Block(c, layout.BlockProps{Variant: layout.BlockOutlined}).Padding(14).Gap(10).Children(func() {
				ui.Text(c, a.T("drawer.updateSettings")).FontSize(13).FontWeight(600).TextColor(tk.Text)
				autoChk := prof.AutoCheckUpdates
				if choice.Checkbox(c, &autoChk, a.T("drawer.autoCheckUpdates"), choice.CheckboxProps{}).Changed() {
					_ = a.Habits.Update(func(p *habits.Profile) { p.AutoCheckUpdates = autoChk })
				}
				autoDL := prof.AutoDownloadUpdates
				if choice.Checkbox(c, &autoDL, a.T("drawer.autoDownload"), choice.CheckboxProps{}).Changed() {
					_ = a.Habits.Update(func(p *habits.Profile) { p.AutoDownloadUpdates = autoDL })
				}
				ch := prof.UpdateChannel
				ui.Column(c).Gap(4).Children(func() {
					ui.Text(c, a.T("drawer.releaseChannel")).FontSize(11).TextColor(tk.TextTertiary)
					field.Select(c, &ch, field.SelectProps{
						Options: []field.SelectOption{
							{Value: "stable", Label: a.T("drawer.channelStable")},
							{Value: "prerelease", Label: a.T("drawer.channelPrerelease")},
						},
						Size: theme.Middle,
					})
					if ch != prof.UpdateChannel {
						_ = a.Habits.Update(func(p *habits.Profile) { p.UpdateChannel = ch })
					}
				})
			})
		})
	})
}
