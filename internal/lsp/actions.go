package lsp

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"

	"assx/internal/analysis"
	"assx/internal/ass"
	"assx/internal/lint"
	"assx/internal/sourcepos"
)

type codeAction struct {
	Title string `json:"title"`
	Kind  string `json:"kind"`
	Edit  struct {
		DocumentChanges []textDocumentEdit `json:"documentChanges"`
	} `json:"edit"`
}

type textDocumentEdit struct {
	TextDocument struct {
		URI     string `json:"uri"`
		Version int    `json:"version"`
	} `json:"textDocument"`
	Edits []lspTextEdit `json:"edits"`
}

type lspTextEdit struct {
	Range   sourcepos.Range `json:"range"`
	NewText string          `json:"newText"`
}

func (server *Server) codeActions(ctx context.Context, raw jsontext.Value) ([]codeAction, error) {
	actions := []codeAction{}
	var params struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
		Range   sourcepos.Range `json:"range"`
		Context struct {
			Diagnostics []struct {
				Code  string          `json:"code"`
				Data  string          `json:"data"`
				Range sourcepos.Range `json:"range"`
			} `json:"diagnostics"`
		} `json:"context"`
	}
	if !server.editSupport || raw.Kind() != '{' {
		return actions, fmt.Errorf("code actions are not supported")
	}
	if err := json.Unmarshal(raw, &params); err != nil || !validURI(params.TextDocument.URI) {
		return actions, fmt.Errorf("invalid code action parameters")
	}
	doc, ok := server.documents.items[params.TextDocument.URI]
	if !ok || ass.Recognize(doc.text).Class != ass.Recognized {
		return actions, nil
	}
	start, err := doc.index.Offset(params.Range.Start)
	if err != nil {
		return actions, err
	}
	end, err := doc.index.Offset(params.Range.End)
	if err != nil || end < start {
		return actions, fmt.Errorf("invalid code action range")
	}
	version := int64(doc.version)
	snapshot, err := analysis.NewSnapshot(params.TextDocument.URI, doc.text, &version)
	if err != nil {
		return actions, err
	}
	result, err := analysis.Analyze(ctx, snapshot, server.settings.Options)
	if err != nil {
		return actions, err
	}
	identity := server.identityPrefix(doc)
	findings := result.Findings()
	for ordinal, finding := range findings {
		if !finding.HasFixCandidate || finding.Diagnostic.SourceSpan == nil {
			continue
		}
		span := finding.Diagnostic.SourceSpan
		if span.Start > end || span.End < start {
			continue
		}
		rangeValue, err := doc.index.Range(span.Start, span.End)
		if err != nil {
			continue
		}
		matched := false
		for _, client := range params.Context.Diagnostics {
			if client.Data == fmt.Sprintf("%s:%d", identity, ordinal) && client.Code == finding.Diagnostic.ID && client.Range == rangeValue {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		proved, err := result.ProveFixes(ctx, []analysis.FindingID{finding.ID})
		if err != nil {
			return actions, err
		}
		if len(proved) != 1 || proved[0].FixSafety != lint.SafeFix || len(proved[0].Edits) == 0 {
			continue
		}
		fixed, count, err := result.ApplyFixes(ctx, proved, false)
		if err != nil || count != 1 || fixed == doc.text {
			continue
		}
		action := codeAction{Title: "Apply safe fix: " + proved[0].Title, Kind: "quickfix"}
		change := textDocumentEdit{}
		change.TextDocument.URI = params.TextDocument.URI
		change.TextDocument.Version = doc.version
		valid := true
		for _, sourceEdit := range proved[0].Edits {
			editRange, err := doc.index.EditRange(sourceEdit)
			if err != nil {
				valid = false
				break
			}
			change.Edits = append(change.Edits, lspTextEdit{editRange, sourceEdit.Replacement})
		}
		if !valid {
			continue
		}
		action.Edit.DocumentChanges = append(action.Edit.DocumentChanges, change)
		actions = append(actions, action)
	}
	return actions, ctx.Err()
}
