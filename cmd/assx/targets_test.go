package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"assx/internal/lint"
)

func TestRendererTargetCLI(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want []string
	}{
		{"default", nil, []string{"libass", "xy-VSFilter"}},
		{"explicit primary", []string{"--renderer=libass"}, []string{"libass"}},
		{"single dash primary", []string{"-renderer", "libass"}, []string{"libass"}},
		{"single dash empty compat", []string{"-compat="}, []string{"libass"}},
		{"single dash Mod", []string{"-renderer=vsfiltermod", "-mod-build", "enabled", "-lua-build=disabled"}, []string{"VSFilterMod"}},
		{"explicit empty compat", []string{"--compat="}, []string{"libass"}},
		{"xy primary", []string{"--renderer", "xy-vsfilter"}, []string{"xy-VSFilter"}},
		{"normalized union", []string{"--renderer=xy-vsfilter", "--compat=xy-vsfilter,libass,libass"}, []string{"libass", "xy-VSFilter"}},
		{"Mod unknown", []string{"--renderer=vsfiltermod"}, nil},
		{"Mod known", []string{"--renderer=vsfiltermod", "--mod-build=enabled", "--lua-build=disabled"}, []string{"VSFilterMod"}},
		{"Mod compatibility unknown", []string{"--compat=vsfiltermod"}, nil},
		{"Mod Lua scripted", []string{"--renderer=vsfiltermod", "--mod-build=enabled", "--lua-build=enabled"}, nil},
		{"Lua without Mod extensions", []string{"--renderer=vsfiltermod", "--mod-build=disabled", "--lua-build=enabled"}, nil},
		{"Mod compatibility known", []string{"--compat=vsfiltermod", "--mod-build=enabled", "--lua-build=disabled"}, []string{"libass", "VSFilterMod"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := writeRendererFixture(t, `{\fs10\fs21}x`)
			var out, errOut strings.Builder
			args := append([]string{"--format=json"}, tc.args...)
			if code := run(append(args, path), &out, &errOut); code != 0 {
				t.Fatalf("exit %d: %s", code, errOut.String())
			}
			var diagnostics []lint.Diagnostic
			if err := json.Unmarshal([]byte(out.String()), &diagnostics); err != nil {
				t.Fatal(err)
			}
			var proof *lint.FixProof
			for _, d := range diagnostics {
				if d.FixProof != nil {
					proof = d.FixProof
				}
				if d.Renderer == "" {
					t.Errorf("missing scope for %s", d.ID)
				}
			}
			if tc.want == nil {
				if proof != nil {
					t.Fatal("uncertain Mod or Lua scope authorized a fix")
				}
				before, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				out.Reset()
				errOut.Reset()
				if code := run(append(append([]string{"--format=json", "--fix"}, tc.args...), path), &out, &errOut); code != 0 {
					t.Fatalf("uncertain fix exit %d: %s", code, errOut.String())
				}
				after, err := os.ReadFile(path)
				if err != nil || string(after) != string(before) {
					t.Fatal("uncertain requested target changed file bytes")
				}
				return
			}
			if proof == nil {
				t.Fatalf("missing proof: %s", errOut.String())
			}
			if len(proof.Targets) != len(tc.want) {
				t.Fatalf("targets = %v", proof.Targets)
			}
			for i, name := range tc.want {
				if proof.Targets[i].Renderer != name {
					t.Fatalf("target %d = %s", i, proof.Targets[i].Renderer)
				}
			}
			if tc.name == "Mod known" && (proof.Targets[0].Capabilities.VSFilterMod != "enabled" || proof.Targets[0].Capabilities.Lua != "disabled") {
				t.Fatalf("capabilities = %v", proof.Targets[0].Capabilities)
			}
			out.Reset()
			errOut.Reset()
			if code := run(append(append([]string{"--format=json", "--fix"}, tc.args...), path), &out, &errOut); code != 0 {
				t.Fatalf("fix exit %d: %s", code, errOut.String())
			}
			fixed, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(fixed), `{\fs21}x`) || strings.Contains(string(fixed), `\fs10`) {
				t.Fatal("requested-target fix was not applied")
			}
		})
	}
}

func TestRendererCLIUsageErrors(t *testing.T) {
	for _, args := range [][]string{
		{"--renderer=other"}, {"--compat=libass,,xy-vsfilter"}, {"--compat=other"},
		{"--mod-build=enabled"}, {"--lua-build=unknown"},
		{"--renderer=vsfiltermod", "--mod-build=bad"}, {"--renderer=vsfiltermod", "--lua-build=bad"},
		{"-renderer", "invalid"}, {"-compat=invalid"}, {"-mod-build=enabled"}, {"-lua-build=disabled"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var out, errOut strings.Builder
			if code := run(append(args, "missing.ass"), &out, &errOut); code != 2 || out.Len() != 0 || errOut.Len() == 0 {
				t.Fatalf("exit=%d stdout=%q stderr=%q", code, out.String(), errOut.String())
			}
		})
	}
	var out, errOut strings.Builder
	if code := run([]string{"--help"}, &out, &errOut); code != 0 {
		t.Fatalf("help exit %d", code)
	}
	for _, flag := range []string{"--renderer", "--compat", "--mod-build", "--lua-build"} {
		if !strings.Contains(errOut.String(), flag) {
			t.Errorf("help omits %s", flag)
		}
	}
}

func TestPrimaryValidityAndCompatibilityScopes(t *testing.T) {
	path := writeRendererFixture(t, `{\pos(1,2,3)}x`)
	for _, tc := range []struct {
		args      []string
		wantError bool
	}{
		{nil, true}, {[]string{"--renderer=libass", "--compat=vsfiltermod"}, true},
		{[]string{"--renderer=vsfiltermod", "--compat=libass", "--mod-build=enabled", "--lua-build=disabled"}, false},
	} {
		var out, errOut strings.Builder
		code := run(append(append([]string{"--format=json"}, tc.args...), path), &out, &errOut)
		if (code == 1) != tc.wantError || code == 2 {
			t.Fatalf("args=%v exit=%d stderr=%s", tc.args, code, errOut.String())
		}
		var ds []lint.Diagnostic
		if err := json.Unmarshal([]byte(out.String()), &ds); err != nil {
			t.Fatal(err)
		}
		for _, d := range ds {
			if d.Severity == lint.Error && strings.Contains(d.Renderer, " / ") {
				t.Fatalf("compatibility promoted to primary error: %s", d.ID)
			}
		}
	}
	for _, format := range []string{"plain", "pretty"} {
		var out, errOut strings.Builder
		if code := run([]string{"--format", format, "--renderer=vsfiltermod", path}, &out, &errOut); code == 2 {
			t.Fatal(errOut.String())
		}
		if !strings.Contains(out.String(), "VSFilterMod@") || !strings.Contains(out.String(), "_VSMOD=unknown") {
			t.Fatalf("%s omits build scope", format)
		}
	}
}

func writeRendererFixture(t *testing.T, text string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "renderer.ass")
	source := "[Script Info]\nPlayResX: 640\nPlayResY: 480\nLayoutResX: 640\nLayoutResY: 480\nYCbCr Matrix: None\n[V4+ Styles]\nFormat: Name, Fontname, Fontsize\nStyle: Default, Arial, 20\n[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\nDialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,," + text + "\n"
	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
