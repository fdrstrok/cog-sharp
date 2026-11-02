// Derived from grafana/cog PR #1098. Modified for cog-sharp; see NOTICE.

package csharp

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/grafana/codejen"
	"github.com/grafana/cog/pkg/ir"
	"github.com/grafana/cog/pkg/languages"
	"github.com/grafana/cog/pkg/template"
)

// RawTypes is the C# equivalent of [java.RawTypes]: it walks every
// schema in the input context and emits one `.cs` file per object
// (struct, enum, ref, intersection) plus an aggregated `Constants.cs`
// per package collecting concrete scalar values.
//
// Phase 2 scope: structs, enums, intersections-as-classes,
// concrete-scalar constants. Disjunctions, JSON converters and
// equality overrides are deliberately deferred.
type RawTypes struct {
	config Config
	tmpl   *template.Template

	// Reset per-file in genFilesForObject.
	imports        *importMap
	typeFormatter  *typeFormatter
	jsonMarshaller jsonMarshaller
}

func (jenny RawTypes) JennyName() string {
	return "CSharpRawTypes"
}

func (jenny RawTypes) Generate(context languages.Context) (codejen.Files, error) {
	files := make(codejen.Files, 0)
	jenny.typeFormatter = newTypeFormatter(context, jenny.config, nil)
	jenny.jsonMarshaller = newJSONMarshaller(jenny.config)

	for _, schema := range context.Schemas {
		out, err := jenny.genFilesForSchema(schema)
		if err != nil {
			return nil, err
		}
		files = append(files, out...)
	}

	return files, nil
}

func (jenny RawTypes) genFilesForSchema(schema *ir.Schema) (codejen.Files, error) {
	files := make(codejen.Files, 0)
	scalars := make(map[string]ir.ScalarType)
	pkgFolder := formatPackageName(schema.Package)
	namespace := jenny.config.formatNamespace(schema.Package)

	var iterErr error
	schema.Objects.Iterate(func(_ string, object ir.Object) {
		if iterErr != nil {
			return
		}

		// Concrete scalar definitions are aggregated into a single
		// Constants.cs per package.
		if object.Type.IsScalar() {
			if object.Type.AsScalar().IsConcrete() {
				scalars[object.Name] = object.Type.AsScalar()
			}
			return
		}

		output, err := jenny.generateObject(namespace, object)
		if err != nil {
			iterErr = err
			return
		}
		if output == nil {
			return
		}

		filename := filepath.Join(jenny.config.ProjectPath, pkgFolder, formatObjectName(object.Name)+".cs")
		files = append(files, *codejen.NewFile(filename, output, jenny))
	})
	if iterErr != nil {
		return nil, iterErr
	}

	if len(scalars) > 0 {
		output, err := jenny.formatConstants(namespace, scalars)
		if err != nil {
			return nil, err
		}
		filename := filepath.Join(jenny.config.ProjectPath, pkgFolder, "Constants.cs")
		files = append(files, *codejen.NewFile(filename, output, jenny))
	}

	return files, nil
}

func (jenny RawTypes) generateObject(namespace string, object ir.Object) ([]byte, error) {
	switch object.Type.Kind {
	case ir.KindStruct:
		return jenny.formatStruct(namespace, object)
	case ir.KindEnum:
		return jenny.formatEnum(namespace, object)
	case ir.KindRef:
		return jenny.formatRefAlias(namespace, object)
	case ir.KindIntersection:
		return jenny.formatIntersection(namespace, object)
	}
	return nil, nil
}

// renderClass renders the class template against the given model.
// All field/arg type expressions in `data` must already be formatted
// strings — the template no longer calls formatType so that imports
// are guaranteed to be populated before the {{ .Imports }} block runs.
func (jenny RawTypes) renderClass(data classTemplate) ([]byte, error) {
	return jenny.tmpl.RenderAsBytes("types/class.tmpl", data)
}

func (jenny RawTypes) formatStruct(namespace string, object ir.Object) ([]byte, error) {
	jenny.imports = newImportMap(jenny.config.namespaceRoot(), namespace)
	jenny.typeFormatter = jenny.typeFormatter.withImports(jenny.imports)

	fields := object.Type.AsStruct().Fields
	cls := jenny.buildClassTemplate(namespace, formatObjectName(object.Name), object.Comments, nil, fields, object)

	return jenny.renderClass(cls)
}

// formatRefAlias emits a thin C# subclass for a top-level type that is
// just a ref to another type. Mirrors the Java jenny's behaviour.
func (jenny RawTypes) formatRefAlias(namespace string, object ir.Object) ([]byte, error) {
	jenny.imports = newImportMap(jenny.config.namespaceRoot(), namespace)
	jenny.typeFormatter = jenny.typeFormatter.withImports(jenny.imports)

	ref := object.Type.AsRef()
	parent := jenny.typeFormatter.formatReference(ref)

	cls := classTemplate{
		Namespace: namespace,
		Name:      formatObjectName(object.Name),
		Imports:   jenny.imports,
		Comments:  object.Comments,
		Extends:   []string{parent},
	}
	return jenny.renderClass(cls)
}

