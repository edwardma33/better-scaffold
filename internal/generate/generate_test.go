package generate

import (
	"better-scaffold/internal/config"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func fixture(t *testing.T) config.Config {
	t.Helper()
	return config.Config{Name: "demo", Directory: filepath.Join(t.TempDir(), "demo"), PackageManager: "bun", Payments: "none", AI: "none"}
}
func render(config.Config) (map[string]string, error) {
	return map[string]string{"package.json": "{}"}, nil
}
func TestRefusesNonemptyDirectory(t *testing.T) {
	c := fixture(t)
	os.MkdirAll(c.Directory, 0755)
	os.WriteFile(filepath.Join(c.Directory, "keep"), []byte("original"), 0644)
	called := false
	err := run(context.Background(), c, nil, render, func(context.Context, string, command) error { called = true; return nil })
	if err == nil || called {
		t.Fatal("nonempty destination was accepted")
	}
	b, _ := os.ReadFile(filepath.Join(c.Directory, "keep"))
	if string(b) != "original" {
		t.Fatal("existing file changed")
	}
}
func TestMigrationOnlyWithDatabaseURL(t *testing.T) {
	for _, database := range []string{"", "postgres://user:pass@localhost/db"} {
		t.Run(database, func(t *testing.T) {
			c := fixture(t)
			c.Secrets = map[string]string{"DATABASE_URL": database}
			var commands []string
			err := run(context.Background(), c, nil, render, func(_ context.Context, _ string, cmd command) error {
				commands = append(commands, cmd.name+" "+strings.Join(cmd.args, " "))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			all := strings.Join(commands, "\n")
			if !strings.Contains(all, "drizzle-kit generate") {
				t.Fatal("missing migration generation")
			}
			if strings.Contains(all, "drizzle-kit migrate") != (database != "") {
				t.Fatal("incorrect migration application")
			}
			if commands[0] != "bun install" {
				t.Fatal(commands)
			}
			info, _ := os.Stat(filepath.Join(c.Directory, ".env"))
			if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
				t.Fatal("environment file permissions")
			}
		})
	}
}
func TestEnvironmentEscapesAndExamplesContainNoSecrets(t *testing.T) {
	c := fixture(t)
	value := "secret\"\nline\\$value"
	c.Secrets = map[string]string{"BETTER_AUTH_SECRET": value}
	env, example, err := environment(c)
	if err != nil {
		t.Fatal(err)
	}
	expected, err := dotenvValue(value)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(env, "BETTER_AUTH_SECRET="+expected+"\n") {
		t.Fatal("secret did not retain dotenv value")
	}

	if strings.Contains(example, "secret") {
		t.Fatal("secret leaked into example")
	}
}
func TestFailureRedactsCredentialsAndPreservesFiles(t *testing.T) {
	c := fixture(t)
	c.Secrets = map[string]string{"DATABASE_URL": "postgres://user:secret-pass@localhost/db"}
	err := run(context.Background(), c, nil, render, func(context.Context, string, command) error {
		return errors.New("cannot connect postgres://user:secret-pass@localhost/db secret-pass")
	})
	if err == nil || strings.Contains(err.Error(), "secret-pass") {
		t.Fatal("credentials leaked")
	}
	if _, err := os.Stat(filepath.Join(c.Directory, "package.json")); err != nil {
		t.Fatal("partial project removed")
	}
}
func TestPresetIsOneArgument(t *testing.T) {
	c := fixture(t)
	c.Preset = "https://ui.shadcn.com/init?x=$(touch /tmp/nope)"
	var init command
	err := run(context.Background(), c, nil, render, func(_ context.Context, _ string, cmd command) error {
		if strings.Contains(strings.Join(cmd.args, " "), " init ") {
			init = cmd
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, a := range init.args {
		if a == c.Preset {
			found = true
		}
	}
	if !found {
		t.Fatal("preset was not preserved as one argument")
	}
}

func TestEnvironmentWithNextParser(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node.js unavailable")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("home unavailable")
	}
	cached, err := filepath.Glob(filepath.Join(home, ".bun/install/cache/@next/env@*/dist/index.js"))
	if err != nil || len(cached) == 0 {
		t.Skip("cached Next.js environment parser unavailable")
	}
	c := fixture(t)
	c.OAuth = []string{"github"}
	c.Secrets = map[string]string{"BETTER_AUTH_SECRET": "quotes\" and single'\nbackslash\\ dollar$literal", "DATABASE_URL": "postgres://user:pass@localhost/db?sslmode=require&application_name=$literal", "GITHUB_CLIENT_SECRET": "quote`and dollar$value"}
	env, _, err := environment(c)
	if err != nil {
		t.Fatal(err)
	}
	os.MkdirAll(c.Directory, 0755)
	if err = os.WriteFile(filepath.Join(c.Directory, ".env"), []byte(env), 0600); err != nil {
		t.Fatal(err)
	}
	script := `const {loadEnvConfig}=require(process.argv[1]); const result=loadEnvConfig(process.argv[2],true,{info(){},error(e){throw e}}); process.stdout.write(JSON.stringify(result.parsedEnv));`
	cmd := exec.Command(node, "-e", script, cached[len(cached)-1], c.Directory)
	cmd.Env = []string{"PATH=" + os.Getenv("PATH")}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Next environment parser: %v %s", err, out)
	}
	var parsed map[string]string
	if err = json.Unmarshal(out, &parsed); err != nil {
		t.Fatal(err)
	}
	for key, want := range c.Secrets {
		if parsed[key] != want {
			t.Errorf("%s did not survive Next environment parsing", key)
		}
	}
}
