package preview

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"better-scaffold/internal/config"
)

func fixture(t *testing.T) config.Config {
	return config.Config{Name: "preview", Directory: filepath.Join(t.TempDir(), "app"), PackageManager: "bun", Payments: "none", AI: "none", Secrets: map[string]string{}}
}
func files() map[string]string {
	return map[string]string{"package.json": `{"name":"preview","dependencies":{"next":"1"},"devDependencies":{}}`, stylesheet: "@import \"tailwindcss\";\nbody{}", "src/db/schema.ts": "schema", "obsolete.ts": "old"}
}
func initFake(c config.Config, files map[string]string) error {
	for p, b := range files {
		if err := write(c.Directory, p, b); err != nil {
			return err
		}
	}
	if err := write(c.Directory, ".env", "DATABASE_URL=private\nBETTER_AUTH_SECRET=private-key\nBETTER_AUTH_URL=http://localhost:3000\n"); err != nil {
		return err
	}
	write(c.Directory, stylesheet, "@import \"tailwindcss\";\n:root{--background:white}")
	write(c.Directory, "node_modules/keep", "dependency")
	write(c.Directory, "bun.lock", "lock")
	return nil
}
func TestRefreshPreservesDevelopmentState(t *testing.T) {
	c := fixture(t)
	current := files()
	render := func(config.Config) (map[string]string, error) { return current, nil }
	init := func(_ context.Context, c config.Config, _ func(string)) error { return initFake(c, current) }
	var commands []string
	exec := func(_ context.Context, _ string, name string, args ...string) error {
		commands = append(commands, name+" "+strings.Join(args, " "))
		return nil
	}
	if err := sync(context.Background(), c, render, init, exec, nil); err != nil {
		t.Fatal(err)
	}
	initialEnv, _ := os.ReadFile(filepath.Join(c.Directory, ".env"))
	current = files()
	delete(current, "obsolete.ts")
	current["new.ts"] = "new"
	current[stylesheet] = "@import \"tailwindcss\";\nbody{color:red}"
	if err := sync(context.Background(), c, render, init, exec, nil); err != nil {
		t.Fatal(err)
	}
	for p, want := range map[string]string{".env": string(initialEnv), "node_modules/keep": "dependency", "bun.lock": "lock"} {
		got, err := os.ReadFile(filepath.Join(c.Directory, p))
		if err != nil || string(got) != want {
			t.Fatalf("lost %s: %q %v", p, got, err)
		}
	}
	if _, err := os.Stat(filepath.Join(c.Directory, "obsolete.ts")); !os.IsNotExist(err) {
		t.Fatal("stale managed file remains")
	}
	css, _ := os.ReadFile(filepath.Join(c.Directory, stylesheet))
	if !strings.Contains(string(css), "color:red") || !strings.Contains(string(css), "preview-theme.css") || strings.Contains(string(css), "tailwindcss") {
		t.Fatal(string(css))
	}
	preserved, _ := os.ReadFile(filepath.Join(c.Directory, theme))
	if !strings.Contains(string(preserved), "--background:white") {
		t.Fatal("theme lost")
	}
	for _, cmd := range commands {
		if strings.Contains(cmd, "migrate") || strings.Contains(cmd, "install") {
			t.Fatalf("unexpected command %s", cmd)
		}
	}
}
func TestRefusesUnownedDirectory(t *testing.T) {
	c := fixture(t)
	write(c.Directory, "keep", "original")
	err := sync(context.Background(), c, func(config.Config) (map[string]string, error) { return files(), nil }, nil, nil, nil)
	if err == nil {
		t.Fatal("accepted unowned directory")
	}
	got, _ := os.ReadFile(filepath.Join(c.Directory, "keep"))
	if string(got) != "original" {
		t.Fatal("changed unowned file")
	}
}
func TestDependencyRefreshAndRetry(t *testing.T) {
	c := fixture(t)
	current := files()
	render := func(config.Config) (map[string]string, error) { return current, nil }
	init := func(_ context.Context, c config.Config, _ func(string)) error { return initFake(c, current) }
	noop := func(context.Context, string, string, ...string) error { return nil }
	if err := sync(context.Background(), c, render, init, noop, nil); err != nil {
		t.Fatal(err)
	}
	var disk map[string]any
	json.Unmarshal([]byte(current["package.json"]), &disk)
	disk["dependencies"].(map[string]any)["shadcn-dependency"] = "1"
	raw, _ := json.Marshal(disk)
	write(c.Directory, "package.json", string(raw))
	current = files()
	current["package.json"] = `{"name":"preview","dependencies":{"next":"2"},"devDependencies":{}}`
	calls := 0
	retry := func(_ context.Context, _ string, _ string, args ...string) error {
		if len(args) > 0 && args[0] == "install" {
			calls++
			if calls == 1 {
				return errors.New("install failed")
			}
		}
		return nil
	}
	if err := sync(context.Background(), c, render, init, retry, nil); err == nil {
		t.Fatal("expected failed install")
	}
	if err := sync(context.Background(), c, render, init, retry, nil); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatal("failed dependency sync was not retried")
	}
	raw, _ = os.ReadFile(filepath.Join(c.Directory, "package.json"))
	if !strings.Contains(string(raw), "shadcn-dependency") {
		t.Fatal("lost UI dependency")
	}
	c.Preset = "different"
	if err := sync(context.Background(), c, render, init, noop, nil); err == nil {
		t.Fatal("accepted preset change without reinitialization")
	}
}
func TestPreflightFailureDoesNotClaimEmptyDirectory(t *testing.T) {
	c := fixture(t)
	os.MkdirAll(c.Directory, 0755)
	err := sync(context.Background(), c, func(config.Config) (map[string]string, error) { return files(), nil }, func(context.Context, config.Config, func(string)) error { return errors.New("missing package manager") }, nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if _, err = os.Stat(filepath.Join(c.Directory, marker)); !os.IsNotExist(err) {
		t.Fatal("claimed incomplete directory before rendering")
	}
}
func TestFingerprintReadsTemplateContents(t *testing.T) {
	root := t.TempDir()
	write(root, "internal/templates/base/page.tmpl", "old")
	write(root, "cmd/preview/main.go", "package main")
	write(root, "go.mod", "module example")
	write(root, "preset.json", "{}")
	first, err := Fingerprint(root, filepath.Join(root, "preset.json"))
	if err != nil {
		t.Fatal(err)
	}
	write(root, "internal/templates/base/page.tmpl", "new")
	second, err := Fingerprint(root, filepath.Join(root, "preset.json"))
	if err != nil || first == second {
		t.Fatal("source content change was not detected", err)
	}
	write(root, ".preview/app/src/page.tsx", "user change")
	third, _ := Fingerprint(root, filepath.Join(root, "preset.json"))
	if second != third {
		t.Fatal("preview output triggered source refresh")
	}
}
