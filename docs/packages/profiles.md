# Pinned renderer profiles and source-backed dispatch

## Package and ownership

The isolated package at internal/ass/renderer consumes the lossless
ConcreteExpression / ConcreteDialogue created by internal/ass. It does not
rewrite the CST or assign a global dialect to it.
All CLI and API validity checks use this profile path.
The default primary is libass, with xy-VSFilter compatibility targets.

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
- Source spans and exact raw argument components; the shared decoder and semantic engine own value conversion and state transitions.

An XML signature records applicability separately from per-renderer
verification **and per-signature compile-time requirements**. In VSFilterMod,
the three-coordinate pos and argument-bearing fsc variants require _VSMOD;
their core two-coordinate / zero-argument variants remain available without
it. The gated-only blend, frs and fsvp signatures carry the same requirement.
Profile.Signatures reports each variant's Availability and Requires along with
evidence scoped to the actual build; an unknown or unavailable guard cannot
yield SignatureVerified. An unknown optional branch prevents proving that the
available signatures exhaust all arities; a known-disabled branch is excluded
from that set. Empty bare values are not the same as invalid arguments:
the pinned scalar handlers often restore a style property or apply a
renderer-defined default. The evidence matrix therefore records explicit
zero-argument bare signatures for 47 additional scalar names, as well as
the existing fsc and r forms. This includes fs, bord, fn, color/alpha
reset tags, transforms of scalar properties, and the build-gated
VSFilterMod blend, frs and fsvp defaults. Each signature retains its
renderer-scoped source verification or inference and optional build gate.
Tags requiring coordinate tuples, such as bare pos, have no zero-arity
evidence and remain rejectable when the build is known.

Absence of signatures is unknown, never an implicit valid default. More
importantly, even a list consisting entirely of verified signatures is not
proof that every accepted arity has been enumerated: scalar handlers can
consume only their first value or default a missing one. The generated matrix
therefore contains a separate, renderer-scoped **exhaustive** declaration with
independent pinned-source citations. pos, move, clip and iclip claim this proof
for all three renderers. Three-coordinate pos remains gated on _VSMOD. move
accepts only four or six non-empty arguments; eight arguments are rejected.
clip and iclip accept 0, 1, 2 or 4 arguments; three or five arguments are
rejected. Every pinned clip handler falls through those arities without touching
the clipper, so a rejected clip arity is a no-op that keeps the previous clip.
Without that declaration, unlisted forms remain SignatureUnknown, not
SignatureRejected. The rejection algorithm additionally requires every
build-applicable signature to be verified and no conditional signature to
remain unresolved. A verified signature is *not* proof that its effect is
understood or that an edit is SafeFix; those decisions belong to the semantic evaluator and the comparison APIs.

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
arm. The fallback counter does not stop the exact apply chain.
A bogus inline fad suffix must not be accepted as an alternate form.

VSFilterMod extensions gated by _VSMOD require an explicit enabled capability.
The explicit lua command is a Mod name with an unmodeled effect.
The default Build is unknown, even for a VSFilterMod profile.
An unknown optional branch records the candidate and its disabled-build
fallback instead of pretending to know which parser was compiled.
See [build capabilities](#vsfiltermod-build-capabilities).

The normalized command is distinguished from the original raw lexeme:

| Source | libass | xy-VSFilter | VSFilterMod (_VSMOD enabled) |
| --- | --- | --- | --- |
| \fsvp6 | fs, suffix vp6 | fs, suffix vp6 | fsvp, suffix 6 |
| \frs10 | fr, suffix s10 | fr, suffix s10 | frs, suffix 10 |
| \blend(add) | b (shadow) | b (shadow) | blend |
| \pos(1,2,3) | pos, arity rejected | pos, arity rejected | pos, arity verified |

The CLI selects profiles with `--renderer` and `--compat`.
Mod capabilities remain unknown unless explicitly selected.
See [usage](../../README.md) and [the evidence contract](../rules/verification.md).

## VSFilterMod build capabilities

`_VSMOD` enables the Mod extensions. assx models only that gate:

| `_VSMOD` | Dispatch |
| --- | --- |
| off | xy-VSFilter |
| on | Mod extensions |
| unknown | conditional signatures, blocked proofs |

An unknown gate keeps its signatures conditional and records the disabled-build
fallback. Known and unknown states never default to enabled.
See [semantics](semantics.md) and [lint](lint.md).

The pinned project defines `_LUA` in every configuration. A Lua build needs
`_VSMOD`: `CSimpleTextSubtitle` inherits `CMyLua` only when both gates are
defined (src/subtitles/STS.h:332-336), and `ParseSSATag` calls a `CMyLua`
method under `_LUA` alone (src/subtitles/RTS.cpp:2466-2520). All eight pinned
project configurations define both gates.

The gate compiles Lua code, but the pinned source loads no script. The only
`luaL_dofile` call is in `CMyLua::LoadLuaFile` (src/subtitles/STS.cpp:1404),
and only a UUE text resource reaches it (STS.cpp:1746). No format entry or
playback setting calls that loader. Without a script, no global exists and
`\lua` has no effect.

assx does not model Lua. A loaded script could define a global with any name
and shadow any command, so those cases would not end. No flag and no editor
setting selects the gate. This scope limit is deliberate.

## Evidence pipeline and verification

docs/ass-tags.xml is the authoritative per-signature evidence source.
go generate ./internal/ass/renderer produces matrix_generated.go with
applicability and independently verified renderer masks, citations and raw
signature shapes. The source digest canonicalizes CRLF and standalone CR to LF
before hashing, so different Git checkout settings cannot invalidate the
evidence hash. The generator's -check mode compares canonical line endings;
.gitattributes also pins the XML and generated Go file to LF on checkout.
Unverified tag groups become explicit stubs, without inventing signatures.
A test pins the XML digest to generated code and
compares source revision hashes to Profile.Version. Existing
internal/ass/spec evidence validators remain authoritative.

Optional source-checkout tests independently extract actual dispatch order
from ref/renderers/* at the pinned revisions. The build and normal tests
do not require those ignored local checkouts. Keep the hand-audited dispatch
tables synchronized with those tests when pinned code changes. Run:

    go generate ./internal/ass/renderer
    (cd internal/ass/renderer && go run ./cmd/genmatrix -check)
    go test ./...
    go test -race ./internal/ass/renderer/...
    go vet ./...

The string `blend` begins with `bl`, so the `be` check cannot match it.
`blend` is checked before the generic `b` branch and has a reachable apply
branch (`RTS.cpp:2551, 3716–3730`). Both traditional renderers recognize the
shorter `b` prefix.
