package lintreport

import (
	"testing"

	"assx/internal/lint"
)

func TestPlainPreservesDiagnosticCoordinatesAndText(t *testing.T) {
	views := Plain([]lint.Diagnostic{{ID: "ASS002", Severity: lint.Warning, Line: 12, Column: 4, Title: "Invalid value", Detail: "Value is invalid.", Tag: "fs", Renderer: "libass@test"}})
	if len(views) != 1 {
		t.Fatalf("views = %#v", views)
	}
	v := views[0]
	if v.ID != "ASS002" || v.Severity != "warning" || v.Line != 12 || v.Column != 4 || v.Tag != "fs" || v.Detail != "Value is invalid." || v.Renderer != "libass@test" {
		t.Fatalf("plain view changed: %#v", v)
	}
}
