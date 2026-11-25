//go:build windows

package update

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// Apply launches a detached Windows updater script that waits briefly for the caller to exit,
// installs the verified MSI (clearing Start Menu NotContentIndexed), and optionally relaunches GUI.
func Apply(msiPath string, opts ApplyOptions) error {
	if _, err := os.Stat(msiPath); err != nil {
		return fmt.Errorf("update msi not found: %w", err)
	}
	exe, _ := os.Executable()
	guiExe := `C:\Program Files\DrawGraph\drawgraph-gui.exe`
	if exe != "" && strings.EqualFold(filepath.Base(exe), "drawgraph-gui.exe") {
		guiExe = exe
	}
	dir := filepath.Join(DefaultCacheDir(), "apply")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	script := filepath.Join(dir, "apply-update.cmd")
	modeFlag := "/passive"
	if opts.Silent {
		modeFlag = "/qn"
	}
	quotedMSI := strings.ReplaceAll(msiPath, `"`, `""`)
	quotedGUI := strings.ReplaceAll(guiExe, `"`, `""`)

	var sb strings.Builder
	sb.WriteString("@echo off\r\n")
	sb.WriteString("ping 127.0.0.1 -n 3 >nul\r\n")
	fmt.Fprintf(&sb, "msiexec /i \"%s\" %s /norestart\r\n", quotedMSI, modeFlag)
	sb.WriteString("attrib -I +A \"%ProgramData%\\Microsoft\\Windows\\Start Menu\\Programs\\DrawGraph\" >nul 2>&1\r\n")
	sb.WriteString("attrib -I +A \"%ProgramData%\\Microsoft\\Windows\\Start Menu\\Programs\\DrawGraph\\DrawGraph.lnk\" >nul 2>&1\r\n")
	if opts.Relaunch {
		fmt.Fprintf(&sb, "if exist \"%s\" start \"\" \"%s\"\r\n", quotedGUI, quotedGUI)
	}
	sb.WriteString("del \"%~f0\" >nul 2>&1\r\n")

	if err := os.WriteFile(script, []byte(sb.String()), 0o644); err != nil {
		return err
	}
	cmd := exec.Command("cmd.exe", "/c", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}
