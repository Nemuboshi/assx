# Lint pipeline

This page states how one lint run moves from source bytes to reported
diagnostics. The package pages define the API of each stage.

## Layers

| Layer | Package | Job |
| --- | --- | --- |
| Source | `internal/ass` | Decode, recognize input, parse the concrete tree, keep spans |
| Dispatch | `internal/ass/spec`, `internal/ass/renderer` | Select a profile, project the document, resolve tag signatures |
| Evaluation | `internal/semantic` | Evaluate effective state, notify observers, compare renderers |
| Policy | `internal/lint` | Apply rule policy, group edit candidates, prove fixes |
| Orchestration | `internal/analysis` | Own the snapshot, the options, and the deferred proof |
| Presentation | `internal/lintreport`, `internal/report` | Render pretty, plain, or JSON output and set the exit code |

A lower layer never imports a higher layer. `internal/lint` uses `internal/ass`
and `internal/semantic`; it does not own parser or state logic.

## One run

```mermaid
flowchart TD
    A[Source bytes] --> B[ass.DecodeSource]
    B --> C[ass.Recognize]
    C -->|Rejected| X[Exit 2]
    C --> D[analysis.NewSnapshot]
    D --> E[analysis.Analyze]
    E --> F[ass.Parse]
    F --> G[lint.Discover]
    G --> H[PrepareDocument: one view per target]
    H --> I[Document pass: headers, keywords, styles, event fields]
    I --> J[Dialogue pass: semantic evaluation with observers]
    J --> K[Compatibility pass: pairwise renderer results]
    K --> L[Scope, sort, dedupe, suppress overlaps]
    L --> M[Candidates: SafeFix with edits]
    M --> N[lint.Session.ProveFixes]
    N --> O[lintreport view]
    O --> P[pretty, plain, or JSON]
    P --> Q[Exit 0 or 1]
```

`lint.Discover` records candidates and never builds a proof. It prepares the
document once, so validation, compatibility, and the redundant-Style collector
share one projected view and one resolved expression per profile.

## Deferred proof

The split exists because proof is expensive and most runs never apply an edit.

```mermaid
sequenceDiagram
    participant CLI
    participant Analysis
    participant Lint
    CLI->>Analysis: Analyze(snapshot, options)
    Analysis->>Lint: Discover(ctx, doc, targets, policy)
    Lint-->>Analysis: Session with findings and candidates
    Analysis->>Lint: ProveFixes(ctx, handles)
    Lint->>Lint: rebuild each target view from the source
    Lint->>Lint: compare interpretation before and after the edit group
    Lint-->>Analysis: authorized diagnostics with FixProof
    Analysis-->>CLI: diagnostics in report order
```

A `SafeFix` label without a proof carries no edits. A finding that fails proof
keeps its observation and loses the fix. `--unsafe-fix` adds the unsafe class;
it does not bypass proof.

## CLI path

```mermaid
sequenceDiagram
    participant Main as cmd/assx
    participant Analysis
    participant Lint
    participant File
    Main->>File: read, symlink resolve
    Main->>Main: decode, recognize, parse flags
    Main->>Analysis: Analyze
    Analysis->>Lint: Discover
    Main->>Analysis: Diagnostics
    Analysis->>Lint: ProveFixes
    alt --fix or --unsafe-fix
        Main->>Analysis: ApplyFixes
        Main->>Main: verify lossless round trip
        Main->>File: writeAtomically
        Main->>Analysis: Analyze again on the fixed text
    end
    Main->>Main: lint.CountFixes, render, exit
```

The CLI refuses a write when the file does not round-trip byte for byte. A
`Tentative` recognition result permits analysis and blocks every write.

## Editor path

The language server uses the same orchestration with a different proof point.

```mermaid
sequenceDiagram
    participant Editor
    participant Server as internal/lsp
    participant Analysis
    participant Lint
    Editor->>Server: initialize
    Server->>Server: settings become analysis.Options
    Editor->>Server: textDocument/didOpen or didChange
    Server->>Analysis: Analyze(snapshot with version)
    Analysis->>Lint: Discover
    Server-->>Editor: publishDiagnostics from Findings, no proof
    Editor->>Server: textDocument/codeAction
    Server->>Analysis: Analyze again
    Server->>Analysis: ProveFixes one finding
    Analysis->>Lint: prove the edit group
    Server->>Analysis: ApplyFixes in memory
    Server-->>Editor: textDocumentEdit
```

The diagnostic `Data` field holds `identity:ordinal`. The server accepts a code
action only when the client sends back that identity, the rule ID, and the
range. A stale or foreign identity yields no action.

## Invariants

- One profile produces one projected view. A renderer-scoped pass strips fixes.
- A location may carry different rules, but an identical observation appears once.
- Output order is by line, then column, in every format.
- SafeFix requires one proof for the complete edit group across every target.
- Equal modeled state does not prove equal frames. `tools/rendercheck` checks frames.
