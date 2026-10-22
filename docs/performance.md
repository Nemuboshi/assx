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

## Implementation record: secondary scans, hashing, and AST storage

This record covers the secondary-scan removal, the digest paths, and the
concrete-AST evaluation. It replaces no earlier number; measure every claim
here with the commands it names.

### Secondary scans no longer dominate large edit sets

Two passes compared every element against every other element. Both are now
indexed, and both keep a counter so a test decides the shape of the work
instead of a wall-clock reading:

- Redundant-Style suppression (`suppressOverlappingNoEffect`,
  `internal/lint/document.go`) compared every candidate edit against every
  redundant edit through `Interval.Overlaps`. It now answers the same
  half-open predicate from a start-sorted interval index with a prefix
  maximum of interval ends: one binary search plus one comparison per query.
- SafeFix edit authorization (`applyFixes`, `internal/lint/fix.go`) scanned a
  proof's whole source-edit list for every candidate edit. It now tests one
  map membership of the complete proof entry.

Measured with the counters `overlapComparisons` and
`proofMembershipComparisons`:

| Workload | Suppression decisions, before | after | Membership decisions, before | after |
| --- | --- | --- | --- | --- |
| 250 dialogues | 62,750 | 500 | 45,450 (300 edits) | 300 |
| 1000 dialogues | 1,001,000 | 2,000 | 721,800 (1200 edits) | 1,200 |
| 4000 dialogues | — | 8,000 | — | 4,800 |

The before columns grew 15.95× and 15.9× for a 4× input, which is the square
of the edit count. The after columns are two decisions per dialogue in the
suppression pass and exactly one decision per edit in authorization, so
`BenchmarkApplyFixesScaling` reports `membership/op` equal to `edit/op` at
every scale. The index is tested against the predicate it replaced: a corpus
of touching, zero-width, contained, disjoint, and reversed intervals, then
deterministic random interval sets, must agree with `Interval.Overlaps` on
every probe. Suppression output is unchanged, so overlap semantics — half-open
ranges, where touching intervals do not overlap — are exact.

### Digest paths

`hashJSON` hashed its own encoding through `string(encoded)`, which copies the
marshaled bytes twice. A byte path halves the copies and the proof-trace
digest no longer rounds through a string. Measured by `AllocsPerRun` over the
same value: **7 allocations before, 5 after**, which is marshaling plus the
digest, so no copy remains.

The proof batch also re-hashed the whole source on every fallback attempt,
even though no edit can change it. One batch now computes its source digest
once (`proofBatch.sourceDigest`) and every attempt reuses it. The scaling
workloads drive **64 proof attempts** per run:

| Workload | Source bytes | Proof attempts | Source digests, before | after |
| --- | --- | --- | --- | --- |
| scaling_250 | 37,076 | 64 | 64 | 1 |
| scaling_1000 | 145,685 | 64 | 64 | 1 |

`BenchmarkProofHashing` isolates the two paths on a 1 MB source
(`source_digest` covers a whole source and runs once per batch;
`proof_digest` covers a marshaled trace). Neither change uses an unsafe
conversion, and both leave every proof byte identical: a one-off rebuild of
each group still marshals equal to the batch proof.

### Benchmark comparison against the previous commit

`benchstat` compares the previous commit (`5579100`, Task 6) with this
change on the same host: `-benchmem -benchtime=200ms -count=5`, five samples
per stage. The allocation columns are the stable result. The timing columns
carry the host noise.

