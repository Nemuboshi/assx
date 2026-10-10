# Pinned renderer profiles and source-backed dispatch (P04/10)

Implementation: [Issue #19](https://github.com/Nemuboshi/assx/issues/19), under [Epic #16](https://github.com/Nemuboshi/assx/issues/16).

## Package and ownership

The isolated package at internal/ass/renderer consumes the lossless
ConcreteExpression / ConcreteDialogue created by internal/ass. It does not
rewrite the CST, assign a global dialect to it, alter legacy_tags.go, or
change the existing command-line acceptance envelope.

A Profile identifies exactly one renderer source revision, plus build-scoped
feature states. It is a value type with immutable configuration. The name
dispatch tables are private and read-only. A Result is a detached value:
source offsets and argument text refer to the original decoded dialogue;
copies of mutable TagSpec slice fields are made before returning them.

The caller can inspect independently:

- Name matching: unknown, recognized, ignored by a no-op normalization arm,
  conditional on an unknown build, or disabled in a known build.
- A selected command and consumed head suffix, and a longer, competing
  documented command that was shadowed by a prefix.
- Whether a signature has source-verified evidence, only inferred evidence,
  an evidence-backed rejection, or is still unknown.
- Source spans and exact raw argument components; parsing a number or applying
  a state transition is outside this PR.

An XML signature records applicability separately from per-renderer
verification. Absence of signatures is unknown, never an implicit valid
default. A rejection requires a complete verified signature set. A verified
signature is *not* proof that its effect is understood or that an edit is
SafeFix; those decisions belong to P05/P07/P08.

## Dispatch mechanics

libass is an **ordered, case-sensitive byte-prefix chain** from
libass/ass_parse.c:355–916. xy-VSFilter uses **case-sensitive registered
prefixes in descending length** (maximum five bytes), from
src/subtitles/RTS.cpp:1716–1769 and 2155–2163.

VSFilterMod first runs its **ordered case-sensitive normalization chain**
(RTS.cpp:2524–2669), then dispatches the normalized command by exact match.
The distinction matters for no-op normalization branches that leave the
unmodified command for exact dispatch, such as clip, pos and some extensions.
The command fad is handled by an exact apply branch without a normalization
arm; a bogus inline fad suffix must not be accepted as an alternate form.

VSFilterMod extensions gated by _VSMOD and Lua functionality gated by
_VSMOD plus _LUA require explicit compile-time capabilities. The default
Build represents unknown capabilities, even for a VSFilterMod profile.
An unknown optional branch records the candidate and its disabled-build
fallback instead of pretending to know which parser was compiled.

The normalized command is distinguished from the original raw lexeme:

| Source | libass | xy-VSFilter | VSFilterMod (_VSMOD enabled) |
| --- | --- | --- | --- |
| \fsvp6 | fs, suffix vp6 | fs, suffix vp6 | fsvp, suffix 6 |
| \frs10 | fr, suffix s10 | fr, suffix s10 | frs, suffix 10 |
| \blend(add) | b (shadow) | b (shadow) | blend |
| \pos(1,2,3) | pos, arity rejected | pos, arity rejected | pos, arity verified |

The legacy behavior and diagnostics remain unchanged until the renderer
semantics and scoped lint migration in later PRs.

## Evidence pipeline and verification

docs/ass-tags.xml is the authoritative per-signature evidence source.
go generate ./internal/ass/renderer produces matrix_generated.go with
applicability and independently verified renderer masks, citations and raw
signature shapes. The source digest canonicalizes CRLF and standalone CR to LF
before hashing, so different Git checkout settings cannot invalidate the
evidence hash. The generator's -check mode compares canonical line endings;
.gitattributes also pins the XML and generated Go file to LF on checkout.
Unverified tag groups become explicit stubs, without inventing signatures. A test pins the XML digest to generated code and
compares source revision hashes to Profile.Version. Existing
internal/ass/spec evidence validators remain authoritative.

Optional source-checkout tests independently extract actual dispatch order
from ref/renderers/* at the pinned revisions. The build and normal tests
do not require those ignored local checkouts. Keep the hand-audited dispatch
tables synchronized with those tests when pinned code changes. Run:

    go generate ./internal/ass/renderer
    go run ./internal/ass/renderer/cmd/genmatrix -check
    go test ./...
    go test -race ./internal/ass/renderer/...
    go vet ./...

The exact current source contradicts a previous XML statement that VSFilterMod
blend is shadowed by be. The string blend begins with bl, so the be check
cannot match. blend is checked before the generic b branch and has a reachable
apply branch (RTS.cpp:2551, 3716–3730). XML name-match evidence is corrected
in this PR. Both traditional renderers recognize the shorter b prefix.
Golden default diagnostics are intentionally *not* rewritten for this
renderer-specific metadata correction.

P05 should evaluate Result operations per renderer, with state and provenance;
P06 should migrate validity/diagnostic decisions; P08 must require
renderer-scoped proof before classifying any mutation as SafeFix.
