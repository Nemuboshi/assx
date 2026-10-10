// Package renderer resolves neutral ASS syntax against pinned implementations.
// It does not mutate the syntax tree or the historical default parser.
package renderer

import (
	"assx/internal/ass"
	"assx/internal/ass/spec"
	"fmt"
)

type Kind uint8

const (
	Libass Kind = iota
	XYVSFilter
	VSFilterMod
)

func (k Kind) String() string {
	switch k {
	case Libass:
		return "libass"
	case XYVSFilter:
		return "xy-VSFilter"
	case VSFilterMod:
		return "VSFilterMod"
	default:
		return "unknown"
	}
}

// FeatureUnknown must never be treated as enabled when proving edit safety.
type Feature uint8

const (
	FeatureUnknown Feature = iota
	FeatureDisabled
	FeatureEnabled
)

// Build models the independent optional VSFilterMod compile-time gates.
type Build struct {
	Mod Feature // _VSMOD
	Lua Feature // _LUA, effective only with _VSMOD
}

// Profiles have value semantics. Shared tables are private and read-only.
type Profile struct {
	kind  Kind
	build Build
}

func New(kind Kind, build Build) (Profile, error) {
	if kind > VSFilterMod {
		return Profile{}, fmt.Errorf("unknown renderer %d", kind)
	}
	if build.Mod > FeatureEnabled || build.Lua > FeatureEnabled {
		return Profile{}, fmt.Errorf("unknown build capability")
	}
	if kind != VSFilterMod && build != (Build{}) {
		return Profile{}, fmt.Errorf("%s does not have VSFilterMod build features", kind)
	}
	if build.Mod == FeatureDisabled && build.Lua == FeatureEnabled {
		return Profile{}, fmt.Errorf("_LUA requires _VSMOD")
	}
	return Profile{kind: kind, build: build}, nil
}
func Standard(kind Kind) (Profile, error) { return New(kind, Build{}) }
func (p Profile) Kind() Kind              { return p.kind }
func (p Profile) Build() Build            { return p.build }
func (p Profile) Version() string {
	switch p.kind {
	case Libass:
		return "f61db567e6593df3470e91594bcd4ad2d0473aff"
	case XYVSFilter:
		return "135a30153a38fa846cb5c39df0f258403e92096e"
	case VSFilterMod:
		return "7a00567e4a49b6310691b9a6791646b2a018bfa2"
	default:
		return ""
	}
}

type MatchStatus uint8

const (
	UnknownName MatchStatus = iota
	Matched
	Conditional
	Disabled
	Ignored // normalization matched but the apply command cannot consume the suffix
)

type SignatureStatus uint8

const (
	SignatureUnknown SignatureStatus = iota
	SignatureVerified
	SignatureInferred
	SignatureRejected
)

type Form uint8

const (
	Bare Form = iota
	Paren
	Both
)

type Argument struct {
	Span ass.ConcreteSpan
	Raw  string
}

// SignatureAvailability is compile-time reachability of an individual
// argument shape. It is independent of the command-name match.
type SignatureAvailability uint8

const (
	SignatureAvailable   SignatureAvailability = iota
	SignatureConditional                       // an unknown build flag may enable the signature
	SignatureUnavailable                       // a required build flag is known to be disabled
)

// FeatureRequirements describes the guards associated with one signature.
type FeatureRequirements struct {
	Mod bool // _VSMOD
	Lua bool // _LUA, together with _VSMOD
}

type Signature struct {
	Count        int
	Form         Form
	Evidence     SignatureStatus // Unknown unless available in this build
	Availability SignatureAvailability
	Requires     FeatureRequirements
	Citation     string
}
type Result struct {
	Renderer Kind
	Source   ass.ConcreteSpan
	Raw      string
	Head     string
	Name     string
	Suffix   string
	Shadowed string
	Status   MatchStatus
	Fallback string
	Args     []Argument
	Form     Form
	Closed   bool
	// EmptyComponents records neutral parenthesized empties before the
	// profile's parameter normalization; Raw and Source remain lossless.
	EmptyComponents bool
	// ParametersUnresolved prevents an apparent CST arity from certifying
	// parser-specific scans of nested arguments or parenthesized suffixes.
	ParametersUnresolved bool
	Signature            SignatureStatus
	Citation             string
	Policy               spec.TagSpec
	HasPolicy            bool
}

// Signatures returns detached copies of XML-generated evidence rows.
func (p Profile) Signatures(name string) []Signature {
	meta, ok := matrixTags[name]
	if !ok {
		return nil
	}
	out := make([]Signature, 0, len(meta.signatures))
	for _, s := range meta.signatures {
		mask := bit(p.kind)
		if s.scope&mask == 0 {
			continue
		}
		availability := p.signatureAvailability(s.requires)
		evidence := SignatureUnknown
		if availability == SignatureAvailable {
			evidence = SignatureInferred
			if s.verified&mask != 0 {
				evidence = SignatureVerified
			}
		}
		out = append(out, Signature{
			Count: s.count, Form: s.form, Evidence: evidence,
			Availability: availability,
			Requires:     FeatureRequirements{Mod: s.requires&requiresMod != 0, Lua: s.requires&requiresLua != 0},
			Citation:     s.citation,
		})
	}
	return out
}

// Disabled requirements take precedence over unknown flags. A conditional
// signature neither proves acceptance nor exhausts the available arities.
func (p Profile) signatureAvailability(requires featureMask) SignatureAvailability {
	if requires == 0 {
		return SignatureAvailable
	}
	result := SignatureAvailable
	for _, feature := range []struct {
		mask  featureMask
		state Feature
	}{{requiresMod, p.build.Mod}, {requiresLua, p.build.Lua}} {
		if requires&feature.mask == 0 {
			continue
		}
		switch feature.state {
		case FeatureDisabled:
			return SignatureUnavailable
		case FeatureUnknown:
			result = SignatureConditional
		}
	}
	return result
}

func bit(k Kind) uint8 { return 1 << k }
