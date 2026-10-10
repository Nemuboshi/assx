package renderer

import (
	"strings"

	"assx/internal/ass"
	"assx/internal/ass/spec"
)

// Pinned first-match branch order: libass/libass/ass_parse.c:355-916.
var libassOrder = strings.Fields("xbord ybord xshad yshad fax fay iclip blur fscx fscy fsc fsp fs bord move frx fry frz fr fn alpha an a pos fade fad org t clip c 1c 2c 3c 4c 1a 2a 3a 4a r be b i kt kf K ko k shad s u pbo p q fe")

// Exact map registrations: xy-VSFilter/src/subtitles/RTS.cpp:1716-1769.
// xy independently searches exact keys from length 5 down to length 1.
var xyRegistry = strings.Fields("1c 2c 3c 4c 1a 2a 3a 4a alpha an a blur bord be b clip iclip c fade fad fax fay fe fn frx fry frz fr fscx fscy fsc fsp fs i kt kf K ko k move org pbo pos p q r shad s t u xbord xshad ybord yshad")

// Ordered normalization: VSFilterMod/src/subtitles/RTS.cpp:2524-2669.
// Enabled Mod extensions are interleaved with core prefix checks.
var modOrder = strings.Fields("1c 2c 3c 4c 1a 2a 3a 4a 1img 2img 3img 4img 1vc 2vc 3vc 4vc 1va 2va 3va 4va alpha an a blur bord be blend b clip c distort fade fe fn frx fry frz frs fax fay fr fscx fscy fsc fsp fsvp fshp fs iclip i jitter kt kf ko k K lua mover moves3 moves4 movevc move org pbo pos p q rndx rndy rndz rnds rnd r shad s t u xbord xshad ybord yshad xblur yblur z ortho")

// Normalization no-ops preserve the entire command for exact application
// matching. A suffix on one of these branches is NOT consumed as an argument.
func exactOnlyMod(name string) bool {
	switch name {
	case "1img", "2img", "3img", "4img", "1vc", "2vc", "3vc", "4vc",
		"1va", "2va", "3va", "4va", "clip", "iclip", "distort", "fade",
		"jitter", "lua", "mover", "moves3", "moves4", "movevc", "move",
		"org", "pos", "t":
		return true
	}
	return false
}

func hasModGate(name string) bool {
	switch name {
	case "1img", "2img", "3img", "4img", "1vc", "2vc", "3vc", "4vc",
		"1va", "2va", "3va", "4va", "blend", "distort", "frs",
		"fsvp", "fshp", "jitter", "lua", "mover", "moves3",
		"moves4", "movevc", "rndx", "rndy", "rndz", "rnds",
		"rnd", "z", "ortho":
		return true
	}
	return false
}

func (p Profile) selectName(head string) (name, fallback string, status MatchStatus) {
	switch p.kind {
	case Libass:
		for _, candidate := range libassOrder {
			if strings.HasPrefix(head, candidate) {
				return candidate, "", Matched
			}
		}
	case XYVSFilter:
		for size := 5; size >= 1; size-- {
			if len(head) < size {
				continue
			}
			for _, candidate := range xyRegistry {
				if len(candidate) == size && strings.HasPrefix(head, candidate) {
					return candidate, "", Matched
				}
			}
		}
	case VSFilterMod:
		// fad is an exact apply command but has no prefix-normalization arm.
		if head == "fad" {
			return "fad", "", Matched
		}
		for _, candidate := range modOrder {
			if !strings.HasPrefix(head, candidate) {
				continue
			}
			if hasModGate(candidate) && p.build.Mod != FeatureEnabled {
				if p.build.Mod == FeatureDisabled {
					continue
				}
				// Keep the build-off alternate instead of guessing the build.
				off := p
				off.build.Mod = FeatureDisabled
				fallback, _, _ = off.selectName(head)
				return candidate, fallback, Conditional
			}
			if exactOnlyMod(candidate) && head != candidate {
				return candidate, "", Ignored
			}
			if candidate == "lua" && p.build.Lua != FeatureEnabled {
				if p.build.Lua == FeatureUnknown {
					return candidate, "", Conditional
				}
				return candidate, "", Disabled
			}
			return candidate, "", Matched
		}
	}
	return "", "", UnknownName
}

