package renderer

//go:generate go run ./cmd/genmatrix

// Signature metadata is generated from docs/ass-tags.xml. Its verification
// scope is distinct from its declared applicability; an inferred signature
// must never become a proof of validity or safe rewriting.
type generatedSignature struct {
	count    int
	form     Form
	scope    uint8
	verified uint8
	citation string
}
type tagMeta struct {
	signatures []generatedSignature
}
