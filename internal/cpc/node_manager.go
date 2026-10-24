package cpc

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Prefer the nearest explicit declaration, then the nearest directory with locks.
// Include the Git root, but never inherit configuration from outside the repository.
func nodeManager(dir string) (string, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	var lockManager string
	var lockErr error
	foundLocks := false
	for {
		path := filepath.Join(dir, "package.json")
		data, err := os.ReadFile(path)
		if err == nil {
			var pkg struct {
				PackageManager string `json:"packageManager"`
			}
			if err := json.Unmarshal(data, &pkg); err != nil {
				return "", problem("解析 %s 失败: %w", "Cannot parse %s: %w", path, err)
			}
			if pkg.PackageManager != "" {
				manager, _, _ := strings.Cut(pkg.PackageManager, "@")
				switch manager {
				case "npm", "pnpm", "yarn", "bun":
					return manager, nil
				default:
					return "", problem("packageManager %q 不受支持（文件 %s；支持 npm、pnpm、yarn、bun）", "Unsupported packageManager %q in %s (supported: npm, pnpm, yarn, bun)", pkg.PackageManager, path)
				}
			}
		} else if !os.IsNotExist(err) {
			return "", err
		}
		if !foundLocks {
			managers := []string{}
			for _, candidate := range []struct {
				name  string
				files []string
			}{
				{"npm", []string{"package-lock.json", "npm-shrinkwrap.json"}},
				{"pnpm", []string{"pnpm-lock.yaml"}},
				{"yarn", []string{"yarn.lock"}},
				{"bun", []string{"bun.lock", "bun.lockb"}},
			} {
				present := false
				for _, name := range candidate.files {
					info, err := os.Stat(filepath.Join(dir, name))
					if err != nil && !os.IsNotExist(err) {
						return "", err
					}
					if err == nil && info.Mode().IsRegular() {
						present = true
					}
				}
				if present {
					managers = append(managers, candidate.name)
				}
			}
			if len(managers) > 0 {
				foundLocks = true
				lockManager = managers[0]
				if len(managers) > 1 {
					lockErr = problem("%s 存在多种包管理器锁文件（%s）；请在 package.json 设置 packageManager", "Multiple package manager lockfiles in %s (%s); set packageManager in package.json", dir, strings.Join(managers, ", "))
				}
			}
		}
		_, err = os.Stat(filepath.Join(dir, ".git"))
		if err == nil {
			break
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	if lockErr != nil {
		return "", lockErr
	}
	if lockManager != "" {
		return lockManager, nil
	}
	return "npm", nil
}
