package cpc

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func writeProjectFile(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
}
func TestScriptsAcrossProjectTypes(t *testing.T) {
	t.Setenv("PATH", "") // Listing tasks must not require or execute project toolchains.
	for _, tc := range []struct {
		manifest, body, key, want string
		count                     int
	}{
		{"package.json", `{"scripts":{"test":"node --test","build":"vite build"}}`, "test", "test", 2},
		{"go.mod", "module example.com/project\n\ngo 1.24\n", "test", "go test ./...", 7},
		{"Cargo.toml", "[package]\nname = 'demo'\nversion = '0.1.0'\n", "build", "cargo build", 8},
		{"Cargo.toml", "[workspace]\nmembers = []\n", "check", "cargo check", 8},
	} {
		t.Run(tc.manifest+tc.key, func(t *testing.T) {
			dir := t.TempDir()
			writeProjectFile(t, filepath.Join(dir, tc.manifest), tc.body)
			child := filepath.Join(dir, "src", "nested")
			os.MkdirAll(child, 0755)
			entries, err := scripts(child)
			if err != nil || len(entries) != tc.count {
				t.Fatalf("%+v %v", entries, err)
			}
			found := false
			for _, entry := range entries {
				if entry.Key == tc.key {
					found = true
					if entry.copyValue() != tc.want {
						t.Fatalf("copied %q", entry.copyValue())
					}
				}
			}
			if !found {
				t.Fatal("missing task", tc.key)
			}
			// Select through the same menu used by cpc script; identifiers aren't copied for Go/Rust.
			m := newChildMenu(context.Background(), en, "script", entries)
			m = search(m, tc.key)
			m = key(m, tea.KeyEnter)
			if m.page != "script-copy" || m.done {
				t.Fatal("expected copy options")
			}
			if tc.manifest != "package.json" {
				m = key(m, tea.KeyDown)
			}
			m = key(m, tea.KeyEnter)
			if !m.done || m.result != tc.want {
				t.Fatalf("selected %q", m.result)
			}
		})
	}
}
func TestNearestProjectAndMixedProjects(t *testing.T) {
	root := t.TempDir()
	writeProjectFile(t, filepath.Join(root, "package.json"), `{"scripts":{"parent":"echo parent"}}`)
	dir := filepath.Join(root, "backend")
	writeProjectFile(t, filepath.Join(dir, "go.mod"), "module example.com/backend\n")
	entries, err := scripts(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Key == "parent" {
			t.Fatal("parent project leaked into child")
		}
	}
	writeProjectFile(t, filepath.Join(dir, "package.json"), `{"scripts":{"build":"vite build"}}`)
	writeProjectFile(t, filepath.Join(dir, "Cargo.toml"), "[workspace]\n")
	entries, err = scripts(dir)
	if err != nil {
		t.Fatal(err)
	}
	wants := map[string]string{"node:build": "build", "go:build": "go build ./...", "rust:build": "cargo build"}
	seen := map[string]bool{}
	for _, e := range entries {
		if seen[e.Key] {
			t.Fatal("duplicate selector", e.Key)
		}
		seen[e.Key] = true
		if want, ok := wants[e.Key]; ok {
			if e.copyValue() != want {
				t.Fatalf("%s copies %q", e.Key, e.copyValue())
			}
			delete(wants, e.Key)
		}
	}
	if len(wants) > 0 {
		t.Fatal("missing tasks", wants)
	}
	m := newMainMenu(context.Background(), dir, en)
	msg := m.load("script")().(previewMsg)
	if msg.hidden || msg.err != nil || msg.value != "Node / Go / Rust" {
		t.Fatalf("%+v", msg)
	}
	m = update(m, msg)
	m = search(m, "script")
	m = key(m, tea.KeyEnter)
	m = search(m, "rust:build")
	m = key(m, tea.KeyEnter)
	m = key(m, tea.KeyDown)
	m = key(m, tea.KeyEnter)
	if m.result != "cargo build" {
		t.Fatal(m.result)
	}
}
func TestProjectDiscoveryFailures(t *testing.T) {
	dir := t.TempDir()
	if _, err := scripts(dir); !errors.Is(err, errNoProject) {
		t.Fatalf("expected no-project error: %v", err)
	}
	msg := newMainMenu(context.Background(), dir, en).load("script")().(previewMsg)
	if !msg.hidden {
		t.Fatal("missing project should hide entry")
	}
	writeProjectFile(t, filepath.Join(dir, "package.json"), `{`)
	msg = newMainMenu(context.Background(), dir, en).load("script")().(previewMsg)
	if msg.hidden || msg.err == nil {
		t.Fatal("invalid project should show error")
	}
	os.Remove(filepath.Join(dir, "package.json"))
	os.Mkdir(filepath.Join(dir, "Cargo.toml"), 0755)
	if _, err := scripts(dir); err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Fatal(err)
	}
}

