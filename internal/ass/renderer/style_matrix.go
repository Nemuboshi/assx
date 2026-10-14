package renderer

//go:generate go run ./cmd/genstylematrix

// styleConversion records the raw-text-to-value conversion of one Style field
// in one renderer. Verified means the conversion steps were read in pinned
// source, not that the resulting rendering is equivalent. The generated table
// is evidence; it is never a SafeFix proof.
type styleConversion struct {
	parse    string
	post     string
	verified bool
	citation string
}

type styleFieldEvidence struct {
	kind  string
	convs map[string]styleConversion
}

// StyleFieldConversion returns the pinned conversion evidence for one Style
// field name (lower case) and renderer name. The second result is false when
// the field or renderer is absent from the matrix.
func (p Profile) StyleFieldConversion(field string) (parse, post string, verified bool, citation string, ok bool) {
	row, found := styleFieldMatrix[field]
	if !found {
		return "", "", false, "", false
	}
	conv, found := row.convs[p.kind.matrixName()]
	if !found {
		return "", "", false, "", false
	}
	return conv.parse, conv.post, conv.verified, conv.citation, true
}

func (k Kind) matrixName() string {
	switch k {
	case Libass:
		return "libass"
	case XYVSFilter:
		return "xy"
	case VSFilterMod:
		return "vsm"
	default:
		return ""
	}
}