| Lint stage | ns/op: base → this | B/op: base → this | allocs/op: base → this |
| --- | ---: | ---: | ---: |
| `dialogue` (96) | 1459.0m → 1133.0m | 977.4Ki → 976.4Ki | 5.958k → 5.957k |
| `drawing` (32) | 7785.0m → 7771.0m | 5.250Mi → 4.909Mi | 27.61k → 27.59k |
| `transforms` (48) | 16450.0m → 16090.0m | 10.61Mi → 10.41Mi | 88.50k → 88.42k |
| `scaling_250` | 301.1m → 288.7m | 212.9Mi → 196.3Mi | 915.3k → 914.2k |
| `scaling_1000` | 744.1m → 756.9m | 595.1Mi → 558.2Mi | 2.345M → 2.343M |
| `scaling_4000` | 1633.0m → 1526.0m | 1.287Gi → 1.226Gi | 5.091M → 5.089M |
| `scaling_10000` | 3218.0m → 2980.0m | 2.506Gi → 2.400Gi | 9.831M → 9.829M |
| `late_candidate_failure_250` | 179.0m → 156.0m | 104.15Mi → 90.24Mi | 417.8k → 416.7k |
| `late_candidate_failure_1000` | 643.0m → 637.3m | 416.6Mi → 361.6Mi | 1.669M → 1.665M |
| `late_candidate_failure_4000` | 2601.0m → 2465.0m | 1.625Gi → 1.410Gi | 6.673M → 6.657M |
| `style_edit_proof_250` | 167.2m → 191.0m | 104.60Mi → 90.69Mi | 419.7k → 418.7k |
| geomean | 189.0m → 180.4m (−4.55%) | −7.81% | −0.13% |

Allocated bytes fall on every stage. The proof-heavy stages fall most:
about 13% at the four `late_candidate_failure_*` and `style_edit_proof_*`
stages, where one whole-document proof trace is large. That is the removed
`string(encoded)` copy: a proof build no longer duplicates its marshaled
trace to hash it. The `scaling_*` stages fall 4–8%, which is that copy plus
the 63 saved source digests. Allocation counts stay almost equal, because
the change removes bytes copied, not allocation events.

The timings need care on this host. Rows that `benchstat` marks as not
significant at p<0.05 are noise. The `dialogue` row reports −22% in the
first sample set; that did not reproduce when both sides were re-measured
in alternating rounds (`dialogue` median about 1.20–1.25 ms at the base
against about 1.17 ms here), so this record claims nothing for that stage.
The significant timing rows agree with the byte reduction: less allocation
means less garbage collection. No stage regressed significantly;
`style_edit_proof_250` reads +14% at p=0.095, which is not significant and
is the one row to re-measure if a later change touches that path.

### Acceptance run against the historical baseline

The acceptance run measured this branch (`387fc97`) against the committed
baseline (`2564e74`) on one host with one toolchain, five samples per stage:
`-benchmem -benchtime=250ms -count=5`. Raw samples:
[baseline-2564e74-windows-amd64.txt](performance/baseline-2564e74-windows-amd64.txt)
and
[optimized-387fc97-windows-amd64.txt](performance/optimized-387fc97-windows-amd64.txt).

Five samples are fewer than the six `benchstat` needs for a confidence
interval, so no interval is available and 0.008 is the smallest p value this
sample size can produce. Read `p=0.008` as "the samples differ", not as a
precise significance. The comparison against `5579100` above re-measures both
sides in this checkout. That comparison stays the primary evidence for the
last three commits; this run against the committed baseline is the end-to-end
corroboration, and its allocation columns are the stable part.

