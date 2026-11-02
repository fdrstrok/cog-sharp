package sdkgen

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

func ValidateDashboard(root, document string) error {
	schemaPath := filepath.Join(root, ".cache", "foundation-overlay", "jsonschema", "dashboardv2.jsonschema.json")
	schemaFile, err := os.Open(schemaPath)
	if err != nil {
		return err
	}
	defer schemaFile.Close()
	schema, err := jsonschema.UnmarshalJSON(schemaFile)
	if err != nil {
		return err
	}
	compiler := jsonschema.NewCompiler()
	const schemaURL = "https://cog-sharp.invalid/dashboardv2.jsonschema.json"
	if err := compiler.AddResource(schemaURL, schema); err != nil {
		return err
	}
	compiled, err := compiler.Compile(schemaURL + "#/definitions/Dashboard")
	if err != nil {
		return err
	}
	file, err := os.Open(document)
	if err != nil {
		return err
	}
	defer file.Close()
	value, err := jsonschema.UnmarshalJSON(file)
	if err != nil {
		return err
	}
	if err := compiled.Validate(value); err != nil {
		return fmt.Errorf("dashboard does not match the pinned upstream schema: %w", err)
	}
	return nil
}
