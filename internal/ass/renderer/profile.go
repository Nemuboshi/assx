// Package renderer resolves neutral ASS syntax against pinned implementations.
// It does not mutate the syntax tree or the default document parser.
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

// Profiles have value semantics. Shared tables are private and read-only.
// One pinned build per renderer, so no build capability is part of a profile.
type Profile struct {
	kind Kind
}

func New(kind Kind) (Profile, error) {
	if kind > VSFilterMod {
		return Profile{}, fmt.Errorf("unknown renderer %d", kind)
	}
	return Profile{kind: kind}, nil
}
func Standard(kind Kind) (Profile, error) { return New(kind) }
func (p Profile) Kind() Kind              { return p.kind }

type MatchStatus uint8

const (
	UnknownName MatchStatus = iota
	Matched
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

type Signature struct {
	Count    int
	Form     Form
	Evidence SignatureStatus
	Citation string
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
		evidence := SignatureInferred
		if s.verified&mask != 0 {
			evidence = SignatureVerified
		}
		out = append(out, Signature{
			Count: s.count, Form: s.form, Evidence: evidence,
			Citation: s.citation,
		})
	}
	return out
}

func bit(k Kind) uint8 { return 1 << k }
