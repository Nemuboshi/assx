package renderer

import "assx/internal/ass/spec"

// generatedParam is one matrix argument row with structured value fields.
type generatedParam struct {
	index    int
	scope    uint8
	value    string
	min, max int
	hasRange bool
}

func valueKind(name string) spec.ValueKind {
	switch name {
	case "integer":
		return spec.IntegerValue
	case "number":
		return spec.NumberValue
	case "hex":
		return spec.HexValue
	case "bold":
		return spec.BoldValue
	case "font":
		return spec.FontNameValue
	case "list":
		return spec.NumberListValue
	case "rect":
		return spec.RectValue
	}
	return spec.NoValue
}

type commandSpecKey struct {
	name  string
	count int
	form  Form
}

type commandSpecValue struct {
	value    spec.ValueKind
	min, max int
	hasRange bool
}

// commandSpecs holds the matrix override for each profile, command, arity,
// and form. Building it once keeps resolution free of matrix walks.
var commandSpecs [3]map[commandSpecKey]commandSpecValue

func init() { rebuildCommandSpecs() }

// rebuildCommandSpecs refreshes the override table. Tests that replace a
// matrix entry call it after the change.
func rebuildCommandSpecs() {
	for index := range commandSpecs {
		commandSpecs[index] = make(map[commandSpecKey]commandSpecValue)
	}
	for name, meta := range matrixTags {
		for _, signature := range meta.signatures {
			for _, param := range signature.params {
				if param.index != 1 {
					continue
				}
				for kind := Kind(0); kind <= VSFilterMod; kind++ {
					if param.scope&bit(kind) == 0 || signature.scope&bit(kind) == 0 {
						continue
					}
					commandSpecs[kind][commandSpecKey{name, signature.count, signature.form}] = commandSpecValue{
						value: valueKind(param.value), min: param.min, max: param.max, hasRange: param.hasRange,
					}
				}
			}
		}
	}
}

// CommandSpec returns the tag spec for one profile, command, arity, and form.
// The matrix argument row overrides the fallback tag table when it carries a
// structured value. A missing row keeps the fallback entry unchanged.
func (p Profile) CommandSpec(name string, count int, form Form) (spec.TagSpec, bool) {
	tagSpec, known := spec.TagSpecs[name]
	if p.kind == VSFilterMod {
		if mod, ok := spec.VSFilterModTagSpecs[name]; ok {
			tagSpec, known = mod, true
		}
	}
	if !known {
		return spec.TagSpec{}, false
	}
	override, ok := commandSpecs[p.kind][commandSpecKey{name, count, form}]
	if !ok {
		override, ok = commandSpecs[p.kind][commandSpecKey{name, count, Both}]
	}
	if !ok {
		return tagSpec, true
	}
	tagSpec.Value = override.value
	if override.hasRange {
		tagSpec.Min, tagSpec.Max = override.min, override.max
	}
	return tagSpec, true
}
