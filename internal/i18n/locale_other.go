//go:build !windows

package i18n

import (
	"os"
	"strings"
)

func detectOSLocale() string {
	for _, k := range []string{"LC_ALL", "LC_MESSAGES", "LANG", "LANGUAGE"} {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			if i := strings.IndexByte(v, ':'); i > 0 {
				v = v[:i]
			}
			return v
		}
	}
	return "en"
}