func TestScriptCopyChoices(t *testing.T) {
	for _, tc := range []struct{ name, command, content string }{
		{"build", "npm run build", "vite build"},
		{"build", "go build ./...", "go build ./..."},
		{"build", "cargo build", "cargo build"},
		{"empty", "npm run empty", ""},
		{"multiline", "npm run multiline", "echo first\necho second\n"},
	} {
		for _, format := range []string{"name", "command", "content"} {
			values := &scriptValues{Name: tc.name, Command: tc.command, Content: tc.content}
			entries := []item{{Key: "node:" + tc.name, Title: "node:" + tc.name, Description: tc.content, Script: values}}
			m := newChildMenu(context.Background(), zh, "script", entries)
			m = search(m, tc.name)
			m = key(m, tea.KeyEnter)
			if m.done || m.page != "script-copy" || len(m.rows) != 3 {
				t.Fatal("expected three choices before copying")
			}
			m = update(m, tea.WindowSizeMsg{Width: 40, Height: 10})
			m = search(m, format)
			m = key(m, tea.KeyEnter)
			if !m.done || m.result != values.value(format) {
				t.Fatalf("%s: copied %q", format, m.result)
			}
		}
	}
}
func TestScriptCopyBackAndPreview(t *testing.T) {
	dir := t.TempDir()
	writeProjectFile(t, filepath.Join(dir, "package.json"), `{"scripts":{"build":"vite build","dev":"vite"}}`)
	m := newMainMenu(context.Background(), dir, en)
	m = update(m, m.load("script")())
	m = search(m, "Project scripts")
	m = key(m, tea.KeyEnter)
	m = search(m, "build")
	m = key(m, tea.KeyEnter)
	m = update(m, previewMsg{id: "public-ip", value: "203.0.113.5"})
	m = search(m, "content")
	m = key(m, tea.KeyEsc)
	if m.page != "script" || m.input.Value() != "build" || m.selectedID() != "build" {
		t.Fatalf("did not restore: page=%q query=%q selected=%q", m.page, m.input.Value(), m.selectedID())
	}
	m = key(m, tea.KeyEnter)
	m = key(m, tea.KeyCtrlN)
	m = key(m, tea.KeyCtrlN)
	m = key(m, tea.KeyCtrlP)
	m = key(m, tea.KeyEnter)
	if m.result != "npm run build" {
		t.Fatal(m.result)
	}
}
func TestNodeRunCommandQuotesNames(t *testing.T) {
	for name, want := range map[string]string{
		"build": "npm run build", "build:web": "npm run build:web", "build web": "npm run 'build web'",
		"$(touch /tmp/nope)": "npm run '$(touch /tmp/nope)'", "it's": "npm run 'it'\"'\"'s'", "-x": "npm run -- -x",
	} {
		if got := nodeRunCommand("npm", name); got != want {
			t.Fatalf("%q: %q", name, got)
		}
	}
}
func TestScriptFormatSkipsOnlyCopyPage(t *testing.T) {
	entries := commandTasks([][2]string{{"test", "go test ./..."}})
	m := newChildMenu(context.Background(), en, "script", entries)
	m.scriptFormat = "content"
	if m.done {
		t.Fatal("task not yet selected")
	}
	m = key(m, tea.KeyEnter)
	if !m.done || m.result != "go test ./..." {
		t.Fatal(m.result)
	}
}
