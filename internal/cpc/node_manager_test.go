package cpc

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNodeManagerSignals(t *testing.T) {
	t.Setenv("PATH", "")
	for _, tc := range []struct{ declaration, lock, want string }{
		{"", "", "npm"}, {"", "package-lock.json", "npm"}, {"", "npm-shrinkwrap.json", "npm"},
		{"", "pnpm-lock.yaml", "pnpm"}, {"", "yarn.lock", "yarn"}, {"", "bun.lock", "bun"}, {"", "bun.lockb", "bun"},
		{"pnpm@10.0.0", "package-lock.json", "pnpm"}, {"yarn@4.0.0+sha224.hash", "pnpm-lock.yaml", "yarn"},
		{"bun@1.2.0", "yarn.lock", "bun"}, {"npm@11.0.0", "bun.lock", "npm"},
	} {
		t.Run(tc.declaration+tc.lock, func(t *testing.T) {
			dir := t.TempDir()
			os.Mkdir(filepath.Join(dir, ".git"), 0755)
			writeProjectFile(t, filepath.Join(dir, "package.json"), fmt.Sprintf(`{"packageManager":%q,"scripts":{"build":"vite build"}}`, tc.declaration))
			if tc.lock != "" {
				writeProjectFile(t, filepath.Join(dir, tc.lock), "")
			}
			entries, err := scripts(dir)
			if err != nil {
				t.Fatal(err)
			}
			task := entries[0].Script
			if task.Name != "build" || task.Content != "vite build" || task.Command != tc.want+" run build" {
				t.Fatalf("%+v", task)
			}
			m := newChildMenu(context.Background(), en, "script", entries)
			m = key(m, tea.KeyEnter)
			m = key(m, tea.KeyDown)
			m = key(m, tea.KeyEnter)
			if m.result != task.Command {
				t.Fatal(m.result)
			}
		})
	}
}
func TestNodeManagerWorkspaceAndBoundaries(t *testing.T) {
	root := t.TempDir()
	os.Mkdir(filepath.Join(root, ".git"), 0755)
	child := filepath.Join(root, "packages", "web")
	writeProjectFile(t, filepath.Join(root, "package.json"), `{"packageManager":"pnpm@10.0.0"}`)
	writeProjectFile(t, filepath.Join(child, "package.json"), `{"scripts":{"build":"vite build"}}`)
	writeProjectFile(t, filepath.Join(child, "yarn.lock"), "")
	if got, err := nodeManager(child); err != nil || got != "pnpm" {
		t.Fatalf("inherited declaration: %s %v", got, err)
	}
	writeProjectFile(t, filepath.Join(child, "package.json"), `{"packageManager":"bun@1.2.0"}`)
	if got, err := nodeManager(child); err != nil || got != "bun" {
		t.Fatalf("child declaration: %s %v", got, err)
	}
	writeProjectFile(t, filepath.Join(child, "package.json"), `{}`)
	writeProjectFile(t, filepath.Join(root, "package.json"), `{}`)
	writeProjectFile(t, filepath.Join(root, "pnpm-lock.yaml"), "")
	if got, err := nodeManager(child); err != nil || got != "yarn" {
		t.Fatalf("nearest lock: %s %v", got, err)
	}
	os.Remove(filepath.Join(child, "yarn.lock"))
	if got, err := nodeManager(child); err != nil || got != "pnpm" {
		t.Fatalf("inherited lock: %s %v", got, err)
	}
	// A .git file (worktree/submodule) is also a repository boundary.
	writeProjectFile(t, filepath.Join(child, ".git"), "gitdir: elsewhere")
	if got, err := nodeManager(child); err != nil || got != "npm" {
		t.Fatalf("escaped repo boundary: %s %v", got, err)
	}
}
func TestNodeManagerAmbiguousAndInvalid(t *testing.T) {
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, ".git"), 0755)
	writeProjectFile(t, filepath.Join(dir, "yarn.lock"), "")
	writeProjectFile(t, filepath.Join(dir, "pnpm-lock.yaml"), "")
	if _, err := nodeManager(dir); err == nil || !strings.Contains(err.Error(), "Multiple") {
		t.Fatal(err)
	}
	writeProjectFile(t, filepath.Join(dir, "package.json"), `{"packageManager":"pnpm@10"}`)
	if got, err := nodeManager(dir); err != nil || got != "pnpm" {
		t.Fatalf("explicit resolves conflict: %s %v", got, err)
	}
	for _, data := range []string{`{"packageManager":"unknown@1"}`, `{"packageManager":42}`, `{`} {
		writeProjectFile(t, filepath.Join(dir, "package.json"), data)
		if _, err := nodeManager(dir); err == nil {
			t.Fatal("expected invalid manifest error", data)
		}
	}
	os.Remove(filepath.Join(dir, "yarn.lock"))
	os.Remove(filepath.Join(dir, "pnpm-lock.yaml"))
	os.Remove(filepath.Join(dir, "package.json"))
	writeProjectFile(t, filepath.Join(dir, "bun.lock"), "")
	writeProjectFile(t, filepath.Join(dir, "bun.lockb"), "")
	if got, err := nodeManager(dir); err != nil || got != "bun" {
		t.Fatalf("same manager locks: %s %v", got, err)
	}
}
