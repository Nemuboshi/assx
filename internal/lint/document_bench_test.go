package lint

import (
	"strings"
	"testing"

	"assx/internal/ass"
)

func BenchmarkAnalyzeDocument(b *testing.B) {
	for _, test := range []struct {
		name  string
		count int
	}{{"100_dialogues", 100}, {"1000_dialogues", 1000}} {
		text := benchmarkASSDocument(test.count)
		doc := ass.Parse(text)
		b.Run(test.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = AnalyzeDocument(doc)
			}
		})
	}
}

func BenchmarkParseAndAnalyzeDocument(b *testing.B) {
	for _, test := range []struct {
		name  string
		count int
	}{{"100_dialogues", 100}, {"1000_dialogues", 1000}} {
		text := benchmarkASSDocument(test.count)
		b.Run(test.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = AnalyzeDocument(ass.Parse(text))
			}
		})
	}
}

func benchmarkASSDocument(dialogues int) string {
	var text strings.Builder
	text.WriteString("[Script Info]\nPlayResX: 1920\nPlayResY: 1080\nYCbCr Matrix: None\nLayoutResX: 1920\nLayoutResY: 1080\n")
	text.WriteString("[V4+ Styles]\nFormat: Name, Fontname, Fontsize, Bold, Italic, ScaleX, ScaleY, Spacing, Outline, Shadow\nStyle: Default,Arial,20,0,0,100,100,0,2,2\n")
	text.WriteString("[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n")
	const line = "Dialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,{\\b1\\b1\\fs20\\fs20\\t(0,500,\\fscx100)}Benchmark subtitle\n"
	for i := 0; i < dialogues; i++ {
		text.WriteString(line)
	}
	return text.String()
}
