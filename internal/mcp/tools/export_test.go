package tools

import (
	"context"
	"strings"
	"testing"
)

func TestExportCSSVars(t *testing.T) {
	_, out, err := handleExportCSS(context.Background(), nil, ExportCSSParams{
		Colors: []string{"#3366cc", "#ff9900"},
		Name:   "brand",
	})
	if err != nil {
		t.Fatalf("handleExportCSS: %v", err)
	}
	if out.Result.Format != "css" {
		t.Errorf("format = %q, want css", out.Result.Format)
	}
	if out.Result.Kind != "vars" {
		t.Errorf("kind = %q, want vars (default)", out.Result.Kind)
	}
	if !strings.Contains(out.Result.Content, "--brand-1") {
		t.Errorf("content missing --brand-1:\n%s", out.Result.Content)
	}
	if !strings.Contains(out.Result.Content, "#3366cc") {
		t.Errorf("content missing color hex:\n%s", out.Result.Content)
	}
	if out.Result.Filename != "brand.css" {
		t.Errorf("filename = %q, want brand.css", out.Result.Filename)
	}
}

func TestExportCSSSCSS(t *testing.T) {
	_, out, err := handleExportCSS(context.Background(), nil, ExportCSSParams{
		Colors: []string{"red"},
		Kind:   "scss",
	})
	if err != nil {
		t.Fatalf("handleExportCSS: %v", err)
	}
	if out.Result.Format != "scss" {
		t.Errorf("format = %q, want scss", out.Result.Format)
	}
	if !strings.HasPrefix(strings.TrimSpace(out.Result.Content), "$") {
		t.Errorf("scss output should start with $, got:\n%s", out.Result.Content)
	}
}

func TestExportCSSLESS(t *testing.T) {
	_, out, err := handleExportCSS(context.Background(), nil, ExportCSSParams{
		Colors: []string{"red"},
		Kind:   "less",
	})
	if err != nil {
		t.Fatalf("handleExportCSS: %v", err)
	}
	if out.Result.Format != "less" {
		t.Errorf("format = %q, want less", out.Result.Format)
	}
	if !strings.HasPrefix(strings.TrimSpace(out.Result.Content), "@") {
		t.Errorf("less output should start with @, got:\n%s", out.Result.Content)
	}
}

func TestExportCSSUnknownKind(t *testing.T) {
	_, _, err := handleExportCSS(context.Background(), nil, ExportCSSParams{
		Colors: []string{"red"},
		Kind:   "fictional",
	})
	if err == nil {
		t.Errorf("expected error for unknown kind")
	}
}

func TestExportCSSEmptyInput(t *testing.T) {
	_, _, err := handleExportCSS(context.Background(), nil, ExportCSSParams{})
	if err == nil {
		t.Errorf("expected error for empty colors")
	}
}

func TestExportTailwindFlat(t *testing.T) {
	_, out, err := handleExportTailwind(context.Background(), nil, ExportTailwindParams{
		Colors: []string{"#3366cc", "#ff9900"},
		Name:   "brand",
	})
	if err != nil {
		t.Fatalf("handleExportTailwind: %v", err)
	}
	if out.Result.Format != "tailwind" {
		t.Errorf("format = %q, want tailwind", out.Result.Format)
	}
	if !strings.Contains(out.Result.Content, "brand-1") {
		t.Errorf("content missing brand-1:\n%s", out.Result.Content)
	}
	if !strings.Contains(out.Result.Content, "#3366cc") {
		t.Errorf("content missing color:\n%s", out.Result.Content)
	}
	if !strings.HasSuffix(out.Result.Filename, ".js") {
		t.Errorf("filename = %q, want *.js", out.Result.Filename)
	}
}

func TestExportTailwindShades5(t *testing.T) {
	_, out, err := handleExportTailwind(context.Background(), nil, ExportTailwindParams{
		Colors: []string{"#3366cc"},
		Name:   "brand",
		Shades: 5,
	})
	if err != nil {
		t.Fatalf("handleExportTailwind: %v", err)
	}
	for _, want := range []string{`"brand-1":`, `"100":`, `"300":`, `"500":`, `"700":`, `"900":`} {
		if !strings.Contains(out.Result.Content, want) {
			t.Errorf("missing %q in shaded output:\n%s", want, out.Result.Content)
		}
	}
	for _, unwanted := range []string{`"50":`, `"200":`, `"400":`} {
		if strings.Contains(out.Result.Content, unwanted) {
			t.Errorf("unexpected %q in 5-shade output:\n%s", unwanted, out.Result.Content)
		}
	}
}

func TestExportTailwindShades10(t *testing.T) {
	_, out, err := handleExportTailwind(context.Background(), nil, ExportTailwindParams{
		Colors: []string{"#3366cc"},
		Shades: 10,
	})
	if err != nil {
		t.Fatalf("handleExportTailwind: %v", err)
	}
	for _, want := range []string{`"50":`, `"100":`, `"500":`, `"900":`} {
		if !strings.Contains(out.Result.Content, want) {
			t.Errorf("missing %q in 10-shade output:\n%s", want, out.Result.Content)
		}
	}
}

func TestExportTailwindShadesNegative(t *testing.T) {
	_, _, err := handleExportTailwind(context.Background(), nil, ExportTailwindParams{
		Colors: []string{"red"},
		Shades: -1,
	})
	if err == nil {
		t.Errorf("expected error for negative shades")
	}
}

func TestExportTailwindEmptyInput(t *testing.T) {
	_, _, err := handleExportTailwind(context.Background(), nil, ExportTailwindParams{})
	if err == nil {
		t.Errorf("expected error for empty colors")
	}
}

func TestExportTailwindParseError(t *testing.T) {
	_, _, err := handleExportTailwind(context.Background(), nil, ExportTailwindParams{
		Colors: []string{"not-a-color"},
	})
	if err == nil {
		t.Errorf("expected parse error")
	}
}
