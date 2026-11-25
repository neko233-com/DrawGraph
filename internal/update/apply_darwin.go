//go:build darwin

package update

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Apply mounts the verified macOS DMG via hdiutil, replaces /Applications/DrawGraph.app
// via ditto, detaches the image, refreshes any CLI symlink, and optionally relaunches the app.
func Apply(dmgPath string, opts ApplyOptions) error {
	if _, err := os.Stat(dmgPath); err != nil {
		return fmt.Errorf("update dmg not found: %w", err)
	}
	mnt, err := os.MkdirTemp("", "drawgraph-dmg-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(mnt)

	attach := exec.Command("hdiutil", "attach", "-nobrowse", "-readonly", "-mountpoint", mnt, dmgPath)
	if out, err := attach.CombinedOutput(); err != nil {
		return fmt.Errorf("hdiutil attach: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	defer func() {
		_ = exec.Command("hdiutil", "detach", "-force", mnt).Run()
	}()

	srcApp := filepath.Join(mnt, "DrawGraph.app")
	if _, err := os.Stat(srcApp); err != nil {
		return fmt.Errorf("DrawGraph.app not found inside %s", dmgPath)
	}
	dstApp := "/Applications/DrawGraph.app"
	if err := exec.Command("ditto", srcApp, dstApp).Run(); err != nil {
		return fmt.Errorf("install to /Applications: %w", err)
	}
	// Refresh CLI symlink in ~/.local/bin if user installed it there.
	home, _ := os.UserHomeDir()
	if home != "" {
		cliSrc := filepath.Join(dstApp, "Contents", "Helpers", "drawgraph")
		cliLink := filepath.Join(home, ".local", "bin", "drawgraph")
		if _, err := os.Lstat(cliLink); err == nil {
			_ = os.Remove(cliLink)
			_ = os.Symlink(cliSrc, cliLink)
		}
	}
	if opts.Relaunch {
		_ = exec.Command("open", "-n", dstApp).Start()
	}
	return nil
}
