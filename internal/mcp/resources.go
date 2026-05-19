package mcp

import (
	"context"
	"embed"
	"fmt"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// schemaFS holds the static envelope JSON Schema. Embedded at build time
// so the binary stays self-contained — the resource handler reads from
// this FS, never from the filesystem.
//
//go:embed schemas/v1.json
var schemaFS embed.FS

// EnvelopeSchemaURI is the canonical URI for the huetension/v1 envelope
// JSON Schema. Exposed as a const so tests and external clients can refer
// to it without hard-coding the string.
const EnvelopeSchemaURI = "huetension://schemas/v1"

// ResourceDescriptor mirrors Descriptor (tools) but for MCP resources.
// Carries enough metadata to drive both registration and the namespaced
// --enable/--disable filter without spinning up a server.
type ResourceDescriptor struct {
	URI            string
	Name           string
	Title          string
	Description    string
	MIMEType       string
	DefaultEnabled bool
	register       func(*sdk.Server)
}

// allResources is the canonical resource catalogue.
var allResources = []ResourceDescriptor{
	{
		URI:            EnvelopeSchemaURI,
		Name:           "schema.envelope.v1",
		Title:          "huetension/v1 envelope JSON Schema",
		Description:    "JSON Schema for the result envelope every huetension tool emits. Use this to validate tool responses or to teach a model the wire shape.",
		MIMEType:       "application/schema+json",
		DefaultEnabled: true,
		register:       registerEnvelopeSchemaResource,
	},
}

// AllResources returns a copy of the resource catalogue.
func AllResources() []ResourceDescriptor {
	out := make([]ResourceDescriptor, len(allResources))
	copy(out, allResources)
	return out
}

// ResolveEnabledResources applies the same enable/disable selectors used
// for tools — namespaced under "resources:" — to the resource catalogue.
// Bare names in --enable / --disable target tools, not resources, so a
// caller that only writes "color.convert" leaves resources at their
// DefaultEnabled state.
func ResolveEnabledResources(cfg Config) ([]ResourceDescriptor, error) {
	enable, disable, err := parseSelectorsBoth(cfg)
	if err != nil {
		return nil, err
	}
	known := resourceNames(allResources)
	if err := validateNames(enable, kindResource, known, "enable"); err != nil {
		return nil, err
	}
	if err := validateNames(disable, kindResource, known, "disable"); err != nil {
		return nil, err
	}

	out := make([]ResourceDescriptor, 0, len(allResources))
	for _, r := range allResources {
		if enable.hasAny(kindResource) {
			if !enable.matches(kindResource, r.URI) {
				continue
			}
		} else if !r.DefaultEnabled {
			continue
		}
		if disable.matches(kindResource, r.URI) {
			continue
		}
		out = append(out, r)
	}
	return out, nil
}

func resourceNames(in []ResourceDescriptor) map[string]struct{} {
	out := make(map[string]struct{}, len(in))
	for _, r := range in {
		out[r.URI] = struct{}{}
	}
	return out
}

// registerResources installs every enabled resource onto srv. Called by
// Build after the tool catalogue is wired.
func registerResources(srv *sdk.Server, cfg Config) error {
	enabled, err := ResolveEnabledResources(cfg)
	if err != nil {
		return err
	}
	for _, r := range enabled {
		r.register(srv)
	}
	return nil
}

// registerEnvelopeSchemaResource serves the embedded JSON Schema bytes as
// a single TextResourceContents. The schema is small enough that we don't
// bother with chunking or content-range support.
func registerEnvelopeSchemaResource(srv *sdk.Server) {
	srv.AddResource(&sdk.Resource{
		Name:        "schema.envelope.v1",
		Title:       "huetension/v1 envelope JSON Schema",
		Description: "JSON Schema for the result envelope every huetension tool emits. Use this to validate tool responses or to teach a model the wire shape.",
		MIMEType:    "application/schema+json",
		URI:         EnvelopeSchemaURI,
	}, envelopeSchemaHandler)
}

// envelopeSchemaHandler reads schemas/v1.json from the embedded FS. The
// file is bundled at build time, so the only error path here is a missing
// embed (caught by tests, never seen by clients in a release build).
func envelopeSchemaHandler(_ context.Context, req *sdk.ReadResourceRequest) (*sdk.ReadResourceResult, error) {
	data, err := schemaFS.ReadFile("schemas/v1.json")
	if err != nil {
		return nil, fmt.Errorf("mcp: read embedded envelope schema: %w", err)
	}
	return &sdk.ReadResourceResult{
		Contents: []*sdk.ResourceContents{
			{
				URI:      req.Params.URI,
				MIMEType: "application/schema+json",
				Text:     string(data),
			},
		},
	}, nil
}
