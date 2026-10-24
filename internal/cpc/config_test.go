package cpc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLanguageConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("LC_ALL", "")
	t.Setenv("LC_MESSAGES", "")
	t.Setenv("LANG", "zh_CN.UTF-8")
	if got, err := loadLanguage(); err != nil || got != zh {
		t.Fatalf("missing config: %q %v", got, err)
	}
	path := filepath.Join(home, ".cpc.config.toml")
	for _, tc := range []struct {
		data string
		want language
		bad  bool
	}{
		{`language = "en"`, en, false}, {`language = "zh"`, zh, false}, {`language = "auto"`, zh, false}, {``, zh, false},
		{`language = "fr"`, zh, true}, {`language = [`, zh, true}, {`language = 5`, zh, true},
	} {
		os.WriteFile(path, []byte(tc.data), 0600)
		got, err := loadLanguage()
		if (err != nil) != tc.bad || got != tc.want {
			t.Fatalf("%s: %q %v", tc.data, got, err)
		}
	}
	os.Remove(path)
	os.Mkdir(path, 0700)
	if _, err := loadLanguage(); err == nil {
		t.Fatal("unreadable config should fail")
	}
}
func TestLocalePrecedence(t *testing.T) {
	for _, tc := range []struct {
		all, messages, lang string
		want                language
	}{
		{"en_US.UTF-8", "zh_CN", "zh_TW", en}, {"", "zh_TW.UTF-8", "en_US", zh},
		{"", "", "zh-Hans", zh}, {"C", "zh_CN", "zh_CN", en}, {"", "", "", en},
	} {
		t.Setenv("LC_ALL", tc.all)
		t.Setenv("LC_MESSAGES", tc.messages)
		t.Setenv("LANG", tc.lang)
		if got := systemLanguage(); got != tc.want {
			t.Fatalf("%+v: %s", tc, got)
		}
	}
}
func TestSingleLanguageHelpAndErrors(t *testing.T) {
	err := problem("找不到脚本 %q", "Script not found: %q", "dev")
	if strings.Contains(errorText(zh, err), "Script not found") || strings.Contains(errorText(en, err), "找不到") {
		t.Fatal("mixed error languages")
	}
	if strings.Contains(help(en), "复制") || strings.Contains(help(zh), "Copy context") {
		t.Fatal("mixed help languages")
	}
}
