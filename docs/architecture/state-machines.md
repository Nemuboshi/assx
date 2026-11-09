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

## Accepted implicit state

These machines stay as they are. An enum would add no safety.

- `semantic.slotStore`: per-slot `present`, `known`, owner, uncertain, and
  latch bits. They are independent facts. The store methods already own the
  transitions.
- `semantic.evaluator`: `baseValid`, `proofsDisabled`, and
  `collisionDisabled` are independent facts. They describe proof and
  collision safety, not one phase.
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

1. **Document projection cursor.** Three passes derive section and Format
   state: `documentFromConcrete`, `ResolveDocumentLayouts`, and
   `projectDocument`. Extract one cursor that answers the current section and
   the current Style/Event format. Keep each pass's interpretation in its own
   code. This removes drift between the three passes.
2. **Tag fold transitions.** The evaluator has many early returns. Extract one
   transition result only after a table test covers every branch. Do not
   merge independent flags into one enum.
3. **Proof lifecycle.** `analysis.Result`, `lint.Session`, and `proofBatch`
   already use opaque IDs and source checks. Add a phase only if a new caller
   needs to observe it.

## Verification

- Add a transition test for each explicit state value.
- Run `go test ./...` and the pinned CLI contract test.
- Do not change diagnostics or SafeFix authorization in a state-only change.
