// Derived from grafana/cog PR #1098. Modified for cog-sharp; see NOTICE.

package csharp

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/grafana/cog/pkg/apiref"
	"github.com/grafana/cog/pkg/ir"
)

func TestDisjunctionConverterSurvivesPluginJSONTransport(t *testing.T) {
	object := ir.NewObject("demo", "FirstOrSecond", ir.NewStruct(
		ir.NewStructField("First", ir.NewRef("demo", "First")),
		ir.NewStructField("Second", ir.NewRef("demo", "Second")),
	))
	object.Type.Hints[ir.HintDiscriminatedDisjunctionOfRefs] = ir.DisjunctionType{
		Discriminator: "kind",
		DiscriminatorMapping: map[string]string{
			"first":  "First",
			"second": "Second",
		},
	}

	payload, err := json.Marshal(object)
	if err != nil {
		t.Fatal(err)
	}
	var transported ir.Object
	if err := json.Unmarshal(payload, &transported); err != nil {
		t.Fatal(err)
	}

	config := Config{NamespaceRoot: "Example.Foundation"}
	jenny := Converters{
		config: config,
		tmpl:   initTemplates(config, apiref.NewAPIReferenceCollector()),
	}
	before, err := jenny.renderRefs("Example.Foundation.Demo", "FirstOrSecond", object)
	if err != nil {
		t.Fatal(err)
	}
	after, err := jenny.renderRefs("Example.Foundation.Demo", "FirstOrSecond", transported)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("plugin transport changed the discriminator converter")
	}
}

func TestDisjunctionConverterQualifiesForeignBranches(t *testing.T) {
	object := ir.NewObject("demo", "FirstOrSecond", ir.NewStruct(
		ir.NewStructField("First", ir.NewRef("shared", "First")),
		ir.NewStructField("Second", ir.NewRef("shared", "Second")),
	))
	object.Type.Hints[ir.HintDiscriminatedDisjunctionOfRefs] = ir.DisjunctionType{
		Discriminator: "kind",
		DiscriminatorMapping: map[string]string{
			"first": "First", "second": "Second",
		},
	}
	config := Config{NamespaceRoot: "Example.Foundation"}
	jenny := Converters{
		config: config,
		tmpl:   initTemplates(config, apiref.NewAPIReferenceCollector()),
	}
	output, err := jenny.renderRefs("Example.Foundation.Demo", "FirstOrSecond", object)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"First", "Second"} {
		if !strings.Contains(string(output), "global::Example.Foundation.Shared."+name) {
			t.Fatalf("foreign branch %s has no fully qualified reference", name)
		}
	}
}
