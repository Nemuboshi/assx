package ass

import (
	"strings"
	"testing"
)

func BenchmarkParseConcreteDialogue(b *testing.B) {
	for _, test := range dialogueTextBenchmarkCases() {
		b.Run(test.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_ = ParseConcreteDialogue(test.text)
			}
		})
	}
}

func BenchmarkConcreteDialogueWalk(b *testing.B) {
	for _, test := range dialogueTextBenchmarkCases() {
		tree := ParseConcreteDialogue(test.text)
		var visited int
		visit := func(ConcreteExpression, bool) bool {
			visited++
			return true
		}
		b.Run(test.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				WalkConcreteExpressions(tree, visit)
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
