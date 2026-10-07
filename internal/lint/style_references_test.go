package lint

import (
	"strings"
	"testing"

	"assx/internal/ass"
)

func TestDialogueStyleReferencesFollowRendererResolution(t *testing.T) {
	styles := "Style: Default,Arial,20\nStyle: Main,Arial,20\n"
	for _, name := range []string{"Default", "default", "DEFAULT", "*Default", "*default", "Main", "*Main", "**Main"} {
		t.Run("valid/"+name, func(t *testing.T) {
			if got := undefinedStyleFindings(styleReferenceDocument(styles, name, "")); len(got) != 0 {
				t.Fatalf("style %q produced diagnostics: %#v", name, got)
			}
		})
	}
	for _, name := range []string{"main", "MAIN", "*main"} {
		t.Run("undefined/"+name, func(t *testing.T) {
			diagnostics := undefinedStyleFindings(styleReferenceDocument(styles, name, ""))
			if len(diagnostics) != 1 {
				t.Fatalf("style %q diagnostics = %#v", name, diagnostics)
			}
			diagnostic := diagnostics[0]
			if diagnostic.ID != IssueUndefinedStyle || diagnostic.Severity != Warning || diagnostic.Field != "Style" || diagnostic.FixSafety != "" || len(diagnostic.Edits) != 0 {
				t.Fatalf("undefined dialogue style diagnostic = %#v", diagnostic)
			}
		})
	}
}

func TestDialogueStyleNamesRemainCaseSensitiveExceptDefault(t *testing.T) {
	styles := "Style: Default,Arial,20\nStyle: Main,Arial,20\nStyle: main,Arial,20\n"
	for _, name := range []string{"Main", "main", "default", "DEFAULT", "*main"} {
		if got := undefinedStyleFindings(styleReferenceDocument(styles, name, "")); len(got) != 0 {
			t.Fatalf("defined style %q produced diagnostics: %#v", name, got)
		}
	}
	for _, name := range []string{"MAIN", "*MAIN"} {
		if got := undefinedStyleFindings(styleReferenceDocument(styles, name, "")); len(got) != 1 {
			t.Fatalf("undefined style %q diagnostics = %#v", name, got)
		}
	}
}

func TestResetStyleReferencesUseExactLookupAndIgnoreBareReset(t *testing.T) {
	styles := "Style: Default,Arial,20\nStyle: Main,Arial,20\n"
	for _, reset := range []string{"r", "rDefault", "rMain"} {
		doc := styleReferenceDocument(styles, "Default", "{\\"+reset+"}text")
		if got := undefinedStyleFindings(doc); len(got) != 0 {
			t.Fatalf("reset %q produced diagnostics: %#v", reset, got)
		}
	}
	for _, reset := range []string{"rdefault", "rmain", "rMAIN"} {
		doc := styleReferenceDocument(styles, "Default", "{\\"+reset+"}text")
		diagnostics := undefinedStyleFindings(doc)
		if len(diagnostics) != 1 || diagnostics[0].ID != IssueUndefinedStyle || diagnostics[0].Severity != Warning || diagnostics[0].Tag != "r" || diagnostics[0].FixSafety != "" || len(diagnostics[0].Edits) != 0 {
			t.Fatalf("reset %q diagnostics = %#v", reset, diagnostics)
		}
	}
}

func TestDialogueAndResetReferencesShareUndefinedStyleRule(t *testing.T) {
	doc := styleReferenceDocument("Style: Default,Arial,20\nStyle: Main,Arial,20\n", "Typo", `text{\rMissing}`)
	diagnostics := undefinedStyleFindings(doc)
	if len(diagnostics) != 2 {
		t.Fatalf("undefined references = %#v", diagnostics)
	}
	for _, diagnostic := range diagnostics {
		if diagnostic.ID != "ASS017" || diagnostic.Severity != Warning {
			t.Errorf("diagnostic = %#v", diagnostic)
		}
	}
}

func TestUndefinedDialogueStylePointsToStyleField(t *testing.T) {
	line := "Dialogue: 0,0,1,  Missing  ,text"
	doc := ass.Parse("[V4+ Styles]\nFormat: Name, Fontname, Fontsize\nStyle: Default,Arial,20\n" +
		"[Events]\nFormat: Layer, Start, End, Style, Text\n" + line + "\n")
	diagnostics := undefinedStyleFindings(doc)
	if len(diagnostics) != 1 {
		t.Fatalf("diagnostics = %#v", diagnostics)
	}
	wantColumn := strings.Index(line, "Missing") + 1
	if diagnostics[0].Line != 6 || diagnostics[0].Column != wantColumn {
		t.Fatalf("location = %d:%d, want 6:%d", diagnostics[0].Line, diagnostics[0].Column, wantColumn)
	}
}

func styleReferenceDocument(styles, dialogueStyle, text string) ass.Document {
	return ass.Parse("[V4+ Styles]\nFormat: Name, Fontname, Fontsize\n" + styles +
		"[Events]\nFormat: Layer, Start, End, Style, Text\n" +
		"Dialogue: 0,0,1," + dialogueStyle + "," + text + "\n")
}

func undefinedStyleFindings(doc ass.Document) []Diagnostic {
	var findings []Diagnostic
	for _, diagnostic := range AnalyzeDocument(doc) {
		if diagnostic.ID == IssueUndefinedStyle {
			findings = append(findings, diagnostic)
		}
	}
	return findings
}
