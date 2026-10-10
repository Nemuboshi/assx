package spec

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// The pin is a source revision, NOT a claim that any particular binary or
// optional feature was built. See docs/renderer-contracts.md for build scopes.
var evidencePins = map[string]struct {
	commit string
	path   string
}{
	"libass":      {"f61db567e6593df3470e91594bcd4ad2d0473aff", "ref/renderers/libass"},
	"xy-VSFilter": {"135a30153a38fa846cb5c39df0f258403e92096e", "ref/renderers/xy-VSFilter"},
	"VSFilterMod": {"7a00567e4a49b6310691b9a6791646b2a018bfa2", "ref/renderers/VSFilterMod"},
}

func TestMatrixRendererSourcePins(t *testing.T) {
	matrix := loadMatrix(t)
	if len(matrix.Meta.Pins) != len(evidencePins) {
		t.Fatalf("expected %d unique renderer pins, got %d", len(evidencePins), len(matrix.Meta.Pins))
	}
	seen := make(map[string]bool)
	for _, pin := range matrix.Meta.Pins {
		expected, ok := evidencePins[pin.Renderer]
		if !ok || seen[pin.Renderer] {
			t.Errorf("unexpected or duplicate renderer pin %q", pin.Renderer)
			continue
		}
		seen[pin.Renderer] = true
		if pin.Commit != expected.commit || pin.Path != expected.path {
			t.Errorf("%s pin drift: commit=%q path=%q (want %q, %q)",
				pin.Renderer, pin.Commit, pin.Path, expected.commit, expected.path)
		}
		if !regexp.MustCompile(`^[a-f0-9]{40}$`).MatchString(pin.Commit) {
			t.Errorf("%s must use a full 40-character commit SHA", pin.Renderer)
		}
		// Source mirrors are optional and gitignored. When available, ensure
		// their checked-out HEAD matches the matrix, not just its prose.
		local := filepath.Join("..", "..", "..", filepath.FromSlash(pin.Path))
		if _, err := os.Stat(filepath.Join(local, ".git")); err == nil {
			output, err := exec.Command("git", "-C", local, "rev-parse", "HEAD").CombinedOutput()
			if err != nil {
				t.Errorf("%s local checkout: %v: %s", pin.Renderer, err, output)
			} else if got := strings.TrimSpace(string(output)); got != expected.commit {
				t.Errorf("%s local checkout at %s, want pinned %s", pin.Renderer, got, expected.commit)
			}
		}
	}
	// Existing rendercheck must not silently move away from the evidence pin.
	workflow, err := os.ReadFile(filepath.Join("..", "..", "..", ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(workflow), "LIBASS_COMMIT: "+evidencePins["libass"].commit) {
		t.Error("rendercheck libass build pin differs from the evidence matrix")
	}
}

// Every V row must explicitly state which of its three renderer assertions
// was actually read in pinned source. Other cells are source-inferred (or
// unchecked): neither a V row by itself nor a citation for a different
// renderer is sufficient proof for SafeFix.
func TestMatrixVerifiedCellScope(t *testing.T) {
	m := loadMatrix(t)
	for _, tag := range m.Tags {
		if tag.Params != nil {
			checkVerifiedScope(t, "tag "+tag.Name+" params", tag.Params.Status,
				tag.Params.Verified, tag.Params.Cite, nil)
		}
		for _, scen := range tag.Scens {
			results := map[string]string{
				"libass": scen.Base, "xy": scen.VSFilter, "vsm": scen.VSFilterMod,
			}
			checkVerifiedScope(t, "tag "+tag.Name+" scen "+scen.ID, scen.Status,
				scen.Verified, scen.Cite, results)
		}
	}
}

func checkVerifiedScope(t *testing.T, location, status, scope, cite string, results map[string]string) {
	t.Helper()
	allowed := map[string]bool{"libass": true, "xy": true, "vsm": true}
	cited := make(map[string]bool)
	for _, token := range strings.Fields(cite) {
		name, _, _ := strings.Cut(token, ":")
		cited[name] = true
	}
	var expected []string
	for _, name := range []string{"libass", "xy", "vsm"} {
		if !cited[name] || status != "V" {
			continue
		}
		if results != nil && (results[name] == "" || results[name] == "unchecked") {
			continue
		}
		expected = append(expected, name)
	}
	got := strings.Fields(scope)
	if len(got) != len(slices.Compact(slices.Clone(got))) {
		t.Errorf("%s: duplicate verified renderer", location)
	}
	for _, name := range got {
		if !allowed[name] {
			t.Errorf("%s: unknown verified renderer %q", location, name)
		}
	}
	if !slices.Equal(got, expected) {
		t.Errorf("%s: verified=%q; evidence permits only %q", location, scope, strings.Join(expected, " "))
	}
	if status == "V" && len(got) == 0 {
		t.Errorf("%s: V without any renderer-backed cell must be marked S", location)
	}
}

// A local pinned source checkout is optional in normal go test / CI; when
// available, confirm the build flags that gate VSFilterMod extensions.
func TestVSFilterModOptionalBuildCapabilities(t *testing.T) {
	project := filepath.Join("..", "..", "..", "ref", "renderers", "VSFilterMod")
	projectFile := filepath.Join(project, "src", "subtitles", "subtitles.vcxproj")
	data, err := os.ReadFile(projectFile)
	if os.IsNotExist(err) {
		t.Skip("local VSFilterMod source checkout is absent (pin is validated separately)")
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, macro := range []string{"_VSMOD", "_LUA"} {
		if !strings.Contains(string(data), macro) {
			t.Errorf("pinned VSFilterMod subtitles.vcxproj no longer declares optional macro %s", macro)
		}
	}
	source, err := os.ReadFile(filepath.Join(project, "src", "subtitles", "RTS.cpp"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(source), "#ifdef _VSMOD") ||
		!strings.Contains(string(source), "defined(_LUA)") {
		t.Error("pinned RTS.cpp does not show expected conditional VSFilterMod/Lua handlers")
	}
}
