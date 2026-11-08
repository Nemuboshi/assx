# Performance benchmarks

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

## Pinned baseline

`main@6ba745336615f102d5de39f76550f0a45da7bbb6` is the pinned comparison
baseline.

- Raw benchstat input: [baseline-6ba7453-windows-amd64.txt](baseline-6ba7453-windows-amd64.txt)
- Environment: Windows/amd64, Go 1.27.0
- Sample settings: `-benchmem -benchtime=250ms -count=5`
- Evidence contract: [the evidence contract](../rules/verification.md)

Compare against it with equivalent software and hardware. Use `benchstat`.
A single-sample difference is not a regression.

