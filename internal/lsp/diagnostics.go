package lsp

import (
	"context"
	"crypto/sha256"
	"encoding/json/v2"
	"fmt"
	"io"

	"assx/internal/analysis"
	"assx/internal/ass"
	"assx/internal/lint"
	"assx/internal/sourcepos"
)

type protocolDiagnostic struct {
	Range    sourcepos.Range `json:"range"`
	Severity int             `json:"severity"`
	Code     string          `json:"code"`
	Source   string          `json:"source"`
	Message  string          `json:"message"`
	Data     string          `json:"data,omitempty"`
}

func (server *Server) identityPrefix(doc document) string {
	return fmt.Sprintf("%x:%d:%d", sha256.Sum256([]byte(doc.text)), doc.version, server.settingsGeneration)
}

func (server *Server) publish(ctx context.Context, output io.Writer, uri string) error {
	params := struct {
		URI         string               `json:"uri"`
		Version     *int                 `json:"version,omitzero"`
		Diagnostics []protocolDiagnostic `json:"diagnostics"`
	}{URI: uri, Diagnostics: []protocolDiagnostic{}}
	if doc, ok := server.documents.items[uri]; ok {
		params.Version = new(doc.version)
		identity := server.identityPrefix(doc)
		recognition := ass.Recognize(doc.text)
		analyze := recognition.Class != ass.Rejected
		if !analyze && server.settings.ForceASS {
			analyze = true
			for _, evidence := range recognition.Evidence {
				if evidence.ID != "input.no-ass-structure" {
					analyze = false
					break
				}
			}
		}
		if analyze {
			version := int64(doc.version)
			snapshot, err := analysis.NewSnapshot(uri, doc.text, &version)
			if err != nil {
				return err
			}
			result, err := analysis.Analyze(ctx, snapshot, server.settings.Options)
			if err != nil {
				return err
			}
			for ordinal, finding := range result.Findings() {
				diagnostic := finding.Diagnostic
				if diagnostic.SourceSpan == nil {
					continue
				}
				span, err := doc.index.Range(diagnostic.SourceSpan.Start, diagnostic.SourceSpan.End)
				if err != nil {
					server.warn(err)
					continue
				}
				severity := 3
				switch diagnostic.Severity {
				case lint.Error:
					severity = 1
				case lint.Warning:
					severity = 2
				}
				message := diagnostic.Title
				if diagnostic.Detail != "" {
					message += ": " + diagnostic.Detail
				}
				if diagnostic.Renderer != "" {
					message += " [" + diagnostic.Renderer + "]"
				}
				params.Diagnostics = append(params.Diagnostics, protocolDiagnostic{
					Range: span, Severity: severity, Code: diagnostic.ID, Source: "assx", Message: message,
					Data: fmt.Sprintf("%s:%d", identity, ordinal),
				})
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	message, err := json.Marshal(struct {
		JSONRPC string `json:"jsonrpc"`
		Method  string `json:"method"`
		Params  any    `json:"params"`
	}{JSONRPC: "2.0", Method: "textDocument/publishDiagnostics", Params: params})
	if err != nil {
		return err
	}
	return writeFrame(output, message)
}
