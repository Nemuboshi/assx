# State machines

This page states which state machines are explicit, which implicit state is
accepted, and which code needs migration.

## Policy

- Use one state value when two flags can form an illegal combination, or when
  a lifecycle has more than two phases.
- Keep one bool when it gates one branch.
- Keep independent bits when the flags are facts, not phases.
- Put transitions in one owner function. A test asserts that an illegal
  transition changes nothing.

## Explicit machines

### LSP session

Owner: `internal/lsp/session.go`.

States:

- `serverUninitialized`
- `serverInitialized`
- `serverShutdown`

Transitions:

- `initialize` moves uninitialized to initialized.
- `shutdown` moves initialized to shutdown.
- `exit` ends the run only after shutdown.
- Requests before initialization and after shutdown get an error.
- Notifications after shutdown are ignored.

The previous two booleans could represent an illegal state. The enum removes
that state.

### Document projection cursor

Owner: `internal/ass/renderer/document_cursor.go`.

State:

- current section
- Style Format
- Event Format
- ScriptType v4.00+ flag

Transitions:

- `enter` changes the section.
- `setStyleFormat` and `setEventFormat` replace one format and apply the
  Actor-to-Name alias.
- `resetStyleFormat` restores the dialect default path.

The default document pass, the layout resolver, and the view projector use the
same cursor. Each pass keeps its own section and exact-key policy.

### Tag transitions

Owner: `internal/semantic/effective.go`. Test: `tag_branch_test.go`.

`consumeTag` has branches that end the fold. Each one returns a
`tagTransition` value instead of calling the effect methods inline, and
`applyTransition` is the only place that applies it. The value has independent
fields, not one phase:

- `live` marks current owners as used by source text.
- `invalidate` selects the scope: nothing, listed slots, all slots from the
  current position, or the malformed-tag slots.
- `uncertain` marks first-wins latches that an unsupported form might claim.
- `barrier` revokes SafeFix proofs and records why.

A branch that continues into the assignment fold applies its barrier before
the fold, because the fold's candidate proofs read `proofsDisabled`. The table
test pins the effect set of every branch: uncertainty reason, barrier, ignored,
applied, known, and the slots left known after the tag.

## Accepted implicit state

These machines stay as they are. An enum would add no safety.

- `semantic.slotStore`: per-slot `present`, `known`, owner, uncertain, and
  latch bits. They are independent facts. The store methods already own the
  transitions.
- `semantic.evaluator`: `baseValid`, `proofsDisabled`, and
  `collisionDisabled` are independent facts. They describe proof and
  collision safety, not one phase. The branch effects that set them are
  collected in `tagTransition`, but the flags stay separate.
- `ass.ParseConcreteDocument` and `ParseConcreteDialogue`: local `section`,
  offsets, and parenthesis depth. They are parser cursors, not lifecycle
  states.
- `ass.ParseDrawing`: `current *DrawingCommand` is a local accumulator.
- `ass.RecognizeParsed`: `credible`, `tentative`, and `foreign` are evidence
  flags. The final `Classification` is the explicit result.
- `lint.styleRunCollector`: `stopped` gates one branch. It is not a phase.
- `resolvedKaraokeCursor` and `resolvedDrawingCollector`: local observers.
- `fontcheck.Catalog` and `semantic.DialoguePreparations`: cache validity is
  data, not a phase.
- `cmd/assx.scanProgress`: channel close and `sync.Once` own the lifecycle.

## Migration order

1. **Proof lifecycle.** `analysis.Result`, `lint.Session`, and `proofBatch`
   already use opaque IDs and source checks. Add a phase only if a new caller
   needs to observe it.

## Verification

- Add a transition test for each explicit state value.
- Run `go test ./...` and the pinned CLI contract test.
- Do not change diagnostics or SafeFix authorization in a state-only change.
