package cpc

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type project struct{ kind, path string }

var errNoProject = problem("当前目录及父目录未找到 package.json、go.mod 或 Cargo.toml", "No package.json, go.mod or Cargo.toml in this directory or its parents")

// Stop at the nearest project directory; do not mix parent and child projects.
func findProjects(dir string) ([]project, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	for {
		projects := []project{}
		for _, candidate := range []struct{ kind, name string }{{"node", "package.json"}, {"go", "go.mod"}, {"rust", "Cargo.toml"}} {
			path := filepath.Join(dir, candidate.name)
			info, err := os.Stat(path)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				return nil, err
			}
			if !info.Mode().IsRegular() {
				return nil, problem("项目文件不是普通文件: %s", "Project manifest is not a regular file: %s", path)
			}
			projects = append(projects, project{candidate.kind, path})
		}
		if len(projects) > 0 {
			return projects, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil, errNoProject
		}
		dir = parent
	}
}

func nodeScripts(path string) ([]item, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, problem("解析 %s 失败: %w", "Cannot parse %s: %w", path, err)
	}
	manager, err := nodeManager(filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	entries := []item{}
	for name, command := range pkg.Scripts {
		entries = append(entries, item{Key: name, Title: name, Description: command, Script: &scriptValues{Name: name, Command: nodeRunCommand(manager, name), Content: command}})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Key < entries[j].Key })
	return entries, nil
}

func projectScripts(projects []project) ([]item, error) {
	entries := []item{}
	for _, p := range projects {
		var tasks []item
		switch p.kind {
		case "node":
			var err error
			tasks, err = nodeScripts(p.path)
			if err != nil {
				return nil, err
			}
		case "go":
			tasks = commandTasks([][2]string{
				{"build", "go build ./..."}, {"test", "go test ./..."}, {"run", "go run ."},
				{"fmt", "go fmt ./..."}, {"vet", "go vet ./..."}, {"tidy", "go mod tidy"}, {"generate", "go generate ./..."},
			})
		case "rust":
			tasks = commandTasks([][2]string{
				{"build", "cargo build"}, {"check", "cargo check"}, {"test", "cargo test"}, {"run", "cargo run"},
				{"fmt", "cargo fmt"}, {"clippy", "cargo clippy"}, {"doc", "cargo doc --open"}, {"clean", "cargo clean"},
			})
		}
		for _, task := range tasks {
			// Keep the copied value independent of the selector and display title.
			if task.Value == "" {
				task.Value = task.Key
			}
			if len(projects) > 1 {
				task.Key = p.kind + ":" + task.Key
				task.Title = task.Key
			}
			entries = append(entries, task)
		}
	}
	if len(entries) == 0 {
		return nil, problem("项目中没有可用脚本", "No scripts available in this project")
	}
	return entries, nil
}

func commandTasks(commands [][2]string) []item {
	tasks := make([]item, 0, len(commands))
	for _, command := range commands {
		tasks = append(tasks, item{Key: command[0], Title: command[0], Description: command[1], Value: command[1], Script: &scriptValues{Name: command[0], Command: command[1], Content: command[1]}})
	}
	return tasks
}
func scripts(dir string) ([]item, error) {
	projects, err := findProjects(dir)
	if err != nil {
		return nil, err
	}
	return projectScripts(projects)
}

// Script selectors may be namespaced; these values always retain the original task name.
type scriptValues struct{ Name, Command, Content string }

func (s scriptValues) value(format string) string {
	switch format {
	case "name":
		return s.Name
	case "command":
		return s.Command
	case "content":
		return s.Content
	}
	return ""
}
func nodeRunCommand(manager, name string) string {
	prefix := manager + " run "
	if strings.HasPrefix(name, "-") {
		prefix += "-- "
	}
	if name != "" && strings.Trim(name, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_-.:/") == "" {
		return prefix + name
	}
	return prefix + "'" + strings.ReplaceAll(name, "'", "'\"'\"'") + "'"
}
