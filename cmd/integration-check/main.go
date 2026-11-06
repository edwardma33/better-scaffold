package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"better-scaffold/internal/generate"
	"better-scaffold/internal/profile"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := check(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, dir, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	cmd.Env = append(os.Environ(), "NEXT_TELEMETRY_DISABLED=1")
	cmd.Cancel = func() error { return cmd.Process.Signal(os.Interrupt) }
	cmd.WaitDelay = 10 * time.Second
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s failed: %w", name, err)
	}
	return nil
}

func check(ctx context.Context) error {
	database := os.Getenv("TEST_DATABASE_URL")
	if database == "" {
		return fmt.Errorf("set TEST_DATABASE_URL to a disposable PostgreSQL database; DATABASE_URL is never used as a fallback")
	}
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	workspace, err := os.MkdirTemp("", "better-scaffold-integration-")
	if err != nil {
		return err
	}
	success := false
	defer func() {
		if success {
			_ = os.RemoveAll(workspace)
		} else {
			fmt.Fprintln(os.Stderr, "Integration artifacts retained in", workspace)
		}
	}()
	tooling := filepath.Join(workspace, "tooling")
	if err = os.MkdirAll(tooling, 0755); err != nil {
		return err
	}
	if err = run(ctx, tooling, "npm", "install", "--no-audit", "--no-fund", "--no-save", "playwright@1.56.1"); err != nil {
		return err
	}
	if err = run(ctx, tooling, "node", filepath.Join(tooling, "node_modules/playwright/cli.js"), "install", "--with-deps", "chromium"); err != nil {
		return err
	}
	for _, name := range []string{"minimal", "polar", "stripe"} {
		c, err := profile.Load(filepath.Join(root, "dev/presets", name+".json"), filepath.Join(workspace, name))
		if err != nil {
			return err
		}
		c.PackageManager = "npm"
		for _, key := range c.EnvKeys() {
			if err = os.Unsetenv(key); err != nil {
				return err
			}
		}
		if name == "minimal" {
			c.Secrets["DATABASE_URL"] = database
		}
		fmt.Println("Checking generated app:", name)
		if err = generate.Run(ctx, c, func(s string) { fmt.Println(s) }); err != nil {
			return err
		}
		if name == "minimal" {
			if err = run(ctx, c.Directory, "npm", "run", "db:migrate"); err != nil {
				return err
			}
		}
		for _, script := range []string{"typecheck", "build"} {
			if err = run(ctx, c.Directory, "npm", "run", script); err != nil {
				return err
			}
		}
		if name == "minimal" {
			if err = run(ctx, root, "node", filepath.Join(root, "scripts/auth-smoke.mjs"), c.Directory, tooling); err != nil {
				return err
			}
		}
	}
	success = true
	fmt.Println("Generated app integration checks passed.")
	return nil
}
