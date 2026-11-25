//go:build !windows && !darwin

package update

import "errors"

// Apply is only implemented for Windows MSI and macOS DMG releases.
func Apply(_ string, _ ApplyOptions) error {
	return errors.New("update: self-install is supported on Windows (amd64 MSI) and macOS (arm64 DMG)")
}
