package preview

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"better-scaffold/internal/config"
	"better-scaffold/internal/generate"
	"better-scaffold/internal/profile"
	"better-scaffold/internal/templates"
)

const marker = ".better-scaffold-preview.json"
const stylesheet = "src/app/globals.css"
const theme = "src/app/preview-theme.css"

type state struct {
	Version                int
	Ready                  bool
	PackageManager, Preset string
	OAuth                  []string
	Files                  []string
	Package                string
}
type runner func(context.Context, string, string, ...string) error

func Sync(ctx context.Context, root, profilePath string, log func(string)) (config.Config, error) {
	name := strings.TrimSuffix(filepath.Base(profilePath), filepath.Ext(profilePath))
	if name == "" || name == "." || name == ".." {
		return config.Config{}, fmt.Errorf("invalid profile filename")
	}
	c, err := profile.Load(profilePath, filepath.Join(root, ".preview", name))
	if err != nil {
		return c, err
	}
	err = sync(ctx, c, templates.Files, generate.Run, run, log)
	return c, err
}

func sync(ctx context.Context, c config.Config, render func(config.Config) (map[string]string, error), initialize func(context.Context, config.Config, func(string)) error, exec runner, log func(string)) error {
	if log == nil {
		log = func(string) {}
	}
	files, err := render(c)
	if err != nil {
		return err
	}
	var previous state
	raw, err := os.ReadFile(filepath.Join(c.Directory, marker))
	if os.IsNotExist(err) {
		entries, e := os.ReadDir(c.Directory)
		if e == nil && len(entries) > 0 {
			return fmt.Errorf("refusing nonempty preview without ownership marker: %s", c.Directory)
		}
		if e != nil && !os.IsNotExist(e) {
			return e
		}
		initErr := initialize(ctx, c, log)
		if _, e = os.Stat(filepath.Join(c.Directory, ".env")); e != nil {
			return initErr
		}
		current := newState(c, files, initErr == nil)
		if initErr == nil {
			if err = preserveTheme(c.Directory, files[stylesheet]); err != nil {
				return err
			}
		}
		if err = save(c.Directory, current); err != nil {
			return err
		}
		return initErr
	}
	if err != nil {
		return err
	}
	if err = json.Unmarshal(raw, &previous); err != nil || previous.Version != 1 {
		return fmt.Errorf("invalid preview ownership marker in %s", c.Directory)
	}
	if previous.PackageManager != c.PackageManager || previous.Preset != c.Preset {
		return fmt.Errorf("package manager or shadcn preset changed; use a new profile filename to initialize a separate preview")
	}
	if err = refresh(c, files, previous); err != nil {
		return err
	}
	if !previous.Ready {
		log("Retrying preview initialization")
		if err = exec(ctx, c.Directory, c.PackageManager, "install"); err != nil {
			return err
		}
		init := []string{"init", "--defaults"}
		if c.Preset != "" {
			init = []string{"init", "--preset", c.Preset, "--yes"}
		}
		if err = shadcn(ctx, c, exec, init...); err != nil {
			return err
		}
		if err = installUI(ctx, c, exec); err != nil {
			return err
		}
		if err = preserveTheme(c.Directory, files[stylesheet]); err != nil {
			return err
		}
		if err = write(c.Directory, "src/app/layout.tsx", files["src/app/layout.tsx"]); err != nil {
			return err
		}
	} else {
		_, modulesErr := os.Stat(filepath.Join(c.Directory, "node_modules"))
		if previous.Package != files["package.json"] || os.IsNotExist(modulesErr) {
			log("Installing changed dependencies")
			if err = exec(ctx, c.Directory, c.PackageManager, "install"); err != nil {
				return err
			}
		}
		added := []string{"add", "--yes"}
		for _, provider := range c.OAuth {
			if !contains(previous.OAuth, provider) {
				added = append(added, "@svgl/"+provider)
			}
		}
		if len(added) > 2 {
			if err = shadcn(ctx, c, exec, added...); err != nil {
				return err
			}
		}
	}
	oldSchema, _ := os.ReadFile(filepath.Join(c.Directory, ".preview-schema"))
	if !previous.Ready || string(oldSchema) != files["src/db/schema.ts"] {
		log("Generating SQL migration files; database migrations are never applied by preview")
		if err = exec(ctx, c.Directory, c.PackageManager, "run", "db:generate"); err != nil {
			return err
		}
	}
	if err = write(c.Directory, ".preview-schema", files["src/db/schema.ts"]); err != nil {
		return err
	}
	log("Preview refreshed. Next.js will reload changed pages.")
	return save(c.Directory, newState(c, files, true))
}
func newState(c config.Config, files map[string]string, ready bool) state {
	s := state{Version: 1, Ready: ready, PackageManager: c.PackageManager, Preset: c.Preset, OAuth: c.OAuth, Package: files["package.json"]}
	for path := range files {
		s.Files = append(s.Files, path)
	}
	sort.Strings(s.Files)
	return s
}
func refresh(c config.Config, files map[string]string, previous state) error {
	var example strings.Builder
	for _, key := range c.EnvKeys() {
		fmt.Fprintf(&example, "%s=\n", key)
	}
	files[".env.example"] = example.String()
	for _, path := range previous.Files {
		if !safe(path) {
			return fmt.Errorf("invalid preview manifest path")
		}
		if _, ok := files[path]; !ok && path != ".env" {
			if err := os.Remove(filepath.Join(c.Directory, path)); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
	}
	for path, body := range files {
		if path == ".env" {
			continue
		}
		if path == stylesheet && previous.Ready {
			body = previewCSS(body)
		}
		if path == "package.json" {
			merged, err := mergePackage(c.Directory, previous.Package, body)
			if err != nil {
				return err
			}
			body = merged
		}
		if err := write(c.Directory, path, body); err != nil {
			return err
		}
	}
	raw, err := os.ReadFile(filepath.Join(c.Directory, ".env"))
	if err != nil {
		return err
	}
	body := string(raw)
	for _, key := range c.EnvKeys() {
		found := false
		for _, line := range strings.Split(body, "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), key+"=") {
				found = true
				break
			}
		}
		if !found {
			body += "\n" + key + "=\n"
		}
	}
	return os.WriteFile(filepath.Join(c.Directory, ".env"), []byte(body), 0600)
}
func mergePackage(dir, old, next string) (string, error) {
	disk, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return "", err
	}
	var current, before, after map[string]json.RawMessage
	for _, entry := range []struct {
		raw    []byte
		target *map[string]json.RawMessage
	}{{disk, &current}, {[]byte(old), &before}, {[]byte(next), &after}} {
		if err = json.Unmarshal(entry.raw, entry.target); err != nil {
			return "", err
		}
	}
	for _, key := range []string{"dependencies", "devDependencies"} {
		existing, prev, want := map[string]string{}, map[string]string{}, map[string]string{}
		if err = json.Unmarshal(current[key], &existing); err != nil {
			return "", err
		}
		if err = json.Unmarshal(before[key], &prev); err != nil {
			return "", err
		}
		if err = json.Unmarshal(after[key], &want); err != nil {
			return "", err
		}
		for k := range prev {
			if _, ok := want[k]; !ok {
				delete(existing, k)
			}
		}
		for k, v := range want {
			existing[k] = v
		}
		after[key], err = json.Marshal(existing)
		if err != nil {
			return "", err
		}
	}
	if pm, ok := current["packageManager"]; ok {
		after["packageManager"] = pm
	}
	raw, err := json.MarshalIndent(after, "", "  ")
	return string(raw) + "\n", err
}
func preserveTheme(dir, base string) error {
	raw, err := os.ReadFile(filepath.Join(dir, stylesheet))
	if err != nil {
		return err
	}
	if err = write(dir, theme, string(raw)); err != nil {
		return err
	}
	return write(dir, stylesheet, previewCSS(base))
}
func previewCSS(base string) string {
	return "@import \"./preview-theme.css\";\n" + strings.ReplaceAll(base, "@import \"tailwindcss\";", "")
}
func safe(path string) bool {
	clean := filepath.Clean(path)
	return clean != "." && !filepath.IsAbs(clean) && clean != ".." && !strings.HasPrefix(clean, ".."+string(filepath.Separator))
}
func write(dir, path, body string) error {
	if !safe(path) {
		return fmt.Errorf("invalid preview path %q", path)
	}
	target := filepath.Join(dir, path)
	if previous, err := os.ReadFile(target); err == nil && string(previous) == body {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}
	return os.WriteFile(target, []byte(body), 0644)
}
func save(dir string, s state) error {
	raw, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return write(dir, marker, string(raw))
}
func contains(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}
func run(ctx context.Context, dir, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), "NEXT_TELEMETRY_DISABLED=1", "CI=1")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s failed: %w", name, strings.Join(args, " "), err)
	}
	return nil
}
func shadcn(ctx context.Context, c config.Config, exec runner, args ...string) error {
	pkg := "shadcn@" + generate.ShadcnVersion
	switch c.PackageManager {
	case "bun":
		return exec(ctx, c.Directory, "bun", append([]string{"x", pkg}, args...)...)
	case "pnpm":
		return exec(ctx, c.Directory, "pnpm", append([]string{"dlx", pkg}, args...)...)
	default:
		return exec(ctx, c.Directory, "npx", append([]string{"--yes", pkg}, args...)...)
	}
}
func installUI(ctx context.Context, c config.Config, exec runner) error {
	args := []string{"add", "--yes", "card", "button", "input", "label"}
	for _, p := range c.OAuth {
		args = append(args, "@svgl/"+p)
	}
	return shadcn(ctx, c, exec, args...)
}
