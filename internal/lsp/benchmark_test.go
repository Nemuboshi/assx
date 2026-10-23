package lsp

import (
	"bytes"
	"encoding/json/v2"
	"fmt"
	"strings"
	"testing"

	"assx/internal/sourcepos"
)

func BenchmarkLSPPublishDiagnostics(b *testing.B) {
	for _, count := range []int{10, 10000} {
		b.Run(fmt.Sprintf("dialogues=%d", count), func(b *testing.B) {
			var text strings.Builder
			text.WriteString("[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n")
			for range count {
				text.WriteString("Dialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,line\n")
			}
			var server Server
			const uri = "file:///benchmark.ass"
			if err := server.documents.Open(uri, 1, text.String()); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			for b.Loop() {
				var output bytes.Buffer
				if err := server.publish(b.Context(), &output, uri); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkLSPRapidEditAndHover(b *testing.B) {
	const uri = "file:///edit-benchmark.ass"
	text := "[Events]\nDialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,{\\pos(1,2)}x\n"
	var server Server
	if err := server.documents.Open(uri, 1, text); err != nil {
		b.Fatal(err)
	}
	request, err := json.Marshal(map[string]any{"textDocument": map[string]string{"uri": uri}, "position": sourcepos.Position{Line: 1, Character: 54}})
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	version := 1
	for b.Loop() {
		version++
		if err := server.documents.Change(uri, version, []Change{{Range: &sourcepos.Range{Start: sourcepos.Position{Line: 1, Character: 0}, End: sourcepos.Position{Line: 1, Character: 0}}, Text: ""}}); err != nil {
			b.Fatal(err)
		}
		if _, err := server.hover(b.Context(), request); err != nil {
			b.Fatal(err)
		}
	}
}
