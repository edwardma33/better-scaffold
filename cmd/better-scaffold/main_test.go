package main

import (
	"bytes"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestInitArguments(t *testing.T) {
	for _, args := range [][]string{{}, {"init"}} {
		got, err := parseOptions(args, io.Discard)
		if err != nil || got.directory != "" || got.preview || got.showVersion {
			t.Fatalf("unexpected wizard options for %v: %+v %v", args, got, err)
		}
	}
	for _, args := range [][]string{{"sample-app"}, {"init", "sample-app"}} {
		got, err := parseOptions(args, io.Discard)
		if err != nil || got.directory != "sample-app" {
			t.Fatalf("lost destination for %v: %+v %v", args, got, err)
		}
	}
	for _, args := range [][]string{
		{"init", "--dry-run", "--profile", "sample.json", "sample-app"},
		{"--profile", "sample.json", "init", "--dry-run", "sample-app"},
	} {
		got, err := parseOptions(args, io.Discard)
		if err != nil || !got.preview || got.profilePath != "sample.json" || got.directory != "sample-app" {
			t.Fatalf("lost init options for %v: %+v %v", args, got, err)
		}
	}
}

func TestInitHelpVersionAndInvalidArguments(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"init", "--help"}} {
		var output bytes.Buffer
		if _, err := parseOptions(args, &output); !errors.Is(err, flag.ErrHelp) || !strings.Contains(output.String(), "better-scaffold init") {
			t.Fatalf("missing help for %v: %v %s", args, err, output.String())
		}
	}
	for _, args := range [][]string{{"--version"}, {"init", "--version"}} {
		got, err := parseOptions(args, io.Discard)
		if err != nil || !got.showVersion {
			t.Fatalf("missing version for %v: %+v %v", args, got, err)
		}
	}
	for _, args := range [][]string{{"init", "one", "two"}, {"one", "two"}, {"init", "--profile"}, {"init", "--unknown"}} {
		if _, err := parseOptions(args, io.Discard); err == nil {
			t.Fatalf("accepted invalid arguments %v", args)
		}
	}
}

func TestWizardDefaultsWithoutProfile(t *testing.T) {
	c, err := wizardConfig("", "")
	if err != nil {
		t.Fatal(err)
	}
	if c.Name != "my-app" || c.PackageManager != "bun" || c.Payments != "none" || c.AI != "none" {
		t.Fatalf("unexpected default selections: %+v", c)
	}
	c, err = wizardConfig("", filepath.Join("apps", "custom-app"))
	if err != nil || c.Name != "custom-app" || c.Directory != filepath.Join("apps", "custom-app") {
		t.Fatalf("destination did not seed project name: %+v, %v", c, err)
	}
}

func TestWizardLoadsSelectedBatteries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile.json")
	err := os.WriteFile(path, []byte(`{"name":"review-app","packageManager":"npm","payments":"stripe","ai":"anthropic","oauth":["google","github"],"motion":true,"query":true,"zod":true,"elysia":true}`), 0600)
	if err != nil {
		t.Fatal(err)
	}
	c, err := wizardConfig(path, "custom-preview")
	if err != nil {
		t.Fatal(err)
	}
	if c.Name != "review-app" || c.Directory != "custom-preview" || c.PackageManager != "npm" || c.Payments != "stripe" || c.AI != "anthropic" || !c.Motion || !c.Query || !c.Zod || !c.Elysia || !reflect.DeepEqual(c.OAuth, []string{"google", "github"}) {
		t.Fatalf("profile choices were lost: %+v", c)
	}
	if len(c.Secrets) != 0 {
		t.Fatal("profile unexpectedly supplied secrets")
	}
}