| `AnalyzeDocumentFixtures` stage | sec/op: base → this | B/op: base → this | allocs/op: base → this |
| --- | ---: | ---: | ---: |
| `typesetting` (48) | 25.35m → 17.66m (−30.33%) | 12.74Mi → 11.10Mi (−12.83%) | 103.61k → 58.45k (−43.58%) |
| `dialogue` (96) | 2.404m → 1.084m (−54.92%) | 1170.6Ki → 975.9Ki (−16.64%) | 16.723k → 5.957k (−64.38%) |
| `drawing` (32) | 10.620m → 8.460m (−20.34%) | 5.588Mi → 4.911Mi (−12.11%) | 47.12k → 27.59k (−41.44%) |
| `transforms` (48) | 21.49m → 16.94m (−21.18%) | 13.21Mi → 10.41Mi (−21.21%) | 128.37k → 88.42k (−31.12%) |
| `long_10000` | 1.996s → 1.103s (−44.71%) | 1.567Gi → 1.024Gi (−34.66%) | 7.263M → 3.666M (−49.53%) |
| `scaling_250` | 412.3m → 272.8m (−33.82%) | 249.3Mi → 196.2Mi (−21.32%) | 1669.9k → 914.1k (−45.26%) |
| `scaling_1000` | 1295.7m → 635.5m (−50.95%) | 723.6Mi → 558.2Mi (−22.85%) | 4.556M → 2.343M (−48.57%) |
| `scaling_4000` | 2.340s → 1.352s (−42.23%) | 1.598Gi → 1.226Gi (−23.24%) | 9.582M → 5.089M (−46.89%) |
| `scaling_10000` | 4.024s → 2.555s (−36.50%) | 3.162Gi → 2.400Gi (−24.09%) | 18.312M → 9.829M (−46.32%) |
| `late_candidate_failure_10000` | 8.906s → 5.571s (−37.45%) | 3.762Gi → 3.521Gi (−6.41%) | 24.72M → 16.64M (−32.68%) |
| `style_edit_proof_250` | 232.6m → 142.8m (−38.60%) | 96.98Mi → 90.67Mi (−6.50%) | 620.7k → 418.7k (−32.55%) |
| geomean, both lint document benchmarks | 268.6m → 93.43m (−42.08%) | −18.02% | −43.75% |

The long and scaling fixtures carry the required result: allocated bytes fall
21–35% and allocation events fall 45–50%. The proof-bearing stages fall most
in allocation events — 32.6% at `late_candidate_failure_*` and
`style_edit_proof_250` — which is the removed trace copy and the digested
proof work. `ParseAndAnalyzeDocumentFixtures` shows the same shape
(`long_10000` −54.84% time, `scaling_*` −50.8…−56.4%).

These new stages did not exist at `2564e74`, so they state cost, not
improvement:

| New stage (head only) | sec/op | B/op | allocs/op |
| --- | ---: | ---: | ---: |
| `SessionDiscovery/typesetting` | 3.406m | 2.911Mi | 17.58k |
| `SessionProveFixes/typesetting` | 26.42m | 4.808Mi | 19.78k |
| `SessionProveFixes/scaling_1000` | 721.4m | 380.7Mi | 1.319M |
| `SessionProveFixes/style_edit_proof_250` | 69.51m | 67.75Mi | 232.0k |
| `ApplyFixesScaling/dialogues_4000` | 2.732s | 1.662Gi | 7.239M |
| `OverlapSuppressionScaling/4000_dialogues` | 69.02m | 64.29Mi | 352.2k |
| `ProofHashing/source_digest` (1 MB source) | 269.4µ | 104.1Ki | 3 |
| `ProofHashing/proof_digest` | 5.070µ | 1.156Ki | 5 |

Three measurements move the wrong way. Each one is a measured trade, not an
unnoticed regression:

- **`CompareDialogue` bytes, +6.75…+11.17% on every shared fixture, with
  allocation events −11.5…−12.9% and time equal or better (−5.1…−21.6% on
  the large fixtures).** One prepared view per dialogue now keeps its replay
  entries, which is the +~9.5% retained bytes measured when the shared view
  was introduced, so lint validity and the document compatibility pass
  resolve each dialogue once. The accepted comparison then decides on compact
  state and materializes its wire trace, which allocates the trace twice. The
  bytes pay for one resolution per dialogue and for fewer allocation events.
- **`EvaluateDialogue` bytes, +0.67…+2.62%, with allocation events
  −8.83…−20.37% and time equal or −2.71%.** The slot index keeps dense
  `[44]` tables — values, owner index, owner start, latches — where a map
  allocated and freed per state. A state therefore retains a small fixed
  array instead of paying a map per slot. A state that overflows the modeled
  table falls back to the map storage and stays correct.
