package spec

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// Signature applicability is explicit and independent of a parent <params>
// row. The latter is only a source overview, never evidence for an individual
// arity. "inferred" means reported but not source-verified for that signature.
var rendererOrder = []string{"libass", "xy", "vsm"}

func rendererScope(raw string, required bool) ([]string, error) {
	if raw == "" && !required {
		return nil, nil
	}
	names := strings.Fields(raw)
	if len(names) == 0 {
		return nil, fmt.Errorf("renderer scope is required")
	}
	var canonical []string
	for _, name := range rendererOrder {
		if slices.Contains(names, name) {
			canonical = append(canonical, name)
		}
	}
	if len(names) != len(canonical) || strings.Join(canonical, " ") != raw {
		return nil, fmt.Errorf("renderer scope %q must be unique, explicit and ordered as libass xy vsm", raw)
	}
	return names, nil
}

func validateSignatureEvidence(sig xmlSig, parentVerified string) error {
	applies, err := rendererScope(sig.Renderer, true)
	if err != nil {
		return fmt.Errorf("applicability: %w", err)
	}
	verified, err := rendererScope(sig.Verified, false)
	if err != nil {
		return fmt.Errorf("verified: %w", err)
	}
	inferred, err := rendererScope(sig.Inferred, false)
	if err != nil {
		return fmt.Errorf("inferred: %w", err)
	}
	parent, err := rendererScope(parentVerified, false)
	if err != nil {
		return fmt.Errorf("parent scope: %w", err)
	}
	var combined []string
	for _, renderer := range rendererOrder {
		if slices.Contains(verified, renderer) && slices.Contains(inferred, renderer) {
			return fmt.Errorf("renderer %s is both verified and inferred", renderer)
		}
		if slices.Contains(verified, renderer) || slices.Contains(inferred, renderer) {
			combined = append(combined, renderer)
		}
	}
	if !slices.Equal(applies, combined) {
		return fmt.Errorf("applicability %q must equal verified %q plus inferred %q",
			sig.Renderer, sig.Verified, sig.Inferred)
	}
	switch sig.Requires {
	case "":
	case "_VSMOD", "_VSMOD _LUA":
		if !slices.Equal(applies, []string{"vsm"}) {
			return fmt.Errorf("build requirements %q must apply only to VSFilterMod", sig.Requires)
		}
	default:
		return fmt.Errorf("unsupported signature build requirements %q", sig.Requires)
	}
	if sig.Status != "V" && sig.Status != "S" {
		return fmt.Errorf("signature status must be V or S, got %q", sig.Status)
	}
	if sig.Status == "V" && len(verified) == 0 {
		return fmt.Errorf("verified signature requires at least one verified renderer")
	}
	if sig.Status == "S" && len(verified) > 0 {
		return fmt.Errorf("unverified signature cannot declare verified renderers")
	}
	cited := make(map[string]bool)
	if sig.Cite == "" {
		return fmt.Errorf("signature requires its own evidence citation")
	}
	for _, token := range strings.Fields(sig.Cite) {
		if !citeToken.MatchString(token) {
			return fmt.Errorf("invalid signature citation %q", token)
		}
		name, _, _ := strings.Cut(token, ":")
		if slices.Contains(rendererOrder, name) {
			cited[name] = true
		}
	}
	for _, renderer := range rendererOrder {
		if slices.Contains(verified, renderer) {
			if !slices.Contains(parent, renderer) {
				return fmt.Errorf("signature claims verified %s absent from parent source evidence", renderer)
			}
			if !cited[renderer] {
				return fmt.Errorf("signature claims verified %s without per-signature citation", renderer)
			}
		} else if sig.Status == "V" && cited[renderer] {
			return fmt.Errorf("signature citation claims %s outside its verified scope", renderer)
		}
		if slices.Contains(inferred, renderer) && slices.Contains(parent, renderer) {
			// The group already cites pinned renderer source. Calling an
			// additional arity "inferred" could mask an unsupported form.
			return fmt.Errorf("cannot infer %s applicability when the parent has verified source coverage", renderer)
		}
	}
	return nil
}

