package control

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"arcdesk/internal/config"
	"arcdesk/internal/i18n"
	"arcdesk/internal/mcpcmd"
)

func (c *Controller) handleAutoPlanSlash(trimmed string) {
	args := mcpcmd.TokenizeArgs(trimmed)
	if len(args) < 2 {
		cfg, err := config.Load()
		if err != nil {
			c.notice("auto-plan: " + err.Error())
			return
		}
		c.notice(fmt.Sprintf("auto-plan: %s (usage: /auto-plan off|on)", autoPlanDisplay(cfg.Agent.AutoPlan)))
		return
	}
	if len(args) > 2 {
		c.notice("usage: /auto-plan off|on")
		return
	}
	if c.Running() {
		c.notice("finish or cancel the current turn before changing auto-plan")
		return
	}
	path := config.UserConfigPath()
	if path == "" {
		c.notice("auto-plan: cannot resolve config path")
		return
	}
	edit := config.LoadForEdit(path)
	if err := edit.SetAutoPlan(args[1]); err != nil {
		c.notice("auto-plan: " + err.Error())
		return
	}
	if err := edit.SaveTo(path); err != nil {
		c.notice("auto-plan: " + err.Error())
		return
	}
	mode := edit.Agent.AutoPlan
	c.SetAutoPlan(mode)
	c.notice(fmt.Sprintf("auto-plan set to %s", autoPlanDisplay(mode)))
}

func (c *Controller) handleLanguageSlash(trimmed string) {
	args := mcpcmd.TokenizeArgs(trimmed)
	if len(args) < 2 {
		cfg, err := config.Load()
		if err != nil {
			c.notice("language: " + err.Error())
			return
		}
		saved := languageDisplay(cfg.Language)
		resolved := i18n.DetectLanguage(cfg.Language)
		c.notice(i18n.M.LanguageHeader + "\n" + describeLanguages(saved, resolved) + "\n" + i18n.M.LanguageHint)
		return
	}
	if len(args) > 2 {
		c.notice(i18n.M.LanguageHint)
		return
	}
	lang, err := normalizeLanguageArg(args[1])
	if err != nil {
		c.notice(err.Error())
		return
	}
	path := config.SourcePath()
	if path == "" {
		path = config.UserConfigPath()
	}
	if path == "" {
		c.notice("language: cannot resolve config path")
		return
	}
	edit := config.LoadForEdit(path)
	if err := edit.SetLanguage(lang); err != nil {
		c.notice("language: " + err.Error())
		return
	}
	if err := edit.SaveTo(path); err != nil {
		c.notice("language: " + err.Error())
		return
	}
	if lang == "" {
		if err := clearUserLanguageOverride(path); err != nil {
			c.notice("language: " + err.Error())
			return
		}
	}
	resolved := i18n.DetectLanguage(lang)
	c.notice(fmt.Sprintf(i18n.M.LanguageChangedFmt, languageDisplay(lang), resolved))
}

func autoPlanDisplay(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "on", "ask":
		return "on"
	default:
		return "off"
	}
}

func normalizeLanguageArg(s string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "auto", "detect", "default":
		return "", nil
	case "en", "english":
		return "en", nil
	case "zh", "cn", "chinese", "中文":
		return "zh", nil
	default:
		return "", fmt.Errorf("usage: /language auto|en|zh")
	}
}

func languageDisplay(lang string) string {
	if strings.TrimSpace(lang) == "" {
		return "auto"
	}
	return lang
}

func describeLanguages(current, resolved string) string {
	items := []struct {
		tag  string
		hint string
	}{
		{"auto", i18n.M.ArgLanguageAuto},
		{"en", i18n.M.ArgLanguageEn},
		{"zh", i18n.M.ArgLanguageZh},
	}
	var b strings.Builder
	for _, it := range items {
		marker := "  "
		if it.tag == current {
			marker = "• "
		}
		hint := it.hint
		if it.tag == current {
			hint += " · " + i18n.M.ArgThemeCurrent
		}
		if it.tag == "auto" && current == "auto" {
			hint += " · " + resolved
		}
		fmt.Fprintf(&b, "%s%-6s %s\n", marker, it.tag, hint)
	}
	return strings.TrimRight(b.String(), "\n")
}

func clearUserLanguageOverride(primaryPath string) error {
	userPath := config.UserConfigPath()
	if userPath == "" || sameConfigPath(primaryPath, userPath) {
		return nil
	}
	if _, err := os.Stat(userPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	edit := config.LoadForEdit(userPath)
	if strings.TrimSpace(edit.Language) == "" {
		return nil
	}
	if err := edit.SetLanguage(""); err != nil {
		return err
	}
	return edit.SaveTo(userPath)
}

func sameConfigPath(a, b string) bool {
	aa, errA := filepath.Abs(a)
	bb, errB := filepath.Abs(b)
	if errA == nil {
		a = aa
	}
	if errB == nil {
		b = bb
	}
	return filepath.Clean(a) == filepath.Clean(b)
}
