# Performance benchmarks

Issue: [#2](https://github.com/Nemuboshi/assx/issues/2)

The benchmark suite separates source parsing, dialogue CST construction,
effective-state evaluation, lint analysis, and complete parse-and-lint work.
This is a measurement harness, not a production optimization.

## Workloads

Four deterministic, hand-curated **synthetic but production-shaped** ASS files
are checked in under `testdata/perf/`. Each has complete Script Info, Styles,
and Events sections.

| Fixture | Dialogues | Intended feature coverage |
| --- | ---: | --- |
| `dialogue.ass` | 96 | Mostly plain multilingual dialogue, punctuation and line breaks |
| `typesetting.ass` | 48 | Signs, positioning, resets, karaoke, clipping and style switching |
| `drawing.ass` | 32 | Vector drawings, bezier paths, rectangular/vector clipping |
| `transforms.ass` | 48 | Nested/repeated transforms, resets and animated properties |
| `long_10000` | 10,000 | Deterministically cycles through all four checked-in files' Dialogue lines |
| `scaling_250` … `scaling_10000` | 250, 1000, 4000, 10000 | Curated dialogue lines under one header, each prefixed with one no-effect override block |
| `late_candidate_failure_250` … `_10000` | 250, 1000, 4000, 10000 | Uniform benchmark dialogues; one unknown tag in the final dialogue revokes its three SafeFix candidates |
| `style_edit_proof_250` | 250 | One fractional-integer Style field edit plus two proved no-effect edits per dialogue |

`internal/perftest.Long` constructs the long document and
`internal/perftest.Scaling` the scaling documents **before benchmark
timing**; no bulky generated file is tracked. `internal/perftest.StyleEdit`
builds the Style-edit workload the same way. The late-proof-failure workload
reuses `benchmarkASSDocument` and replaces the final dialogue's override
block with an unknown tag. Existing synthetic benchmarks remain available,
including `nested_1000` as a pathological stress case.
The lint stage records `cand/op` (SafeFix candidates) and the semantic
compatibility stage records `finding/op`, so allocation cost is readable per
candidate. Stage tests assert fixture findings and fixed output before any
timing.
These feature-focused fixtures are not an empirical sample of real subtitle
frequency distributions. Licensed/anonymized real-world corpus input may be
a useful later extension.

## Timed regions

| Benchmark | What is timed per operation |
| --- | --- |
| `BenchmarkParseConcreteDialogueFixtures` | Lossless CST parsing for each Dialogue, without document parsing |
| `BenchmarkASSDocumentParse` | `ass.Parse` document fields and spans, without dialogue CST construction |
| `BenchmarkEvaluateDialogue` | Explicit libass effective-state evaluation on pre-parsed CSTs |
| `BenchmarkCompareDialogue` | Pairwise compatibility comparison on pre-parsed CSTs |
| `BenchmarkAnalyzeDocumentFixtures` | All lint passes on a pre-parsed document, including the `late_candidate_failure_N` and `style_edit_proof_N` stages |
| `BenchmarkSessionDiscovery` | `lint.Discover` on the same documents: every lint pass and candidate recording, no SafeFix proof construction |
| `BenchmarkParseAndAnalyzeDocumentFixtures` | Complete `ass.Parse` plus `lint.AnalyzeDocument` |

The scaling fixtures (`scaling_250` … `scaling_10000`) separate source-size
growth from feature mix. The lint analysis of `long_10000` is dominated by
source scans shared with other stages; the `scaling_*` and
`late_candidate_failure_*` stages isolate the SafeFix proof path, where
candidate count scales with document size. `BenchmarkSessionDiscovery`
measures the same documents with proof construction removed. Proof and
application stage benchmarks arrive with their deferred APIs.

Each operation processes one **whole fixture**. Fixture loading, large-input
construction, and prerequisite parsing are outside timed regions. Go reports
`ns/op`, `B/op`, and `allocs/op`; some benchmarks also set bytes processed.

## Reproduce and compare

Use the same benchmark environment and Go toolchain, preferably with background
processes minimized. Record the full commit, Go version, and OS/architecture.
Before publishing benchmark samples, replace host-specific CPU metadata
with the value 'undisclosed' and omit processor models, core counts, and
other device details. Go appends the parallelism level to each benchmark
name (for example `-8`); strip that suffix before publishing.
Keep hardware-specific notes outside the repository.

~~~sh
go test ./...
go test ./internal/ass ./internal/semantic ./internal/lint \
  -run '^$' \
  -bench 'Benchmark(ASSDocumentParse|ParseConcreteDialogueFixtures|EvaluateDialogue|AnalyzeDocumentFixtures|ParseAndAnalyzeDocumentFixtures)$' \
  -benchmem -benchtime=250ms -count=5 > before.txt
~~~

Run the **same** command on an optimization candidate, writing `after.txt`.
Install the separate developer tool (not a project/runtime dependency), then
compare distributions rather than individual timing samples:

~~~sh
go install golang.org/x/perf/cmd/benchstat@latest
benchstat before.txt after.txt
~~~

CI runs the new benchmarks once with `-benchtime=1x` as a smoke test, with
no fixed performance threshold: shared CI runners are noisy.

## Implementation record: Initial baseline

- **Measured commit:** `b3080c8c22591f0aa28532d501c3970d7b8b2df3`
- **Environment:** Windows/amd64, Go 1.27.0
- **Flags:** `-benchmem -benchtime=250ms -count=5`
- **All 125 raw samples:** [baseline-b3080c8-windows-amd64.txt](performance/baseline-b3080c8-windows-amd64.txt)
- The rows below show the median of five samples for each workload/stage.

| Stage | Typesetting (48): ns/op | B/op | allocs/op |
| --- | ---: | ---: | ---: |
| Dialogue AST | 144,770 | 96,960 | 1,020 |
| ASS document parse | 259,048 | 255,032 | 1,496 |
| Effective state | 232,296 | 192,578 | 2,148 |
| Lint only | 785,806 | 374,858 | 3,754 |
| Parse + lint | 1,230,451 | 636,068 | 5,252 |

| Stage | Long (10,000): ns/op | B/op | allocs/op |
| --- | ---: | ---: | ---: |
| Dialogue AST | 16,765,975 | 10,903,758 | 123,964 |
| ASS document parse | 34,941,062 | 37,511,744 | 194,128 |
| Effective state | 26,521,942 | 22,024,232 | 243,572 |
| Lint only | 92,071,867 | 39,414,696 | 388,789 |
| Parse + lint | 129,586,650 | 76,946,156 | 582,921 |

The independently measured stage timings and allocations should **not** be
summed to reconstruct an end-to-end run; GC and object lifetimes differ.

## Implementation record: scaling and proof-stage baseline

- **Measured commit:** `2564e744464af3b30412afd8dd1cab625aad9d99`
- **Environment:** Windows/amd64, Go 1.27.0, GOMAXPROCS 8
- **Flags:** `-benchmem -benchtime=250ms -count=5`
- **All raw samples:** [baseline-2564e74-windows-amd64.txt](performance/baseline-2564e74-windows-amd64.txt)
- The rows below show the median of five samples for each workload/stage.

| Lint stage | ns/op | B/op | allocs/op |
| --- | ---: | ---: | ---: |
| `typesetting` | 25,353,638 | 13,355,338 | 103,607 |
| `long_10000` | 1,995,520,800 | 1,682,278,176 | 7,262,831 |
| `scaling_250` | 412,252,300 | 261,423,656 | 1,669,880 |
| `scaling_1000` | 1,295,661,200 | 758,743,168 | 4,555,926 |
| `scaling_4000` | 2,340,116,200 | 1,715,716,072 | 9,582,038 |
| `scaling_10000` | 4,024,070,500 | 3,395,075,744 | 18,311,642 |
| `late_candidate_failure_250` (750) | 181,016,000 | 101,138,708 | 617,900 |
| `late_candidate_failure_1000` (3,000) | 779,539,900 | 405,230,784 | 2,472,205 |
| `late_candidate_failure_4000` (12,000) | 3,005,337,700 | 1,617,665,448 | 9,888,606 |
| `late_candidate_failure_10000` (30,000) | 8,906,064,500 | 4,039,018,400 | 24,721,213 |
| `style_edit_proof_250` (751) | 232,570,850 | 101,686,392 | 620,714 |

The late-failure and Style-edit stage guards assert their candidate counts
(3 per dialogue, plus one Style edit; 3 proved per earlier dialogue, and the
barriered final dialogue keeps its findings without any automatic edit).

Source bytes: `scaling_250` 37,076; `scaling_1000` 145,685;
`scaling_4000` 586,541; `scaling_10000` 1,464,770; `long_10000` 1,081,230;
`style_edit_proof_250` 26,793. The `late_candidate_failure_*` documents reuse
the uniform benchmark dialogue line and scale with dialogue count.

The candidate funnel for the curated `scaling_*` stages and `long_10000`
(SafeFix candidates discovered before proof, versus candidates that leave
`AnalyzeDocument` with a proof) is:

| Stage | discovered | proved |
| --- | ---: | ---: |
| `typesetting` | 16 | 16 |
| `scaling_250` | 480 | 342 |
| `scaling_1000` | 1,924 | 753 |
| `scaling_4000` | 7,647 | 119 |
| `scaling_10000` | 19,136 | 0 |
| `long_10000` | 1,776 | 0 |

Reading the stages:

- Scaling cost is dominated by the SafeFix proof path, not by source scans.
  `scaling_10000` carries 1.46 MB of text (comparable to `long_10000`'s
  1.08 MB) and 19,136 discovered candidates, and costs 2.0x the time and
  2.0x the bytes of `long_10000`, which proved none of its 1,776
  discovered candidates.
- The funnel above is the bounded subdivision schedule, not a per-candidate
  semantic change. One failing candidate sends its group through the queue of
  at most 64 proof builds. Curated documents mix provable and unprovable
  edits, so many groups split until attempts run out, and unattached
  candidates lose their automatic edits. At `scaling_10000` and `long_10000`
  no group survived. Whether the unproved candidates are individually
  provable is not established here; Task 6 measures reused proof work
  against this funnel.
- `late_candidate_failure_10000` proves 29,997 of its 30,000 candidates: the
  final dialogue's unknown tag revokes exactly its three candidates during
  semantic evaluation, before proof batching. The uniform document's first
  aggregate build therefore succeeds, and its 4.04 GB still costs 2.0x the
  time of `scaling_10000`, which proves nothing. The aggregate proof itself —
  every dialogue, every target, full traces — dominates.
- `style_edit_proof_250` measures the document-wide path a Style edit forces:
  the same three dialogue candidates per dialogue plus one Style edit, all
  751 proved, 232 ms and 102 MB. The Style proof selects every dialogue
  regardless of edit locality.
- Evaluation cost is linear in the semantic work, not in proof outcome:
  `BenchmarkEvaluateDialogue` allocates 22.6 MB at `scaling_1000` (2,000
  no-effect records) and 93.2 MB at `scaling_4000` (8,000 records), 4.1x over
  4.0x records. `long_10000` (517,996 records) allocates 80.7 MB: record
  count alone does not predict evaluation cost, because candidate-free
  dialogues take the skip path.

These are median single-stage samples on one machine. They do not establish
a speed ratio against other revisions and no quadratic claim is made;
`-count=5` distributions are in the raw file, and `benchstat` remains the
comparison tool.

Allocation profiles for `long_10000`, `late_candidate_failure_1000`, and
`scaling_10000` on this commit (captured outside the repository with
`-cpuprofile`/`-memprofile`, read with `go tool pprof -top -alloc_space`):

- Proof hashing and evidence dominate: `lint.hashSource` 9.3–9.6% flat and
  `lint.hashJSON` 7.9% flat (45.2% cumulative in the late-failure profile),
  reached through `buildFixProofWithDocument` (cumulative 78.4% in the
  long profile).
- Semantic preparation repeats: `semantic.resolveTokens` ~10% cumulative and
  `semantic.StateView.Snapshot` 7.5–9.6% flat per stage.
- The long CPU profile shows `encoding/json/v2` struct marshaling at 27.6%
  cumulative and GC scanning (`runtime.tryDeferToSpanScan`,
  `runtime.scanObject*`) above 10%, consistent with the trace-heavy proof
  path.


## Implementation record: CPU and allocation profiling

Sampled `BenchmarkAnalyzeDocumentFixtures/typesetting` on the same commit:

~~~sh
go test ./internal/lint -run '^$' \
  -bench '^BenchmarkAnalyzeDocumentFixtures$/^typesetting$' \
  -benchtime=3s -count=1 -benchmem \
  -cpuprofile cpu.prof -memprofile heap.prof
go tool pprof -top -nodecount=20 cpu.prof
go tool pprof -top -alloc_space -nodecount=20 heap.prof
~~~

Put profile outputs in a temporary directory outside the worktree or remove
them after inspection. CPU profiling attributed **51.57% cumulative**
(6.02% flat) of sampled time to `ass.DialogueText.WalkTokens`, and **19.76%
cumulative** (1.20% flat) to the effective-state engine's `consumeTag`.
In allocation-space profiling, `consumeTag` accounted for **36.38% flat**
of allocated bytes and `maps.clone` for **21.90% flat**. The Style-overrides
token walk was visible in both profiles. Allocation profiles may include
benchmark setup; these are hotspot indicators, not exact per-operation budgets.

Possible future investigation: repeated AST walks and transient map/state
allocations. This issue does not attempt such optimizations; future changes
must preserve correctness, SafeFix behavior, and renderer-equivalence checks,
and should be compared against this baseline with `benchstat`.

## Renderer architecture baseline (issue #17)

The renderer-aware migration freezes the original `main@6ba745336615f102d5de39f76550f0a45da7bbb6` performance separately from the issue #2 optimization baseline.

- Raw benchstat input: [baseline-6ba7453-windows-amd64.txt](performance/baseline-6ba7453-windows-amd64.txt)
- Environment: Windows/amd64, Go 1.27.0
- Sample settings: `-benchmem -benchtime=250ms -count=5`
- Historical samples use the initial dialogue parser and default semantic adapter.
- P10 uses the concrete parser and profile engine; timed boundaries now follow the current table.
- Detailed regression and evidence contract: [renderer-contracts.md](renderer-contracts.md)

P10 replaces the LF-only line scanner with the concrete parser and adds default
compatibility comparison.
The parent integration compares benchmark samples, runs lint and frame checks, and records the measured effect.
No new performance result is claimed here.

Treat the two baseline commits as distinct measurement points. Compare P02–P10 against the renderer architecture baseline using equivalent software and hardware; use `benchstat` instead of treating single-sample differences as regressions.
