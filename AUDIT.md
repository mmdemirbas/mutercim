# Codebase Audit

## Correctness

### [CORRECTNESS] API key logged in plaintext via Gemini URL
- **Severity**: critical
- **File**: internal/provider/gemini.go:122, internal/apiclient/client.go:97-100
- **Issue**: Gemini embeds the API key as a URL query parameter (`?key=...`). The apiclient logs the full URL on retries: `c.logger.Info("retrying request", "url", req.URL)`. This writes the API key to mutercim.log.
- **Fix**: Either move the API key to an Authorization header, or sanitize URLs before logging (strip query parameters containing "key").

### [CORRECTNESS] Panic on empty TargetLangs in Write phase
- **Severity**: critical
- **File**: internal/pipeline/write.go:41
- **Issue**: `cfg.Book.TargetLangs[0]` accessed without length check. If TargetLangs is empty (config bug, manual edit), this panics with array index out of bounds.
- **Fix**: Add `if len(cfg.Book.TargetLangs) == 0 { return error }` guard at top of Write().

### [CORRECTNESS] Data loss in footnote entry number conversion
- **Severity**: high
- **File**: internal/translation/translator.go:109-123
- **Issue**: `convertTranslatedFootnotes` only keeps the first entry number when the API returns `entry_numbers: [1, 2, 3]`. The model's `TranslatedFootnote.EntryNumber` is a single int, so references to entries 2 and 3 are silently dropped.
- **Fix**: Either change `TranslatedFootnote` to support `[]int`, or create one footnote record per entry number.

### [CORRECTNESS] Footnote model inconsistency across pipeline
- **Severity**: high
- **File**: internal/model/entry.go:13-19 vs :69-73
- **Issue**: Input `Footnote` has both `EntryNumber *int` and `EntryNumbers []int`. Output `TranslatedFootnote` has only `EntryNumber int`. Data narrows as it flows through the pipeline — multi-entry footnotes lose references.
- **Fix**: Align the translated footnote model with the input model.

### [CORRECTNESS] Log file handle never closed
- **Severity**: high
- **File**: internal/cli/root.go:46-48
- **Issue**: `os.OpenFile` for mutercim.log is called in PersistentPreRunE but the file handle `f` is never closed. It stays open for the entire process lifetime. No cleanup on context cancellation.
- **Fix**: Store the file handle and close it in a PostRunE or defer.

### [CORRECTNESS] signal.NotifyContext cancel function discarded
- **Severity**: medium
- **File**: internal/cli/root.go:61-62
- **Issue**: `ctx, cancel := signal.NotifyContext(...)` — cancel is assigned to `_`. This means the signal notification is never unregistered. For a CLI tool this is harmless (process exits), but violates the signal.NotifyContext contract.
- **Fix**: Call cancel in a deferred cleanup or at process exit.

### [CORRECTNESS] Write phase sourceInputs variable unused
- **Severity**: low
- **File**: internal/pipeline/write.go:40-48
- **Issue**: `sourceInputs` is computed but never used. The actual rendering loop uses `inputs` from per-lang discovery. The `sourceInputs` fallback code is dead code.
- **Fix**: Remove the sourceInputs discovery or use it for the source language markdown rendering.

## Robustness

### [ROBUSTNESS] RateLimiter.Close() panics on double-close
- **Severity**: high
- **File**: internal/apiclient/ratelimit.go:55-56
- **Issue**: `Close()` calls `close(rl.refillStop)`. If called twice (e.g., deferred close + explicit close), it panics. The `Client.Close()` calls `RateLimiter.Close()` directly.
- **Fix**: Use `sync.Once` to make Close() idempotent.

### [ROBUSTNESS] Pages phase returns nil even when all inputs fail
- **Severity**: medium
- **File**: internal/pipeline/pages.go:28-57
- **Issue**: `Pages()` logs errors per input but always returns nil. The caller (`make` command) proceeds to the read phase even if pagination failed entirely.
- **Fix**: Track failure count and return error if all inputs failed.

### [ROBUSTNESS] Translate phase silently does nothing with empty TargetLangs
- **Severity**: medium
- **File**: internal/pipeline/translate.go:62
- **Issue**: If `cfg.Book.TargetLangs` is empty, the for-loop body never executes and Translate returns nil (success). Caller thinks translation succeeded.
- **Fix**: Add early return with error if no target languages configured.

### [ROBUSTNESS] writePageOutput uses non-atomic write
- **Severity**: medium
- **File**: internal/pipeline/translate.go:317-318
- **Issue**: `os.WriteFile()` without tmp+rename. If Ctrl+C hits during write, the markdown file could be partially written. CLAUDE.md requires atomic writes for all state files.
- **Fix**: Use the same tmp+rename pattern as saveTranslatedPage.

### [ROBUSTNESS] writePageOutput uses string concatenation in loop
- **Severity**: low
- **File**: internal/pipeline/translate.go:312-315
- **Issue**: `content += l + "\n"` in a loop. For pages with many entries this creates O(n^2) allocations.
- **Fix**: Use `strings.Builder` or `strings.Join`.

