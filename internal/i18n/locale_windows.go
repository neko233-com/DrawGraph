//go:build windows

package i18n

import (
	"os"
	"strings"
	"syscall"
)

func detectOSLocale() string {
	for _, k := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return v
		}
	}
	k32 := syscall.NewLazyDLL("kernel32.dll")
	proc := k32.NewProc("GetUserDefaultUILanguage")
	if err := proc.Find(); err == nil {
		r1, _, _ := proc.Call()
		langID := uint16(r1)
		switch langID {
		case 0x0804, 0x1004:
			return "zh-CN"
		case 0x0404, 0x0C04, 0x1404:
			return "zh-TW"
		case 0x0411:
			return "ja"
		case 0x0412:
			return "ko"
		}
		primary := langID & 0x03FF
		switch primary {
		case 0x04: // LANG_CHINESE
			return "zh-CN"
		case 0x11: // LANG_JAPANESE
			return "ja"
		case 0x12: // LANG_KOREAN
			return "ko"
		case 0x09: // LANG_ENGLISH
			return "en"
		}
	}
	return "en"
}
