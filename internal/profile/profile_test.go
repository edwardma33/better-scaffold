package profile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProfileSelections(t *testing.T) {
	for _, input := range []string{`{"secrets":{"DATABASE_URL":"secret"}}`, `{"directory":"/tmp/other"}`, `{"unknown":true}`, `{} {}`} {
		p := filepath.Join(t.TempDir(), "profile.json")
		os.WriteFile(p, []byte(input), 0600)
		if _, err := Load(p, ""); err == nil {
			t.Fatalf("accepted %s", input)
		}
	}
	p := filepath.Join(t.TempDir(), "profile.json")
	os.WriteFile(p, []byte(`{"name":"sample","oauth":["google"]}`), 0600)
	c, err := Load(p, "")
	if err != nil {
		t.Fatal(err)
	}
	if c.Directory != "sample" || c.PackageManager != "bun" || c.Payments != "none" || len(c.Secrets) != 0 {
		t.Fatal(c)
	}
	c, err = Load(p, "destination")
	if err != nil || c.Directory != "destination" {
		t.Fatal(c, err)
	}
}
