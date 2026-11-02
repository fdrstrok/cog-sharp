package plugin

import (
	"strings"
	"testing"

	"github.com/grafana/cog/pkg/ir"
	"github.com/grafana/cog/pkg/languages"
)

func TestReadConfigAcceptsSnakeCaseAndRejectsUnknownOptions(t *testing.T) {
	config, err := readConfig(map[string]any{
		"namespace_root":           "Example.Foundation",
		"generate_json_converters": true,
		"overrides_templates":      "/example/overrides",
	})
	if err != nil {
		t.Fatal(err)
	}
	if config.NamespaceRoot != "Example.Foundation" || !config.GenerateJSONConverters {
		t.Fatalf("snake_case options were not applied: %+v", config)
	}
	if len(config.OverridesTemplatesDirectories) != 1 || config.OverridesTemplatesDirectories[0] != "/example/overrides" {
		t.Fatal("scalar override directory was not decoded")
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

func TestRawQueriesDoNotReferenceSkippedRuntime(t *testing.T) {
	schema := ir.NewSchema("sample", ir.SchemaMeta{})
	query := ir.NewObject("sample", "Query", ir.NewStruct(
		ir.NewStructField("nested", ir.NewComposableSlot(ir.SchemaVariantDataQuery)),
	))
	query.Type.Hints[ir.HintImplementsVariant] = string(ir.SchemaVariantDataQuery)
	schema.AddObject(query)
	files, err := New(nil).Generate(
		languages.Config{Types: true}, map[string]any{"skip_runtime": true},
		languages.Context{Schemas: ir.Schemas{schema}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || strings.Contains(string(files[0].Data), "Cog.Variants") {
		t.Fatalf("raw-only query output depends on omitted runtime: %v", files)
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
