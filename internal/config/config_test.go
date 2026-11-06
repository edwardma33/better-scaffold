package config

import "testing"

func TestRejectInvalidConfiguration(t *testing.T) {
	base := Config{Name: "my-app", Directory: "/tmp/my-app", PackageManager: "bun", Payments: "none", AI: "none"}
	cases := []struct {
		name   string
		mutate func(*Config)
	}{
		{"name", func(c *Config) { c.Name = "../../escape" }},
		{"manager", func(c *Config) { c.PackageManager = "shell" }},
		{"payments", func(c *Config) { c.Payments = "other" }},
		{"AI", func(c *Config) { c.AI = "other" }},
		{"OAuth", func(c *Config) { c.OAuth = []string{"google", "google"} }},
		{"database", func(c *Config) { c.Secrets = map[string]string{"DATABASE_URL": "https://example.com"} }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := base
			tc.mutate(&c)
			if c.Validate() == nil {
				t.Fatal("accepted invalid configuration")
			}
		})
	}
	if err := base.Validate(); err != nil {
		t.Fatal(err)
	}
}
