package tui

import (
	"strings"
	"testing"

	"better-scaffold/internal/config"
)

func TestSummaryHidesCredentialsAndReportsMigration(t *testing.T) {
	c := config.Config{Name: "test-app", Directory: "/tmp/test-app", PackageManager: "bun", Payments: "polar", AI: "openai", Secrets: map[string]string{"DATABASE_URL": "postgres://secret-password@localhost/db", "POLAR_ACCESS_TOKEN": "private-token"}}
	summary := Summary(c)
	for _, secret := range c.Secrets {
		if strings.Contains(summary, secret) {
			t.Fatal("summary exposed a credential")
		}
	}
	if !strings.Contains(summary, "Will run against your supplied database") {
		t.Fatal("missing automatic migration notice")
	}
	c.Secrets["DATABASE_URL"] = ""
	if !strings.Contains(Summary(c), "Deferred.") {
		t.Fatal("missing deferred migration notice")
	}
}

func TestCredentialsFollowSelectedProviders(t *testing.T) {
	c := config.Config{Payments: "stripe", AI: "anthropic", OAuth: []string{"github"}}
	keys := strings.Join(credentialKeys(c), ",")
	for _, wanted := range []string{"DATABASE_URL", "STRIPE_SECRET_KEY", "STRIPE_PRICE_ID", "ANTHROPIC_API_KEY", "GITHUB_CLIENT_SECRET"} {
		if !strings.Contains(keys, wanted) {
			t.Fatalf("missing %s", wanted)
		}
	}
	for _, excluded := range []string{"POLAR_", "OPENAI_", "GOOGLE_"} {
		if strings.Contains(keys, excluded) {
			t.Fatalf("unexpected provider %s", excluded)
		}
	}
}
