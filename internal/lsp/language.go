package lsp

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"strings"

	"assx/internal/ass"
	"assx/internal/ass/renderer"
	"assx/internal/lint"
	"assx/internal/sourcepos"
)

type commandLocation struct {
	expression ass.ConcreteExpression
	source     string
	base       int
	offset     int
}

type hoverResult struct {
	Contents struct {
		Kind  string `json:"kind"`
		Value string `json:"value"`
	} `json:"contents"`
	Range sourcepos.Range `json:"range"`
}

type signatureInformation struct {
	Label         string `json:"label"`
	Documentation string `json:"documentation,omitempty"`
	Parameters    []struct {
		Label string `json:"label"`
	} `json:"parameters"`
}

type signatureHelpResult struct {
	Signatures      []signatureInformation `json:"signatures"`
	ActiveSignature int                    `json:"activeSignature"`
	ActiveParameter int                    `json:"activeParameter"`
}

func (server *Server) commandAt(raw jsontext.Value) (document, commandLocation, bool, error) {
	var params struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
		Position sourcepos.Position `json:"position"`
	}
	if raw.Kind() != '{' || json.Unmarshal(raw, &params) != nil || !validURI(params.TextDocument.URI) {
		return document{}, commandLocation{}, false, fmt.Errorf("invalid text document position")
	}
	doc, ok := server.documents.items[params.TextDocument.URI]
	if !ok || ass.Recognize(doc.text).Class == ass.Rejected {
		return document{}, commandLocation{}, false, nil
	}
	offset, err := doc.index.Offset(params.Position)
	if err != nil {
		return document{}, commandLocation{}, false, err
	}
	for _, dialogue := range doc.parsed.Dialogues {
		if offset < dialogue.TextStart || offset >= dialogue.TextStart+len(dialogue.Text) {
			continue
		}
		if found, ok := findCommand(dialogue.Text, dialogue.TextStart, offset, server.settings.Options.Targets.Primary, 0); ok {
			found.offset = offset
			return doc, found, true, nil
		}
	}
	return doc, commandLocation{}, false, nil
}

func findCommand(source string, base, offset int, profile renderer.Profile, depth int) (commandLocation, bool) {
	if depth > 8 {
		return commandLocation{}, false
	}
	for _, node := range ass.ParseConcreteDialogue(source).Nodes {
		if node.Block == nil {
			continue
		}
		for _, item := range node.Block.Items {
			expr := item.Expression
			if expr == nil || offset < base+expr.Span.Start || offset > base+expr.Span.End {
				continue
			}
			if offset == base+expr.Span.End && expr.Closed() {
				continue
			}
			resolved := profile.Resolve(*expr, source)
			if resolved.Name == "t" && expr.Parenthesized && offset >= base+expr.ContentSpan.Start && offset <= base+expr.ContentSpan.End {
				inner := "{" + source[expr.ContentSpan.Start:expr.ContentSpan.End] + "}"
				if nested, ok := findCommand(inner, base+expr.ContentSpan.Start-1, offset, profile, depth+1); ok {
					return nested, true
				}
			}
			return commandLocation{expression: *expr, source: source, base: base}, true
		}
	}
	return commandLocation{}, false
}

func signatureStatus(result renderer.Result) string {
	if !result.Closed {
		return "unresolved signature (incomplete expression)"
	}
	switch result.Status {
	case renderer.UnknownName:
		return "unknown name"
	case renderer.Conditional:
		return "conditional build feature"
	case renderer.Disabled:
		return "disabled"
	case renderer.Ignored:
		return "ignored form"
	}
	switch result.Signature {
	case renderer.SignatureVerified:
		return "verified signature"
	case renderer.SignatureInferred:
		return "inferred signature"
	case renderer.SignatureRejected:
		return "rejected signature"
	default:
		return "unresolved signature"
	}
}

