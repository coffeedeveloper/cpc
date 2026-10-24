package cpc

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

func copyText(ctx context.Context, value string) error {
	var candidates [][]string
	switch runtime.GOOS {
	case "darwin":
		candidates = [][]string{{"pbcopy"}}
	case "linux":
		if os.Getenv("WAYLAND_DISPLAY") != "" {
			candidates = append(candidates, []string{"wl-copy"})
		}
		if os.Getenv("DISPLAY") != "" {
			candidates = append(candidates, []string{"xclip", "-selection", "clipboard"}, []string{"xsel", "--clipboard", "--input"})
		}
	default:
		return problem("不支持 %s 系统剪贴板；请使用 --print", "Unsupported clipboard platform: %s; use --print", runtime.GOOS)
	}
	var lastErr error
	for _, args := range candidates {
		path, err := exec.LookPath(args[0])
		if err != nil {
			lastErr = err
			continue
		}
		childCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		cmd := exec.CommandContext(childCtx, path, args[1:]...)
		cmd.Stdin = strings.NewReader(value)
		// Clipboard helpers may fork a background owner; do not wait on inherited output pipes.
		err = cmd.Run()
		cancel()
		if err == nil {
			return nil
		}
		lastErr = err
	}
	if lastErr != nil {
		return problem("剪贴板写入失败: %w；可使用 --print（Linux 需安装 wl-clipboard、xclip 或 xsel）", "Clipboard failed: %w; use --print (Linux: install wl-clipboard, xclip or xsel)", lastErr)
	}
	return problem("剪贴板不可用；请使用 --print（Linux 需要 Wayland/X11 图形会话）", "Clipboard unavailable; use --print (Linux requires a graphical Wayland/X11 session)")
}
