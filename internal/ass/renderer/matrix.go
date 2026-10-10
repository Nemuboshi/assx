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
type generatedScenario struct {
	id       string
	verified uint8
	outcomes [3]string
	citation string
	requires featureMask // optional guards on the VSFilterMod outcome only
}

// BehaviorEvidence describes one explicitly documented scenario. Verified
// means the outcome, not all invocations of the tag, was source-checked.
// Absent rows never inherit the matrix's editorial scenario defaults.
type BehaviorEvidence struct {
	Outcome  string
	Verified bool
	Citation string
}

func (p Profile) Behavior(name, scenario string) BehaviorEvidence {
	for _, row := range matrixTags[name].scenarios {
		if row.id == scenario {
			verified := row.verified&bit(p.kind) != 0
			if p.kind == VSFilterMod && p.signatureAvailability(row.requires) != SignatureAvailable {
				verified = false
			}
			return BehaviorEvidence{Outcome: row.outcomes[p.kind], Verified: verified, Citation: row.citation}
		}
	}
	return BehaviorEvidence{}
}

type tagMeta struct {
	scenarios          []generatedScenario
	signatures         []generatedSignature
	exhaustive         uint8 // renderer-specific, source-proven complete set of accepted arities
	exhaustiveCitation string
}
