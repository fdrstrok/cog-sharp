// Derived from grafana/cog PR #1098. Modified for cog-sharp; see NOTICE.

package csharp

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/fdrstrok/cog-sharp/internal/tools"
	"github.com/grafana/codejen"
	"github.com/grafana/cog/pkg/apiref"
	"github.com/grafana/cog/pkg/ir"
	"github.com/grafana/cog/pkg/ir/transforms"
	"github.com/grafana/cog/pkg/jennies"
	"github.com/grafana/cog/pkg/languages"
)

const LanguageRef = "csharp"

// defaultNamespaceRoot is used when Config.NamespaceRoot is empty.
const defaultNamespaceRoot = "Grafana.Foundation"

// Config holds the C# code-generation options.
//
// The C# jenny targets .NET 10 with nullable reference types enabled and
// uses System.Text.Json for (de)serialization. Generated files are laid out
// as <ProjectPath>/<Package>/<Type>.cs. The consuming project supplies
// its own .csproj.
type Config struct {
	// ProjectPath is the on-disk root for generated sources. Computed from
	// NamespaceRoot during InterpolateParameters; not user-configurable.
	ProjectPath string `yaml:"-"`

	// NamespaceRoot is the C# namespace prefix applied to every generated
	// package. Each schema package becomes `<NamespaceRoot>.<Package>`.
	// Defaults to "Grafana.Foundation" when empty.
	NamespaceRoot string `yaml:"namespace_root"`

	// OverridesTemplatesDirectories holds a list of directories containing
	// templates defining blocks used to override parts of
	// builders/types/....
	OverridesTemplatesDirectories []string `yaml:"overrides_templates"`
	// OverridesTemplatesFS holds an embedded filesystem containing
	// override templates.
	OverridesTemplatesFS fs.FS `yaml:"-"`
	// OverridesTemplateFuncs holds additional template functions to be
	// injected into the override templates.
	OverridesTemplateFuncs map[string]any `yaml:"-"`

	// ExtraFilesTemplatesDirectories holds a list of directories
	// containing templates describing files to be added to the generated
	// output.
	ExtraFilesTemplatesDirectories []string `yaml:"extra_files_templates"`

	// ExtraFilesTemplatesData holds additional data to be injected into
	// the templates described in ExtraFilesTemplatesDirectories.
	ExtraFilesTemplatesData map[string]string `yaml:"-"`

	// SkipRuntime disables runtime-related code generation when enabled.
	// Builders can NOT be generated with this flag turned on, as they
	// rely on the runtime to function.
	SkipRuntime bool `yaml:"skip_runtime"`

	// GenerateEquality controls the generation of `Equals()` and
	// `GetHashCode()` overrides on types.
	GenerateEquality bool `yaml:"generate_equality"`

	// GenerateJSONConverters controls the generation of
	// System.Text.Json `JsonConverter<T>` types alongside generated
	// classes. Required for disjunction/discriminator support.
	GenerateJSONConverters bool `yaml:"generate_json_converters"`

	GenerateBuilders   bool `yaml:"-"`
	GenerateConverters bool `yaml:"-"`
}

// namespaceRoot returns the configured namespace root or the default.
func (config *Config) namespaceRoot() string {
	if config.NamespaceRoot != "" {
		return config.NamespaceRoot
	}
	return defaultNamespaceRoot
}

// formatNamespace converts a schema package name into a fully-qualified C#
// namespace, e.g. "dashboard" -> "Grafana.Foundation.Dashboard".
func (config *Config) formatNamespace(pkg string) string {
	return fmt.Sprintf("%s.%s", config.namespaceRoot(), pascalCase(pkg))
}

func (config *Config) InterpolateParameters(interpolator func(input string) string) {
	config.NamespaceRoot = interpolator(config.NamespaceRoot)
	config.OverridesTemplatesDirectories = tools.Map(config.OverridesTemplatesDirectories, interpolator)
	config.ExtraFilesTemplatesDirectories = tools.Map(config.ExtraFilesTemplatesDirectories, interpolator)
	// Sources land under src/<NamespaceRoot-as-folders>/ so a future
	// switch to per-package .csproj projects is a no-op for source paths.
	config.ProjectPath = filepath.Join("src", strings.ReplaceAll(config.namespaceRoot(), ".", string(filepath.Separator)))
}

func (config *Config) MergeWithGlobal(global languages.Config) Config {
	newConfig := *config
	newConfig.GenerateBuilders = global.Builders
	// Option converters (JSON to builder source) are not implemented.
	newConfig.GenerateConverters = false
	return newConfig
}

type Language struct {
	config          Config
	apiRefCollector *apiref.APIReferenceCollector
}

func New(config Config) *Language {
	return &Language{
		config:          config,
		apiRefCollector: apiref.NewAPIReferenceCollector(),
	}
}

func (language *Language) Name() string {
	return LanguageRef
}

func (language *Language) Jennies(globalConfig languages.Config) *codejen.JennyList[languages.Context] {
	config := language.config.MergeWithGlobal(globalConfig)
	tmpl := initTemplates(config, language.apiRefCollector)

	jenny := codejen.JennyListWithNamer(func(_ languages.Context) string {
		return LanguageRef
	})
	jenny.AppendOneToMany(
		jennies.If(globalConfig.Types, RawTypes{config: config, tmpl: tmpl}),
		jennies.If(globalConfig.Types && config.GenerateJSONConverters && !config.SkipRuntime, &Converters{config: config, tmpl: tmpl}),
		jennies.If((globalConfig.Types || config.GenerateBuilders) && !config.SkipRuntime, Runtime{config: config, tmpl: tmpl}),
		jennies.If(config.GenerateBuilders && !config.SkipRuntime, Builder{
			config:          config,
			tmpl:            tmpl,
			apiRefCollector: language.apiRefCollector,
		}),
	)
	jenny.AddPostprocessors(jennies.GeneratedCommentHeader(globalConfig.Debug))

	return jenny
}

func (language *Language) CompilerPasses() transforms.Transforms {
	// Mirror Java's pass list as the starting point — C# shares the same
	// type-system constraints (nominal, nullable refs, no untagged unions).
	return transforms.Transforms{
		&transforms.AnonymousStructsToNamed{},
		&transforms.NotRequiredFieldAsNullableType{},
		&transforms.DisjunctionWithNullToOptional{},
		&transforms.DisjunctionOfConstantsToEnum{},
		&transforms.AnonymousEnumToExplicitType{},
		&transforms.FlattenDisjunctions{},
		&transforms.DisjunctionInferMapping{},
		&transforms.UndiscriminatedDisjunctionToAny{},
		&transforms.DisjunctionToType{},
		&transforms.RemoveIntersections{},
		&transforms.InlineObjectsWithTypes{InlineTypes: []ir.Kind{ir.KindScalar, ir.KindMap, ir.KindArray}},
	}
}

func (language *Language) NullableKinds() languages.NullableConfig {
	return languages.NullableConfig{
		Kinds:              []ir.Kind{ir.KindMap, ir.KindArray, ir.KindRef, ir.KindStruct},
		ProtectArrayAppend: true,
		AnyIsNullable:      true,
	}
}
