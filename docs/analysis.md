# Snapshot analysis

Issue: [#42](https://github.com/Nemuboshi/assx/issues/42)

`internal/analysis` owns one analysis request. It keeps the snapshot, the
selected targets and rule policy, and the `internal/lint` session. Font
discovery, source decoding, encoding, reports, and flags stay with their
current owners. The CLI adapts this package.

The package direction is fixed: `cmd/assx` calls `internal/analysis`,
`internal/analysis` calls `internal/lint`, `internal/lint` calls
`internal/semantic` and `internal/ass`. `internal/analysis` holds no ASS
syntax, no rule policy, and no edit code. It does not implement input
classification (issue #40) or an LSP transport.

## Types

```go
type Snapshot struct { /* private URI, UTF-8 text, optional version */ }
type Options struct {
    Targets lint.AnalysisTargets
    Rules   lint.RulePolicy
}
type FindingID struct { /* private result identity and finding ordinal */ }
type Finding struct {
    ID              FindingID
    Diagnostic      lint.Diagnostic
    HasFixCandidate bool
}
type Result struct { /* private snapshot, options, lint session */ }
```

## Requests

`NewSnapshot` accepts an opaque URI or path. It rejects an empty URI and
text that is not valid UTF-8. It copies the optional version, so a later
change to the caller value cannot change the snapshot. A snapshot holds no
parsed form and no file handle.

`Analyze` parses the snapshot text, runs discovery through `lint.Discover`,
and returns a `Result`. Discovery runs every lint pass and records each fix
candidate. It never constructs a `FixProof`, a proof label, or an executable
SafeFix edit. `Findings` returns copies in report order. Mutating a returned
finding cannot change result state.

Empty `Options` select the current default targets and the default rule
catalog. An explicit libass-only selection must pass a non-nil
compatibility slice, because the libass profile is the zero `renderer.Profile`.
`Analyze` copies the targets and the rule policy into the result and rejects
an invalid policy. It never mutates the global rule catalog. The policy
controls findings and authorization. It does not disable required semantic
state transitions, and it applies before cross-rule overlap suppression: a
disabled ASS013 does not suppress enabled ASS006 candidates.

## Deferred proof

`ProveFixes` takes finding IDs from this result and proves them as one batch
in report order. It clones the stored candidates, so a later call cannot see
caller edits, and a failed batch cannot damage the result. Rule IDs and
finding IDs stay separate. It rejects a foreign ID, an unknown ID, a
duplicate ID, and a non-candidate ID. A candidate that cannot be proved
returns its diagnostic without automatic edits, as the eager path does.

`Diagnostics` proves every candidate and merges the result into the pending
findings. Its output equals `lint.AnalyzeDocumentForTargets` for the same
snapshot, targets, and policy. The CLI uses it for its unchanged bytes.

## Application

`ApplyFixes` returns changed UTF-8 text and the applied diagnostic count. It
counts applied diagnostics, not individual `TextEdit` values. It performs no
file access.

It applies to the session source of the snapshot, under the result targets
and rule policy. Before it writes anything it checks that every SafeFix
finding matches a stored session candidate, re-verifies each proof against
the current source, and keeps the combined-group check for multi-group
input. It rejects changed edits, foreign candidates, and incomplete proof
groups. UnsafeFix findings apply only with `includeUnsafe`, as today.

## Cancellation

Every request method honors its `context.Context`. Discovery checks the
context at entry and between dialogue units; one dialogue is the atomic
unit, so a canceled `Analyze` returns no result. Proof checks the context
before each proof build; a canceled `ProveFixes` returns no diagnostics, so
no partial proof output is visible. A canceled request never damages an
existing `Result`: a later request on the same result still works.

## CLI adapter

`cmd/assx` builds a snapshot from the decoded source text and passes its
parsed targets as explicit options. Reports, `--fix` file writes, the
lossless round-trip refusal, summaries, and exit status are unchanged. The
default CLI runs `Analyze` plus `Diagnostics`, which reproduces the eager
diagnostic bytes.
