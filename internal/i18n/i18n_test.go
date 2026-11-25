package i18n_test

import (
	"testing"

	"github.com/neko233-com/DrawGraph/internal/i18n"
)

func TestMultiLanguageTranslationAndExtension(t *testing.T) {
	if got := i18n.T("en", "tb.format"); got != "Format" {
		t.Fatalf("expected English Format, got %q", got)
	}
	if got := i18n.T("zh-CN", "tb.format"); got != "一键排版" {
		t.Fatalf("expected zh-CN 一键排版, got %q", got)
	}
	if got := i18n.T("zh_TW.UTF-8", "tb.format"); got != "一鍵排版" {
		t.Fatalf("expected zh-TW 一鍵排版, got %q", got)
	}
	if got := i18n.T("ja_JP", "tb.format"); got != "自動整形" {
		t.Fatalf("expected ja 自動整形, got %q", got)
	}
	if got := i18n.T("ko-KR", "tb.format"); got != "자동 정렬" {
		t.Fatalf("expected ko 자동 정렬, got %q", got)
	}
	// Missing key in secondary locale must fall back cleanly to English.
	if got := i18n.T("ko", "tb.exportDrawio"); got != ".drawio" {
		t.Fatalf("expected English fallback '.drawio', got %q", got)
	}
	// Format arguments work cleanly.
	if got := i18n.T("zh-CN", "status.nodes", 42); got != "节点: 42" {
		t.Fatalf("expected formatted translation, got %q", got)
	}
	// Env override controls "auto" resolution deterministically.
	t.Setenv("DRAWGRAPH_LANG", "ja")
	if got := i18n.Resolve("auto"); got != "ja" {
		t.Fatalf("expected auto->ja with env override, got %q", got)
	}
	// Custom locale extension point works cleanly.
	i18n.RegisterLocale("fr", "Français", map[string]string{"tb.format": "Mise en page"})
	if got := i18n.T("fr", "tb.format"); got != "Mise en page" {
		t.Fatalf("expected custom fr locale, got %q", got)
	}
}