func (server *Server) hover(ctx context.Context, raw jsontext.Value) (*hoverResult, error) {
	doc, location, ok, err := server.commandAt(raw)
	if err != nil {
		return nil, err
	}
	if !ok {
		return server.fieldHover(ctx, doc, raw)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	profiles := server.settings.Options.Targets.Profiles()
	var lines []string
	for _, profile := range profiles {
		resolved := profile.Resolve(location.expression, location.source)
		name := resolved.Name
		if name == "" {
			name = location.expression.Head
		}
		line := fmt.Sprintf("%s: \\%s - %s", lint.ProfileScope(profile), name, signatureStatus(resolved))
		if resolved.Shadowed != "" {
			line += "; shadows \\" + resolved.Shadowed
		}
		if resolved.Citation != "" {
			line += "; evidence: " + resolved.Citation
		}
		lines = append(lines, line)
	}
	rangeValue, err := doc.index.Range(location.base+location.expression.HeadSpan.Start, location.base+location.expression.HeadSpan.End)
	if err != nil {
		return nil, err
	}
	hover := &hoverResult{Range: rangeValue}
	hover.Contents.Kind = "plaintext"
	hover.Contents.Value = strings.Join(lines, "\n")
	return hover, nil
}

func (server *Server) fieldHover(ctx context.Context, doc document, raw jsontext.Value) (*hoverResult, error) {
	if doc.text == "" {
		return nil, nil
	}
	var params struct {
		Position sourcepos.Position `json:"position"`
	}
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, err
	}
	offset, err := doc.index.Offset(params.Position)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	index := server.indexSymbols(doc)
	for _, use := range index.uses {
		if offset >= use.span.Start && offset < use.span.End && len(index.definitions[use.name]) == 1 {
			return plainHover(doc, use.span, "Style reference: "+use.name)
		}
	}
	section := ""
	for _, line := range ass.ParseConcreteDocument(doc.text).Lines {
		if line.Header != nil {
			section = strings.ToLower(strings.TrimSpace(line.Header.RawName))
		}
		if section != "script info" || line.Record == nil || offset < line.Record.KeySpan.Start || offset >= line.Record.KeySpan.End {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(line.Record.Key))
		if _, known := doc.parsed.Headers[key]; known {
			return plainHover(doc, line.Record.KeySpan, "Script field: "+line.Record.Key)
		}
	}
	return nil, nil
}

func plainHover(doc document, span ass.ConcreteSpan, value string) (*hoverResult, error) {
	rangeValue, err := doc.index.Range(span.Start, span.End)
	if err != nil {
		return nil, err
	}
	hover := &hoverResult{Range: rangeValue}
	hover.Contents.Kind = "plaintext"
	hover.Contents.Value = value
	return hover, nil
}

func (server *Server) signatureHelp(ctx context.Context, raw jsontext.Value) (*signatureHelpResult, error) {
	_, location, ok, err := server.commandAt(raw)
	if err != nil || !ok {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	expr := location.expression
	if !expr.Parenthesized || location.offset <= location.base+expr.Open || (expr.Close >= 0 && location.offset > location.base+expr.Close) {
		return nil, nil
	}
	primary := server.settings.Options.Targets.Primary
	resolved := primary.Resolve(expr, location.source)
	if resolved.Name == "" || resolved.Status != renderer.Matched {
		return nil, nil
	}
	result := &signatureHelpResult{Signatures: []signatureInformation{}}
	for _, signature := range primary.Signatures(resolved.Name) {
		if signature.Form == renderer.Bare || signature.Availability == renderer.SignatureUnavailable {
			continue
		}
		parts := make([]string, signature.Count)
		info := signatureInformation{Parameters: make([]struct {
			Label string `json:"label"`
		}, signature.Count)}
		for i := range parts {
			parts[i] = fmt.Sprintf("arg%d", i+1)
			info.Parameters[i].Label = parts[i]
		}
		info.Label = "\\" + resolved.Name + "(" + strings.Join(parts, ", ") + ")"
		switch signature.Availability {
		case renderer.SignatureConditional:
			info.Documentation = "Conditional build feature; not verified."
		default:
			info.Documentation = signatureEvidence(signature.Evidence)
		}
		if signature.Citation != "" {
			info.Documentation += " Evidence: " + signature.Citation
		}
		if signature.Count == len(resolved.Args) && signature.Availability == renderer.SignatureAvailable {
			result.ActiveSignature = len(result.Signatures)
		}
		result.Signatures = append(result.Signatures, info)
	}
	if len(result.Signatures) == 0 {
		return nil, nil
	}
	// Count delimiters before the cursor in the expression's source coordinates.
	for _, comma := range expr.Commas {
		if location.base+comma < location.offset {
			result.ActiveParameter++
		}
	}
	if count := len(result.Signatures[result.ActiveSignature].Parameters); count > 0 {
		result.ActiveParameter = min(result.ActiveParameter, count-1)
	}
	return result, nil
}

func signatureEvidence(status renderer.SignatureStatus) string {
	switch status {
	case renderer.SignatureVerified:
		return "Source-verified signature."
	case renderer.SignatureInferred:
		return "Inferred signature; not verified."
	default:
		return "Unresolved signature."
	}
}
