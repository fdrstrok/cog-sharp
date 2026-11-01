package plugin

import (
	"strings"
	"testing"

	"github.com/grafana/cog/pkg/languages"
)

func TestReadConfigAcceptsSnakeCaseAndRejectsUnknownOptions(t *testing.T) {
	config, err := readConfig(map[string]any{
		"namespace_root":           "Example.Foundation",
		"generate_json_converters": true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if config.NamespaceRoot != "Example.Foundation" || !config.GenerateJSONConverters {
		t.Fatalf("snake_case options were not applied: %+v", config)
	}

	_, err = readConfig(map[string]any{"typo_in_option": true})
	if err == nil {
		t.Fatal("unknown options were silently accepted")
	}
}

func TestGenerateRejectsBuildersWithoutRuntime(t *testing.T) {
	language := &Language{}
	_, err := language.Generate(
		languages.Config{Builders: true},
		map[string]any{"skip_runtime": true},
		languages.Context{},
	)
	if err == nil || !strings.Contains(err.Error(), "skip_runtime") {
		t.Fatalf("expected runtime configuration error, got %v", err)
	}
}

func TestGenerateRejectsUnsupportedFeatures(t *testing.T) {
	tests := []struct {
		name   string
		global languages.Config
		config map[string]any
	}{
		{name: "equality", config: map[string]any{"generate_equality": true}},
		{name: "JSON without runtime", config: map[string]any{"skip_runtime": true, "generate_json_converters": true}},
		{name: "option converters", global: languages.Config{Converters: true}},
		{name: "API reference", global: languages.Config{APIReference: true}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := New(nil).Generate(test.global, test.config, languages.Context{})
			if err == nil {
				t.Fatal("unsupported feature was silently accepted")
			}
		})
	}
}
