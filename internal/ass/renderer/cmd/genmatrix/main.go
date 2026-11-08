// Command genmatrix generates renderer-scoped signatures from the validated
// evidence matrix. Generated code is deterministic and has no runtime XML IO.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/xml"
	"flag"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type param struct {
	Index    string `xml:"i,attr"`
	Kind     string `xml:"kind,attr"`
	Value    string `xml:"value,attr"`
	Min      string `xml:"min,attr"`
	Max      string `xml:"max,attr"`
	Renderer string `xml:"renderer,attr"`
}

type signature struct {
	N        string  `xml:"n,attr"`
	Form     string  `xml:"form,attr"`
	Scope    string  `xml:"renderer,attr"`
	Verified string  `xml:"verified,attr"`
	Requires string  `xml:"requires,attr"`
	Cite     string  `xml:"cite,attr"`
	Params   []param `xml:"p"`
}
type scenario struct {
	ID       string `xml:"id,attr"`
	Status   string `xml:"status,attr"`
	Verified string `xml:"verified,attr"`
	Base     string `xml:"base,attr"`
	XY       string `xml:"vsfilter,attr"`
	Mod      string `xml:"vsfiltermod,attr"`
	Requires string `xml:"requires,attr"`
	Cite     string `xml:"cite,attr"`
}

type behavior struct {
	Renderer          string `xml:"renderer,attr"`
	TransformChildren string `xml:"transform-children,attr"`
	EffectInTransform string `xml:"effect-in-transform,attr"`
	StyleTransform    string `xml:"style-transform,attr"`
}

type tagSpec struct {
	Renderer      string `xml:"renderer,attr"`
	PositionDepth string `xml:"position-depth,attr"`
}

type tag struct {
	Name      string     `xml:"name,attr"`
	Specs     []tagSpec  `xml:"spec"`
	Scenarios []scenario `xml:"scen"`
	Params    struct {
		Exhaustive     string      `xml:"exhaustive,attr"`
		ExhaustiveCite string      `xml:"exhaustive-cite,attr"`
		Signatures     []signature `xml:"sig"`
	} `xml:"params"`
}
type matrix struct {
	Behaviors []behavior `xml:"meta>behavior"`
	Tags      []tag      `xml:"tag"`
	Groups    []struct {
		Names string `xml:"names,attr"`
	} `xml:"taggroup"`
}

var valueKinds = map[string]string{
	"integer": "integer", "number": "number", "hex": "hex", "bold": "bold",
	"font": "font", "list": "list", "rect": "rect", "none": "none",
}

// paramSpec validates one argument row and returns its generated form.
// The attribute subset is optional: a row without value, min, or max leaves
// the fallback tag table in charge.
func paramSpec(row param, sigScope uint8) (string, int, uint8, error) {
	if row.Value == "" && row.Min == "" && row.Max == "" {
		return "", 0, 0, nil
	}
	if _, ok := valueKinds[row.Value]; !ok {
		return "", 0, 0, fmt.Errorf("invalid value kind %q", row.Value)
	}
	if (row.Min == "") != (row.Max == "") {
		return "", 0, 0, fmt.Errorf("min and max must be set together")
	}
	scope := sigScope
	if row.Renderer != "" {
		var err error
		scope, err = mask(row.Renderer)
		if err != nil {
			return "", 0, 0, err
		}
		if scope&^sigScope != 0 {
			return "", 0, 0, fmt.Errorf("parameter scope %q exceeds signature scope", row.Renderer)
		}
	}
	index, err := strconv.Atoi(row.Index)
	if err != nil || index < 1 {
		return "", 0, 0, fmt.Errorf("invalid argument index %q", row.Index)
	}
	if row.Min == "" {
		return fmt.Sprintf("{index:%d,scope:%d,value:%q}", index, scope, row.Value), index, scope, nil
	}
	lo, err := strconv.Atoi(row.Min)
	if err != nil {
		return "", 0, 0, fmt.Errorf("invalid min %q", row.Min)
	}
	hi, err := strconv.Atoi(row.Max)
	if err != nil {
		return "", 0, 0, fmt.Errorf("invalid max %q", row.Max)
	}
	if lo > hi {
		return "", 0, 0, fmt.Errorf("min %d exceeds max %d", lo, hi)
	}
	if row.Value != "integer" && row.Value != "number" {
		return "", 0, 0, fmt.Errorf("range applies only to integer or number values, got %q", row.Value)
	}
	return fmt.Sprintf("{index:%d,scope:%d,value:%q,min:%d,max:%d,hasRange:true}", index, scope, row.Value, lo, hi), index, scope, nil
}

