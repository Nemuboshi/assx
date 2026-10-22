# Language server

Run `assx lsp` from an LSP client. The server uses stdio for JSON-RPC messages. It writes protocol frames only to stdout. It writes at most ten document warnings to stderr per session. No server command reads or writes an open document on disk.

## Current capabilities

The server uses LSP 3.17 and UTF-16 positions. It advertises incremental text synchronization, open and close notifications, and save notifications. It supports `initialize`, `initialized`, `shutdown`, `exit`, and `$/cancelRequest`. It does not advertise diagnostics, code actions, hover, completion, or navigation yet.

Each open URI has a versioned text snapshot. A change must have a newer version. The server applies change entries in order. It rejects the complete batch if one range is invalid. It keeps the prior snapshot on failure. A full replacement has no range. A close removes the snapshot. A save does not replace it with disk content. The server accepts URI schemes with a valid absolute URI, including escaped file paths and `untitled:` URIs.

The transport accepts `Content-Length` frames up to 8 MiB. It rejects malformed frames and closes the session. Invalid JSON receives a JSON-RPC parse error. Valid JSON with an invalid request shape receives an invalid-request error. Notifications with invalid document data produce a bounded stderr warning and leave the old snapshot unchanged. Requests before initialization receive a server-not-initialized error. EOF or `exit` before shutdown is an error. Normal `shutdown` then `exit` ends with status 0.

The server reparses and reindexes the complete modified text. It has no incremental ASS syntax mutation. Cancellation of the session closes a closeable input to stop a blocked read. A non-closeable input must unblock its own read. `$/cancelRequest` has no pending work to cancel in this sequential server. L04 will add diagnostics and renderer configuration. Do not configure a client to expect them yet.
