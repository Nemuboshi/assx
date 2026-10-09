package lint

import (
	"fmt"
	"strings"
	"testing"

	"assx/internal/ass"
)

func styleLoadDocument(styles, dialogues int, dialogueText string) ass.Document {
	var text strings.Builder
	text.WriteString("[V4+ Styles]\nFormat: Name, Fontname, Fontsize, Bold, Italic, ScaleX, ScaleY, Spacing, Outline, Shadow\n")
	for i := 0; i < styles; i++ {
		fmt.Fprintf(&text, "Style: S%d,Go,20,0,0,100,100,0,2,2\n", i)
	}
	text.WriteString("[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n")
	text.WriteString(strings.Repeat("Dialogue: 0,0:00:00.00,0:00:01.00,S0,,0,0,0,,"+dialogueText+"\n", dialogues))
	return ass.Parse(text.String())
}

func TestStyleStateAllocationsDoNotMultiplyStylesByDialogues(t *testing.T) {
	perDialogue := func(styles int) float64 {
		small, large := styleLoadDocument(styles, 1, `{\fs20}A`), styleLoadDocument(styles, 1000, `{\fs20}A`)
		base := testing.AllocsPerRun(2, func() { AnalyzeRedundantStyleOverrides(small) })
		all := testing.AllocsPerRun(2, func() { AnalyzeRedundantStyleOverrides(large) })
		return (all - base) / 999
	}
	one, hundred := perDialogue(1), perDialogue(100)
	if hundred > one*2+1 {
		t.Fatalf("per-dialogue allocations grew with style count: one style %.1f, 100 styles %.1f", one, hundred)
	}
}

func BenchmarkRedundantStyleStateReuse(b *testing.B) {
	for _, styles := range []int{1, 100} {
		doc := styleLoadDocument(styles, 1000, "A")
		b.Run(fmt.Sprintf("%d_styles_1000_dialogues", styles), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				AnalyzeRedundantStyleOverrides(doc)
			}
		})
	}
}
