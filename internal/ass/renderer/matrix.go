package renderer

//go:generate go run ./cmd/genmatrix

// Signature metadata is generated from docs/reference/ass-tags.xml. Its verification
// scope is distinct from its declared applicability; an inferred signature
// must never become a proof of validity or safe rewriting.
type featureMask uint8

type generatedSignature struct {
	count    int
	form     Form
	scope    uint8
	verified uint8
	requires featureMask // per-signature guards, not command-name dispatch
	citation string
	params   []generatedParam // structured argument value fields
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
			return BehaviorEvidence{Outcome: row.outcomes[p.kind], Verified: row.verified&bit(p.kind) != 0, Citation: row.citation}
		}
	}
	return BehaviorEvidence{}
}

type transformChildrenMode uint8

const (
	transformChildrenStatic transformChildrenMode = iota
	transformChildrenRecursive
)

type styleTransformMode uint8

const (
	styleTransformNone styleTransformMode = iota
	styleTransformMod
)

// generatedProfileBehavior holds the evaluation modes fixed for one profile.
type generatedProfileBehavior struct {
	transformChildren transformChildrenMode
	effectInTransform bool
	styleTransform    styleTransformMode
}

// generatedTagSpec holds per-renderer evaluation flags for one tag.
type generatedTagSpec struct {
	scope         uint8
	positionDepth bool
}

// EvaluationModes returns the profile's evaluation modes.
func (p Profile) EvaluationModes() generatedProfileBehavior { return matrixBehavior[p.kind] }

// RecursiveTransformChildren reports whether transform children apply their
// handlers recursively instead of publishing a static value.
func (p Profile) RecursiveTransformChildren() bool {
	return p.EvaluationModes().transformChildren == transformChildrenRecursive
}

// EffectInTransform reports whether effect handlers run inside a transform.
func (p Profile) EffectInTransform() bool { return p.EvaluationModes().effectInTransform }

// ModStyleTransform reports whether Style values use the Mod parsing model.
func (p Profile) ModStyleTransform() bool {
	return p.EvaluationModes().styleTransform == styleTransformMod
}

// PositionDepth reports whether the tag's third argument sets Mod depth.
func (p Profile) PositionDepth(name string) bool {
	for _, spec := range matrixTagSpecs[name] {
		if spec.scope&bit(p.kind) != 0 {
			return spec.positionDepth
		}
	}
	return false
}

type tagMeta struct {
	scenarios          []generatedScenario
	signatures         []generatedSignature
	exhaustive         uint8 // renderer-specific, source-proven complete set of accepted arities
	exhaustiveCitation string
}
