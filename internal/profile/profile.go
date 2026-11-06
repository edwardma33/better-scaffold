package profile

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"better-scaffold/internal/config"
)

type selections struct {
	Name           string   `json:"name"`
	PackageManager string   `json:"packageManager"`
	Preset         string   `json:"preset"`
	Payments       string   `json:"payments"`
	AI             string   `json:"ai"`
	OAuth          []string `json:"oauth"`
	Motion         bool     `json:"motion"`
	Elysia         bool     `json:"elysia"`
	Query          bool     `json:"query"`
	Zod            bool     `json:"zod"`
}

func Load(path, directory string) (config.Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return config.Config{}, err
	}
	defer f.Close()
	s := selections{Name: "preview-app", PackageManager: "bun", Payments: "none", AI: "none"}
	d := json.NewDecoder(f)
	d.DisallowUnknownFields()
	if err = d.Decode(&s); err != nil {
		return config.Config{}, fmt.Errorf("read profile %s: %w", path, err)
	}
	if err = d.Decode(new(any)); err != io.EOF {
		return config.Config{}, fmt.Errorf("profile must contain one JSON object")
	}
	if directory == "" {
		directory = s.Name
	}
	c := config.Config{Name: s.Name, Directory: directory, PackageManager: s.PackageManager, Preset: s.Preset, Payments: s.Payments, AI: s.AI, OAuth: s.OAuth, Motion: s.Motion, Elysia: s.Elysia, Query: s.Query, Zod: s.Zod, Secrets: map[string]string{}}
	return c, c.Validate()
}
