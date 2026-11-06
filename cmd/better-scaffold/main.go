package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"better-scaffold/internal/config"
	"better-scaffold/internal/profile"
	"better-scaffold/internal/tui"
	"github.com/charmbracelet/huh"
)

var version = "0.1.0"

type options struct {
	preview     bool
	profilePath string
	showVersion bool
	directory   string
}

func parseOptions(args []string, output io.Writer) (options, error) {
	var selected options
	flags := flag.NewFlagSet("better-scaffold", flag.ContinueOnError)
	flags.SetOutput(output)
	flags.BoolVar(&selected.preview, "dry-run", false, "Preview the wizard without generating files or running commands")
	flags.StringVar(&selected.profilePath, "profile", "", "Start the wizard with selections from a nonsecret JSON profile")
	flags.BoolVar(&selected.showVersion, "version", false, "Print version")
	flags.Usage = func() {
		fmt.Fprint(output, "better-scaffold init [options] [directory]\n\nCreate a connected Next.js app through an interactive terminal wizard.\nThe init command is optional. Place options before the destination directory.\n\n")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return selected, err
	}
	if flags.NArg() > 0 && flags.Arg(0) == "init" {
		if err := flags.Parse(flags.Args()[1:]); err != nil {
			return selected, err
		}
	}
	if flags.NArg() > 1 {
		return selected, fmt.Errorf("expected at most one destination directory; place options before the directory")
	}
	if flags.NArg() == 1 {
		selected.directory = flags.Arg(0)
	}
	return selected, nil
}

func main() {
	selected, err := parseOptions(os.Args[1:], os.Stdout)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if selected.showVersion {
		fmt.Println("better-scaffold " + version)
		return
	}

	c, err := wizardConfig(selected.profilePath, selected.directory)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Print("\033[2J\033[H")
	if err := tui.Run(context.Background(), c, selected.preview); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			fmt.Println("Cancelled. No project was generated.")
			return
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func wizardConfig(profilePath, directory string) (config.Config, error) {
	if profilePath != "" {
		return profile.Load(profilePath, directory)
	}
	c := config.Config{Name: "my-app", Directory: directory, PackageManager: "bun", Payments: "none", AI: "none", Secrets: map[string]string{}}
	if directory != "" {
		c.Name = filepath.Base(filepath.Clean(directory))
	}
	return c, nil
}
