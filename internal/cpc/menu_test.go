package cpc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func update(m menuModel, msg tea.Msg) menuModel { next, _ := m.Update(msg); return next.(menuModel) }
func key(m menuModel, k tea.KeyType) menuModel  { return update(m, tea.KeyMsg{Type: k}) }
func search(m menuModel, s string) menuModel {
	return update(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)})
}

func TestMenuFitsTerminal(t *testing.T) {
	for _, l := range []language{zh, en} {
		m := newMainMenu(context.Background(), "/"+strings.Repeat("中文路径👩‍💻/", 60), l)
		m = update(m, previewMsg{id: "branch", value: strings.Repeat("feature/长分支", 50)})
		for _, size := range [][2]int{{120, 24}, {80, 12}, {40, 8}, {20, 6}, {8, 4}, {1, 1}, {120, 24}} {
			m = update(m, tea.WindowSizeMsg{Width: size[0], Height: size[1]})
			for i := 0; i < 12; i++ {
				m = key(m, tea.KeyCtrlN)
				lines := strings.Split(m.View(), "\n")
				if len(lines) > size[1] {
					t.Fatalf("too many lines: %d > %d", len(lines), size[1])
				}
				for _, line := range lines {
					if ansi.StringWidth(line) > max(1, size[0]-1) {
						t.Fatalf("overflow at %v: %q (%d)", size, line, ansi.StringWidth(line))
					}
				}
			}
		}
	}
}
func TestMenuNavigationAndFullValues(t *testing.T) {
	m := newMainMenu(context.Background(), "/project", en)
	m = key(m, tea.KeyCtrlN)
	m = key(m, tea.KeyDown)
	m = key(m, tea.KeyCtrlN)
	m = key(m, tea.KeyCtrlP)
	if m.selectedID() != "pwd" {
		t.Fatal(m.selectedID())
	}
	m = key(m, tea.KeyUp)
	if m.selectedID() != "public-ip" {
		t.Fatal(m.selectedID())
	}
	hash := strings.Repeat("a1", 20)
	m = update(m, previewMsg{id: "commit", value: hash})
	m = search(m, "提交")
	m = key(m, tea.KeyEnter)
	if !m.done || m.result != hash {
		t.Fatalf("truncated copy: %q", m.result)
	}
	m = newMainMenu(context.Background(), "/"+strings.Repeat("长路径/", 100), zh)
	full := m.rows[2].value
	m = search(m, "working")
	m = key(m, tea.KeyEnter)
	if m.result != full {
		t.Fatal("path was truncated")
	}
}
func TestAsyncPreviewKeepsSelectionAndQuery(t *testing.T) {
	m := newMainMenu(context.Background(), "/project", en)
	m = search(m, "git")
	m = key(m, tea.KeyCtrlN)
	id := m.selectedID()
	m = update(m, previewMsg{id: "branch", value: "main"})
	m = update(m, previewMsg{id: "public-ip", value: "203.0.113.8"})
	if m.selectedID() != id || m.input.Value() != "git" {
		t.Fatal("async update changed selection or query")
	}
	m = key(m, tea.KeyEnter)
	if m.done || m.notice == "" {
		t.Fatal("loading entry should not copy")
	}
	m = update(m, previewMsg{id: "commit", err: problem("无提交", "No commits")})
	if m.notice != "" {
		t.Fatal("stale loading notice after preview completes")
	}
	m = key(m, tea.KeyEnter)
	if m.done || !strings.Contains(m.notice, "No commits") {
		t.Fatal("failed entry should not copy")
	}
}
func TestScriptSubmenuAndIP(t *testing.T) {
	m := newMainMenu(context.Background(), "/project", zh)
	entries := []item{{Key: "build", Title: "build", Description: strings.Repeat("vite build ", 50), Script: &scriptValues{Name: "build", Command: "npm run build", Content: strings.Repeat("vite build ", 50)}}, {Key: "dev", Title: "dev", Description: "vite", Script: &scriptValues{Name: "dev", Command: "npm run dev", Content: "vite"}}}
	m = update(m, previewMsg{id: "script", children: entries})
	m = search(m, "script")
	m = key(m, tea.KeyEnter)
	if m.page != "script" {
		t.Fatal("missing submenu")
	}
	m = update(m, previewMsg{id: "public-ip", value: "203.0.113.8"})
	m = search(m, "vite")
	m = key(m, tea.KeyCtrlN)
	m = key(m, tea.KeyCtrlP)
	m = key(m, tea.KeyEnter)
	if m.page != "script-copy" {
		t.Fatal("expected copy options")
	}
	m = key(m, tea.KeyEnter)
	if m.result != "build" {
		t.Fatal(m.result)
	}
	m = newMainMenu(context.Background(), "/project", en)
	m = update(m, previewMsg{id: "ip", children: []item{{Key: "10.0.0.1", Title: "en0", Description: "IPv4"}, {Key: "2001:db8::1", Title: "en0", Description: "IPv6"}}})
	m = key(m, tea.KeyEnter)
	if m.page != "ip" {
		t.Fatal("expected IP submenu")
	}
	m = key(m, tea.KeyEsc)
	if m.page != "" || m.mainRows != nil {
		t.Fatal("expected main menu")
	}
	m = key(m, tea.KeyEsc)
	if !m.cancelled {
		t.Fatal("expected cancellation")
	}
}
func TestSearchEditingAndNoResults(t *testing.T) {
	m := newMainMenu(context.Background(), "/project", zh)
	m = search(m, "工作x")
	if len(m.visible()) != 0 {
		t.Fatal("expected no matches")
	}
	m = key(m, tea.KeyEnter)
	if m.done {
		t.Fatal("empty search must not copy")
	}
	m = key(m, tea.KeyBackspace)
	if m.selectedID() != "pwd" {
		t.Fatal(m.input.Value())
	}
}
func TestPublicPreviewLoadingRetryAndCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("203.0.113.9")) }))
	defer server.Close()
	m := newMainMenu(context.Background(), "/project", en)
	m.endpoint = server.URL
	msg := m.load("public-ip")().(previewMsg)
	if msg.err != nil || msg.value != "203.0.113.9" {
		t.Fatalf("%+v", msg)
	}
	m = update(m, previewMsg{id: "public-ip", err: problem("失败", "Failed")})
	m = search(m, "public-ip")
	model, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = model.(menuModel)
	if cmd == nil || !m.rows[1].loading {
		t.Fatal("retry did not start")
	}
	m = update(m, cmd())
	m = key(m, tea.KeyEnter)
	if m.result != "203.0.113.9" {
		t.Fatal(m.result)
	}
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer slow.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	m = newMainMenu(ctx, "/project", en)
	m.endpoint = slow.URL
	if msg := m.load("public-ip")().(previewMsg); msg.err == nil {
		t.Fatal("timeout must fail")
	}
}
