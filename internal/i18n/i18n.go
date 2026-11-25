// Package i18n provides multi-language localization with OS auto-detection,
// fallback chains, and extensible locale registration.
package i18n

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
)

// LocaleInfo describes a supported UI language option.
type LocaleInfo struct {
	Code       string `json:"code"`
	NativeName string `json:"nativeName"`
}

var (
	mu           sync.RWMutex
	bundles      = map[string]map[string]string{}
	localeNames  = map[string]string{}
	detectOnce   sync.Once
	detectedLang string
)

func init() {
	RegisterLocale("en", "English", dictEN)
	RegisterLocale("zh-CN", "简体中文", dictZHCN)
	RegisterLocale("zh-TW", "繁體中文", dictZHTW)
	RegisterLocale("ja", "日本語", dictJA)
	RegisterLocale("ko", "한국어", dictKO)
}

// RegisterLocale registers or merges a translation dictionary for code (extensible).
func RegisterLocale(code, nativeName string, entries map[string]string) {
	code = NormalizeTag(code)
	if code == "" || code == "auto" {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	if bundles[code] == nil {
		bundles[code] = make(map[string]string, len(entries))
	}
	for k, v := range entries {
		bundles[code][k] = v
	}
	if strings.TrimSpace(nativeName) != "" {
		localeNames[code] = nativeName
	} else if localeNames[code] == "" {
		localeNames[code] = code
	}
}

// SupportedLocales returns all registered UI locales plus "auto" first.
func SupportedLocales() []LocaleInfo {
	mu.RLock()
	defer mu.RUnlock()
	out := []LocaleInfo{{Code: "auto", NativeName: "Auto (System)"}}
	var codes []string
	for c := range bundles {
		codes = append(codes, c)
	}
	sort.Slice(codes, func(i, j int) bool {
		order := map[string]int{"en": 1, "zh-CN": 2, "zh-TW": 3, "ja": 4, "ko": 5}
		oi, oki := order[codes[i]]
		oj, okj := order[codes[j]]
		if oki && okj {
			return oi < oj
		}
		if oki != okj {
			return oki
		}
		return codes[i] < codes[j]
	})
	for _, c := range codes {
		out = append(out, LocaleInfo{Code: c, NativeName: localeNames[c]})
	}
	return out
}

// NormalizeTag normalizes locale codes such as "zh_CN.UTF-8", "zh-hans", "ja_JP" -> "zh-CN", "ja".
func NormalizeTag(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	if i := strings.IndexAny(s, ".@"); i > 0 {
		s = s[:i]
	}
	s = strings.ReplaceAll(s, "_", "-")
	low := strings.ToLower(s)
	switch low {
	case "auto", "system":
		return "auto"
	case "en", "en-us", "en-gb", "c", "posix":
		return "en"
	case "zh", "zh-cn", "zh-sg", "zh-hans", "zh-hans-cn":
		return "zh-CN"
	case "zh-tw", "zh-hk", "zh-mo", "zh-hant", "zh-hant-tw":
		return "zh-TW"
	case "ja", "ja-jp":
		return "ja"
	case "ko", "ko-kr":
		return "ko"
	}
	// Prefix match for unknown regional tags like en-AU or ja-Latn.
	if strings.HasPrefix(low, "zh-hant") {
		return "zh-TW"
	}
	if strings.HasPrefix(low, "zh") {
		return "zh-CN"
	}
	if strings.HasPrefix(low, "ja") {
		return "ja"
	}
	if strings.HasPrefix(low, "ko") {
		return "ko"
	}
	if strings.HasPrefix(low, "en") {
		return "en"
	}
	return s
}

// DetectSystemLocale detects the host OS / environment language.
// DRAWGRAPH_LANG always takes immediate precedence (useful for scripts and deterministic tests).
func DetectSystemLocale() string {
	if env := strings.TrimSpace(os.Getenv("DRAWGRAPH_LANG")); env != "" {
		tag := NormalizeTag(env)
		if tag != "" && tag != "auto" {
			return tag
		}
	}
	detectOnce.Do(func() {
		tag := NormalizeTag(detectOSLocale())
		if tag == "" || tag == "auto" {
			tag = "en"
		}
		detectedLang = tag
	})
	return detectedLang
}

// Resolve resolves preference ("auto" or concrete tag) into a concrete registered locale tag.
func Resolve(pref string) string {
	tag := NormalizeTag(pref)
	if tag == "" || tag == "auto" {
		tag = DetectSystemLocale()
	}
	mu.RLock()
	defer mu.RUnlock()
	if _, ok := bundles[tag]; ok {
		return tag
	}
	if i := strings.IndexByte(tag, '-'); i > 0 {
		base := tag[:i]
		if _, ok := bundles[base]; ok {
			return base
		}
	}
	return "en"
}

// T resolves pref to a locale and translates key using fmt.Sprintf if args are passed.
func T(pref, key string, args ...any) string {
	loc := Resolve(pref)
	mu.RLock()
	tmpl := ""
	if m := bundles[loc]; m != nil {
		tmpl = m[key]
	}
	if tmpl == "" && loc != "en" {
		tmpl = bundles["en"][key]
	}
	mu.RUnlock()
	if tmpl == "" {
		tmpl = key
	}
	if len(args) == 0 {
		return tmpl
	}
	return fmt.Sprintf(tmpl, args...)
}

var dictEN = map[string]string{
	"app.name":                 "DrawGraph",
	"app.untitled":             "Untitled Diagram",
	"menu.file":                "File",
	"menu.file.new":            "New Diagram",
	"menu.file.open":           "Open…",
	"menu.file.save":           "Save",
	"menu.file.saveAs":         "Save As…",
	"menu.file.importMermaid":  "Import Mermaid…",
	"menu.file.exportSVG":      "Export SVG…",
	"menu.file.exportPNG":      "Export PNG…",
	"menu.file.exportDrawio":   "Export draw.io…",
	"menu.edit":                "Edit",
	"menu.format":              "Format",
	"menu.format.oneClick":     "One-Click Format",
	"menu.format.layered":      "Layered DAG",
	"menu.format.tree":         "Tree / Org Chart",
	"menu.format.grid":         "Clean Grid",
	"menu.format.radial":       "Radial Topology",
	"menu.format.swimlane":     "Swimlanes",
	"menu.format.compact":      "Tidy Overlaps",
	"menu.format.lr":           "Layout Left to Right",
	"menu.format.tb":           "Layout Top to Bottom",
	"menu.format.icons":        "Icon Plugins (Git)…",
	"menu.format.habits":       "Persistent User Habits…",
	"menu.view":                "View",
	"menu.view.zoomIn":         "Zoom In",
	"menu.view.zoomOut":        "Zoom Out",
	"menu.view.zoom100":        "Zoom to 100%",
	"menu.view.zoomFit":        "Zoom to Fit",
	"menu.view.commandPalette": "Find / Command Palette…",
	"menu.view.sidebar":        "Toggle Sidebar",
	"menu.view.inspector":      "Toggle Inspector",
	"menu.view.minimap":        "Toggle Minimap",
	"menu.language":            "Language",
	"menu.help":                "Help",
	"menu.help.checkUpdates":   "Check for Updates…",
	"menu.help.updateSettings": "Software Update Settings…",
	"tb.new":                   "New Diagram",
	"tb.open":                  "Open (.drawgraph / .drawio)",
	"tb.save":                  "Save",
	"tb.media":                 "Media",
	"tb.mermaid":               "Mermaid",
	"tb.exportSVG":             "Export SVG",
	"tb.exportPNG":             "PNG",
	"tb.exportDrawio":          ".drawio",
	"tb.undo":                  "Undo",
	"tb.redo":                  "Redo",
	"tb.toolSelect":            "Select (V)",
	"tb.toolHand":              "Pan (H)",
	"tb.toolConnect":           "Connect (C)",
	"tb.format":                "Format",
	"tb.alignLeft":             "Align Left",
	"tb.alignCenterX":          "Align Center X",
	"tb.alignTop":              "Align Top",
	"tb.group":                 "Group",
	"tb.ungroup":               "Ungroup",
	"tb.search":                "Search / Commands (Ctrl+K)",
	"tb.zoomOut":               "Zoom Out",
	"tb.zoomIn":                "Zoom In",
	"tb.zoomFit":               "Zoom to Fit",
	"tb.grid":                  "Toggle Grid",
	"tb.theme":                 "Toggle Theme",
	"tb.iconPlugins":           "Icon Plugins",
	"tb.habits":                "Habits",
	"side.searchPlaceholder":   "Search shapes & icons...",
	"side.coreShapes":          "CORE SHAPES",
	"side.favorites":           "REMEMBERED FAVORITE ICONS",
	"side.iconLibrary":         "ICON LIBRARY",
	"side.addGitPack":          "+ Git Pack",
	"side.allPacks":            "All Enabled Packs",
	"side.templates":           "TEMPLATES",
	"page.add":                 "Add Page",
	"page.rename":              "Rename Page",
	"page.duplicate":           "Duplicate Page",
	"page.delete":              "Delete Page",
	"insp.nodeTitle":           "NODE PROPERTIES",
	"insp.edgeTitle":           "CONNECTOR PROPERTIES",
	"insp.docTitle":            "CANVAS & ONE-CLICK STYLE",
	"insp.label":               "Label",
	"insp.sublabel":            "Sublabel",
	"insp.sublabelPlaceholder": "Optional subtitle",
	"insp.shape":               "Shape",
	"insp.fillColor":           "Fill Color",
	"insp.strokeColor":         "Stroke Color",
	"insp.dash":                "Line Style",
	"insp.dashSolid":           "Solid",
	"insp.dashDashed":          "Dashed",
	"insp.dashDotted":          "Dotted",
	"insp.radius":              "Corner Radius",
	"insp.fontSize":            "Font Size",
	"insp.dropShadow":          "Drop shadow",
	"insp.locked":              "Lock position",
	"insp.clear":               "Clear",
	"insp.saveHabit":           "Save as Habit",
	"insp.delete":              "Delete",
	"insp.duplicate":           "Duplicate",
	"insp.bringFront":          "Front",
	"insp.sendBack":            "Back",
	"insp.routing":             "Routing",
	"insp.routeOrthogonal":     "Orthogonal (draw.io)",
	"insp.routeCurved":         "Curved Bezier",
	"insp.routeStraight":       "Straight",
	"insp.startArrow":          "Start Arrow",
	"insp.endArrow":            "End Arrow",
	"insp.swapEnds":            "Swap Direction",
	"insp.arrowClassic":        "Classic Arrow",
	"insp.arrowOpen":           "Open Arrow",
	"insp.arrowDiamond":        "Diamond",
	"insp.arrowCircle":         "Circle",
	"insp.arrowNone":           "None",
	"insp.deleteEdge":          "Delete Connector",
	"insp.diagramTitle":        "Diagram Title",
	"insp.pageName":            "Active Page Name",
	"insp.snapToGrid":          "Snap to grid",
	"insp.stylePreset":         "Style Preset",
	"insp.layoutStrategy":      "Layout Algorithm",
	"insp.formatLR":            "Format LR",
	"insp.formatTB":            "Format TB",
	"insp.stats":               "Nodes: %d  ·  Edges: %d  ·  Pages: %d",
	"media.title":              "HD MEDIA & RESOLUTION",
	"media.fit":                "HD Fit",
	"media.scaleMode":          "Scale Mode",
	"media.contain":            "Contain (Full Aspect)",
	"media.cover":              "Cover (Crop Fill)",
	"media.stretch":            "Stretch Fill",
	"media.showBar":            "Show caption footer bar",
	"media.play":               "Play Video",
	"media.pause":              "Pause Video",
	"media.loop":               "Loop video",
	"media.muted":              "Muted",
	"canvas.dropHint":          "Drop images or videos to create high-resolution nodes",
	"status.tool":              "Tool: %s",
	"status.nodes":             "Nodes: %d",
	"status.edges":             "Edges: %d",
	"status.assets":            "Assets: %d",
	"status.visible":           "Visible: %d/%d",
	"status.origin":            "Origin",
	"status.habitOn":           "Habit Memory: ON",
	"drawer.pluginsTitle":      "Icon Plugins (Git & Local SVG)",
	"drawer.pluginsDesc":       "Extend DrawGraph with any Git icon repository or local SVG folder. Installed packs stay available across every diagram.",
	"drawer.installGit":        "Install from Git URL",
	"drawer.cloneGit":          "Clone Git Pack",
	"drawer.importFolder":      "Import Local Folder…",
	"drawer.installedPacks":    "Installed Icon Packs",
	"drawer.sync":              "Sync",
	"drawer.remove":            "Remove",
	"drawer.habitsTitle":       "Persistent User Habits (Global)",
	"drawer.habitsDesc":        "Saved globally to %s. New projects and One-Click Format inherit these defaults automatically.",
	"drawer.autoRemember":      "Auto-remember style, icon, and layout habits across projects",
	"drawer.language":          "Interface Language",
	"drawer.prefDir":           "Preferred Layout Direction",
	"drawer.prefStrategy":      "Default Layout Strategy",
	"drawer.defaultRouting":    "Default Connector Routing",
	"drawer.wheelMode":         "Mouse Wheel Action",
	"wheel.pan":                "Pan Canvas (Ctrl+Wheel zooms)",
	"wheel.zoom":               "Zoom Canvas Directly",
	"drawer.gapX":              "Horizontal Gap",
	"drawer.gapY":              "Vertical Gap",
	"drawer.mediaMax":          "Imported HD Image/Video Initial Max Size (DIPs)",
	"drawer.applyHabits":       "Apply Habits Now",
	"drawer.resetDefaults":     "Reset Defaults",
	"drawer.updatesTitle":      "Software Updates",
	"drawer.checkNow":          "Check Now",
	"drawer.downloadUpdate":    "Download Update",
	"drawer.installRestart":    "Install & Restart",
	"drawer.updateSettings":    "Automatic & Manual Update Settings",
	"drawer.autoCheckUpdates":  "Automatically check for updates on startup",
	"drawer.autoDownload":      "Automatically download verified installers in background",
	"drawer.releaseChannel":    "Release Channel",
	"drawer.channelStable":     "Stable Releases",
	"drawer.channelPrerelease": "Preview / Prerelease",
	"update.checking":          "Checking...",
	"update.available":         "Update Available",
	"update.uptodate":          "Up to Date",
	"update.error":             "Update Error",
	"update.restart":           "Restart to Update",
	"palette.title":            "Command Palette & Infinite Canvas Finder",
	"palette.placeholder":      "Search nodes, pages, layouts, languages, or actions...",
	"palette.jumpNode":         "Jump to node",
	"palette.close":            "Close (Esc)",
	"mermaid.title":            "Import Mermaid Diagram",
	"mermaid.placeholder":      "flowchart LR\n  Client --> Gateway --> API\n  API --> DB[(Postgres)]",
	"mermaid.importBtn":        "Import & Format",
	"ctx.formatSel":            "One-Click Format",
	"ctx.duplicate":            "Duplicate",
	"ctx.group":                "Group",
	"ctx.ungroup":              "Ungroup",
	"ctx.lock":                 "Lock",
	"ctx.unlock":               "Unlock",
	"ctx.front":                "Front",
	"ctx.back":                 "Back",
	"ctx.delete":               "Delete",
	"strat.layered":            "Layered DAG",
	"strat.tree":               "Balanced Tree",
	"strat.grid":               "Clean Grid",
	"strat.radial":             "Radial Hub",
	"strat.swimlane":           "Swimlanes",
	"strat.compact":            "Tidy Overlaps",
	"toast.formatted":          "Formatted Diagram",
	"toast.habitSaved":         "Habit Saved",
	"toast.habitSavedDesc":     "Default style updated across all projects",
	"toast.habitsReset":        "Habits Reset",
	"toast.habitsResetDesc":    "Restored factory defaults",
	"toast.mediaAdded":         "Media Added",
	"toast.copied":             "Copied Selection",
	"toast.mermaidDone":        "Imported Mermaid Diagram",
}

var dictZHCN = map[string]string{
	"app.name":                 "DrawGraph",
	"app.untitled":             "未命名图表",
	"menu.file":                "文件",
	"menu.file.new":            "新建图表",
	"menu.file.open":           "打开…",
	"menu.file.save":           "保存",
	"menu.file.saveAs":         "另存为…",
	"menu.file.importMermaid":  "导入 Mermaid…",
	"menu.file.exportSVG":      "导出 SVG…",
	"menu.file.exportPNG":      "导出 PNG…",
	"menu.file.exportDrawio":   "导出 draw.io…",
	"menu.edit":                "编辑",
	"menu.format":              "排版与格式",
	"menu.format.oneClick":     "一键优雅排版",
	"menu.format.layered":      "分层架构图 (Layered)",
	"menu.format.tree":         "树形结构图 (Tree)",
	"menu.format.grid":         "整齐网格 (Grid)",
	"menu.format.radial":       "放射拓扑图 (Radial)",
	"menu.format.swimlane":     "泳道图 (Swimlanes)",
	"menu.format.compact":      "智能消重 (保留手绘位置)",
	"menu.format.lr":           "从左到右排版 (LR)",
	"menu.format.tb":           "从上到下排版 (TB)",
	"menu.format.icons":        "图标插件扩展 (Git)…",
	"menu.format.habits":       "永久用户习惯配置…",
	"menu.view":                "视图",
	"menu.view.zoomIn":         "放大",
	"menu.view.zoomOut":        "缩小",
	"menu.view.zoom100":        "缩放到 100%",
	"menu.view.zoomFit":        "适应画布",
	"menu.view.commandPalette": "全局搜索 / 命令面板…",
	"menu.view.sidebar":        "切换左侧栏",
	"menu.view.inspector":      "切换右侧属性栏",
	"menu.view.minimap":        "切换缩略图",
	"menu.language":            "语言 / Language",
	"menu.help":                "帮助",
	"menu.help.checkUpdates":   "检查更新…",
	"menu.help.updateSettings": "软件更新设置…",
	"tb.new":                   "新建图表",
	"tb.open":                  "打开 (.drawgraph / .drawio)",
	"tb.save":                  "保存",
	"tb.media":                 "媒体",
	"tb.mermaid":               "Mermaid",
	"tb.exportSVG":             "导出 SVG",
	"tb.exportPNG":             "PNG",
	"tb.exportDrawio":          ".drawio",
	"tb.undo":                  "撤销",
	"tb.redo":                  "重做",
	"tb.toolSelect":            "选择 (V)",
	"tb.toolHand":              "抓手平移 (H)",
	"tb.toolConnect":           "连线 (C)",
	"tb.format":                "一键排版",
	"tb.alignLeft":             "左对齐",
	"tb.alignCenterX":          "水平居中",
	"tb.alignTop":              "顶对齐",
	"tb.group":                 "组合",
	"tb.ungroup":               "取消组合",
	"tb.search":                "节点定位 / 命令面板 (Ctrl+K)",
	"tb.zoomOut":               "缩小",
	"tb.zoomIn":                "放大",
	"tb.zoomFit":               "适应画布",
	"tb.grid":                  "切换网格",
	"tb.theme":                 "切换明暗主题",
	"tb.iconPlugins":           "图标插件",
	"tb.habits":                "用户习惯",
	"side.searchPlaceholder":   "搜索图形与图标...",
	"side.coreShapes":          "基础图形",
	"side.favorites":           "常用记忆图标",
	"side.iconLibrary":         "图标库",
	"side.addGitPack":          "+ Git 扩展包",
	"side.allPacks":            "全部已启用图标包",
	"side.templates":           "快速模板",
	"page.add":                 "新建分页",
	"page.rename":              "重命名分页",
	"page.duplicate":           "复制分页",
	"page.delete":              "删除分页",
	"insp.nodeTitle":           "节点属性",
	"insp.edgeTitle":           "连线属性",
	"insp.docTitle":            "画布与一键样式",
	"insp.label":               "主标题",
	"insp.sublabel":            "副标题",
	"insp.sublabelPlaceholder": "可选说明文字",
	"insp.shape":               "图形类型",
	"insp.fillColor":           "填充颜色",
	"insp.strokeColor":         "边框颜色",
	"insp.dash":                "线条样式",
	"insp.dashSolid":           "实线",
	"insp.dashDashed":          "虚线",
	"insp.dashDotted":          "点线",
	"insp.radius":              "圆角半径",
	"insp.fontSize":            "字号大小",
	"insp.dropShadow":          "开启阴影",
	"insp.locked":              "锁定位置",
	"insp.clear":               "清除",
	"insp.saveHabit":           "记住为默认习惯",
	"insp.delete":              "删除",
	"insp.duplicate":           "复制",
	"insp.bringFront":          "置顶",
	"insp.sendBack":            "置底",
	"insp.routing":             "连线路径",
	"insp.routeOrthogonal":     "正交折线 (draw.io)",
	"insp.routeCurved":         "平滑曲线",
	"insp.routeStraight":       "直线",
	"insp.startArrow":          "起点箭头",
	"insp.endArrow":            "终点箭头",
	"insp.swapEnds":            "调换方向",
	"insp.arrowClassic":        "标准箭头",
	"insp.arrowOpen":           "开放箭头",
	"insp.arrowDiamond":        "菱形",
	"insp.arrowCircle":         "圆点",
	"insp.arrowNone":           "无",
	"insp.deleteEdge":          "删除连线",
	"insp.diagramTitle":        "图表标题",
	"insp.pageName":            "当前分页名称",
	"insp.snapToGrid":          "对齐网格",
	"insp.stylePreset":         "风格预设",
	"insp.layoutStrategy":      "排版算法",
	"insp.formatLR":            "横向排版 LR",
	"insp.formatTB":            "纵向排版 TB",
	"insp.stats":               "节点: %d  ·  连线: %d  ·  分页: %d",
	"media.title":              "高清媒体与分辨率",
	"media.fit":                "高清自适应",
	"media.scaleMode":          "填充模式",
	"media.contain":            "完整等比 (Contain)",
	"media.cover":              "裁剪铺满 (Cover)",
	"media.stretch":            "拉伸填充 (Fill)",
	"media.showBar":            "显示底部标题栏",
	"media.play":               "播放视频",
	"media.pause":              "暂停视频",
	"media.loop":               "循环播放",
	"media.muted":              "静音",
	"canvas.dropHint":          "拖入高清图片或视频以自动按合理比例创建节点",
	"status.tool":              "工具: %s",
	"status.nodes":             "节点: %d",
	"status.edges":             "连线: %d",
	"status.assets":            "资源: %d",
	"status.visible":           "可视裁剪: %d/%d",
	"status.origin":            "回到原点",
	"status.habitOn":           "习惯记忆: 已开启",
	"drawer.pluginsTitle":      "图标插件扩展 (Git 与本地 SVG)",
	"drawer.pluginsDesc":       "支持从任意 Git 仓库地址或本地文件夹扩展 SVG 图标包，跨所有项目永久生效。",
	"drawer.installGit":        "从 Git 地址安装",
	"drawer.cloneGit":          "克隆 Git 图标包",
	"drawer.importFolder":      "导入本地文件夹…",
	"drawer.installedPacks":    "已安装图标包",
	"drawer.sync":              "同步更新",
	"drawer.remove":            "卸载",
	"drawer.habitsTitle":       "永久用户习惯配置 (全局生效)",
	"drawer.habitsDesc":        "全局持久化保存至 %s。所有新建图表及一键排版将自动继承。",
	"drawer.autoRemember":      "跨项目自动记忆样式、常用图标和排版偏好",
	"drawer.language":          "界面语言 (Language)",
	"drawer.prefDir":           "默认排版方向",
	"drawer.prefStrategy":      "默认排版算法",
	"drawer.defaultRouting":    "默认连线方式",
	"drawer.wheelMode":         "鼠标滚轮行为",
	"wheel.pan":                "平移动画布 (Ctrl+滚轮缩放)",
	"wheel.zoom":               "直接缩放画布",
	"drawer.gapX":              "水平间距",
	"drawer.gapY":              "垂直间距",
	"drawer.mediaMax":          "导入高清图/视频默认最大显示尺寸 (DIP)",
	"drawer.applyHabits":       "立即应用习惯",
	"drawer.resetDefaults":     "恢复出厂默认",
	"drawer.updatesTitle":      "软件更新",
	"drawer.checkNow":          "立即检查",
	"drawer.downloadUpdate":    "下载更新",
	"drawer.installRestart":    "安装并重启",
	"drawer.updateSettings":    "手动与自动更新设置",
	"drawer.autoCheckUpdates":  "启动时自动检查 GitHub 新版本",
	"drawer.autoDownload":      "后台自动下载并校验安装包",
	"drawer.releaseChannel":    "发布通道",
	"drawer.channelStable":     "稳定版 (Stable)",
	"drawer.channelPrerelease": "预览版 (Prerelease)",
	"update.checking":          "检查中...",
	"update.available":         "发现新版本",
	"update.uptodate":          "已是最新版",
	"update.error":             "检查出错",
	"update.restart":           "点击重启更新",
	"palette.title":            "命令面板与无限画布定位",
	"palette.placeholder":      "搜索节点名称、分页、排版算法、语言或快捷动作...",
	"palette.jumpNode":         "定位到节点",
	"palette.close":            "关闭 (Esc)",
	"mermaid.title":            "导入 Mermaid 流程图 / 架构图",
	"mermaid.placeholder":      "flowchart LR\n  Client --> Gateway --> API\n  API --> DB[(Postgres)]",
	"mermaid.importBtn":        "导入并一键排版",
	"ctx.formatSel":            "一键排版",
	"ctx.duplicate":            "复制节点",
	"ctx.group":                "打组容器",
	"ctx.ungroup":              "解散分组",
	"ctx.lock":                 "锁定",
	"ctx.unlock":               "解锁",
	"ctx.front":                "置顶",
	"ctx.back":                 "置底",
	"ctx.delete":               "删除",
	"strat.layered":            "分层架构",
	"strat.tree":               "平衡树",
	"strat.grid":               "规整网格",
	"strat.radial":             "中心放射",
	"strat.swimlane":           "泳道布局",
	"strat.compact":            "智能消重",
	"toast.formatted":          "已完成一键排版",
	"toast.habitSaved":         "习惯已保存",
	"toast.habitSavedDesc":     "跨项目默认风格已更新",
	"toast.habitsReset":        "已恢复默认习惯",
	"toast.habitsResetDesc":    "所有全局设置已重置",
	"toast.mediaAdded":         "已导入高清媒体",
	"toast.copied":             "已复制所选元素",
	"toast.mermaidDone":        "已导入 Mermaid 图表",
}

var dictZHTW = map[string]string{
	"app.name":                 "DrawGraph",
	"app.untitled":             "未命名圖表",
	"menu.file":                "檔案",
	"menu.file.new":            "新增圖表",
	"menu.file.open":           "開啟…",
	"menu.file.save":           "儲存",
	"menu.file.saveAs":         "另存新檔…",
	"menu.file.importMermaid":  "匯入 Mermaid…",
	"menu.file.exportSVG":      "匯出 SVG…",
	"menu.file.exportPNG":      "匯出 PNG…",
	"menu.file.exportDrawio":   "匯出 draw.io…",
	"menu.edit":                "編輯",
	"menu.format":              "排版與格式",
	"menu.format.oneClick":     "一鍵優雅排版",
	"menu.format.layered":      "分層架構 (Layered)",
	"menu.format.tree":         "樹狀結構 (Tree)",
	"menu.format.grid":         "整齊網格 (Grid)",
	"menu.format.radial":       "放射拓樸 (Radial)",
	"menu.format.swimlane":     "泳道圖 (Swimlanes)",
	"menu.format.compact":      "智慧消重 (Compact)",
	"menu.format.lr":           "由左至右排版 (LR)",
	"menu.format.tb":           "由上至下排版 (TB)",
	"menu.format.icons":        "圖示外掛 (Git)…",
	"menu.format.habits":       "永久使用者習慣…",
	"menu.view":                "檢視",
	"menu.view.zoomIn":         "放大",
	"menu.view.zoomOut":        "縮小",
	"menu.view.zoom100":        "縮放至 100%",
	"menu.view.zoomFit":        "適應畫布",
	"menu.view.commandPalette": "搜尋 / 命令面板…",
	"menu.view.sidebar":        "切換側邊欄",
	"menu.view.inspector":      "切換屬性欄",
	"menu.view.minimap":        "切換縮略圖",
	"menu.language":            "語言 / Language",
	"menu.help":                "說明",
	"menu.help.checkUpdates":   "檢查更新…",
	"menu.help.updateSettings": "更新設定…",
	"tb.media":                 "媒體",
	"tb.mermaid":               "Mermaid",
	"tb.exportSVG":             "匯出 SVG",
	"tb.format":                "一鍵排版",
	"tb.iconPlugins":           "圖示外掛",
	"tb.habits":                "使用習慣",
	"side.searchPlaceholder":   "搜尋圖形與圖示...",
	"side.coreShapes":          "基礎圖形",
	"side.favorites":           "常用記憶圖示",
	"side.iconLibrary":         "圖示庫",
	"side.addGitPack":          "+ Git 擴充包",
	"side.allPacks":            "所有已啟用圖示包",
	"insp.nodeTitle":           "節點屬性",
	"insp.edgeTitle":           "連線屬性",
	"insp.docTitle":            "畫布與一鍵樣式",
	"insp.label":               "標籤",
	"insp.sublabel":            "副標題",
	"insp.shape":               "形狀",
	"insp.fillColor":           "填滿色彩",
	"insp.strokeColor":         "框線色彩",
	"insp.saveHabit":           "儲存為預設習慣",
	"insp.delete":              "刪除",
	"drawer.language":          "介面語言",
	"drawer.habitsTitle":       "永久使用者習慣 (全域)",
	"drawer.updatesTitle":      "軟體更新",
	"palette.title":            "命令面板與無限畫布搜尋",
	"mermaid.title":            "匯入 Mermaid 圖表",
	"status.origin":            "回到原點",
	"status.habitOn":           "習慣記憶: 開啟",
	"strat.layered":            "分層架構",
	"strat.tree":               "平衡樹",
	"strat.grid":               "規整網格",
	"strat.radial":             "中心放射",
	"strat.swimlane":           "泳道佈局",
	"strat.compact":            "智慧消重",
}

var dictJA = map[string]string{
	"app.name":                 "DrawGraph",
	"app.untitled":             "無題の図表",
	"menu.file":                "ファイル",
	"menu.file.new":            "新規図表",
	"menu.file.open":           "開く…",
	"menu.file.save":           "保存",
	"menu.file.saveAs":         "名前を付けて保存…",
	"menu.file.importMermaid":  "Mermaid をインポート…",
	"menu.file.exportSVG":      "SVG を書き出す…",
	"menu.file.exportPNG":      "PNG を書き出す…",
	"menu.file.exportDrawio":   "draw.io を書き出す…",
	"menu.edit":                "編集",
	"menu.format":              "レイアウトと整形",
	"menu.format.oneClick":     "ワンクリック自動整形",
	"menu.format.layered":      "階層構造 (Layered)",
	"menu.format.tree":         "ツリー / 組織図 (Tree)",
	"menu.format.grid":         "グリッド整列 (Grid)",
	"menu.format.radial":       "放射状トポロジ (Radial)",
	"menu.format.swimlane":     "スイムレーン (Swimlanes)",
	"menu.format.compact":      "重複解消 (Compact)",
	"menu.format.lr":           "左から右へ (LR)",
	"menu.format.tb":           "上から下へ (TB)",
	"menu.format.icons":        "アイコンプラグイン (Git)…",
	"menu.format.habits":       "永続ユーザー設定…",
	"menu.view":                "表示",
	"menu.view.zoomIn":         "拡大",
	"menu.view.zoomOut":        "縮小",
	"menu.view.zoom100":        "100% に戻す",
	"menu.view.zoomFit":        "全体を表示",
	"menu.view.commandPalette": "検索 / コマンドパレット…",
	"menu.language":            "言語 / Language",
	"menu.help":                "ヘルプ",
	"menu.help.checkUpdates":   "アップデートを確認…",
	"tb.media":                 "メディア",
	"tb.format":                "自動整形",
	"tb.iconPlugins":           "アイコン拡張",
	"tb.habits":                "習慣設定",
	"side.searchPlaceholder":   "図形・アイコンを検索...",
	"side.coreShapes":          "基本図形",
	"side.favorites":           "よく使うアイコン",
	"side.iconLibrary":         "アイコンライブラリ",
	"insp.nodeTitle":           "ノードプロパティ",
	"insp.edgeTitle":           "コネクタプロパティ",
	"insp.docTitle":            "キャンバス & 自動スタイル",
	"insp.label":               "ラベル",
	"insp.shape":               "図形",
	"insp.fillColor":           "塗りつぶし",
	"insp.strokeColor":         "枠線色",
	"insp.saveHabit":           "習慣として保存",
	"insp.delete":              "削除",
	"drawer.language":          "表示言語",
	"drawer.habitsTitle":       "永続ユーザー設定 (グローバル)",
	"drawer.updatesTitle":      "ソフトウェア更新",
	"palette.title":            "コマンドパレット & ノード検索",
	"mermaid.title":            "Mermaid 図表をインポート",
	"status.origin":            "原点へ",
	"status.habitOn":           "習慣記憶: ON",
	"strat.layered":            "階層 DAG",
	"strat.tree":               "ツリー",
	"strat.grid":               "グリッド",
	"strat.radial":             "放射状",
	"strat.swimlane":           "スイムレーン",
	"strat.compact":            "重複解消",
}

var dictKO = map[string]string{
	"app.name":                "DrawGraph",
	"app.untitled":            "제목 없는 다이어그램",
	"menu.file":               "파일",
	"menu.file.new":           "새 다이어그램",
	"menu.file.open":          "열기…",
	"menu.file.save":          "저장",
	"menu.file.saveAs":        "다른 이름으로 저장…",
	"menu.file.importMermaid": "Mermaid 가져오기…",
	"menu.file.exportSVG":     "SVG 내보내기…",
	"menu.file.exportPNG":     "PNG 내보내기…",
	"menu.file.exportDrawio":  "draw.io 내보내기…",
	"menu.edit":               "편집",
	"menu.format":             "서식 및 레이아웃",
	"menu.format.oneClick":    "원클릭 자동 정렬",
	"menu.format.lr":          "왼쪽에서 오른쪽 (LR)",
	"menu.format.tb":          "위에서 아래 (TB)",
	"menu.format.icons":       "아이콘 플러그인 (Git)…",
	"menu.format.habits":      "사용자 습관 설정…",
	"menu.view":               "보기",
	"menu.language":           "언어 / Language",
	"menu.help":               "도움말",
	"menu.help.checkUpdates":  "업데이트 확인…",
	"tb.media":                "미디어",
	"tb.format":               "자동 정렬",
	"tb.iconPlugins":          "아이콘 확장",
	"tb.habits":               "습관 설정",
	"side.searchPlaceholder":  "도형 및 아이콘 검색...",
	"side.coreShapes":         "기본 도형",
	"side.favorites":          "자주 쓰는 아이콘",
	"side.iconLibrary":        "아이콘 라이브러리",
	"insp.nodeTitle":          "노드 속성",
	"insp.edgeTitle":          "연결선 속성",
	"insp.docTitle":           "캔버스 및 스타일",
	"drawer.language":         "인터페이스 언어",
	"drawer.habitsTitle":      "영구 사용자 습관 (전역)",
	"drawer.updatesTitle":     "소프트웨어 업데이트",
	"palette.title":           "명령 팔레트 & 노드 찾기",
	"mermaid.title":           "Mermaid 다이어그램 가져오기",
	"status.origin":           "원점으로",
	"status.habitOn":          "습관 기억: ON",
	"strat.layered":           "계층형 DAG",
	"strat.tree":              "트리형",
	"strat.grid":              "그리드",
	"strat.radial":            "방사형",
	"strat.swimlane":          "스윔레인",
	"strat.compact":           "겹침 정리",
}
