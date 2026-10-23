package lsp

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"maps"
	"slices"
	"strings"

	"assx/internal/ass"
	"assx/internal/ass/renderer"
	"assx/internal/ass/spec"
	"assx/internal/sourcepos"
)

type completionItem struct {
	Label            string      `json:"label"`
	Kind             int         `json:"kind"`
	Detail           string      `json:"detail"`
	InsertTextFormat int         `json:"insertTextFormat,omitzero"`
	TextEdit         lspTextEdit `json:"textEdit"`
}

func (server *Server) completion(ctx context.Context, raw jsontext.Value) ([]completionItem, error) {
	items := []completionItem{}
	var params struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
		Position sourcepos.Position `json:"position"`
	}
	if raw.Kind() != '{' || json.Unmarshal(raw, &params) != nil || !validURI(params.TextDocument.URI) {
		return nil, fmt.Errorf("invalid completion position")
	}
	doc, ok := server.documents.items[params.TextDocument.URI]
	if !ok || ass.Recognize(doc.text).Class == ass.Rejected {
		return items, nil
	}
	offset, err := doc.index.Offset(params.Position)
	if err != nil {
		return nil, err
	}
	for _, dialogue := range doc.parsed.Dialogues {
		if field, ok := dialogue.Field("style"); ok && offset >= field.Start && offset <= field.End {
			return server.styleCompletions(doc, field, offset), ctx.Err()
		}
		if offset < dialogue.TextStart || offset > dialogue.TextStart+len(dialogue.Text) {
			continue
		}
		if items, ok := server.completeDialogue(doc, dialogue.Text, dialogue.TextStart, offset, 0); ok {
			return items, ctx.Err()
		}
	}
	return items, ctx.Err()
}

func (server *Server) completeDialogue(doc document, source string, base, offset, depth int) ([]completionItem, bool) {
	if depth > 8 {
		return nil, false
	}
	for _, node := range ass.ParseConcreteDialogue(source).Nodes {
		if node.Block == nil || offset <= base+node.Block.ContentSpan.Start || offset > base+node.Block.ContentSpan.End {
			continue
		}
		for _, item := range node.Block.Items {
			expr := item.Expression
			if expr == nil || offset < base+expr.HeadSpan.Start || offset > base+expr.Span.End {
				continue
			}
			resolved := server.settings.Options.Targets.Primary.Resolve(*expr, source)
			if resolved.Name == "t" && expr.Parenthesized && offset >= base+expr.ContentSpan.Start && offset <= base+expr.ContentSpan.End {
				inner := "{" + source[expr.ContentSpan.Start:expr.ContentSpan.End] + "}"
				if items, ok := server.completeDialogue(doc, inner, base+expr.ContentSpan.Start-1, offset, depth+1); ok {
					return items, true
				}
			}
			if offset > base+expr.HeadSpan.End {
				return nil, false
			}
			prefix := source[expr.HeadSpan.Start : offset-base]
			if resolved.Name == "r" && resolved.Status == renderer.Matched && strings.HasPrefix(prefix, "r") {
				return server.styleNameCompletions(doc, prefix[1:], base+expr.HeadSpan.Start+1, offset), true
			}
			return server.tagCompletions(doc, prefix, base+expr.HeadSpan.Start, offset), true
		}
	}
	return nil, false
}

func (server *Server) tagCompletions(doc document, prefix string, start, end int) []completionItem {
	rangeValue, err := doc.index.Range(start, end)
	if err != nil {
		return []completionItem{}
	}
	profile := server.settings.Options.Targets.Primary
	names := make(map[string]struct{}, len(spec.TagSpecs)+len(spec.VSFilterModTagSpecs))
	for name := range spec.TagSpecs {
		names[name] = struct{}{}
	}
	for name := range spec.VSFilterModTagSpecs {
		names[name] = struct{}{}
	}
	items := []completionItem{}
	for _, name := range slices.Sorted(maps.Keys(names)) {
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		tree := ass.ParseConcreteDialogue("{\\" + name + "}")
		expr := tree.Nodes[0].Block.Items[0].Expression
		resolved := profile.Resolve(*expr, tree.Source)
		if resolved.Name != name || resolved.Status == renderer.Disabled || resolved.Status == renderer.Ignored || resolved.Status == renderer.UnknownName {
			continue
		}
		detail := signatureStatus(resolved)
		if resolved.Status == renderer.Matched {
			detail = "renderer command; signature depends on arguments"
		}
		item := completionItem{Label: "\\" + name, Kind: 3, Detail: detail, TextEdit: lspTextEdit{Range: rangeValue, NewText: name}}
		if server.snippetSupport && (end >= len(doc.text) || doc.text[end] != '(') {
			for _, signature := range profile.Signatures(name) {
				if signature.Form == renderer.Bare || signature.Availability != renderer.SignatureAvailable || signature.Evidence != renderer.SignatureVerified || signature.Count == 0 {
					continue
				}
				args := make([]string, signature.Count)
				for i := range args {
					args[i] = fmt.Sprintf("${%d:arg%d}", i+1, i+1)
				}
				item.InsertTextFormat = 2
				item.TextEdit.NewText = name + "(" + strings.Join(args, ",") + ")"
				break
			}
		}
		items = append(items, item)
	}
	return items
}

func (server *Server) styleCompletions(doc document, field ass.EventField, offset int) []completionItem {
	return server.styleNameCompletions(doc, strings.TrimSpace(doc.text[field.Start:offset]), field.Start, offset)
}

func (server *Server) styleNameCompletions(doc document, prefix string, start, offset int) []completionItem {
	rangeValue, err := doc.index.Range(start, offset)
	if err != nil {
		return []completionItem{}
	}
	definitions := server.indexSymbols(doc).definitions
	items := []completionItem{}
	for _, name := range slices.Sorted(maps.Keys(definitions)) {
		if len(definitions[name]) != 1 || !strings.HasPrefix(name, prefix) {
			continue
		}
		items = append(items, completionItem{Label: name, Kind: 12, Detail: "Style", TextEdit: lspTextEdit{Range: rangeValue, NewText: name}})
	}
	return items
}