func mask(fields string) (uint8, error) {
	var out uint8
	for _, field := range strings.Fields(fields) {
		var b uint8
		switch field {
		case "libass":
			b = 1
		case "xy":
			b = 2
		case "vsm":
			b = 4
		default:
			return 0, fmt.Errorf("unknown renderer %q", field)
		}
		if out&b != 0 {
			return 0, fmt.Errorf("duplicate renderer %q", field)
		}
		out |= b
	}
	return out, nil
}

// canonicalNewlines gives a stable evidence digest across Git worktrees with
// LF, CRLF or CR physical line endings. Source-preserving fixtures are not
// involved in this normalization.
func canonicalNewlines(data []byte) []byte {
	data = bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
	return bytes.ReplaceAll(data, []byte("\r"), []byte("\n"))
}

// requirementMask accepts only canonical guard sets from the pinned source.
// The Lua gate is not modeled, so no signature may require it.
func requirementMask(raw string) (uint8, error) {
	switch raw {
	case "":
		return 0, nil
	case "_VSMOD":
		return 1, nil
	default:
		return 0, fmt.Errorf("unsupported signature requirements %q", raw)
	}
}

func run() error {
	check := flag.Bool("check", false, "fail if generated output is stale")
	flag.Parse()
	input := filepath.Join("..", "..", "..", "docs", "reference", "ass-tags.xml")
	data, err := os.ReadFile(input)
	if err != nil {
		return err
	}
	var m matrix
	if err = xml.Unmarshal(data, &m); err != nil {
		return err
	}
	// Group stubs are documented but unverified, with no applicable signatures.
	// Keep their names resolvable without inventing arities or source proofs.
	for _, group := range m.Groups {
		for _, name := range strings.Fields(group.Names) {
			m.Tags = append(m.Tags, tag{Name: name})
		}
	}
	sort.Slice(m.Tags, func(i, j int) bool { return m.Tags[i].Name < m.Tags[j].Name })
	sum := sha256.Sum256(canonicalNewlines(data))
	var out bytes.Buffer
	fmt.Fprintln(&out, "// Code generated by go generate; DO NOT EDIT.")
	fmt.Fprintln(&out, "package renderer")
	fmt.Fprintf(&out, "const matrixSHA256 = %q\n", fmt.Sprintf("%x", sum))
	fmt.Fprintln(&out, "var matrixTags = map[string]tagMeta{")
	seen := make(map[string]bool)
	for _, t := range m.Tags {
		if t.Name == "" || seen[t.Name] {
			return fmt.Errorf("duplicate or unnamed tag %q", t.Name)
		}
		seen[t.Name] = true
		fmt.Fprintf(&out, "%q:{scenarios: []generatedScenario{", t.Name)
		for _, s := range t.Scenarios {
			v, err := mask(s.Verified)
			if err != nil {
				return fmt.Errorf("%s/%s: %w", t.Name, s.ID, err)
			}
			if s.Status != "V" && v != 0 {
				return fmt.Errorf("%s/%s: unverified scenario declares verified scope", t.Name, s.ID)
			}
			for i, value := range []string{s.Base, s.XY, s.Mod} {
				if v&(1<<i) != 0 && (value == "" || value == "unchecked" || s.Cite == "") {
					return fmt.Errorf("%s/%s: missing verified outcome or citation", t.Name, s.ID)
				}
			}
			requires, err := requirementMask(s.Requires)
			if err != nil {
				return fmt.Errorf("%s/%s: %w", t.Name, s.ID, err)
			}
			fmt.Fprintf(&out, "{id:%q,verified:%d,outcomes:[3]string{%q,%q,%q},requires:%d,citation:%q},", s.ID, v, s.Base, s.XY, s.Mod, requires, s.Cite)
		}
		fmt.Fprint(&out, "},signatures: []generatedSignature{")
		for _, s := range t.Params.Signatures {
			n, e := strconv.Atoi(s.N)
			if e != nil || n < 0 {
				return fmt.Errorf("%s: invalid arity %q", t.Name, s.N)
			}
			sc, e := mask(s.Scope)
			if e != nil {
				return fmt.Errorf("%s: %w", t.Name, e)
			}
			v, e := mask(s.Verified)
			if e != nil {
				return fmt.Errorf("%s: %w", t.Name, e)
			}
			if sc == 0 || v & ^sc != 0 {
				return fmt.Errorf("%s: invalid signature scope", t.Name)
			}
			requires, e := requirementMask(s.Requires)
			if e != nil {
				return fmt.Errorf("%s: %w", t.Name, e)
			}
			if requires != 0 && sc != 4 {
				return fmt.Errorf("%s: build requirements apply only to VSFilterMod signatures", t.Name)
			}
			var form string
			switch s.Form {
			case "bare":
				form = "Bare"
			case "paren":
				form = "Paren"
			case "both":
				form = "Both"
			default:
				return fmt.Errorf("%s: invalid form %q", t.Name, s.Form)
			}
			var params []string
			seenScope := make(map[int]uint8)
			for _, row := range s.Params {
				generated, index, scope, err := paramSpec(row, sc)
				if err != nil {
					return fmt.Errorf("%s/%d: %w", t.Name, n, err)
				}
				if generated == "" {
					continue
				}
				if seenScope[index]&scope != 0 {
					return fmt.Errorf("%s/%d: overlapping parameter rows for argument %d", t.Name, n, index)
				}
				seenScope[index] |= scope
				params = append(params, generated)
			}
			fmt.Fprintf(&out, "{count:%d,form:%s,scope:%d,verified:%d,", n, form, sc, v)
			if requires != 0 {
				fmt.Fprintf(&out, "requires:%d,", requires)
			}
			fmt.Fprintf(&out, "citation:%q", s.Cite)
			if len(params) != 0 {
				fmt.Fprintf(&out, ",params:[]generatedParam{%s}", strings.Join(params, ","))
			}
			fmt.Fprint(&out, "},")
		}
		complete, e := mask(t.Params.Exhaustive)
		if e != nil {
			return fmt.Errorf("%s: exhaustive signature scope: %w", t.Name, e)
		}
		if complete == 0 && t.Params.ExhaustiveCite != "" {
			return fmt.Errorf("%s: exhaustive source citation without an exhaustive scope", t.Name)
		}
		if complete != 0 {
			if t.Params.ExhaustiveCite == "" {
				return fmt.Errorf("%s: exhaustive signature scope needs independent source citation", t.Name)
			}
			for _, renderer := range []struct {
				mask uint8
				name string
			}{{1, "libass"}, {2, "xy"}, {4, "vsm"}} {
				if complete&renderer.mask == 0 {
					continue
				}
				cited := false
				for _, token := range strings.Fields(t.Params.ExhaustiveCite) {
					if strings.HasPrefix(token, renderer.name+":") {
						cited = true
					}
				}
				if !cited {
					return fmt.Errorf("%s: missing exhaustive source citation for %s", t.Name, renderer.name)
				}
				found := false
				for _, s := range t.Params.Signatures {
					sc, err := mask(s.Scope)
					if err != nil {
						return err
					}
					if sc&renderer.mask == 0 {
						continue
					}
					found = true
					verified, err := mask(s.Verified)
					if err != nil {
						return err
					}
					if verified&renderer.mask == 0 {
						return fmt.Errorf("%s: incomplete %s evidence cannot prove exhaustive arities", t.Name, renderer.name)
					}
				}
				if !found {
					return fmt.Errorf("%s: no applicable signatures for exhaustive %s scope", t.Name, renderer.name)
				}
			}
			fmt.Fprintf(&out, "}, exhaustive:%d, exhaustiveCitation:%q},\n", complete, t.Params.ExhaustiveCite)
		} else {
			fmt.Fprintln(&out, "}},")
		}
	}
	fmt.Fprintln(&out, "}")
	var modes [3]struct {
		transform int
		effect    bool
		style     int
	}
	for _, row := range m.Behaviors {
		scope, err := mask(row.Renderer)
		if err != nil {
			return fmt.Errorf("behavior: %w", err)
		}
		var transform int
		switch row.TransformChildren {
		case "", "static":
		case "recursive":
			transform = 1
		default:
			return fmt.Errorf("behavior: invalid transform-children %q", row.TransformChildren)
		}
		var style int
		switch row.StyleTransform {
		case "", "none":
		case "mod":
			style = 1
		default:
			return fmt.Errorf("behavior: invalid style-transform %q", row.StyleTransform)
		}
		var effect bool
		switch row.EffectInTransform {
		case "", "false":
		case "true":
			effect = true
		default:
			return fmt.Errorf("behavior: invalid effect-in-transform %q", row.EffectInTransform)
		}
		for kind := 0; kind < 3; kind++ {
			if scope&(1<<kind) != 0 {
				modes[kind] = struct {
					transform int
					effect    bool
					style     int
				}{transform, effect, style}
			}
		}
	}
	fmt.Fprintln(&out, "var matrixBehavior = [3]generatedProfileBehavior{")
	for _, mode := range modes {
		fmt.Fprintf(&out, "{transformChildren: %d, effectInTransform: %t, styleTransform: %d},\n", mode.transform, mode.effect, mode.style)
	}
	fmt.Fprintln(&out, "}")
	fmt.Fprintln(&out, "var matrixTagSpecs = map[string][]generatedTagSpec{")
	for _, t := range m.Tags {
		var rows []string
		seen := make(map[uint8]bool)
		for _, spec := range t.Specs {
			scope, err := mask(spec.Renderer)
			if err != nil {
				return fmt.Errorf("%s spec: %w", t.Name, err)
			}
			if seen[scope] {
				return fmt.Errorf("%s spec: duplicate renderer scope", t.Name)
			}
			seen[scope] = true
			var depth bool
			switch spec.PositionDepth {
			case "", "false":
			case "true":
				depth = true
			default:
				return fmt.Errorf("%s spec: invalid position-depth %q", t.Name, spec.PositionDepth)
			}
			rows = append(rows, fmt.Sprintf("{scope: %d, positionDepth: %t}", scope, depth))
		}
		if len(rows) != 0 {
			fmt.Fprintf(&out, "%q: {%s},\n", t.Name, strings.Join(rows, ", "))
		}
	}
	fmt.Fprintln(&out, "}")
	formatted, err := format.Source(out.Bytes())
	if err != nil {
		return fmt.Errorf("format generated source: %w", err)
	}
	target := "matrix_generated.go"
	if *check {
		current, err := os.ReadFile(target)
		if err != nil {
			return err
		}
		if !bytes.Equal(canonicalNewlines(current), formatted) {
			return fmt.Errorf("%s is stale: run go generate ./internal/ass/renderer", target)
		}
		return nil
	}
	return os.WriteFile(target, formatted, 0644)
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
