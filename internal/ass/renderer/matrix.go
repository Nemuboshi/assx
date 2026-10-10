package renderer

//go:generate go run ./cmd/genmatrix

// Signature metadata is generated from docs/ass-tags.xml. Its verification
// scope is distinct from its declared applicability; an inferred signature
// must never become a proof of validity or safe rewriting.
type featureMask uint8

const (
	requiresMod featureMask = 1 << iota // _VSMOD
	requiresLua                         // _LUA, meaningful only with _VSMOD
)

type generatedSignature struct {
	count    int
	form     Form
	scope    uint8
	verified uint8
	requires featureMask // per-signature guards, not command-name dispatch
	citation string
}
type tagMeta struct {
	signatures         []generatedSignature
	exhaustive         uint8 // renderer-specific, source-proven complete set of accepted arities
	exhaustiveCitation string
}
