package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"

	"better-scaffold/internal/preview"
)

func main() {
	profile := flag.String("profile", "dev/presets/default.json", "Saved selections without credentials")
	once := flag.Bool("once", false, "Generate or refresh the preview without starting a server")
	syncOnly := flag.Bool("sync", false, "Refresh files using freshly compiled templates without starting a server")
	port := flag.Int("port", 3000, "Next.js development server port")
	flag.Parse()
	if flag.NArg() != 0 || *port < 1 || *port > 65535 {
		fmt.Fprintln(os.Stderr, "Use --profile <path> and a port between 1 and 65535.")
		os.Exit(2)
	}
	root, err := os.Getwd()
	if err != nil {
		fail(err)
	}
	if _, err = os.Stat(filepath.Join(root, "go.mod")); err != nil {
		fail(fmt.Errorf("run preview from the better-scaffold repository root"))
	}
	absolute, err := filepath.Abs(*profile)
	if err != nil {
		fail(err)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	c, err := preview.Sync(ctx, root, absolute, func(s string) { fmt.Println(s) })
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		fail(err)
	}
	fmt.Printf("Preview files ready in %s\n", c.Directory)
	if *once || *syncOnly {
		return
	}
	if err = preview.Watch(ctx, root, absolute, c, *port); err != nil {
		fail(err)
	}
}
func fail(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
