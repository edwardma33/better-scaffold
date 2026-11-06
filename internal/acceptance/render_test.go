package acceptance

import (
	"encoding/json"
	"strings"
	"testing"

	"better-scaffold/internal/config"
	"better-scaffold/internal/templates"
)

func TestGeneratedCombinations(t *testing.T) {
	for _, payment := range []string{"none", "polar", "stripe"} {
		for _, ai := range []string{"none", "openai", "anthropic"} {
			for bits := 0; bits < 16; bits++ {
				c := config.Config{Name: "sample-app", Directory: "sample-app", PackageManager: "bun", Payments: payment, AI: ai, OAuth: []string{"google", "github"}, Motion: bits&1 != 0, Elysia: bits&2 != 0, Query: bits&4 != 0, Zod: bits&8 != 0}
				files, err := templates.Files(c)
				if err != nil {
					t.Fatal(err)
				}
				var manifest struct {
					Name         string
					Dependencies map[string]string
				}
				if err := json.Unmarshal([]byte(files["package.json"]), &manifest); err != nil {
					t.Fatal(err)
				}
				if manifest.Name != c.Name || manifest.Dependencies["better-auth"] == "" {
					t.Fatal("missing app identity or authentication dependency")
				}
				for path, content := range files {
					if strings.Contains(content, "__PAYMENT_") || strings.Contains(content, "__SOCIAL_") || strings.Contains(content, "__APP_NAME__") {
						t.Fatalf("unresolved template in %s", path)
					}
				}
				for _, path := range []string{"src/app/(auth)/sign-in/page.tsx", "src/app/(auth)/sign-up/page.tsx", "src/app/dashboard/page.tsx", "src/db/schema.ts", ".gitignore"} {
					if files[path] == "" {
						t.Fatalf("missing %s", path)
					}
				}
				if (files["src/lib/ai.ts"] != "") != (ai != "none") {
					t.Fatal("AI selection mismatch")
				}
				if strings.Contains(files["src/db/schema.ts"], "stripe_customer_id") != (payment == "stripe") {
					t.Fatal("payment schema mismatch")
				}
			}
		}
	}
}

func TestSocialLogosAreIncludedWithSelectedProviders(t *testing.T) {
	for _, providers := range [][]string{nil, {"google"}, {"github"}, {"google", "github"}} {
		c := config.Config{Name: "app", Directory: "app", PackageManager: "bun", Payments: "none", AI: "none", OAuth: providers}
		files, err := templates.Files(c)
		if err != nil {
			t.Fatal(err)
		}
		form := files["src/components/auth-form.tsx"]
		for _, item := range []struct{ provider, name string }{{"google", "google"}, {"github", "githubLight"}, {"github", "githubDark"}} {
			selected := false
			for _, provider := range providers {
				selected = selected || provider == item.provider
			}
			_, included := files["src/components/ui/svgs/"+item.name+".tsx"]
			if included != selected || strings.Contains(form, "svgs/"+item.name) != selected {
				t.Fatalf("%s selection mismatch for %v", item.name, providers)
			}
		}
		if strings.Contains(form, "Wordmark") {
			t.Fatal("wordmark included in social buttons")
		}
		if (files["SVGL-LICENSE"] != "") != (len(providers) > 0) {
			t.Fatal("missing SVGL license notice")
		}
	}
}
