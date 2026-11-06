package preview

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"better-scaffold/internal/config"
)

func Watch(ctx context.Context, root, profilePath string, c config.Config, port int) error {
	initial, err := Fingerprint(root, profilePath)
	if err != nil {
		return err
	}
	args := []string{"run", "dev", "--port", fmt.Sprint(port)}
	if c.PackageManager == "npm" {
		args = []string{"run", "dev", "--", "--port", fmt.Sprint(port)}
	}
	server := exec.CommandContext(ctx, c.PackageManager, args...)
	server.Dir = c.Directory
	server.Stdout = os.Stdout
	server.Stderr = os.Stderr
	server.Env = append(os.Environ(), "NEXT_TELEMETRY_DISABLED=1")
	manageProcess(server)
	if err = server.Start(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- server.Wait() }()
	defer func() {
		if server.Process != nil {
			stopProcess(server)
		}
	}()
	fmt.Printf("\nPreview at http://localhost:%d\nWatching Go templates and %s. Ctrl+C stops the server.\n", port, profilePath)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case err = <-done:
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("Next.js preview stopped: %v", err)
		case <-ticker.C:
			next, e := Fingerprint(root, profilePath)
			if e != nil {
				fmt.Fprintln(os.Stderr, e)
				continue
			}
			if next == initial {
				continue
			}
			initial = next
			fmt.Println("Source changed. Recompiling templates and refreshing preview…")
			sync := exec.CommandContext(ctx, "go", "run", "./cmd/preview", "--sync", "--profile", profilePath)
			sync.Dir = root
			sync.Stdout = os.Stdout
			sync.Stderr = os.Stderr
			manageProcess(sync)
			if e = sync.Run(); e != nil {
				if ctx.Err() != nil {
					return nil
				}
				fmt.Fprintln(os.Stderr, "Refresh failed. Watching for the next source or profile edit.")
				continue
			}
		}
	}
}

func Fingerprint(root, profilePath string) (string, error) {
	var paths []string
	for _, dir := range []string{"internal", "cmd"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d fs.DirEntry, e error) error {
			if e != nil {
				return e
			}
			if !d.IsDir() && (strings.HasSuffix(path, ".go") || strings.HasSuffix(path, ".tmpl")) {
				paths = append(paths, path)
			}
			return nil
		})
		if err != nil {
			return "", err
		}
	}
	paths = append(paths, profilePath, filepath.Join(root, "go.mod"), filepath.Join(root, "go.sum"))
	sort.Strings(paths)
	digest := sha256.New()
	for _, path := range paths {
		f, err := os.Open(path)
		if os.IsNotExist(err) && filepath.Base(path) == "go.sum" {
			continue
		}
		if err != nil {
			return "", err
		}
		fmt.Fprintln(digest, path)
		_, err = io.Copy(digest, f)
		f.Close()
		if err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}
