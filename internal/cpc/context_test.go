package cpc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestScriptsNearestPackage(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "app", "src")
	if err := os.MkdirAll(child, 0755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(root, "package.json"), []byte(`{"scripts":{"outer":"no"}}`), 0600)
	path := filepath.Join(root, "app", "package.json")
	os.WriteFile(path, []byte(`{"scripts":{"test":"echo test","build":"echo build"}}`), 0600)
	entries, err := scripts(child)
	if err != nil || len(entries) != 2 || entries[0].Key != "build" || entries[1].Key != "test" {
		t.Fatalf("%v, %v", entries, err)
	}
	for _, data := range []string{`{`, `{}`, `{"scripts":{"bad":42}}`} {
		os.WriteFile(path, []byte(data), 0600)
		if _, err := scripts(child); err == nil {
			t.Fatalf("expected error for %s", data)
		}
	}
}

func TestPublicIP(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
		valid      bool
	}{
		{"ipv4", "203.0.113.7\n", 200, true}, {"ipv6", "2001:db8::1", 200, true},
		{"invalid", "<html>error</html>", 200, false}, {"status", "203.0.113.7", 503, false},
		{"oversized", strings.Repeat("1", 200), 200, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tc.status); w.Write([]byte(tc.body)) }))
			defer server.Close()
			value, err := publicIP(context.Background(), server.URL)
			if (err == nil) != tc.valid {
				t.Fatalf("value %q, err %v", value, err)
			}
			if tc.valid && value != strings.TrimSpace(tc.body) {
				t.Fatal(value)
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := publicIP(ctx, "https://example.invalid"); err == nil {
		t.Fatal("expected cancellation")
	}
}

func TestGitStates(t *testing.T) {
	dir := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %s %v", args, out, err)
		}
		return strings.TrimSpace(string(out))
	}
	if _, err := gitValue(context.Background(), dir, "branch"); err == nil {
		t.Fatal("nonrepo should fail")
	}
	git("init", "-b", "test-context")
	if _, err := gitValue(context.Background(), dir, "commit"); err == nil {
		t.Fatal("unborn HEAD should fail")
	}
	git("-c", "user.name=Test", "-c", "user.email=test@example.com", "-c", "commit.gpgsign=false", "commit", "--allow-empty", "-m", "initial")
	hash := git("rev-parse", "HEAD")
	for kind, want := range map[string]string{"branch": "test-context", "commit": hash} {
		got, err := gitValue(context.Background(), dir, kind)
		if err != nil || got != want {
			t.Fatalf("%s: %q %v", kind, got, err)
		}
	}
	git("checkout", "--detach")
	if _, err := gitValue(context.Background(), dir, "branch"); err == nil {
		t.Fatal("detached HEAD should fail")
	}
	if got, err := gitValue(context.Background(), dir, "commit"); err != nil || got != hash {
		t.Fatalf("detached commit %q %v", got, err)
	}
}

func TestSearch(t *testing.T) {
	row := newMainMenu(context.Background(), "/project", en).rows[0]
	entry := item{Key: row.id, Title: row.title, Description: row.keywords}
	for _, query := range []string{"本地", "LOCAL ip", "网卡", "ip 本地"} {
		if !matches(query, entry) {
			t.Errorf("query %q did not match", query)
		}
	}
	if matches("外网", entry) {
		t.Fatal("unexpected match")
	}
	if strings.ContainsRune(display("evil\x1b\n\x07"), '\x1b') {
		t.Fatal("control character in display")
	}
}

func TestInvalidArguments(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	for _, args := range [][]string{{"nope"}, {"pwd", "extra"}, {"ip", "a", "b"}, {"script", "--format"}, {"script", "--format", ""}, {"script", "--format", "bad"}, {"pwd", "--format", "name"}} {
		if err := Run(context.Background(), args, "test"); err == nil {
			t.Fatalf("expected error: %v", args)
		}
	}
}
