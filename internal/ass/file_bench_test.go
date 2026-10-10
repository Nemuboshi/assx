package ass

import (
	"testing"

	"assx/internal/perftest"
)

var benchmarkDocumentResult Document

// Measures whole-document parsing, including building all dialogue ASTs.
func BenchmarkASSDocumentParse(b *testing.B) {
	for _, test := range perftest.Cases(b) {
		b.Run(test.Name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(test.Text)))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				benchmarkDocumentResult = Parse(test.Text)
			}
		})
	}
}