func (jenny RawTypes) formatIntersection(namespace string, object ir.Object) ([]byte, error) {
	jenny.imports = newImportMap(jenny.config.namespaceRoot(), namespace)
	jenny.typeFormatter = jenny.typeFormatter.withImports(jenny.imports)

	intersection := object.Type.AsIntersection()
	extends := make([]string, 0)
	fields := make([]ir.StructField, 0)
	inheritedFieldNames := make(map[string]bool)

	for _, branch := range intersection.Branches {
		if branch.IsRef() {
			if obj, found := jenny.typeFormatter.context.GetObjectByRef(branch.AsRef()); found && obj.Type.IsStruct() {
				for _, field := range obj.Type.AsStruct().Fields {
					inheritedFieldNames[field.Name] = true
				}
			}
		}
	}

	for _, branch := range intersection.Branches {
		switch branch.Kind {
		case ir.KindRef:
			extends = append(extends, jenny.typeFormatter.formatReference(branch.AsRef()))
		case ir.KindStruct:
			for _, field := range branch.AsStruct().Fields {
				if inheritedFieldNames[field.Name] {
					continue
				}
				fields = append(fields, field)
			}
		}
	}

	cls := jenny.buildClassTemplate(namespace, formatObjectName(object.Name), object.Comments, extends, fields, object)
	return jenny.renderClass(cls)
}

// buildClassTemplate populates the template-facing classTemplate struct
// from a struct's fields, generating the field declarations and the
// two constructors' assignment lists.
//
// `object` is threaded through so the model can pick up
// disjunction-derived hints (used to nullable-ify value-type scalars
// and skip default initialization).
func (jenny RawTypes) buildClassTemplate(namespace string, name string, comments []string, extends []string, fields []ir.StructField, object ir.Object) classTemplate {
	clsFields := make([]classField, 0, len(fields))
	defaults := make([]assignment, 0, len(fields))
	args := make([]classArg, 0, len(fields))
	argAssignments := make([]assignment, 0, len(fields))

	isDisjunctionStruct := needsCustomConverter(object)
	if object.Type.IsDataqueryVariant() && !jenny.config.SkipRuntime {
		extends = append(extends, "Cog.Variants.Dataquery")
	}
	jsonEnabled := jenny.jsonMarshaller.enabled()
	// Nullable value-type fields and the suppression of default
	// assignments are only required when we'll emit a custom
	// JsonConverter that relies on `null` as the branch sentinel. With
	// JSON marshalling disabled we keep the plain POCO shape so the
	// type stays usable as a regular DTO.
	disjunctionPOCO := isDisjunctionStruct && jsonEnabled

	for _, field := range fields {
		if !field.Required && field.Type.Default == nil {
			field.Type.Nullable = true
		}
		fieldName := formatFieldName(field.Name)
		argName := formatArgName(field.Name)
		typeExpr := jenny.typeFormatter.formatFieldType(field.Type)
		if disjunctionPOCO {
			typeExpr = disjunctionFieldType(typeExpr, field.Type)
		}

		var attrs []string
		if jsonEnabled {
			if attr := jenny.jsonMarshaller.fieldPropertyAttribute(field); attr != "" {
				attrs = append(attrs, attr)
			}
			if !field.Required {
				attrs = append(attrs, "[JsonIgnore(Condition = JsonIgnoreCondition.WhenWritingNull)]")
			}
		}

		clsFields = append(clsFields, classField{
			Name:       fieldName,
			Type:       typeExpr,
			Comments:   field.Comments,
			Attributes: attrs,
		})

		// Disjunction-derived structs leave fields un-initialised so
		// that `null` is the unambiguous "branch not selected" marker.
		if !disjunctionPOCO {
			var defaultExpr string
			if field.Type.Default != nil {
				defaultExpr = jenny.formatDefault(field.Type, field.Type.Default)
			} else if field.Required && !field.Type.Nullable {
				defaultExpr = jenny.typeFormatter.emptyValueForType(field.Type)
			}
			if defaultExpr != "" {
				defaults = append(defaults, assignment{Name: fieldName, Value: defaultExpr})
			}
		}

		args = append(args, classArg{Name: argName, Type: typeExpr})
		argAssignments = append(argAssignments, assignment{
			Name:         fieldName,
			ValueFromArg: argName,
		})
	}

	var classAttrs []string
	if jsonEnabled {
		if attr := jenny.jsonMarshaller.classConverterAttribute(object, jenny.typeFormatter.context); attr != "" {
			classAttrs = append(classAttrs, attr)
			jenny.imports.addNamespace("System.Text.Json.Serialization")
		}
		if len(clsFields) > 0 {
			// At least one [JsonPropertyName] attribute will be emitted.
			jenny.imports.addNamespace("System.Text.Json.Serialization")
		}
	}

	return classTemplate{
		Namespace:          namespace,
		Name:               name,
		Imports:            jenny.imports,
		Comments:           comments,
		ClassAttributes:    classAttrs,
		Extends:            extends,
		Fields:             clsFields,
		DefaultAssignments: defaults,
		HasArgsConstructor: len(args) > 0,
		Args:               args,
		ArgAssignments:     argAssignments,
	}
}

