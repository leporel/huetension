package mcp

import (
	"context"
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/leporel/huetension/internal/exporter"
)

// TestResourcesListAndRead drives a full resources/list → resources/read
// round-trip via the in-memory transport so we exercise the SDK
// dispatcher end-to-end (registration → list → read), not just the
// resource handler in isolation.
func TestResourcesListAndRead(t *testing.T) {
	ctx := context.Background()

	srv, _, err := Build(Config{Version: "test"})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	t1, t2 := sdk.NewInMemoryTransports()
	serverSession, err := srv.Connect(ctx, t1, nil)
	if err != nil {
		t.Fatalf("server.Connect: %v", err)
	}
	t.Cleanup(func() { _ = serverSession.Close() })

	client := sdk.NewClient(&sdk.Implementation{Name: "test-client", Version: "test"}, nil)
	clientSession, err := client.Connect(ctx, t2, nil)
	if err != nil {
		t.Fatalf("client.Connect: %v", err)
	}
	t.Cleanup(func() { _ = clientSession.Close() })

	list, err := clientSession.ListResources(ctx, nil)
	if err != nil {
		t.Fatalf("ListResources: %v", err)
	}
	var found *sdk.Resource
	for _, r := range list.Resources {
		if r.URI == EnvelopeSchemaURI {
			found = r
			break
		}
	}
	if found == nil {
		t.Fatalf("envelope schema resource %q not in resources/list (got %d)", EnvelopeSchemaURI, len(list.Resources))
	}
	if found.MIMEType != "application/schema+json" {
		t.Errorf("MIMEType = %q, want application/schema+json", found.MIMEType)
	}

	res, err := clientSession.ReadResource(ctx, &sdk.ReadResourceParams{URI: EnvelopeSchemaURI})
	if err != nil {
		t.Fatalf("ReadResource: %v", err)
	}
	if len(res.Contents) != 1 {
		t.Fatalf("expected 1 content, got %d", len(res.Contents))
	}
	c := res.Contents[0]
	if c.URI != EnvelopeSchemaURI {
		t.Errorf("content.URI = %q, want %q", c.URI, EnvelopeSchemaURI)
	}
	if c.MIMEType != "application/schema+json" {
		t.Errorf("content.MIMEType = %q, want application/schema+json", c.MIMEType)
	}
	if c.Text == "" {
		t.Fatalf("content.Text is empty")
	}

	var doc map[string]any
	if err := json.Unmarshal([]byte(c.Text), &doc); err != nil {
		t.Fatalf("unmarshal schema body: %v", err)
	}
	if id, _ := doc["$id"].(string); id != EnvelopeSchemaURI {
		t.Errorf("$id = %q, want %q", id, EnvelopeSchemaURI)
	}
	props, ok := topLevelProperties(doc)
	if !ok {
		t.Fatalf("schema is missing top-level properties")
	}
	for _, want := range []string{"schema", "tool", "params", "result"} {
		if _, ok := props[want]; !ok {
			t.Errorf("schema.properties missing %q", want)
		}
	}
}

// TestEnvelopeSchemaDoesNotDriftFromGoStructs is the "structural drift"
// guard for the hand-written schema. We reflect over the exporter's
// PaletteJSON / ColorJSON tags (the types every frontend emits) and
// assert each side knows about the same fields. If someone adds a field
// to ColorJSON without updating schemas/v1.json (or vice versa), this
// test fails.
//
// Scope: this catches field-set drift (the realistic mode) but not type
// or constraint drift — flipping `Hex string` to `Hex int`, or relaxing
// the schema's hex pattern, will not be flagged. Hardening the guard to
// type-level checks is on the table if drift bites us.
func TestEnvelopeSchemaDoesNotDriftFromGoStructs(t *testing.T) {
	doc := readEmbeddedSchema(t)

	t.Run("palette", func(t *testing.T) {
		schemaProps := defsProperties(t, doc, "palette")
		goProps := jsonFields(reflect.TypeFor[exporter.PaletteJSON]())
		assertSameSet(t, "$defs.palette.properties", goProps, schemaProps)
	})

	t.Run("color", func(t *testing.T) {
		schemaProps := defsProperties(t, doc, "color")
		goProps := jsonFields(reflect.TypeFor[exporter.ColorJSON]())
		assertSameSet(t, "$defs.color.properties", goProps, schemaProps)
	})
}

func readEmbeddedSchema(t *testing.T) map[string]any {
	t.Helper()
	data, err := schemaFS.ReadFile("schemas/v1.json")
	if err != nil {
		t.Fatalf("read embedded schema: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("unmarshal embedded schema: %v", err)
	}
	return doc
}

func topLevelProperties(doc map[string]any) (map[string]any, bool) {
	props, ok := doc["properties"].(map[string]any)
	return props, ok
}

func defsProperties(t *testing.T, doc map[string]any, defName string) map[string]any {
	t.Helper()
	defs, ok := doc["$defs"].(map[string]any)
	if !ok {
		t.Fatalf("schema missing $defs")
	}
	def, ok := defs[defName].(map[string]any)
	if !ok {
		t.Fatalf("schema missing $defs.%s", defName)
	}
	props, ok := def["properties"].(map[string]any)
	if !ok {
		t.Fatalf("schema $defs.%s missing properties", defName)
	}
	return props
}

// jsonFields returns the json-tag names of every exported field of t.
// Fields tagged json:"-" are skipped; "omitempty" suffixes are stripped.
// Embedded structs are not handled — none of our envelope types use
// embedding, so this simpler walk is enough.
func jsonFields(t reflect.Type) map[string]struct{} {
	out := map[string]struct{}{}
	for f := range t.Fields() {
		if !f.IsExported() {
			continue
		}
		tag, ok := f.Tag.Lookup("json")
		if !ok {
			continue
		}
		name, _, _ := strings.Cut(tag, ",")
		if name == "" || name == "-" {
			continue
		}
		out[name] = struct{}{}
	}
	return out
}

func assertSameSet(t *testing.T, label string, goSide map[string]struct{}, schemaSide map[string]any) {
	t.Helper()
	missingFromSchema := keysMissing(goSide, schemaSide)
	missingFromGo := keysMissingFromSet(schemaSide, goSide)
	if len(missingFromSchema) > 0 {
		t.Errorf("%s: schema missing fields present in Go struct: %v", label, missingFromSchema)
	}
	if len(missingFromGo) > 0 {
		t.Errorf("%s: Go struct missing fields present in schema: %v", label, missingFromGo)
	}
}

func keysMissing(want map[string]struct{}, have map[string]any) []string {
	var out []string
	for k := range want {
		if _, ok := have[k]; !ok {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

func keysMissingFromSet(have map[string]any, want map[string]struct{}) []string {
	var out []string
	for k := range have {
		if _, ok := want[k]; !ok {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}