// An actual XML mutation test guards the original review finding. Removing a
// VSFilterMod-only restriction must never fall back to "all renderers".
func TestMatrixSignatureApplicabilityMutations(t *testing.T) {
	path := filepath.Join("..", "..", "..", "docs", "ass-tags.xml")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	needle := `<sig n="3" form="paren" renderer="vsm"`
	if strings.Count(source, needle) != 1 {
		t.Fatalf("expected exactly one pinned VSFilterMod-only three-coordinate position signature")
	}
	for _, mutation := range []struct {
		name string
		text string
	}{
		{"implicit-all", `<sig n="3" form="paren"`},
		{"broaden-without-evidence", `<sig n="3" form="paren" renderer="libass xy vsm"`},
		{"broaden-by-inference", `<sig n="3" form="paren" renderer="libass xy vsm" inferred="libass xy"`},
	} {
		t.Run(mutation.name, func(t *testing.T) {
			modified := strings.Replace(source, needle, mutation.text, 1)
			var matrix xmlMatrix
			if err := xml.Unmarshal([]byte(modified), &matrix); err != nil {
				t.Fatal(err)
			}
			for _, tag := range matrix.Tags {
				if tag.Name != "pos" || tag.Params == nil {
					continue
				}
				for _, sig := range tag.Params.Sigs {
					if sig.N != "3" {
						continue
					}
					if err := validateSignatureEvidence(sig, tag.Params.Verified); err == nil {
						t.Fatal("invalid three-coordinate position signature passed evidence validation")
					}
					return
				}
			}
			t.Fatal("three-coordinate position signature missing from mutated matrix")
		})
	}
}

func TestMatrixFeatureRequirementsMatchPinnedSource(t *testing.T) {
	// The source-verified condition belongs to each signature, not the name.
	// Catch removal of a guard even if the generic XML schema remains valid.
	want := map[string]string{
		"pos|3|paren|vsm":   "_VSMOD",
		"fsc|1|both|vsm":    "_VSMOD",
		"blend|0|bare|vsm":  "_VSMOD",
		"blend|1|paren|vsm": "_VSMOD",
		"frs|0|bare|vsm":    "_VSMOD",
		"frs|1|both|vsm":    "_VSMOD",
		"fsvp|0|bare|vsm":   "_VSMOD",
		"fsvp|1|both|vsm":   "_VSMOD",
	}
	matrix := loadMatrix(t)
	for _, tag := range matrix.Tags {
		if tag.Params == nil {
			continue
		}
		for _, sig := range tag.Params.Sigs {
			key := tag.Name + "|" + sig.N + "|" + sig.Form + "|" + sig.Renderer
			if sig.Requires != want[key] {
				t.Errorf("%s requires=%q, pinned requirement=%q", key, sig.Requires, want[key])
			}
			delete(want, key)
		}
	}
	for key := range want {
		t.Errorf("missing pinned signature %s", key)
	}
}

func TestMatrixRejectsInvalidBuildRequirementScope(t *testing.T) {
	matrix := loadMatrix(t)
	for _, tag := range matrix.Tags {
		if tag.Name != "pos" || tag.Params == nil {
			continue
		}
		for _, sig := range tag.Params.Sigs {
			if sig.N != "3" {
				continue
			}
			for _, raw := range []string{"_LUA", "_LUA _VSMOD", "_VSMOD _VSMOD", "_BAD"} {
				mutated := sig
				mutated.Requires = raw
				if err := validateSignatureEvidence(mutated, tag.Params.Verified); err == nil {
					t.Errorf("invalid guard %q accepted", raw)
				}
			}
			mutated := sig
			mutated.Renderer = "libass xy vsm"
			mutated.Verified = "libass xy vsm"
			mutated.Cite = "libass:libass/ass_parse.c:606-621 xy:src/subtitles/RTS.cpp:2615-2627 vsm:src/subtitles/RTS.cpp:3510-3543"
			if err := validateSignatureEvidence(mutated, tag.Params.Verified); err == nil {
				t.Fatal("VSFilterMod-only build requirement accepted for shared signature")
			}
			return
		}
	}
	t.Fatal("missing pinned three-coordinate pos signature")
}

