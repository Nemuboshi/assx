# assx VS Code adapter

This adapter starts `assx lsp` over stdio. It does not include the server binary. Install assx separately and put it on `PATH`, or set `assx.serverPath` to its executable path. The adapter uses the repository [MIT license](../../LICENSE).

Run `pnpm install --frozen-lockfile`, `pnpm check`, and `pnpm test` in this directory. For a development session, open this directory in VS Code, run `pnpm build`, and start the Extension Development Host. Open a valid `.ass` file to check diagnostics and hover. Change `assx.primaryRenderer` to check a diagnostic refresh. The language client requests versioned document edits for proven Quick Fixes.

The adapter version is independent of the server version. Check the server feature matrix in [LSP documentation](../../docs/lsp.md) before a release. Do not publish the adapter without a separate release decision.
