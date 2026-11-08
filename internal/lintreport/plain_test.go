package lintreport

import (
	"testing"

	"assx/internal/lint"
)

func TestPlainPreservesDiagnosticCoordinatesAndText(t *testing.T) {
	views := Plain([]lint.Diagnostic{{Condition: "value", Severity: lint.Warning, Line: 12, Column: 4, Title: "Invalid value", Detail: "Value is invalid.", Tag: "fs", Renderers: []lint.Observation{{Renderer: "libass", Role: lint.RolePrimary, Status: lint.StatusRejected}}}})
	if len(views) != 1 {
		t.Fatalf("views = %#v", views)
	}
	v := views[0]
	if v.Condition != "value" || v.Severity != "warning" || v.Line != 12 || v.Column != 4 || v.Tag != "fs" || v.Detail != "Value is invalid." || v.Renderer != "libass" {
		t.Fatalf("plain view changed: %#v", v)
	}
}
