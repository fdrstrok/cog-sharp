package sdkgen

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/goccy/go-yaml"
)

const (
	CogVersion         = "v0.2.4"
	FoundationRevision = "72b873117be02d3cd352c4ac8590b6555b0d1ce2"
	RegistryRevision   = "f5c7186c5a8c568155e156a3d41c8bdb80bc53ae"
)

func Generate(ctx context.Context, root, foundation, registry string) error {
	root, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(root, "sdk", "Grafana.Foundation", "Grafana.Foundation.csproj")); err != nil {
		return fmt.Errorf("not a cog-sharp SDK checkout: %w", err)
	}
	foundation, err = checkout(ctx, root, foundation, "foundation-inputs",
		"https://github.com/grafana/grafana-foundation-sdk.git", FoundationRevision)
	if err != nil {
		return err
	}
	registry, err = checkout(ctx, root, registry, "kind-registry",
		"https://github.com/grafana/kind-registry.git", RegistryRevision)
	if err != nil {
		return err
	}

	overlay := filepath.Join(root, ".cache", "foundation-overlay")
	if err := os.RemoveAll(overlay); err != nil {
		return err
	}
	if err := os.MkdirAll(overlay, 0o755); err != nil {
		return err
	}
	if err := os.CopyFS(overlay, os.DirFS(filepath.Join(foundation, ".cog"))); err != nil {
		return fmt.Errorf("copy upstream codegen inputs: %w", err)
	}
	if err := os.CopyFS(filepath.Join(overlay, "jsonschema"), os.DirFS(filepath.Join(foundation, "jsonschema"))); err != nil {
		return fmt.Errorf("copy upstream validation schemas: %w", err)
	}
	if err := includeCSharpVeneers(overlay); err != nil {
		return err
	}

	output := filepath.Join(root, "sdk", "Grafana.Foundation", "Generated")
	configFile := filepath.Join(overlay, "config.yaml")
	payload, err := os.ReadFile(configFile)
	if err != nil {
		return err
	}
	var config map[string]any
	if err := yaml.Unmarshal(payload, &config); err != nil {
		return err
	}
	config["output"] = map[string]any{
		"directory": output, "types": true, "builders": true,
		"converters": false, "api_reference": false,
		"language_plugins": map[string]any{
			"csharp": map[string]any{
				"namespace_root":           "Grafana.Foundation",
				"generate_json_converters": true,
				"overrides_templates":      filepath.Join(root, "sdk", "templates", "overrides"),
			},
		},
	}
	parameters, ok := config["parameters"].(map[string]any)
	if !ok {
		return fmt.Errorf("upstream pipeline parameters are missing")
	}
	parameters["output_dir"] = output
	parameters["kind_registry_path"] = registry
	payload, err = yaml.Marshal(config)
	if err != nil {
		return err
	}
	if err := os.WriteFile(configFile, payload, 0o644); err != nil {
		return err
	}
	if err := os.RemoveAll(output); err != nil {
		return err
	}
	if err := run(ctx, root, "go", "run", "github.com/grafana/cog/cmd/cli@"+CogVersion,
		"generate", "--config", configFile, "--plugin-directory", filepath.Join(root, "bin")); err != nil {
		return err
	}
	provenance := fmt.Sprintf("Cog: %s\nFoundation SDK: %s\nKind registry: %s\n", CogVersion, FoundationRevision, RegistryRevision)
	return os.WriteFile(filepath.Join(output, "inputs.txt"), []byte(provenance), 0o644)
}

func checkout(ctx context.Context, root, supplied, name, url, revision string) (string, error) {
	dir := supplied
	if dir == "" {
		dir = filepath.Join(root, ".cache", name)
		if _, err := os.Stat(filepath.Join(dir, ".git")); os.IsNotExist(err) {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return "", err
			}
			if err := run(ctx, root, "git", "init", dir); err != nil {
				return "", err
			}
		}
		head, headErr := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "HEAD").Output()
		if headErr != nil || strings.TrimSpace(string(head)) != revision {
			dirty, err := exec.CommandContext(ctx, "git", "-C", dir, "status", "--porcelain").Output()
			if err != nil {
				return "", err
			}
			if len(dirty) != 0 {
				return "", fmt.Errorf("refusing to update modified input cache %s", dir)
			}
			if err := run(ctx, root, "git", "-C", dir, "fetch", "--depth=1", url, revision); err != nil {
				return "", err
			}
			if err := run(ctx, root, "git", "-C", dir, "switch", "--detach", "FETCH_HEAD"); err != nil {
				return "", err
			}
		}
	}
	dir, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	head, err := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "HEAD").Output()
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(string(head)) != revision {
		return "", fmt.Errorf("%s must be at pinned revision %s", dir, revision)
	}
	dirty, err := exec.CommandContext(ctx, "git", "-C", dir, "status", "--porcelain", "--", ".cog", "grafana", "jsonschema").Output()
	if err != nil {
		return "", err
	}
	if len(dirty) != 0 {
		return "", fmt.Errorf("codegen inputs have local changes in %s", dir)
	}
	return dir, nil
}

func includeCSharpVeneers(root string) error {
	return filepath.WalkDir(filepath.Join(root, "resources"), func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".yaml" {
			return nil
		}
		payload, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var config map[string]any
		if err := yaml.Unmarshal(payload, &config); err != nil {
			return err
		}
		values, ok := config["languages"].([]any)
		if !ok {
			return nil
		}
		languages := make([]string, 0, len(values))
		for _, value := range values {
			language, ok := value.(string)
			if !ok {
				return fmt.Errorf("invalid language selector in %s", path)
			}
			languages = append(languages, language)
		}
		if !slices.Contains(languages, "java") || slices.Contains(languages, "csharp") {
			return nil
		}
		config["languages"] = append(languages, "csharp")
		payload, err = yaml.Marshal(config)
		if err != nil {
			return err
		}
		return os.WriteFile(path, payload, 0o644)
	})
}

func run(ctx context.Context, dir, command string, args ...string) error {
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", command, err)
	}
	return nil
}