// formatDefault renders a literal expression for a field's default value.
// Schema defaults may include enum members, object fields, and array values.
func (jenny RawTypes) formatDefault(t ir.Type, value any) string {
	if t.IsScalar() {
		return formatScalarValue(t.AsScalar().ScalarKind, value)
	}
	if t.IsRef() {
		if object, found := jenny.typeFormatter.context.GetObjectByRef(t.AsRef()); found {
			if object.Type.IsEnum() {
				member, found := object.Type.AsEnum().MemberForValue(value)
				if found {
					return jenny.typeFormatter.referenceName(t.AsRef()) + "." + formatEnumMemberName(member.Name)
				}
			}
			if object.Type.IsStruct() {
				if values, ok := value.(map[string]any); ok {
					var assignments []string
					for _, field := range object.Type.AsStruct().Fields {
						if fieldValue, found := values[field.Name]; found {
							assignments = append(assignments, formatFieldName(field.Name)+" = "+jenny.formatDefault(field.Type, fieldValue))
						}
					}
					return "new " + jenny.typeFormatter.referenceName(t.AsRef()) + " { " + strings.Join(assignments, ", ") + " }"
				}
			}
		}
	}
	if t.IsArray() {
		if values, ok := value.([]any); ok {
			var items []string
			for _, item := range values {
				items = append(items, jenny.formatDefault(t.AsArray().ValueType, item))
			}
			return "new " + jenny.typeFormatter.formatArray(t.AsArray()) + " { " + strings.Join(items, ", ") + " }"
		}
	}
	return jenny.typeFormatter.emptyValueForType(t)
}

func (jenny RawTypes) formatEnum(namespace string, object ir.Object) ([]byte, error) {
	enumDef := object.Type.AsEnum()
	if len(enumDef.Values) == 0 {
		return nil, nil
	}

	isString := enumDef.Values[0].Type.IsScalar() && enumDef.Values[0].Type.AsScalar().ScalarKind == ir.KindString
	underlyingType := ""
	if !isString {
		underlyingType = formatScalarType(enumDef.Values[0].Type.AsScalar())
	}

	values := make([]EnumValue, 0, len(enumDef.Values))
	for _, v := range enumDef.Values {
		var raw any
		if isString {
			s, _ := v.Value.(string)
			raw = s
		} else {
			raw = formatScalarValue(v.Type.AsScalar().ScalarKind, v.Value)
		}
		values = append(values, EnumValue{
			Name:     formatEnumMemberName(v.Name),
			RawValue: raw,
		})
	}

	var enumImports fmt.Stringer
	var enumAttrs []string
	if isString && jenny.jsonMarshaller.enabled() {
		im := newImportMap(jenny.config.namespaceRoot(), namespace)
		im.addNamespace("System.Text.Json.Serialization")
		enumImports = im
		enumAttrs = append(enumAttrs, jenny.jsonMarshaller.enumConverterAttribute(formatObjectName(object.Name)))
	}

	data := enumTemplate{
		Namespace:      namespace,
		Name:           formatObjectName(object.Name),
		UnderlyingType: underlyingType,
		Comments:       object.Comments,
		IsString:       isString,
		Imports:        enumImports,
		EnumAttributes: enumAttrs,
		JSONEnabled:    jenny.jsonMarshaller.enabled(),
		Values:         values,
	}

	return jenny.tmpl.RenderAsBytes("types/enum.tmpl", data)
}

func (jenny RawTypes) formatConstants(namespace string, scalars map[string]ir.ScalarType) ([]byte, error) {
	consts := make([]constant, 0, len(scalars))
	for name, scalar := range scalars {
		consts = append(consts, constant{
			Name:  name,
			Type:  formatScalarType(scalar),
			Value: formatScalarValue(scalar.ScalarKind, scalar.Value),
		})
	}
	sort.SliceStable(consts, func(i, j int) bool { return consts[i].Name < consts[j].Name })

	return jenny.tmpl.RenderAsBytes("types/constants.tmpl", constantsTemplate{
		Namespace: namespace,
		Name:      "Constants",
		Constants: consts,
	})
}

// formatEnumMemberName converts a raw enum-value name into a valid C#
// PascalCased identifier.
func formatEnumMemberName(name string) string {
	if name == "" {
		return "Default"
	}
	formatted := pascalCase(name)
	// Identifier cannot start with a digit.
	if formatted == "" || (formatted[0] >= '0' && formatted[0] <= '9') {
		formatted = "V" + formatted
	}
	return escapeVarName(formatted)
}
