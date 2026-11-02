package sdkgen

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
)

func TestCheckoutResumesInterruptedFetchAndRejectsChangedSchemas(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	git := func(args ...string) string {
		t.Helper()
		output, err := exec.Command("git", args...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, output)
		}
		return strings.TrimSpace(string(output))
	}
	git("init", "-q", source)
	if err := os.Mkdir(filepath.Join(source, "jsonschema"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "jsonschema", "dashboard.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("-C", source, "add", ".")
	git("-C", source, "-c", "user.name=Test", "-c", "user.email=test@example.invalid",
		"-c", "commit.gpgsign=false", "commit", "-qm", "Input fixture")
	revision := git("-C", source, "rev-parse", "HEAD")

	cache := filepath.Join(root, ".cache", "inputs")
	git("init", "-q", cache)
	actual, err := checkout(context.Background(), root, "", "inputs", source, revision)
	if err != nil {
		t.Fatalf("could not resume initialized but unfetched cache: %v", err)
	}
	if actual != cache {
		t.Fatalf("checkout returned %s instead of %s", actual, cache)
	}
	if err := os.WriteFile(filepath.Join(cache, "jsonschema", "dashboard.json"), []byte(`{"modified":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = checkout(context.Background(), root, cache, "unused", source, revision)
	if err == nil || !strings.Contains(err.Error(), "local changes") {
		t.Fatalf("changed validation schemas passed pinned input verification: %v", err)
	}
}

func TestVeneerOverlayPreservesOtherLanguageSelectors(t *testing.T) {
	root := t.TempDir()
	resources := filepath.Join(root, "resources")
	if err := os.Mkdir(resources, 0o755); err != nil {
		t.Fatal(err)
	}

	fixtures := map[string]string{
		"unions": "languages: [go, java]\npackage: dashboardv2\noptions: []\n",
		"go":     "languages: [go]\npackage: dashboard\noptions: []\n",
		"all":    "languages: [all]\npackage: common\noptions: []\n",
	}
	for name, payload := range fixtures {
		if err := os.WriteFile(filepath.Join(resources, name+".yaml"), []byte(payload), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := includeCSharpVeneers(root); err != nil {
		t.Fatal(err)
	}
	for name, original := range fixtures {
		payload, err := os.ReadFile(filepath.Join(resources, name+".yaml"))
		if err != nil {
			t.Fatal(err)
		}
		if name != "unions" {
			if string(payload) != original {
				t.Fatalf("changed unrelated veneer %s", name)
			}
			continue
		}
		var config struct {
			Languages []string `yaml:"languages"`
			Package   string   `yaml:"package"`
		}
		if err := yaml.Unmarshal(payload, &config); err != nil {
			t.Fatal(err)
		}
		if config.Package != "dashboardv2" || len(config.Languages) != 3 ||
			config.Languages[0] != "go" || config.Languages[1] != "java" || config.Languages[2] != "csharp" {
			t.Fatalf("overlay lost existing configuration: %+v", config)
		}
	}
}
