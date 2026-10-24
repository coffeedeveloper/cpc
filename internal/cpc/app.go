package cpc

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"unicode"

	"github.com/charmbracelet/x/ansi"
)

type item struct {
	Key, Title, Description, Value string
	Script                         *scriptValues
}

func (i item) copyValue() string {
	if i.Value != "" {
		return i.Value
	}
	return i.Key
}

func IsCancelled(err error) bool { return errors.Is(err, context.Canceled) }
func display(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, ansi.Strip(s))
}
func matches(query string, entry item) bool {
	haystack := strings.ToLower(entry.Key + " " + entry.Title + " " + entry.Description)
	for _, word := range strings.Fields(strings.ToLower(query)) {
		if !strings.Contains(haystack, word) {
			return false
		}
	}
	return true
}

func help(l language) string {
	return l.text(`cpc — 复制上下文

用法: cpc [--print] [--format name|command|content] [命令] [值]

  cpc                 搜索并选择上下文
  cpc ip [interface]  本地 IP，多地址时选择
  cpc public-ip       外网 IP
  cpc pwd             当前绝对工作目录
  cpc branch          Git 当前分支名
  cpc commit          HEAD 完整提交哈希
  cpc script [name]   选择任务及要复制的内容

  --format            script 内容类型：name、command、content
  --print             仅输出，不写剪贴板
  -h, --help          帮助
  -v, --version       版本

输入搜索；↓ / Ctrl+N 向下；↑ / Ctrl+P 向上；Enter 确认；Ctrl+C 取消
配置: ~/.cpc.config.toml，language = "auto"、"zh" 或 "en"
`, `cpc — Copy context

Usage: cpc [--print] [--format name|command|content] [command] [value]

  cpc                 Search and select context
  cpc ip [interface]  Local IP, select when multiple
  cpc public-ip       Public IP
  cpc pwd             Current absolute working directory
  cpc branch          Current Git branch
  cpc commit          Full HEAD commit hash
  cpc script [name]   Select a task and what to copy

  --format            Script copy format: name, command, content
  --print             Print instead of copying
  -h, --help          Help
  -v, --version       Version

Type to search; ↓ / Ctrl+N next; ↑ / Ctrl+P previous; Enter select; Ctrl+C cancel
Config: ~/.cpc.config.toml, language = "auto", "zh" or "en"
`)
}

func Run(ctx context.Context, args []string, version string) (result error) {
	l, err := loadLanguage()
	defer func() {
		if result != nil && !IsCancelled(result) {
			result = errors.New(errorText(l, result))
		}
	}()
	if err != nil {
		return err
	}
	printOnly := false
	format := ""
	positional := []string{}
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch arg {
		case "--format":
			index++
			if index >= len(args) {
				return problem("--format 需要 name、command 或 content", "--format requires name, command or content")
			}
			format = args[index]
			if format != "name" && format != "command" && format != "content" {
				return problem("无效内容类型 %q", "Invalid copy format %q", format)
			}
		case "--print":
			printOnly = true
		case "--help", "-h":
			fmt.Print(help(l))
			return nil
		case "--version", "-v":
			fmt.Println("cpc " + version)
			return nil
		default:
			positional = append(positional, arg)
		}
	}
	if format != "" && format != "name" && format != "command" && format != "content" {
		return problem("无效内容类型 %q", "Invalid copy format %q", format)
	}
	if format != "" && (len(positional) == 0 || positional[0] != "script") {
		return problem("--format 仅适用于 script", "--format is only supported for script")
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	var value string
	if len(positional) == 0 {
		value, err = runMenu(ctx, newMainMenu(ctx, cwd, l))
	} else {
		command := positional[0]
		if len(positional) > 2 || (len(positional) == 2 && command != "script" && command != "ip") {
			return problem("参数过多；请使用 cpc --help", "Unexpected arguments; use cpc --help")
		}
		switch command {
		case "pwd":
			value = cwd
		case "branch", "commit":
			value, err = gitValue(ctx, cwd, command)
		case "public-ip":
			value, err = publicIP(ctx, publicEndpoint)
		case "ip", "script":
			var entries []item
			if command == "ip" {
				entries, err = localIPs()
			} else {
				entries, err = scripts(cwd)
			}
			if err != nil {
				break
			}
			if len(positional) == 2 {
				filtered := []item{}
				for _, entry := range entries {
					if (command == "ip" && entry.Title == positional[1]) || (command == "script" && entry.Key == positional[1]) {
						filtered = append(filtered, entry)
					}
				}
				entries = filtered
			}
			if len(entries) == 0 {
				err = problem("没有匹配的地址或脚本", "No matching address or script")
			} else if command == "script" {
				m := newChildMenu(ctx, l, "script", entries)
				m.scriptFormat = format
				if len(positional) == 2 {
					if format != "" {
						value = entries[0].Script.value(format)
					} else {
						m.openScriptCopy(m.rows[0])
						value, err = runMenu(ctx, m)
					}
				} else {
					value, err = runMenu(ctx, m)
				}
			} else if len(entries) == 1 {
				value = entries[0].copyValue()
			} else {
				value, err = runMenu(ctx, newChildMenu(ctx, l, command, entries))
			}
		default:
			return problem("未知命令 %q；请使用 cpc --help", "Unknown command %q; use cpc --help", command)
		}
	}
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if printOnly {
		fmt.Println(value)
		return nil
	}
	if err := copyText(ctx, value); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "✓ %s: %s\n", l.text("已复制", "Copied"), display(value))
	return nil
}
