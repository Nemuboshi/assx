package lsp

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"strings"

	"assx/internal/ass"
	"assx/internal/ass/renderer"
	"assx/internal/semantic"
	"assx/internal/sourcepos"
)

type location struct {
	URI   string          `json:"uri"`
	Range sourcepos.Range `json:"range"`
}

type documentSymbol struct {
	Name           string           `json:"name"`
	Kind           int              `json:"kind"`
	Range          sourcepos.Range  `json:"range"`
	SelectionRange sourcepos.Range  `json:"selectionRange"`
	Children       []documentSymbol `json:"children,omitempty"`
}

type styleOccurrence struct {
	name string
	span ass.ConcreteSpan
}

type symbolIndex struct {
	definitions map[string][]ass.ConcreteSpan
	uses        []styleOccurrence
	symbols     []documentSymbol
}

func (server *Server) indexSymbols(doc document) symbolIndex {
	index := symbolIndex{definitions: make(map[string][]ass.ConcreteSpan), symbols: []documentSymbol{}}
	childrenByLine := make(map[int][]documentSymbol)
	for _, field := range doc.parsed.StyleFields {
		if field.Name != "name" || field.Value == "" {
			continue
		}
		span := ass.ConcreteSpan{Start: field.ValueStart, End: field.ValueEnd}
		index.definitions[field.Value] = append(index.definitions[field.Value], span)
		if symbol, ok := makeSymbol(doc, field.Value, 5, span); ok {
			childrenByLine[field.Line] = append(childrenByLine[field.Line], symbol)
		}
	}
	for _, dialogue := range doc.parsed.Dialogues {
		if field, ok := dialogue.Field("style"); ok {
			name := semantic.DialogueStyleLookupName(field.Value)
			start := field.Start + len(field.Value) - len(strings.TrimLeft(field.Value, " \t*"))
			index.uses = append(index.uses, styleOccurrence{name: name, span: ass.ConcreteSpan{Start: start, End: start + len(name)}})
		}
		tree := ass.ParseConcreteDialogue(dialogue.Text)
		for _, node := range tree.Nodes {
			if node.Block == nil {
				continue
			}
			for _, item := range node.Block.Items {
				if item.Expression == nil {
					continue
				}
				expr := item.Expression
				resolved := server.settings.Options.Targets.Primary.Resolve(*expr, dialogue.Text)
				if resolved.Name != "r" || resolved.Status != renderer.Matched || !expr.Closed() || resolved.Suffix == "" || resolved.Signature != renderer.SignatureVerified {
					continue
				}
				name := strings.TrimSpace(resolved.Suffix)
				start := dialogue.TextStart + expr.HeadSpan.End - len(resolved.Suffix) + len(resolved.Suffix) - len(strings.TrimLeft(resolved.Suffix, " \t"))
				index.uses = append(index.uses, styleOccurrence{name: name, span: ass.ConcreteSpan{Start: start, End: start + len(name)}})
			}
		}
		span := ass.ConcreteSpan{Start: dialogue.LineStart, End: dialogue.TextStart + len(dialogue.Text)}
		if symbol, ok := makeSymbol(doc, fmt.Sprintf("Dialogue %d", dialogue.Line), 19, span); ok {
			childrenByLine[dialogue.Line] = append(childrenByLine[dialogue.Line], symbol)
		}
	}
	symbolCount := 0
	for _, line := range ass.ParseConcreteDocument(doc.text).Lines {
		if line.Header != nil {
			if symbolCount >= 1000 {
				break
			}
			if symbol, ok := makeSymbol(doc, line.Header.RawName, 3, line.Header.NameSpan); ok {
				if len(index.symbols) > 0 {
					previous := &index.symbols[len(index.symbols)-1]
					previous.Range.End, _ = doc.index.Position(line.Span.Start)
				}
				index.symbols = append(index.symbols, symbol)
				symbolCount++
			}
		}
		if line.Record != nil && strings.EqualFold(strings.TrimSpace(line.Record.Key), "Format") {
			if symbol, ok := makeSymbol(doc, "Format", 7, line.Record.KeySpan); ok {
				childrenByLine[line.Number] = append(childrenByLine[line.Number], symbol)
			}
		}
		if len(index.symbols) > 0 && symbolCount < 1000 {
			section := &index.symbols[len(index.symbols)-1]
			for _, child := range childrenByLine[line.Number] {
				if symbolCount >= 1000 {
					break
				}
				section.Children = append(section.Children, child)
				symbolCount++
			}
		}
	}
	if len(index.symbols) > 0 {
		index.symbols[len(index.symbols)-1].Range.End, _ = doc.index.Position(len(doc.text))
	}
	return index
}

func makeSymbol(doc document, name string, kind int, span ass.ConcreteSpan) (documentSymbol, bool) {
	rangeValue, err := doc.index.Range(span.Start, span.End)
	if err != nil {
		return documentSymbol{}, false
	}
	return documentSymbol{Name: name, Kind: kind, Range: rangeValue, SelectionRange: rangeValue}, true
}

func (server *Server) documentSymbols(ctx context.Context, raw jsontext.Value) ([]documentSymbol, error) {
	var params struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
	}
	if raw.Kind() != '{' || json.Unmarshal(raw, &params) != nil || !validURI(params.TextDocument.URI) {
		return nil, fmt.Errorf("invalid document symbols request")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	doc, ok := server.documents.items[params.TextDocument.URI]
	if !ok || ass.Recognize(doc.text).Class == ass.Rejected {
		return []documentSymbol{}, nil
	}
	return server.indexSymbols(doc).symbols, nil
}

func (server *Server) styleLocations(ctx context.Context, raw jsontext.Value, includeDefinitions bool) ([]location, error) {
	results := []location{}
	var params struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
		Position sourcepos.Position `json:"position"`
	}
	if raw.Kind() != '{' || json.Unmarshal(raw, &params) != nil || !validURI(params.TextDocument.URI) {
		return nil, fmt.Errorf("invalid style position request")
	}
	doc, ok := server.documents.items[params.TextDocument.URI]
	if !ok || ass.Recognize(doc.text).Class == ass.Rejected {
		return results, nil
	}
	offset, err := doc.index.Offset(params.Position)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	index := server.indexSymbols(doc)
	var name string
	for candidate, spans := range index.definitions {
		for _, span := range spans {
			if offset >= span.Start && offset < span.End {
				name = candidate
			}
		}
	}
	for _, use := range index.uses {
		if offset >= use.span.Start && offset < use.span.End {
			name = use.name
		}
	}
	if len(index.definitions[name]) != 1 {
		return results, nil
	}
	spans := index.definitions[name]
	if !includeDefinitions {
		spans = nil
		for _, use := range index.uses {
			if use.name == name {
				spans = append(spans, use.span)
			}
		}
	}
	for _, span := range spans {
		rangeValue, err := doc.index.Range(span.Start, span.End)
		if err != nil {
			continue
		}
		results = append(results, location{URI: params.TextDocument.URI, Range: rangeValue})
	}
	return results, nil
}

func (server *Server) definition(ctx context.Context, raw jsontext.Value) ([]location, error) {
	return server.styleLocations(ctx, raw, true)
}

func (server *Server) references(ctx context.Context, raw jsontext.Value) ([]location, error) {
	return server.styleLocations(ctx, raw, false)
}
