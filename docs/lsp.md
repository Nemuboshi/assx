# Language server

Run `assx lsp` from an LSP client. The server uses stdio for JSON-RPC messages. It writes protocol frames only to stdout. It writes at most ten document warnings to stderr per session. No server command reads or writes an open document on disk.

## Editors and release

The [VS Code adapter](../editors/vscode) starts the configured `assx lsp` executable. Install its Node dependencies only to build the adapter. Set `assx.serverPath` to an executable path if `assx` is not on `PATH`. Set `assx.primaryRenderer`, `assx.compatibilityRenderers`, and the two `assx.vsfilterMod` flags in VS Code settings. Renderer changes refresh open documents. Reload VS Code after a server path change. The adapter does not download a server or parse ASS.

Copy the [Neovim example](../editors/neovim/init.lua) into Neovim 0.11+ configuration or the [Helix example](../editors/helix/languages.toml) into Helix language configuration. Both require `assx` on `PATH`. The examples configure diagnostics and basic language features. They do not certify Quick Fix versioned-edit support in those clients. Refer to the [Neovim LSP configuration](https://neovim.io/doc/user/lsp/) and [Helix language configuration](https://docs.helix-editor.com/languages.html) for client settings.

The VS Code adapter has its own `0.x` version. It requires the assx LSP 3.17 behavior documented here. Build the server with `go build ./cmd/assx`. Build and check the adapter with `pnpm install --frozen-lockfile`, `pnpm check`, and `pnpm test` in `editors/vscode`. Before a release, run both check suites, test a valid ASS file in the target editor, verify a proven Quick Fix and renderer refresh, and package the server binary separately from the adapter. Marketplace publication is a separate decision.

## Current capabilities

The server uses LSP 3.17 and UTF-16 positions. It advertises incremental text synchronization, open and close notifications, and save notifications. It supports `initialize`, `initialized`, `shutdown`, `exit`, and `$/cancelRequest`.

The server publishes renderer-aware diagnostics for in-memory ASS text. It clears them on close or rejected input. `textDocument/codeAction` is advertised only when the client supports `workspace.workspaceEdit.documentChanges`. Each Quick Fix is proved on demand for the current text, version, renderer targets, build gates, and rule policy. A tentative or rejected document has no Quick Fix. The editor applies a versioned edit; the server does not write the file.

Hover and signature help use the pinned renderer profile. A conditional, inferred, rejected, or unresolved signature is not called verified. The server also advertises document symbols, in-document Style definitions and references, and contextual completion. Navigation does not search other files or rename Styles.

Each open URI has a versioned text snapshot. A change must have a newer version. The server applies change entries in order. It rejects the complete batch if one range is invalid. It keeps the prior snapshot on failure. A full replacement has no range. A close removes the snapshot. A save does not replace it with disk content. The server accepts URI schemes with a valid absolute URI, including escaped file paths and `untitled:` URIs.

The transport accepts `Content-Length` frames up to 8 MiB. It rejects malformed frames and closes the session. Invalid JSON receives a JSON-RPC parse error. Valid JSON with an invalid request shape receives an invalid-request error. Notifications with invalid document data produce a bounded stderr warning and leave the old snapshot unchanged. Requests before initialization receive a server-not-initialized error. EOF or `exit` before shutdown is an error. Normal `shutdown` then `exit` ends with status 0.

The server reparses and reindexes the complete modified text. It has no incremental ASS syntax mutation. Cancellation of the session closes a closeable input to stop a blocked read. A non-closeable input must unblock its own read. `$/cancelRequest` has no pending work to cancel in this sequential server. Diagnostic work does not run SafeFix proof.

## Settings

Pass settings under `assx` in `initializationOptions` or `workspace/didChangeConfiguration`. `primaryRenderer` accepts `libass`, `xy-vsfilter`, or `vsfiltermod`. `compatibilityRenderers` is an optional list of the same names. `vsfilterMod.mod` and `vsfilterMod.lua` accept `unknown`, `disabled`, or `enabled`. `rules` maps rule IDs to `disabled` and optional `severity`. `forceASS` allows an empty buffer without overriding foreign-content rejection. The default is libass validity plus xy-VSFilter compatibility. Invalid settings keep the last valid selection and produce a bounded warning on stderr.

## Performance measurement

Run `go test ./internal/lsp -run '^$' -bench '^BenchmarkLSP' -benchtime=1x -benchmem` on one machine and toolchain. A one-iteration Windows/amd64 Go 1.27.0 measurement on an Intel i5-9300H measured 0.89 ms and 315 KB for ten dialogues. The 10,000-dialogue case measured 205 ms and 306 MB. A rapid edit and hover measured 79 microseconds and 16 KB. These numbers are a baseline, not a performance target. The server reparses each changed document and does not keep an unverified incremental cache.
