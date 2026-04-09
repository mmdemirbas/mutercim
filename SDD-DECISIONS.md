# Decisions Log

## Deferred items — rationale

### P4-3 — Global mutable CLI flags
Ambiguity: the plan suggests capturing flags via closures instead of package-level vars.
Decision: Defer. The current approach is idiomatic Cobra, works correctly, has no concurrency issues (CLI is single-process), and refactoring would touch every command constructor with high regression risk for zero behavioral benefit.
Status: RESOLVED

### P4-9 — readContext/translateContext duplication
Ambiguity: both contexts share ~10 identical fields and methods.
Decision: Defer. Extracting a shared `phaseContext` base would be a large refactor. The two contexts differ meaningfully in their page processing logic. Maintenance cost is real but the risk of introducing bugs in the orchestration layer outweighs the benefit.
Status: RESOLVED

### P4-13 — Migrate yaml.v3 import path
Ambiguity: `gopkg.in/yaml.v3` vs `go.yaml.in/yaml/v3`.
Decision: Defer. API is identical, only import path changes. Mechanical change touching config, knowledge, cli, workspace, and cmd/gen-schema. No behavioral benefit. Can be done in a dedicated commit when convenient.
Status: RESOLVED

### P4-18 — Stream OCR multipart body
Ambiguity: whether the 2x memory overhead matters in practice.
Decision: Defer. Page images are typically 1-5MB. Peak memory increase is negligible. The `io.Pipe` approach requires cross-goroutine error handling that adds complexity disproportionate to the benefit.
Status: RESOLVED

### P4-21 through P4-32 — Complexity scores
Ambiguity: how much refactoring to do on high-complexity functions.
Decision: Defer all except P4-33/P4-34 (reader response parsing dedup). The remaining functions are already decomposed into helper methods where practical. Their complexity is inherent to the orchestration role (multi-phase dispatch, per-page processing, display callbacks). Further decomposition would hide the linear execution flow.
Status: RESOLVED

### P3-4 — Clock injection in rate limiter tests
Ambiguity: whether to refactor the rate limiter to accept a clock interface.
Decision: Defer. The test is slow (61s) but correct. Injecting a clock adds an interface and constructor parameter to production code for test-only benefit. Can be revisited if test suite time becomes a bottleneck.
Status: RESOLVED

### P3-9 — OCR package coverage
Ambiguity: how to test Docker-dependent code without Docker.
Decision: Defer the container lifecycle tests. The HTTP endpoints (`RecognizeRegions`, `RecognizeFullPage`) could be tested with `httptest.NewServer` but that requires extracting the HTTP client or making the base URL configurable. P3-2 already added `loadOCRPage` round-trip tests. Remaining coverage improvement requires mock infrastructure.
Status: RESOLVED

### P3-17 — Python pip package pinning
Ambiguity: what versions to pin to.
Decision: Defer. Requires running each Docker image, capturing `pip freeze` output, and verifying the pinned versions work together. Cannot be done without testing the Docker images. Should be done during a release preparation cycle.
Status: RESOLVED