## Security

### [SECURITY] API key in Gemini URL parameter
- **Severity**: critical
- **File**: internal/provider/gemini.go:122
- **Issue**: Same as the first correctness finding. The API key appears in the URL, which gets logged to mutercim.log and could appear in error messages.
- **Fix**: Use a request header or sanitize logged URLs.

### [SECURITY] No path traversal validation on config inputs
- **Severity**: low
- **File**: internal/config/config.go:282-290
- **Issue**: `Validate()` checks if input paths exist but doesn't verify they're within the workspace. A config with `inputs: [/etc/passwd]` would be accepted. In practice this is harmless since mutercim only reads images, but it's not validated.
- **Fix**: Consider validating that input paths are relative or within workspace root.

## Consistency

### [CONSISTENCY] init() function violates CLAUDE.md
- **Severity**: medium
- **File**: internal/cli/root.go:270
- **Issue**: `func init() { cobra.AddTemplateFunc("formatGroupedCommands", formatGroupedCommands) }` — CLAUDE.md says "No init() functions anywhere".
- **Fix**: Move the template function registration into NewRootCmd().

### [CONSISTENCY] CLI description still says "Arabic into Turkish"
- **Severity**: low
- **File**: internal/cli/root.go:28-30
- **Issue**: `Short: "Translate Arabic Islamic scholarly books into Turkish"` — hardcoded languages despite now supporting configurable source/target languages.
- **Fix**: Update to "Translate scholarly books between languages" or make it generic.

### [CONSISTENCY] fmt.Printf used for CLI output instead of cmd.OutOrStdout()
- **Severity**: low
- **File**: internal/cli/init.go, status.go, validate.go, config_cmd.go, knowledge_cmd.go
- **Issue**: CLI commands use `fmt.Printf` directly to stdout instead of cobra's `cmd.OutOrStdout()`. This makes commands harder to test and doesn't respect cobra's output redirection.
- **Fix**: Use `cmd.OutOrStdout()` for all output in command handlers.

### [CONSISTENCY] Provider registry exists but is unused
- **Severity**: low
- **File**: internal/provider/registry.go, internal/cli/read.go:152-182
- **Issue**: A proper provider registry exists in `provider/registry.go` but the CLI uses a hardcoded switch statement in `createProvider()`. The registry pattern is redundant code.
- **Fix**: Either use the registry or remove it.

## Missing Features

### [MISSING] knowledge diff command not implemented
- **Severity**: medium
- **File**: internal/cli/knowledge_cmd.go
- **Issue**: SPEC.md specifies `mutercim knowledge diff` to show differences between staged and persistent knowledge. Only `list`, `staged`, and `promote` exist.
- **Fix**: Implement the diff subcommand.

### [MISSING] Surya provider not implemented
- **Severity**: medium
- **File**: internal/provider/
- **Issue**: SPEC.md lists Surya as a supported provider for local OCR + AI structural parsing. No surya.go exists.
- **Fix**: Implement or document as "planned".

### [MISSING] Concurrency in read phase not implemented
- **Severity**: low
- **File**: internal/pipeline/read.go, internal/config/config.go:34
- **Issue**: `ReadConfig.Concurrency` field exists in config and is settable via `--concurrency` flag, but the read pipeline processes pages sequentially. The concurrency value is never used.
- **Fix**: Implement parallel page processing with a worker pool, or remove the field.

## Test Quality

### [TESTS] Pipeline solve/translate tests don't verify JSON content deeply
- **Severity**: low
- **File**: internal/pipeline/solve_test.go, translate_test.go
- **Issue**: Tests verify files exist and contain valid JSON, but don't deeply assert the content structure (e.g., that solver actually resolved abbreviations, that translation context was injected).
- **Fix**: Add assertions on specific fields of the output JSON.

### [TESTS] No test for multi-input scenarios
- **Severity**: low
- **File**: internal/pipeline/*_test.go
- **Issue**: All pipeline tests use a single input. No tests verify the per-input namespacing works correctly with 2+ inputs that both have page 1.
- **Fix**: Add a test with two input stems.

### [TESTS] CLI package at 4.8% coverage
- **Severity**: low
- **File**: internal/cli/
- **Issue**: CLI commands are essentially untested. While they're mostly wiring, some contain non-trivial logic (page range resolution, provider creation, preflight checks).
- **Fix**: Test extractable logic (resolveAPIKey, createProvider, parseLogLevel) separately. Consider integration tests for key commands.

## Summary

- **Critical**: 2 (API key logging, panic on empty TargetLangs)
- **High**: 4 (footnote data loss, footnote model inconsistency, log file leak, double-close panic)
- **Medium**: 6 (signal cancel, pages returns nil, empty TargetLangs, non-atomic write, knowledge diff missing, surya missing)
- **Low**: 10 (consistency, performance, test quality)
