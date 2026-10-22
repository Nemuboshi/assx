package lsp

import (
	"bufio"
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"io"

	"assx/internal/sourcepos"
)

type request struct {
	JSONRPC string         `json:"jsonrpc"`
	ID      jsontext.Value `json:"id,omitempty"`
	Method  string         `json:"method"`
	Params  jsontext.Value `json:"params,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      jsontext.Value  `json:"id"`
	Result  *jsontext.Value `json:"result,omitzero"`
	Error   *rpcError       `json:"error,omitzero"`
}

type Server struct {
	documents   Documents
	initialized bool
	shutdown    bool
	warnings    int
	Log         io.Writer
}

// Run owns one sequential JSON-RPC session. It does not read or write documents on disk.
func (server *Server) Run(ctx context.Context, input io.Reader, output io.Writer) error {
	// A closeable input lets cancellation interrupt a blocked frame read.
	if closer, ok := input.(io.ReadCloser); ok {
		stop := context.AfterFunc(ctx, func() { _ = closer.Close() })
		defer stop()
	}
	reader := bufio.NewReaderSize(input, 4096)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		payload, err := readFrame(reader)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err == io.EOF {
			if server.shutdown {
				return nil
			}
			return fmt.Errorf("LSP client disconnected before shutdown")
		}
		if err != nil {
			return err
		}
		if !jsontext.Value(payload).IsValid() {
			if err := server.reply(output, jsontext.Value("null"), nil, &rpcError{-32700, "Parse error"}); err != nil {
				return err
			}
			continue
		}
		var message request
		if err := json.Unmarshal(payload, &message); err != nil || jsontext.Value(payload).Kind() != '{' {
			if err := server.reply(output, jsontext.Value("null"), nil, &rpcError{-32600, "Invalid Request"}); err != nil {
				return err
			}
			continue
		}
		if message.JSONRPC != "2.0" || message.Method == "" || !validID(message.ID) {
			id := message.ID
			if !validID(id) {
				id = jsontext.Value("null")
			}
			if err := server.reply(output, id, nil, &rpcError{-32600, "Invalid Request"}); err != nil {
				return err
			}
			continue
		}
		if message.Method == "exit" && len(message.ID) == 0 {
			if server.shutdown {
				return nil
			}
			return fmt.Errorf("LSP client exited before shutdown")
		}
		if len(message.ID) == 0 {
			server.notify(message)
			continue
		}
		if err := server.handleRequest(output, message); err != nil {
			return err
		}
	}
}

func validID(id jsontext.Value) bool {
	if len(id) == 0 {
		return true
	}
	return id[0] == '"' || id[0] == '-' || (id[0] >= '0' && id[0] <= '9')
}

func (server *Server) handleRequest(output io.Writer, message request) error {
	if message.Method == "initialize" && !server.initialized && !server.shutdown {
		if message.Params.Kind() != '{' {
			return server.reply(output, message.ID, nil, &rpcError{-32602, "Invalid params"})
		}
		server.initialized = true
		capabilities := struct {
			PositionEncoding string `json:"positionEncoding"`
			TextDocumentSync struct {
				OpenClose bool `json:"openClose"`
				Change    int  `json:"change"`
				Save      bool `json:"save"`
			} `json:"textDocumentSync"`
		}{PositionEncoding: "utf-16"}
		capabilities.TextDocumentSync.OpenClose = true
		capabilities.TextDocumentSync.Change = 2
		capabilities.TextDocumentSync.Save = true
		return server.reply(output, message.ID, struct {
			Capabilities any `json:"capabilities"`
		}{Capabilities: capabilities}, nil)
	}
	if !server.initialized {
		return server.reply(output, message.ID, nil, &rpcError{-32002, "Server not initialized"})
	}
	if server.shutdown {
		return server.reply(output, message.ID, nil, &rpcError{-32600, "Server is shut down"})
	}
	if message.Method == "shutdown" {
		server.shutdown = true
		return server.reply(output, message.ID, jsontext.Value("null"), nil)
	}
	return server.reply(output, message.ID, nil, &rpcError{-32601, "Method not found"})
}

func (server *Server) reply(output io.Writer, id jsontext.Value, result any, failure *rpcError) error {
	response := response{JSONRPC: "2.0", ID: id, Error: failure}
	if failure == nil {
		value, err := json.Marshal(result)
		if err != nil {
			return err
		}
		raw := jsontext.Value(value)
		response.Result = &raw
	}
	message, err := json.Marshal(&response)
	if err != nil {
		return err
	}
	return writeFrame(output, message)
}

func (server *Server) notify(message request) {
	if !server.initialized || server.shutdown {
		return
	}
	switch message.Method {
	case "initialized", "$/cancelRequest":
		return
	case "textDocument/didOpen":
		var params struct {
			TextDocument *struct {
				URI     string  `json:"uri"`
				Version int     `json:"version"`
				Text    *string `json:"text"`
			} `json:"textDocument"`
		}
		if err := json.Unmarshal(message.Params, &params); err != nil || params.TextDocument == nil || params.TextDocument.Text == nil {
			server.warn(fmt.Errorf("invalid didOpen parameters"))
			return
		}
		server.warn(server.documents.Open(params.TextDocument.URI, params.TextDocument.Version, *params.TextDocument.Text))
	case "textDocument/didChange":
		var params struct {
			TextDocument *struct {
				URI     string `json:"uri"`
				Version int    `json:"version"`
			} `json:"textDocument"`
			ContentChanges []struct {
				Range *struct {
					Start *sourcepos.Position `json:"start"`
					End   *sourcepos.Position `json:"end"`
				} `json:"range"`
				RangeLength *int    `json:"rangeLength"`
				Text        *string `json:"text"`
			} `json:"contentChanges"`
		}
		if err := json.Unmarshal(message.Params, &params); err != nil || params.TextDocument == nil {
			server.warn(fmt.Errorf("invalid didChange parameters"))
			return
		}
		changes := make([]Change, 0, len(params.ContentChanges))
		for _, entry := range params.ContentChanges {
			if entry.Text == nil || (entry.Range != nil && (entry.Range.Start == nil || entry.Range.End == nil)) {
				server.warn(fmt.Errorf("invalid didChange entry"))
				return
			}
			change := Change{Text: *entry.Text, RangeLength: entry.RangeLength}
			if entry.Range != nil {
				change.Range = &sourcepos.Range{Start: *entry.Range.Start, End: *entry.Range.End}
			}
			changes = append(changes, change)
		}
		server.warn(server.documents.Change(params.TextDocument.URI, params.TextDocument.Version, changes))
	case "textDocument/didClose":
		var params struct {
			TextDocument struct {
				URI string `json:"uri"`
			} `json:"textDocument"`
		}
		if err := json.Unmarshal(message.Params, &params); err != nil || !validURI(params.TextDocument.URI) {
			server.warn(fmt.Errorf("invalid didClose parameters"))
			return
		}
		server.documents.Close(params.TextDocument.URI)
	case "textDocument/didSave":
		// The open buffer stays authoritative; save does not read disk.
	}
}

func (server *Server) warn(err error) {
	if err == nil || server.Log == nil || server.warnings >= 10 {
		return
	}
	server.warnings++
	_, _ = fmt.Fprintf(server.Log, "assx lsp: %v\n", err)
}