- **The `internal/ass` parse stages, up to +23.7% time on individual
  fixtures (`scaling_250`, `ASSDocumentParse/scaling_4000` +19.9%).** The
  parse section geomean is +0.39% (sec/op), and every parse fixture reports
  byte-identical `B/op` and `allocs/op` — `benchstat` prints "all samples are
  equal" for them — so the timed work is identical. `git diff 2564e74 387fc97
  -- internal/ass` changes one file, `ass/renderer/document_view.go`, and no
  parser file. The individual parse rows move in both directions on this host
  (the same section shows −7.15% and −6.46%). The judgement is host noise,
  not a regression.

**Diagnostic-only requests build no proof.** `TestSessionDiscoveryStripsSafeFixAndNeverBuildsProof`
(in `internal/lint/session_test.go`) wraps the proof-builder seam and asserts
that `Discover` calls it zero times, that every pending candidate carries an
empty `FixSafety`, no edits, and no proof reference, and that only a later
`ProveFixes` call reaches the seam. The measured stages agree: at
`typesetting`, discovery alone costs 3.406 ms and 2.911Mi B/op, against
17.66 ms and 11.10Mi B/op for the eager document stage that also builds
proofs.

**The eager path keeps its contracts.** `TestAnalyzeEmptyOptionsMatchDefaultTargets`
(in `internal/analysis`) compares the snapshot path with
`lint.AnalyzeDocumentForTargets` on the same document: pending findings,
proved findings, and merged diagnostics are `DeepEqual`, their JSON bytes are
identical, and `ApplyFixes` returns the same fixed text and count. The CLI
suites pin the report bytes and statuses (`TestRunJSONStaysValidAndReportsUnknownTagsAsSuggestion`,
`TestPlainPresenterPreservesLegacyBytes`, `TestRunSafeFixAndConciseSummary`,
`TestRunUnsafeFixIsOptInAndPreservesExitStatus`), and
`TestRendererBaselineContracts` pins the renderer output.

**Remaining cost, reported and not concealed:**

- An accepted proof still materializes its complete wire trace and hashes it.
  Bounded trace hashing was not added, because the condition for it was not
  met; the repeated digest work was removed by deduplication instead. This is
  the second allocation above: it is why `CompareDialogue` and
  `EvaluateDialogue` bytes rise while their allocation counts fall.
- The bounded subdivision schedule stays. A SafeFix batch proves at most 64
  builds — the aggregate first, then halves. A document whose split reaches
  that bound loses its automatic edits for the remaining candidates. It never
  takes an unsafe path. `late_candidate_failure_10000` measures that path at
  5.571s, −37.45%.
- The retained before-side cache trades allocations for live heap inside one
  request. At `GOMAXPROCS=1` the proof path is slower on large documents
  (`late_1000` session: 1290 ms against 922 ms) because the collector marks
  the retained trees; the same run with `GOGC=400` takes 313 ms. At the
  default parallelism the same stage is faster and proof-only allocations
  fall 44.7%. The CLI runs at default parallelism, so a `GOMAXPROCS=1` user
  on a 1000-dialogue file is the caller who pays this.

Serial work still dominates. One goroutine runs discovery, proof, application,
and reporting. This branch lowers the cost of each stage. It does not divide
the work.

No concurrency change is part of this plan. The plan forbids a worker pool, and
no parallel variant was measured, so a concurrency change has no evidence
here. Such a change needs its own design: a deterministic order for findings
and for proof entries, race tests over the shared session, and a measurement
against this serial baseline. The `GOMAXPROCS=1` row above is the cost of one
lane; the rows at default parallelism are what a caller sees today.

~~~sh
go test ./internal/ass ./internal/semantic ./internal/lint -run '^$' \
  -bench '^Benchmark(ASSDocumentParse|ParseConcreteDialogueFixtures|EvaluateDialogue|CompareDialogue|AnalyzeDocumentFixtures|ParseAndAnalyzeDocumentFixtures|SessionDiscovery|SessionProveFixes|ApplyFixesScaling|ProofHashing|OverlapSuppressionScaling)' \
  -benchmem -benchtime=250ms -count=5 > optimized.txt
benchstat baseline-2564e74-windows-amd64.txt optimized.txt
~~~

### Concrete AST storage

The issue asked whether node or argument storage in the concrete AST is worth
restructuring. Allocations say no. Sampling
`BenchmarkAnalyzeDocumentFixtures/{transforms,drawing}` (the transform-heaviest
workloads) with `-memprofile` and reading `alloc_space`:

- `internal/ass`, the whole concrete-AST package: **478.29 MB, 8.04%** of
  5951.03 MB. Node storage is `parseConcreteItems` at 2.82% cumulative and
  `ParseConcreteDialogue.func1` at 0.31%; lazily materialized argument storage
  is `ConcreteExpression.Components` at 0.75% cumulative.
- `internal/semantic`, the semantic state: **2087.08 MB, 35.07%** — 4.4× the
  entire AST package. Its largest single sites are `captureCompactState`
  (14.73% flat) and `materializeCompactState` (9.71% flat).
- The largest allocation site outside both is the proof-trace wire encoding:
  `encoding/json/v2`, with `bytes.Clone` at 7.32% flat inside
  `json/v2.Marshal`.

Arguments reach the AST by reference: `Head`, `Raw`, `Content`, and item text
are slices of `Source`, so parsing copies no argument text. Argument storage
that does allocate lives in the semantic layer, where resolved tags build
their own `Args` strings — that is semantic state, not AST layout. A compact
node layout would therefore trade roughly 3% of allocations for a parse-time
copy and a lossless-parsing risk, against a semantic layer that is 35%. The
AST keeps its source-backed, lossless storage.

### The final operation lookup pass

`operationsFromTokens` ends by repopulating each token's tag from the
operation map, because children are appended bottom-up after the tokens are
built. Removing that pass is conditional on a measured gain. Profiling the
transform-heavy workloads gives it **40 ms of 62.13 s sampled, 0.06%** (map
build 0.48%, final lookup 0.05%, loop 0.02%), and the map is a return value
the evaluator reads through `operationFor` on every tag, so it cannot be
dropped. The pass stays.

### Reproduce

~~~sh
go test ./internal/lint -run '^$' \
  -bench '^BenchmarkProofHashing$|^BenchmarkApplyFixesScaling$' -benchmem
go test ./internal/lint -run '^$' \
  -bench '^BenchmarkAnalyzeDocumentFixtures$/(transforms|drawing)$' \
  -benchtime=6x -memprofile alloc.mem
go tool pprof -sample_index=alloc_space -focus 'assx/internal/ass\.' -top alloc.mem
~~~

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

## Implementation record: font separation (#32)

The F03 font benchmarks use Go 1.27.0 on Windows/amd64, with an Intel Core i5-9300H.
The comparison uses `ae5e147` before extraction and the F03 implementation.
Each row uses three samples. Catalog timing includes metadata cache I/O and one Go Regular TTF.
Analysis uses `testdata/font-check.ass`, with `MISSING_GLYPH` replaced by U+10FFFF and a warm catalog.

| Benchmark | Before median | F03 median | Before allocations | F03 allocations |
| --- | --- | --- | --- | --- |
| Font catalog | 1.138 ms/op | 0.865 ms/op | 116/op | 119/op |
| Font analysis | 36.59 us/op | 26.06 us/op | 132/op | 116/op |
| Lint, 100 dialogues | 105.37 ms/op | 105.62 ms/op | 168,551/op | 168,583/op |
| Lint, 1,000 dialogues | 867.23 ms/op | 939.60 ms/op | 1,666,682/op | 1,666,686/op |

Lint timing ranges overlap: 90.52-106.79 versus 102.21-112.25 ms for 100 dialogues.
For 1,000 dialogues, ranges are 719.56-1,063.91 versus 848.06-993.20 ms.
These short local samples do not establish a timing regression or a general speed improvement.
Font analysis memory decreases from 21,431 to 20,355 bytes/op in this fixture.
The extraction does not change the lint analyzer.

Run the font checks with:

```sh
go test ./internal/fontcheck -run '^$' -bench 'BenchmarkFont.*Comparison$' -benchmem -count=3
```

The same harness ran against the archived baseline with `NewFontChecker` and `AnalyzeFonts` instead of the extracted APIs.
Recheck after changes to catalog I/O, usage extraction, matching, or the shared evaluator.
