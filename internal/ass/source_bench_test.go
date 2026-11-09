package ass

import (
	"testing"

	"assx/internal/perftest"
)

var benchmarkParsedResult Parsed

// Measures physical parsing only: lines, records, cells, and spans. It does
// not build a default document view.
func BenchmarkParseSourceFixtures(b *testing.B) {
	for _, test := range perftest.Cases(b) {
		b.Run(test.Name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(test.Text)))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				benchmarkParsedResult = ParseSource(test.Text)
			}
		})
	}
}
