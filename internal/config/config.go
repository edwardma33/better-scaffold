package config

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

type Config struct {
	Name, Directory, PackageManager, Preset, Payments, AI string
	OAuth                                                 []string
	Motion, Elysia, Query, Zod                            bool
	Secrets                                               map[string]string
}

var projectName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

func (c Config) Validate() error {
	if !projectName.MatchString(c.Name) || len(c.Name) > 100 {
		return fmt.Errorf("project name must contain lowercase letters, numbers, and hyphens")
	}
	if strings.TrimSpace(c.Directory) == "" {
		return fmt.Errorf("choose a destination directory")
	}
	if c.PackageManager != "bun" && c.PackageManager != "pnpm" && c.PackageManager != "npm" {
		return fmt.Errorf("unsupported package manager")
	}
	if c.Payments != "none" && c.Payments != "polar" && c.Payments != "stripe" {
		return fmt.Errorf("unsupported payment provider")
	}
	if c.AI != "none" && c.AI != "openai" && c.AI != "anthropic" {
		return fmt.Errorf("unsupported AI provider")
	}
	seen := map[string]bool{}
	for _, provider := range c.OAuth {
		if (provider != "google" && provider != "github") || seen[provider] {
			return fmt.Errorf("choose Google and GitHub at most once each")
		}
		seen[provider] = true
	}
	if value := c.Secrets["DATABASE_URL"]; value != "" {
		u, err := url.Parse(value)
		if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || u.Host == "" {
			return fmt.Errorf("DATABASE_URL must be a PostgreSQL connection URL")
		}
	}
	return nil
}

func (c Config) EnvKeys() []string {
	keys := []string{"DATABASE_URL", "BETTER_AUTH_SECRET", "BETTER_AUTH_URL"}
	for _, provider := range c.OAuth {
		p := strings.ToUpper(provider)
		keys = append(keys, p+"_CLIENT_ID", p+"_CLIENT_SECRET")
	}
	switch c.Payments {
	case "polar":
		keys = append(keys, "POLAR_ACCESS_TOKEN", "POLAR_WEBHOOK_SECRET", "POLAR_PRODUCT_ID")
	case "stripe":
		keys = append(keys, "STRIPE_SECRET_KEY", "STRIPE_WEBHOOK_SECRET", "STRIPE_PRICE_ID")
	}
	switch c.AI {
	case "openai":
		keys = append(keys, "OPENAI_API_KEY")
	case "anthropic":
		keys = append(keys, "ANTHROPIC_API_KEY")
	}
	return keys
}