// validateExhaustiveEvidence enforces that a rejection claim is independently
// cited and every applicable arity is source-verified for that renderer.
// Recording all *known* arities is insufficient to establish completeness.
func validateExhaustiveEvidence(params *xmlParams) error {
	if params.Exhaustive == "" {
		if params.ExhaustiveCite != "" {
			return fmt.Errorf("exhaustive-cite without exhaustive renderer scope")
		}
		return nil
	}
	scopes, err := rendererScope(params.Exhaustive, true)
	if err != nil {
		return err
	}
	verified, err := rendererScope(params.Verified, false)
	if err != nil {
		return err
	}
	cited := map[string]bool{}
	for _, token := range strings.Fields(params.ExhaustiveCite) {
		if !citeToken.MatchString(token) {
			return fmt.Errorf("invalid exhaustive citation %q", token)
		}
		name, _, _ := strings.Cut(token, ":")
		cited[name] = true
	}
	for _, scope := range scopes {
		if !slices.Contains(verified, scope) {
			return fmt.Errorf("exhaustive renderer %s lacks parent source verification", scope)
		}
		if !cited[scope] {
			return fmt.Errorf("exhaustive renderer %s lacks independent source citation", scope)
		}
		hasSignature := false
		for _, sig := range params.Sigs {
			if !slices.Contains(strings.Fields(sig.Renderer), scope) {
				continue
			}
			hasSignature = true
			if !slices.Contains(strings.Fields(sig.Verified), scope) {
				return fmt.Errorf("exhaustive %s scope includes inferred signature n=%s form=%s",
					scope, sig.N, sig.Form)
			}
		}
		if !hasSignature {
			return fmt.Errorf("exhaustive renderer %s has no applicable signatures", scope)
		}
	}
	for name := range cited {
		if slices.Contains(rendererOrder, name) && !slices.Contains(scopes, name) {
			return fmt.Errorf("source citation for %s outside exhaustive scope", name)
		}
	}
	return nil
}

func TestPinnedExhaustiveSignatureClaims(t *testing.T) {
	matrix := loadMatrix(t)
	for _, tag := range matrix.Tags {
		if tag.Params == nil {
			continue
		}
		if err := validateExhaustiveEvidence(tag.Params); err != nil {
			t.Errorf("%s: %v", tag.Name, err)
		}
		switch tag.Name {
		case "pos":
			if tag.Params.Exhaustive != "libass xy vsm" {
				t.Fatalf("pos must retain three-renderer pinned arity completeness: %q", tag.Params.Exhaustive)
			}
			if tag.Params.ExhaustiveCite == "" {
				t.Fatal("position arity completeness lost its source citation")
			}
			for _, mutation := range []struct {
				name  string
				apply func(*xmlParams)
			}{
				{"missing-citation", func(p *xmlParams) { p.ExhaustiveCite = "" }},
				{"unverified-renderer", func(p *xmlParams) { p.Verified = "libass xy" }},
				{"unverified-form", func(p *xmlParams) { p.Sigs[0].Verified = "libass xy" }},
				{"scope-without-signature", func(p *xmlParams) { p.Sigs = nil }},
			} {
				t.Run(mutation.name, func(t *testing.T) {
					p := *tag.Params
					p.Sigs = append([]xmlSig(nil), p.Sigs...)
					mutation.apply(&p)
					if err := validateExhaustiveEvidence(&p); err == nil {
						t.Fatal("invalid completeness evidence was accepted")
					}
				})
			}
		default:
			if tag.Params.Exhaustive != "" {
				t.Errorf("%s claims exhaustive arities without independent pinned audit", tag.Name)
			}
		}
	}
}
