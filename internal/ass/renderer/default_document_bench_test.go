package renderer

import (
	"testing"

	"assx/internal/ass"
	"assx/internal/perftest"
)

var benchmarkDocumentResult Document

// Measures default document view construction from a physical parse. The
// parse itself is outside the timer.
func BenchmarkDefaultDocumentFixtures(b *testing.B) {
	for _, test := range perftest.Cases(b) {
		source := ass.ParseSource(test.Text)
		b.Run(test.Name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(test.Text)))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				benchmarkDocumentResult = DefaultDocument(source)
			}
		})
	}
}
