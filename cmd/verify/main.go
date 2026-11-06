package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"

	"better-scaffold/internal/generate"
	"better-scaffold/internal/profile"
)

func main() {
	goOnly := flag.Bool("go-only", false, "Run Go checks without generating apps or downloading JavaScript packages")
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := verify(ctx, *goOnly); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func command(ctx context.Context, dir, name string, args ...string) error {
	fmt.Printf("\nRunning %s %v\n", name, args)
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	cmd.Env = append(os.Environ(), "NEXT_TELEMETRY_DISABLED=1")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %v: %w", name, args, err)
	}
	return nil
}

func verify(ctx context.Context, goOnly bool) error {
	for _, args := range [][]string{{"test", "./..."}, {"vet", "./..."}} {
		if err := command(ctx, "", "go", args...); err != nil {
			return err
		}
	}
	workspace, err := os.MkdirTemp("", "better-scaffold-verify-")
	if err != nil {
		return err
	}
	succeeded := false
	defer func() {
		if succeeded {
			_ = os.RemoveAll(workspace)
		} else {
			fmt.Fprintln(os.Stderr, "Verification artifacts retained in", workspace)
		}
	}()
	if err := command(ctx, "", "go", "build", "-o", filepath.Join(workspace, "better-scaffold"), "./cmd/better-scaffold"); err != nil {
		return err
	}
	if !goOnly {
		if err := command(ctx, "", "node", "--test", "scripts/package.test.mjs", "scripts/notices.test.mjs"); err != nil {
			return err
		}
		for _, name := range []string{"minimal", "polar", "stripe"} {
			c, err := profile.Load(filepath.Join("dev", "presets", name+".json"), filepath.Join(workspace, name))
			if err != nil {
				return err
			}
			fmt.Println("\nGenerating verification app:", name)
			if err := generate.Run(ctx, c, func(message string) { fmt.Println(message) }); err != nil {
				return err
			}
			for _, script := range []string{"typecheck", "build"} {
				if err := command(ctx, c.Directory, c.PackageManager, "run", script); err != nil {
					return err
				}
			}
		}
	}
	succeeded = true
	fmt.Println("\nVerification passed.")
	return nil
}
