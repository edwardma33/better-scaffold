package generate

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"better-scaffold/internal/config"
	"better-scaffold/internal/templates"
)

const ShadcnVersion = "4.12.0"

type command struct {
	name string
	args []string
}
type runner func(context.Context, string, command) error

func WriteFiles(c config.Config) error {
	return run(context.Background(), c, nil, templates.Files, func(context.Context, string, command) error { return nil })
}

func Run(ctx context.Context, c config.Config, progress func(string)) error {
	if err := c.Validate(); err != nil {
		return err
	}
	for _, name := range []string{c.PackageManager, "node"} {
		if _, err := exec.LookPath(name); err != nil {
			return fmt.Errorf("install %s before generating the app: %w", name, err)
		}
	}
	version, err := exec.CommandContext(ctx, "node", "--version").Output()
	if err != nil {
		return fmt.Errorf("check Node.js version: %w", err)
	}
	major, err := strconv.Atoi(strings.Split(strings.TrimPrefix(strings.TrimSpace(string(version)), "v"), ".")[0])
	if err != nil || major < 22 {
		return fmt.Errorf("Node.js 22 or later is required")
	}
	return run(ctx, c, progress, templates.Files, execute)
}

func run(ctx context.Context, c config.Config, progress func(string), render func(config.Config) (map[string]string, error), execute runner) error {
	if err := c.Validate(); err != nil {
		return err
	}
	if progress == nil {
		progress = func(string) {}
	}
	if entries, err := os.ReadDir(c.Directory); err == nil {
		if len(entries) > 0 {
			return fmt.Errorf("destination must be empty")
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	files, err := render(c)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(c.Directory, 0755); err != nil {
		return err
	}
	progress("Writing application files")
	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		clean := filepath.Clean(p)
		if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return fmt.Errorf("invalid template path")
		}
		dest := filepath.Join(c.Directory, clean)
		if err = os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
			return err
		}
		if err = os.WriteFile(dest, []byte(files[p]), 0644); err != nil {
			return err
		}
	}
	env, example, err := environment(c)
	if err != nil {
		return err
	}
	redactions := make(map[string]string)
	for key, value := range c.Secrets {
		redactions[key] = value
	}
	for _, line := range strings.Split(env, "\n") {
		key, encoded, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		var value string
		if len(encoded) >= 2 && (encoded[0] == byte(96) || encoded[0] == byte(39)) {
			value = strings.ReplaceAll(encoded[1:len(encoded)-1], `\$`, "$")
			redactions[key] = value
		} else if json.Unmarshal([]byte(strings.ReplaceAll(encoded, `\$`, "$")), &value) == nil {
			redactions[key] = value
		}
	}
	c.Secrets = redactions
	if err = os.WriteFile(filepath.Join(c.Directory, ".env"), []byte(env), 0600); err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(c.Directory, ".env.example"), []byte(example), 0644); err != nil {
		return err
	}
	commands := []struct {
		label string
		cmd   command
	}{{"Installing dependencies", command{c.PackageManager, []string{"install"}}}}
	init := []string{"init", "--defaults"}
	if c.Preset != "" {
		init = []string{"init", "--preset", c.Preset, "--yes"}
	}
	commands = append(commands, struct {
		label string
		cmd   command
	}{"Initializing shadcn", tool(c.PackageManager, "shadcn@"+ShadcnVersion, init...)})
	add := []string{"add", "--yes", "card", "button", "input", "label"}
	commands = append(commands, struct {
		label string
		cmd   command
	}{"Installing UI components and logos", tool(c.PackageManager, "shadcn@"+ShadcnVersion, add...)})
	for _, step := range commands {
		if err = ctx.Err(); err != nil {
			return err
		}
		progress(step.label)
		if step.label == "Installing UI components and logos" {
			if err = addRegistry(c.Directory); err != nil {
				return err
			}
		}
		if err = execute(ctx, c.Directory, step.cmd); err != nil {
			return failure(c, step.label, fmt.Errorf("%w\nRecovery command: %s", err, commandString(step.cmd)))
		}
	}
	if layout, ok := files["src/app/layout.tsx"]; ok {
		if err = os.WriteFile(filepath.Join(c.Directory, "src/app/layout.tsx"), []byte(layout), 0644); err != nil {
			return err
		}
	}
	if err = normalizeFonts(c.Directory); err != nil {
		return err
	}
	progress("Generating SQL migrations")
	if err = execute(ctx, c.Directory, local(c.PackageManager, "drizzle-kit", "generate")); err != nil {
		return failure(c, "Generating SQL migrations", fmt.Errorf("%w\nRecovery command: %s", err, commandString(local(c.PackageManager, "drizzle-kit", "generate"))))
	}
	if c.Secrets["DATABASE_URL"] != "" {
		progress("Applying database migrations")
		if err = execute(ctx, c.Directory, local(c.PackageManager, "drizzle-kit", "migrate")); err != nil {
			return failure(c, "Applying database migrations", fmt.Errorf("%w\nRecovery command: %s", err, commandString(local(c.PackageManager, "drizzle-kit", "migrate"))))
		}
	} else {
		progress("Database migration skipped. Add DATABASE_URL to .env and run the db:migrate script.")
	}
	progress("Project ready")
	return nil
}

