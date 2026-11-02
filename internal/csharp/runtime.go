// Derived from grafana/cog PR #1098. Modified for cog-sharp; see NOTICE.

package csharp

import (
	"fmt"
	"path/filepath"

	"github.com/grafana/codejen"
	"github.com/grafana/cog/pkg/ir"
	"github.com/grafana/cog/pkg/languages"
	"github.com/grafana/cog/pkg/template"
)

// Runtime emits builder interfaces and the schema-derived JSON variant registry.
type Runtime struct {
	config Config
	tmpl   *template.Template
}

func (jenny Runtime) JennyName() string {
	return "CSharpRuntime"
}

func (jenny Runtime) Generate(context languages.Context) (codejen.Files, error) {
	builder, err := jenny.tmpl.RenderAsBytes("runtime/builder.tmpl", map[string]any{
		"Namespace": jenny.config.formatNamespace("cog"),
	})
	if err != nil {
		return nil, err
	}
	variants, err := jenny.tmpl.RenderAsBytes("runtime/variants.tmpl", map[string]any{
		"Namespace": jenny.config.namespaceRoot() + ".Cog.Variants",
		"JSON":      jenny.config.GenerateJSONConverters,
	})
	if err != nil {
		return nil, err
	}
	files := codejen.Files{
		*codejen.NewFile(filepath.Join(jenny.config.ProjectPath, "Cog", "IBuilder.cs"), builder, jenny),
		*codejen.NewFile(filepath.Join(jenny.config.ProjectPath, "Cog", "Variants", "Variants.cs"), variants, jenny),
	}
	if jenny.config.GenerateJSONConverters {
		type registration struct{ Identifier, Type, FieldConfig string }
		var queries, panels []registration
		typeName := func(pkg, name string) string {
			return "global::" + jenny.config.formatNamespace(pkg) + "." + formatObjectName(name)
		}
		for _, schema := range context.SchemasForVariant(string(ir.SchemaVariantDataQuery)) {
			if schema.EntryPoint == "" {
				return nil, fmt.Errorf("missing query entry point in %s", schema.Package)
			}
			queries = append(queries, registration{
				Identifier: schema.Metadata.Identifier, Type: typeName(schema.Package, schema.EntryPoint),
			})
		}
		for _, schema := range context.SchemasForVariant(string(ir.SchemaVariantPanel)) {
			if !schema.HasObject("Options") {
				continue
			}
			fields := "null"
			if schema.HasObject("FieldConfig") {
				fields = "typeof(" + typeName(schema.Package, "FieldConfig") + ")"
			}
			panels = append(panels, registration{
				Identifier: schema.Metadata.Identifier, Type: typeName(schema.Package, "Options"), FieldConfig: fields,
			})
		}
		registry, err := jenny.tmpl.RenderAsBytes("runtime/registry.tmpl", map[string]any{
			"Namespace": jenny.config.namespaceRoot() + ".Cog.Variants",
			"Queries":   queries, "Panels": panels,
		})
		if err != nil {
			return nil, err
		}
		files = append(files, *codejen.NewFile(filepath.Join(jenny.config.ProjectPath, "Cog", "Variants", "Registry.cs"), registry, jenny))
	}
	return files, nil
}
