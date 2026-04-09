# SPEC: Fix remaining P2-P4 items

## Scope

All remaining open items from PLAN.md P2 through P4. P0 and P1 are already complete.
P5 (features) and P6 (exploratory) are out of scope.

Items verified as already done (P2-14, P2-15, P2-16) will be moved to Done in PLAN.md.

Items deferred (rationale in Decisions):
- P4-3: global CLI flags — idiomatic Cobra, large refactor, no bug
- P4-9: readContext/translateContext duplication — large refactor, maintenance-only benefit
- P4-13: yaml.v3 import migration — mechanical but touches many files, no behavioral change
- P4-18: streaming OCR multipart — moderate complexity, matters only for >50MB images
- P4-21 through P4-32: complexity scores — inherent to orchestration roles, already decomposed where practical

## Slices

### Slice 1: Security hardening

**P2-4** — Change `0750` to `0o750` in `pipeline/cut.go` and `reader/debug.go`.

**P2-6** — In `cli/root.go`, sanitize CLI args before logging by stripping newlines
and control characters. Replace the raw `strings.Join(os.Args[1:], " ")` with a
sanitized version.

**P2-11** — In `provider/failover.go`, compute `nextName` while holding the mutex
to eliminate the TOCTOU window. Consolidate the lock acquisition: check exhaustion
and find next in a single critical section.

**P2-12** — In `ocr/qari.go`, wrap the `freePort` + `docker run` sequence in a
retry loop (max 3 attempts) so that if the port is grabbed between `freePort` and
Docker bind, the start retries with a new port.

### Slice 2: Observability

**P2-22** — In `pipeline/translate.go` `recordSuccess`, add `"regions"` count to
the existing log line. Add a `startTime` field to `translateContext`, record
`time.Now()` before the API call, and log `"elapsed_ms"` on success.

### Slice 3: Performance micro-optimizations

**P4-16** — In `pipeline/translate.go` `processTranslatePage` and
`pipeline/read.go` `loadPagePrereqs`, hoist `cfg.ResolveKnowledgePaths(ws.Root)`
out of the per-page loop. Compute once in the parent function and pass the result.

**P4-17** — `buildInputPageMap` is called in `layout.go`, `ocr.go`, and `read.go`
top-level functions. Move the call to a shared location or compute once at the start
of each phase and pass the map through.

**P4-20** — In `knowledge/loader.go`, add a `cachedKey` field to `Entry`. Compute
`mergeKey` once in `parseEntry` and store it. `mergeEntry` uses the cached key
instead of recomputing.

### Slice 4: Knowledge loader type safety

**P4-5** — In `knowledge/loader.go`, replace `map[string]interface{}` in `rawFile`
with a typed struct. Define `rawEntry` with typed fields for language forms and note.
Remove the `parseEntry(map[string]interface{})` function and unmarshal directly into
the typed struct. Preserve backward compatibility with existing YAML glossary files.

### Slice 5: Provider interface cleanup

**P4-6** — Extract a `ModelTracker` interface from `FailoverChain` with methods:
`ActiveModel(needsVision bool) string`, `LastUsedModel() string`,
`SetRetryCallback(fn)`, `SetOnFailover(fn)`. Implement it on `FailoverChain`.
Update `pipeline/read.go` and `pipeline/translate.go` to use the interface instead
of type-asserting `*FailoverChain`.

**P4-11** — Make `OnFailover` immutable after construction. Replace the public field
with a `SetOnFailover` method (part of `ModelTracker` interface). Store the callback
behind the existing mutex.

### Slice 6: Layout dependency inversion

**P4-10** — `pipeline/layout.go` imports `reader` for `GenerateDebugOverlay`. Move
`GenerateDebugOverlay` to an `internal/imaging` package (or similar) that both
`reader` and `pipeline` can import without circular dependency.

### Slice 7: Reader response parsing dedup

**P4-33/P4-34** — Extract the repeated JSON parse + fallback block in
`reader/region_reader.go` into a shared helper function. Both `ReadRegionPage` and
`ReadRegionPageWithOCR` have identical blocks for: ExtractJSON, Unmarshal, build
fallback RegionPage with warnings. Extract to a `parseRegionResponse` helper.

### Slice 8: Test coverage improvements

**P3-3** — Add test for `cli.Execute()` error path (invalid flag → non-nil error).

**P3-10** — Add unit tests for untested pure functions in the pipeline package:
`contiguousRanges`, `buildTranslateContext`, `buildOCRRegions`, `fileStem`.

**P3-12** — Sweep test files and replace `_ = os.MkdirAll(...)` and similar with
`t.Fatal` checks. Focus on setup code where silent failures produce confusing
downstream errors.

### Slice 9: CI/CD hardening

**P3-18** — Add coverage reporting to CI. Add `go test -coverprofile` and a threshold
check step. Use 50% as initial threshold.

**P3-20** — Pin GitHub Actions to commit SHAs instead of mutable version tags.

**P3-21** — Add `govulncheck` step to CI workflow.

### Slice 10: PLAN.md cleanup

Move P2-14, P2-15, P2-16 to Done (verified already fixed). Move all items completed
in this session to Done. Update remaining item notes.

## Acceptance criteria

- `go build ./...` clean
- `go vet ./...` clean
- `go test ./...` all pass
- Each slice committed separately with descriptive message
- PLAN.md updated after each slice
- No new dependencies added without justification
