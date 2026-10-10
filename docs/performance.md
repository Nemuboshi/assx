# Performance benchmarks

Issue: [#2](https://github.com/Nemuboshi/assx/issues/2)

The benchmark suite separates source parsing, dialogue AST construction,
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

`internal/perftest.Long` constructs the long document **before benchmark
timing**; no bulky generated file is tracked. Existing synthetic benchmarks
remain available, including `nested_1000` as a pathological stress case.
These feature-focused fixtures are not an empirical sample of real subtitle
frequency distributions. Licensed/anonymized real-world corpus input may be
a useful later extension.

## Timed regions

| Benchmark | What is timed per operation |
| --- | --- |
| `BenchmarkParseDialogueTextFixtures` | AST parsing for every Dialogue's text, without document parsing |
| `BenchmarkASSDocumentParse` | `ass.Parse` including the Dialogue AST |
| `BenchmarkEvaluateDialogue` | Effective-state evaluation on pre-parsed ASTs |
| `BenchmarkAnalyzeDocumentFixtures` | All lint passes on a pre-parsed document |
| `BenchmarkParseAndAnalyzeDocumentFixtures` | Complete `ass.Parse` plus `lint.AnalyzeDocument` |

Each operation processes one **whole fixture**. Fixture loading, large-input
construction, and prerequisite parsing are outside timed regions. Go reports
`ns/op`, `B/op`, and `allocs/op`; some benchmarks also set bytes processed.

## Reproduce and compare

Use the same benchmark environment and Go toolchain, preferably with background
processes minimized. Record the full commit, Go version, and OS/architecture.
Before publishing benchmark samples, replace host-specific CPU metadata
with the value 'undisclosed' and omit processor models, core counts, and
other device details.
Keep hardware-specific notes outside the repository.

~~~sh
go test ./...
go test ./internal/ass ./internal/semantic ./internal/lint \
  -run '^$' \
  -bench 'Benchmark(ASSDocumentParse|ParseDialogueTextFixtures|EvaluateDialogue|AnalyzeDocumentFixtures|ParseAndAnalyzeDocumentFixtures)$' \
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

## Initial baseline

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

## CPU and allocation profiling

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
- Workloads and timed regions: unchanged from the table above
- Detailed regression and evidence contract: [renderer-contracts.md](renderer-contracts.md)

Treat the two baseline commits as distinct measurement points. Compare P02–P10 against the renderer architecture baseline using equivalent software and hardware; use `benchstat` instead of treating single-sample differences as regressions.
