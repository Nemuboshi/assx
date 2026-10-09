package ass

import (
	"strings"
	"testing"
)

func BenchmarkParseDialogueText(b *testing.B) {
	for _, test := range dialogueTextBenchmarkCases() {
		b.Run(test.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = ParseDialogueText(test.text)
			}
		})
	}
}

func BenchmarkDialogueTextTokens(b *testing.B) {
	for _, test := range dialogueTextBenchmarkCases() {
		tree := ParseDialogueText(test.text)
		b.Run(test.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = tree.Tokens()
			}
		})
	}
}

func BenchmarkDialogueTextWalk(b *testing.B) {
	for _, test := range dialogueTextBenchmarkCases() {
		tree := ParseDialogueText(test.text)
		var visited int
		visit := func(TokenView) bool {
			visited++
			return true
		}
		b.Run(test.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				tree.WalkTokens(visit)
			}
			if visited == 0 {
				b.Fatal("visitor saw no tokens")
			}
		})
	}
}

func dialogueTextBenchmarkCases() []struct {
	name string
	text string
} {
	return []struct {
		name string
		text string
	}{
		{name: "typical", text: `prefix{\fscx99.33\t(19,937,\fscx100)\clip(m 0 0 l 100 100)}caption suffix`},
		{name: "nested_10", text: nestedTransformInput(10)},
		{name: "nested_100", text: nestedTransformInput(100)},
		{name: "nested_1000", text: nestedTransformInput(1000)},
	}
}

func nestedTransformInput(depth int) string {
	return "{" + strings.Repeat(`\t(0,1,`, depth) + `\fscx100` + strings.Repeat(")", depth) + "}"
}