func tool(pm, pkg string, args ...string) command {
	switch pm {
	case "bun":
		return command{"bun", append([]string{"x", pkg}, args...)}
	case "pnpm":
		return command{"pnpm", append([]string{"dlx", pkg}, args...)}
	default:
		return command{"npx", append([]string{"--yes", pkg}, args...)}
	}
}
func local(pm, pkg string, args ...string) command {
	switch pm {
	case "bun":
		return command{"bun", append([]string{"x", "--no-install", pkg}, args...)}
	case "pnpm":
		return command{"pnpm", append([]string{"exec", pkg}, args...)}
	default:
		return command{"npm", append([]string{"exec", "--", pkg}, args...)}
	}
}

func execute(ctx context.Context, dir string, c command) error {
	cmd := exec.CommandContext(ctx, c.name, c.args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "CI=1", "NEXT_TELEMETRY_DISABLED=1")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s failed: %w\n%s", c.name, err, out.String())
	}
	return nil
}

func environment(c config.Config) (string, string, error) {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return "", "", err
	}
	var env, example strings.Builder
	for _, key := range c.EnvKeys() {
		value := c.Secrets[key]
		if key == "BETTER_AUTH_SECRET" && value == "" {
			value = base64.RawURLEncoding.EncodeToString(secret)
		}
		if key == "BETTER_AUTH_URL" && value == "" {
			value = "http://localhost:3000"
		}
		encoded, err := dotenvValue(value)
		if err != nil {
			return "", "", fmt.Errorf("%s: %w", key, err)
		}
		fmt.Fprintf(&env, "%s=%s\n", key, encoded)
		fmt.Fprintf(&example, "%s=\n", key)
	}
	return env.String(), example.String(), nil
}

func failure(c config.Config, step string, err error) error {
	message := err.Error()
	for _, value := range c.Secrets {
		if value == "" {
			continue
		}
		message = strings.ReplaceAll(message, value, "[redacted]")
		if u, e := url.Parse(value); e == nil && u.User != nil {
			if password, ok := u.User.Password(); ok && password != "" {
				message = strings.ReplaceAll(message, password, "[redacted]")
			}
		}
	}
	return fmt.Errorf("%s: %s\nGenerated files remain in %s. Fix the configuration and rerun the failed command there", step, message, c.Directory)
}

func normalizeFonts(dir string) error {
	path := filepath.Join(dir, "src/app/globals.css")
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	text := string(b)
	text += "\n@theme inline {\n  --font-sans: var(--font-geist-sans);\n  --font-mono: var(--font-geist-mono);\n}\n"
	return os.WriteFile(path, []byte(text), 0644)
}

func commandString(c command) string {
	var out []string
	for _, a := range append([]string{c.name}, c.args...) {
		out = append(out, "'"+strings.ReplaceAll(a, "'", `'"'"'`)+"'")
	}
	return strings.Join(out, " ")
}
func addRegistry(dir string) error {
	p := filepath.Join(dir, "components.json")
	b, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var data map[string]json.RawMessage
	if err = json.Unmarshal(b, &data); err != nil {
		return err
	}
	regs := map[string]string{}
	if raw, ok := data["registries"]; ok {
		if err = json.Unmarshal(raw, &regs); err != nil {
			return err
		}
	}
	regs["@svgl"] = "https://svgl.app/r/{name}.json"
	data["registries"], err = json.Marshal(regs)
	if err != nil {
		return err
	}
	b, err = json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, b, 0644)
}

func dotenvValue(value string) (string, error) {
	value = strings.ReplaceAll(value, "$", `\$`)
	for _, quote := range []string{"`", "'"} {
		if !strings.Contains(value, quote) {
			return quote + value + quote, nil
		}
	}
	if strings.Contains(value, `"`) || strings.Contains(value, `\n`) || strings.Contains(value, `\r`) {
		return "", fmt.Errorf("value contains conflicting dotenv quote characters; enter this value manually after generation")
	}
	return `"` + strings.NewReplacer("\n", `\n`, "\r", `\r`).Replace(value) + `"`, nil
}