// Resolve preserves absolute dialogue UTF-8 byte spans and raw argument bytes.
// Identity, compile-time reachability and signature evidence stay independent.
func (p Profile) Resolve(expr ass.ConcreteExpression, source string) Result {
	r := Result{
		Renderer: p.kind, Source: expr.Span, Raw: expr.Raw, Head: expr.Head,
		Closed: expr.Closed(), Form: Bare,
	}
	if expr.Parenthesized {
		r.Form = Paren
		if expr.ContentSpan.Start >= 0 && expr.ContentSpan.End <= len(source) {
			start := expr.ContentSpan.Start
			for _, comma := range expr.Commas {
				r.Args = append(r.Args, Argument{Span: ass.ConcreteSpan{Start: start, End: comma}, Raw: source[start:comma]})
				start = comma + 1
			}
			r.Args = append(r.Args, Argument{Span: ass.ConcreteSpan{Start: start, End: expr.ContentSpan.End}, Raw: source[start:expr.ContentSpan.End]})
		}
	}
	if expr.SlashSpan.End-expr.SlashSpan.Start != 1 || !expr.Closed() {
		return r
	}
	r.Name, r.Fallback, r.Status = p.selectName(expr.Head)
	if r.Status == UnknownName {
		return r
	}
	r.Suffix = expr.Head[len(r.Name):]
	if !expr.Parenthesized && r.Suffix != "" {
		span := ass.ConcreteSpan{Start: expr.HeadSpan.Start + len(r.Name), End: expr.HeadSpan.End}
		r.Args = []Argument{{Span: span, Raw: r.Suffix}}
	}
	// Retain a known longer competing command even when its argument suffix
	// follows immediately (e.g. fsvp6 versus the traditional fs prefix).
	if len(expr.Head) > len(r.Name) {
		for _, candidate := range modOrder {
			if len(candidate) > len(r.Name) && len(candidate) > len(r.Shadowed) && strings.HasPrefix(expr.Head, candidate) {
				r.Shadowed = candidate
			}
		}
	}
	if s, ok := spec.TagSpecs[r.Name]; ok {
		r.Policy = s
		r.Policy.Slots = append([]string(nil), s.Slots...)
		r.Policy.Counts = append([]int(nil), s.Counts...)
		r.HasPolicy = true
	}
	if r.Status != Matched {
		return r
	}
	sigs := p.Signatures(r.Name)
	if len(sigs) == 0 || (!expr.Parenthesized && r.Suffix == "") {
		return r
	}
	// Missing matches imply rejection only when the entire applicable
	// signature set is source-verified. Inferred entries cannot prove absence.
	fullyVerified := true
	for _, s := range sigs {
		if s.Evidence != SignatureVerified {
			fullyVerified = false
		}
		if s.Count != len(r.Args) || (s.Form != Both && s.Form != r.Form) {
			continue
		}
		if r.Signature != SignatureVerified || s.Evidence == SignatureVerified {
			r.Signature, r.Citation = s.Evidence, s.Citation
		}
	}
	if r.Signature == SignatureUnknown && fullyVerified {
		r.Signature = SignatureRejected
	}
	// Parenthesized suffixes have parser-specific consumption semantics; do
	// not promote an arity match into evidence for this unmodeled combination.
	if expr.Parenthesized && r.Suffix != "" {
		r.Signature = SignatureUnknown
		r.Citation = ""
	}
	return r
}

// WalkDialogue emits candidates in source order, including arbitrarily
// nested transforms. The explicit stack avoids recursion on hostile input;
// only a recognized transform enables walking its nested syntax.
func (p Profile) WalkDialogue(tree ass.ConcreteDialogue, visit func(Result, bool) bool) {
	type pending struct {
		expr   ass.ConcreteExpression
		nested bool
	}
	for _, node := range tree.Nodes {
		if node.Block == nil {
			continue
		}
		for _, item := range node.Block.Items {
			if item.Expression == nil {
				continue
			}
			stack := []pending{{expr: *item.Expression}}
			for len(stack) > 0 {
				last := len(stack) - 1
				current := stack[last]
				stack = stack[:last]
				r := p.Resolve(current.expr, tree.Source)
				if !visit(r, current.nested) {
					return
				}
				if r.Status != Matched || r.Name != "t" || !current.expr.Parenthesized {
					continue
				}
				components := current.expr.Components(tree.Source)
				for i := len(components) - 1; i >= 0; i-- {
					for j := len(components[i].Items) - 1; j >= 0; j-- {
						child := components[i].Items[j].Expression
						if child != nil {
							stack = append(stack, pending{expr: *child, nested: true})
						}
					}
				}
			}
		}
	}
}
