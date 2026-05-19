package mcp

import (
	"context"
	"fmt"
	"strings"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// PromptDescriptor mirrors Descriptor (tools) but for MCP prompts.
// Carries enough metadata for both --enable/--disable filtering and SDK
// registration so the catalogue can be inspected without spinning up a
// server.
type PromptDescriptor struct {
	Name           string
	Title          string
	Description    string
	Arguments      []*sdk.PromptArgument
	DefaultEnabled bool
	register       func(*sdk.Server)
}

// allPrompts is the canonical prompt catalogue. Ships two
// scaffolding prompts: extract-from-mood (creative) and audit-contrast
// (review). Both are pure templates — the LLM is expected to call the
// referenced tools itself.
var allPrompts = []PromptDescriptor{
	{
		Name:           "extract-from-mood",
		Title:          "Build a palette from a mood",
		Description:    "Pick a harmony rule and base color that fit a free-form mood phrase, then call harmony.generate to materialise the palette.",
		DefaultEnabled: true,
		Arguments: []*sdk.PromptArgument{
			{Name: "mood", Description: "Mood or vibe to translate into a palette (e.g. 'foggy seaside dawn', 'late-90s arcade').", Required: true},
			{Name: "n", Description: "Optional: how many colors to aim for. Defaults to 5.", Required: false},
		},
		register: registerExtractFromMoodPrompt,
	},
	{
		Name:           "audit-contrast",
		Title:          "Audit a list of fg/bg pairs for WCAG contrast",
		Description:    "Run a list of foreground/background color pairs through contrast.check and summarise AA/AAA pass/fail status.",
		DefaultEnabled: true,
		Arguments: []*sdk.PromptArgument{
			{Name: "pairs", Description: "Comma-separated fg:bg pairs (e.g. '#fff:#222, royalblue:#fafafa'). Each side accepts any color string contrast.check accepts.", Required: true},
		},
		register: registerAuditContrastPrompt,
	},
}

// AllPrompts returns a copy of the prompt catalogue.
func AllPrompts() []PromptDescriptor {
	out := make([]PromptDescriptor, len(allPrompts))
	copy(out, allPrompts)
	return out
}

// ResolveEnabledPrompts applies the namespaced "prompts:" selectors from
// cfg to the prompt catalogue. Same semantics as ResolveEnabled /
// ResolveEnabledResources.
func ResolveEnabledPrompts(cfg Config) ([]PromptDescriptor, error) {
	enable, disable, err := parseSelectorsBoth(cfg)
	if err != nil {
		return nil, err
	}
	known := promptNames(allPrompts)
	if err := validateNames(enable, kindPrompt, known, "enable"); err != nil {
		return nil, err
	}
	if err := validateNames(disable, kindPrompt, known, "disable"); err != nil {
		return nil, err
	}

	out := make([]PromptDescriptor, 0, len(allPrompts))
	for _, p := range allPrompts {
		if enable.hasAny(kindPrompt) {
			if !enable.matches(kindPrompt, p.Name) {
				continue
			}
		} else if !p.DefaultEnabled {
			continue
		}
		if disable.matches(kindPrompt, p.Name) {
			continue
		}
		out = append(out, p)
	}
	return out, nil
}

func promptNames(in []PromptDescriptor) map[string]struct{} {
	out := make(map[string]struct{}, len(in))
	for _, p := range in {
		out[p.Name] = struct{}{}
	}
	return out
}

// registerPrompts installs every enabled prompt onto srv. Called by Build
// after tools and resources are wired.
func registerPrompts(srv *sdk.Server, cfg Config) error {
	enabled, err := ResolveEnabledPrompts(cfg)
	if err != nil {
		return err
	}
	for _, p := range enabled {
		p.register(srv)
	}
	return nil
}

// registerExtractFromMoodPrompt wires the extract-from-mood prompt onto
// srv. The handler builds a one-message instruction the LLM can act on
// using harmony.generate (and any of the color tools it deems useful).
func registerExtractFromMoodPrompt(srv *sdk.Server) {
	srv.AddPrompt(&sdk.Prompt{
		Name:        "extract-from-mood",
		Title:       "Build a palette from a mood",
		Description: "Pick a harmony rule and base color that fit a free-form mood phrase, then call harmony.generate to materialise the palette.",
		Arguments: []*sdk.PromptArgument{
			{Name: "mood", Description: "Mood or vibe to translate into a palette (e.g. 'foggy seaside dawn', 'late-90s arcade').", Required: true},
			{Name: "n", Description: "Optional: how many colors to aim for. Defaults to 5.", Required: false},
		},
	}, extractFromMoodHandler)
}

func extractFromMoodHandler(_ context.Context, req *sdk.GetPromptRequest) (*sdk.GetPromptResult, error) {
	mood := strings.TrimSpace(req.Params.Arguments["mood"])
	if mood == "" {
		return nil, fmt.Errorf("extract-from-mood: 'mood' argument is required")
	}
	count := strings.TrimSpace(req.Params.Arguments["n"])
	if count == "" {
		count = "5"
	}

	body := strings.Join([]string{
		fmt.Sprintf("Build a color palette of about %s colors that fits this mood:", count),
		"",
		fmt.Sprintf("    %s", mood),
		"",
		"Reasoning steps:",
		"1. Choose a base color (any CSS color string — hex, named, rgb(), hsl(), oklch()) that anchors the mood.",
		"2. Choose a harmony type from {complementary, analogous, triadic, split-complementary, tetradic, square, double-complementary, compound, monochromatic, shades} that best matches the mood.",
		fmt.Sprintf("3. Call the harmony.generate tool with arguments {type, base, count: %s}. Return the colors from result.palette.colors as the answer.", count),
		"4. Briefly justify the base color and harmony type you picked. Do not invent colors outside the harmony.generate output.",
	}, "\n")

	return &sdk.GetPromptResult{
		Description: fmt.Sprintf("Palette generation for mood %q.", mood),
		Messages: []*sdk.PromptMessage{
			{
				Role:    "user",
				Content: &sdk.TextContent{Text: body},
			},
		},
	}, nil
}

// registerAuditContrastPrompt wires the audit-contrast prompt. The
// handler does not parse `pairs` itself — the LLM is told the format and
// asked to dispatch contrast.check calls. This keeps the prompt tool-
// agnostic to syntax tweaks in contrast.check.
func registerAuditContrastPrompt(srv *sdk.Server) {
	srv.AddPrompt(&sdk.Prompt{
		Name:        "audit-contrast",
		Title:       "Audit a list of fg/bg pairs for WCAG contrast",
		Description: "Run a list of foreground/background color pairs through contrast.check and summarise AA/AAA pass/fail status.",
		Arguments: []*sdk.PromptArgument{
			{Name: "pairs", Description: "Comma-separated fg:bg pairs (e.g. '#fff:#222, royalblue:#fafafa'). Each side accepts any color string contrast.check accepts.", Required: true},
		},
	}, auditContrastHandler)
}

func auditContrastHandler(_ context.Context, req *sdk.GetPromptRequest) (*sdk.GetPromptResult, error) {
	pairs := strings.TrimSpace(req.Params.Arguments["pairs"])
	if pairs == "" {
		return nil, fmt.Errorf("audit-contrast: 'pairs' argument is required")
	}

	body := strings.Join([]string{
		"Audit the following foreground/background color pairs for WCAG 2.1 contrast.",
		"",
		fmt.Sprintf("Pairs (comma-separated, each is fg:bg): %s", pairs),
		"",
		"Reasoning steps:",
		"1. Split the input on commas, then on the first ':' inside each entry. Trim whitespace.",
		"2. For each pair, call the contrast.check tool with arguments {fg, bg, algo: \"wcag21\"}. Each result carries result.wcag21 with fields {ratio, aa, aa_large, aaa, aaa_large}.",
		"3. Collect the results into a single table: pair, ratio, aa, aa_large, aaa, aaa_large.",
		"4. Summarise: how many pairs pass aa, how many pass aaa, how many pass both. Flag any pair where aa_large is false — those fail the lowest bar and are the urgent fixes.",
		"5. For failing pairs, suggest a darker/lighter variant of the foreground that would pass aa. Do not invent contrast numbers — call contrast.check again on each suggestion to verify.",
	}, "\n")

	return &sdk.GetPromptResult{
		Description: "WCAG contrast audit for the supplied fg/bg pairs.",
		Messages: []*sdk.PromptMessage{
			{
				Role:    "user",
				Content: &sdk.TextContent{Text: body},
			},
		},
	}, nil
}
