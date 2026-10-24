package cpc

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

type language string

const (
	zh language = "zh"
	en language = "en"
)

func (l language) text(cn, english string) string {
	if l == zh {
		return cn
	}
	return english
}

func systemLanguage() language {
	for _, key := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if value := os.Getenv(key); value != "" {
			value = strings.ToLower(value)
			if value == "zh" || strings.HasPrefix(value, "zh_") || strings.HasPrefix(value, "zh-") || strings.HasPrefix(value, "zh.") {
				return zh
			}
			return en
		}
	}
	return en
}

func loadLanguage() (language, error) {
	fallback := systemLanguage()
	home, err := os.UserHomeDir()
	if err != nil {
		return fallback, err
	}
	path := filepath.Join(home, ".cpc.config.toml")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return fallback, nil
	}
	if err != nil {
		return fallback, problem("无法读取配置 %s: %v", "Cannot read config %s: %v", path, err)
	}
	var config struct {
		Language string `toml:"language"`
	}
	if err := toml.Unmarshal(data, &config); err != nil {
		return fallback, problem("配置格式错误 %s: %v", "Invalid config %s: %v", path, err)
	}
	switch config.Language {
	case "", "auto":
		return fallback, nil
	case "zh":
		return zh, nil
	case "en":
		return en, nil
	default:
		return fallback, problem("配置 language 不支持 %q（可选 auto、zh、en）", "Unsupported language %q (expected auto, zh, en)", config.Language)
	}
}

type translatedError struct{ cn, english error }

func (e *translatedError) Error() string { return e.english.Error() }
func (e *translatedError) Unwrap() error { return e.english }
func problem(cn, english string, args ...any) error {
	return &translatedError{fmt.Errorf(cn, args...), fmt.Errorf(english, args...)}
}
func errorText(l language, err error) string {
	var translated *translatedError
	if errors.As(err, &translated) {
		return l.text(translated.cn.Error(), translated.english.Error())
	}
	return err.Error()
}
