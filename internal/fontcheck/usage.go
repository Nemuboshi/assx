package fontcheck

import (
	"strconv"
	"strings"
	"unicode/utf8"

	"assx/internal/ass"
	"assx/internal/ass/renderer"
	"assx/internal/semantic"
)

// Request retains the certainty of each renderer-resolved font property.
type Request struct {
	Family      string `json:"family"`
	Bold        bool   `json:"bold"`
	Italic      bool   `json:"italic"`
	FamilyKnown bool   `json:"family_known"`
	BoldKnown   bool   `json:"bold_known"`
	ItalicKnown bool   `json:"italic_known"`
}

// Span uses zero-based byte offsets in the decoded ASS source.
type Span struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

// Provenance gives tag indexes; -1 means no proven tag owner.
type Provenance struct {
	Style  string `json:"style"`
	Family int    `json:"family_tag"`
	Bold   int    `json:"bold_tag"`
	Italic int    `json:"italic_tag"`
}

// Usage is one text boundary from the shared semantic evaluator.
type Usage struct {
	Line            int              `json:"line"`
	Column          int              `json:"column"`
	SourceColumn    int              `json:"source_column"`
	Span            Span             `json:"source_span"`
	Text            string           `json:"text"`
	Request         Request          `json:"requested"`
	Known           bool             `json:"known"`
	Drawing         bool             `json:"drawing"`
	VisibilityKnown bool             `json:"visibility_known"`
	Provenance      Provenance       `json:"provenance"`
	Profile         renderer.Profile `json:"-"`
}

// ExtractUsages does no font I/O. Unknown text state remains in the result.
func ExtractUsages(doc ass.Document, profile renderer.Profile) []Usage {
	doc = profile.DocumentView(doc)
	definitions := semantic.StyleDefinitionsByName(doc.StyleFields)
	styles := semantic.StyleStatesByName(doc.StyleFields)
	for name, fields := range definitions {
		style := styles[name]
		for _, property := range []string{"bold", "italic"} {
			if _, present := fields[property]; !present {
				style.Values[property] = semantic.KnownValue("0")
			}
		}
		styles[name] = style
	}
	var usages []Usage
	for _, dialogue := range doc.Dialogues {
		semantic.EvaluateResolved(ass.ParseConcreteDialogue(dialogue.Text), profile, semantic.EvaluationOptions{
			Styles: styles, DialogueStyle: dialogue.Style, SkipNoEffectProofs: true,
			Observer: semantic.Observer{Text: func(text string, start int, state semantic.StateView) {
				family, bold, italic := state.Value("fontname"), state.Value("bold"), state.Value("italic")
				drawing := state.Value("drawing_scale")
				scale, err := strconv.ParseInt(drawing.Value, 10, 32)
				visibilityKnown := drawing.Known && err == nil
				usages = append(usages, Usage{
					Line: dialogue.Line, Column: start + 1,
					SourceColumn: dialogue.TextStart - dialogue.LineStart + start + 1,
					Span:         Span{Start: dialogue.TextStart + start, End: dialogue.TextStart + start + len(text)},
					Text:         fontVisibleText(text), Profile: profile,
					Request: Request{Family: family.Value, Bold: styleBoolean(bold.Value), Italic: styleBoolean(italic.Value),
						FamilyKnown: family.Known, BoldKnown: bold.Known, ItalicKnown: italic.Known},
					Known:   family.Known && bold.Known && italic.Known && visibilityKnown,
					Drawing: visibilityKnown && scale > 0, VisibilityKnown: visibilityKnown,
					Provenance: Provenance{Style: state.ActiveStyle(), Family: state.Source("fontname"), Bold: state.Source("bold"), Italic: state.Source("italic")},
				})
			}},
		})
	}
	return usages
}

func styleBoolean(value string) bool {
	value = strings.TrimSpace(value)
	return value != "" && value != "0"
}

func fontVisibleText(text string) string {
	var visible strings.Builder
	for i := 0; i < len(text); {
		if text[i] == '\\' && i+1 < len(text) {
			switch text[i+1] {
			case 'N', 'n':
				i += 2
				continue
			case 'h':
				visible.WriteRune('\u00a0')
				i += 2
				continue
			}
		}
		r, size := utf8.DecodeRuneInString(text[i:])
		visible.WriteRune(r)
		i += size
	}
	return visible.String()
}
