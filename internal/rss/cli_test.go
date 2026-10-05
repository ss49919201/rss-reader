package rss

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func buildCLI(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "rss-reader")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/rss-reader")
	cmd.Dir = filepath.Join("..", "..")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	return bin
}

func runCLI(t *testing.T, bin string, env []string, args ...string) (string, string, int) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Env = env
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return stdout.String(), stderr.String(), 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return stdout.String(), stderr.String(), exitErr.ExitCode()
	}
	t.Fatalf("run: %v\n%s", err, stderr.String())
	return "", "", 1
}

func r2Env(endpoint string) []string {
	return []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + os.TempDir(),
		"R2_ACCOUNT_ID=0123456789abcdef0123456789abcdef",
		"R2_ACCESS_KEY_ID=testaccesskey",
		"R2_SECRET_ACCESS_KEY=testsecretkey",
		"R2_ENDPOINT=" + endpoint,
	}
}

func TestCLIUsesSitesDatabaseOnR2(t *testing.T) {
	bin := buildCLI(t)

	t.Run("bad interval", func(t *testing.T) {
		_, stderr, code := runCLI(t, bin, []string{"PATH=" + os.Getenv("PATH")}, "-interval", "1ms")
		if code != 2 || !strings.Contains(stderr, "interval") {
			t.Fatalf("code=%d stderr=%s", code, stderr)
		}
	})

	t.Run("missing config", func(t *testing.T) {
		_, stderr, code := runCLI(t, bin, []string{"PATH=" + os.Getenv("PATH")}, "-list")
		if code != 2 || !strings.Contains(stderr, "R2_ACCOUNT_ID") {
			t.Fatalf("code=%d stderr=%s", code, stderr)
		}
	})

	fake := newR2Fake(t)
	env := r2Env(fake.srv.URL)

	t.Run("list seeds sites.db", func(t *testing.T) {
		stdout, stderr, code := runCLI(t, bin, env, "-list")
		if code != 0 {
			t.Fatalf("code=%d stderr=%s", code, stderr)
		}
		for _, id := range []string{"go-blog", "zenn", "github-blog", "hatena-hotentry"} {
			if !strings.Contains(stdout, id) {
				t.Fatalf("stdout missing %s:\n%s", id, stdout)
			}
		}
		if fake.puts != 1 {
			t.Fatalf("puts=%d reqs=%v", fake.puts, fake.reqs)
		}
		foundPath := false
		for _, req := range fake.reqs {
			if strings.Contains(req, "/"+DefaultBucket+"/"+SitesObjectKey) {
				foundPath = true
			}
		}
		if !foundPath {
			t.Fatalf("requests=%v", fake.reqs)
		}
	})

	feed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(sampleRSS))
	}))
	defer feed.Close()

	t.Run("fetch reads sites from r2", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "sites.db")
		db, err := openSiteDB(path, true)
		if err != nil {
			t.Fatal(err)
		}
		if err := db.upsert(context.Background(), Site{ID: "demo", Name: "Demo", URL: feed.URL + "/rss"}); err != nil {
			t.Fatal(err)
		}
		if err := db.close(); err != nil {
			t.Fatal(err)
		}
		blob, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		fake.mu.Lock()
		fake.objs[DefaultBucket+"/"+SitesObjectKey] = blob
		puts := fake.puts
		fake.mu.Unlock()

		outDir := t.TempDir()
		_, stderr, code := runCLI(t, bin, env, "-out", outDir)
		if code != 0 {
			t.Fatalf("code=%d stderr=%s", code, stderr)
		}
		body, err := os.ReadFile(filepath.Join(outDir, "demo.html"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(body), "新しい記事") {
			t.Fatalf("html missing article:\n%s\nstderr=%s", body, stderr)
		}
		index, err := os.ReadFile(filepath.Join(outDir, "index.html"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(index), "Demo") {
			t.Fatalf("index missing site:\n%s", index)
		}
		fake.mu.Lock()
		defer fake.mu.Unlock()
		if fake.puts != puts {
			t.Fatalf("fetch uploaded the database: puts %d -> %d", puts, fake.puts)
		}
	})

	t.Run("add and remove", func(t *testing.T) {
		_, stderr, code := runCLI(t, bin, env, "-add", "-id", "local", "-name", "Local", "-url", feed.URL+"/rss")
		if code != 0 {
			t.Fatalf("add code=%d stderr=%s", code, stderr)
		}
		stdout, stderr, code := runCLI(t, bin, env, "-list")
		if code != 0 || !strings.Contains(stdout, "local") || !strings.Contains(stdout, feed.URL+"/rss") {
			t.Fatalf("code=%d stdout=%s stderr=%s", code, stdout, stderr)
		}
		_, stderr, code = runCLI(t, bin, env, "-remove", "local")
		if code != 0 {
			t.Fatalf("remove code=%d stderr=%s", code, stderr)
		}
		stdout, stderr, code = runCLI(t, bin, env, "-list")
		if code != 0 || strings.Contains(stdout, "\tlocal\t") || strings.HasPrefix(stdout, "local\t") {
			t.Fatalf("code=%d stdout=%s stderr=%s", code, stdout, stderr)
		}
	})
}
