# Quality Findings — 2026-04-01

Run: `task quality` — golangci-lint v2, config: .golangci.yml

Total: 134 issues (cyclop:20, errcheck:50, funlen:3, gocognit:41, gocritic:3, gosec:16, staticcheck:1)

Coverage: 57.1% total
Low-coverage packages: cmd/gen-schema(0%), cmd/mutercim(0%), docker(18.4%), cli(26.8%), ocr(48.3%), pipeline(49.3%), input(60%), reader(66.7%)

---

## Priority 1 — Quick wins (easy, high-value)

### staticcheck (1) — DONE ✓ (commit ed6fa0f)
- [x] `internal/ocr/qari.go:198` S1016: convert struct literal to type conversion

### gocritic (3) — DONE ✓ (commit d029aac)
- [x] `internal/display/tty.go:397` ifElseChain → switch
- [x] `internal/display/tty.go:422` ifElseChain → switch
- [x] `internal/reader/debug.go:135` ifElseChain → switch

---

## Priority 2 — Security (gosec, production code)

### File/dir permissions (G301/G302/G306)
- [ ] `internal/cli/root.go:69` G302: file perm 0700, expect ≤0600
- [ ] `internal/workspace/init.go:52` G301: dir perm 0755, expect ≤0750
- [ ] `internal/workspace/init.go:59` G306: file perm 0644, expect ≤0600
- [ ] `internal/workspace/init.go:65` G306: file perm 0644, expect ≤0600
- [ ] `internal/pipeline/cut.go:99` G301: dir perm 0755, expect ≤0750
- [ ] `internal/reader/debug.go:92` G301: dir perm 0755, expect ≤0750

### Other security
- [ ] `internal/apiclient/client.go:212` G404: math/rand instead of crypto/rand
- [ ] `internal/cli/root.go:83` G706: log injection via taint
- [ ] `internal/docker/docker.go:41,52,76` G204: subprocess with variable

### gosec in tests (lower risk, may nolint)
- [ ] `internal/input/loader_test.go:15` G306: test WriteFile perm
- [ ] `internal/provider/provider_extra_test.go:10` G101: hardcoded credentials (test key)
- [ ] `internal/workspace/workspace_test.go:43,75` G304: file inclusion (test paths)

---

## Priority 3 — Unchecked errors (errcheck, production code)

### display package
- [ ] `internal/display/line.go:67,92,103` fmt.Fprintf
- [ ] `internal/display/render.go:68` fmt.Fprintln
- [ ] `internal/display/status.go:26,40` fmt.Fprintln
- [ ] `internal/display/tty.go:205,266` io.WriteString

### ocr package
- [ ] `internal/ocr/qari.go:159,222,289` resp.Body.Close
- [ ] `internal/ocr/qari.go:208,275` writer.Close
- [ ] `internal/ocr/qari.go:364` f.Close
- [ ] `internal/ocr/qari.go:381` l.Close

### pipeline/reader
- [ ] `internal/pipeline/atomic.go:9` os.Remove
- [ ] `internal/reader/debug.go:101` os.Remove
- [ ] `internal/reader/debug.go:104` f.Close

---

## Priority 4 — Unchecked errors (errcheck, test code)

- [ ] `internal/apiclient/client_extra_test.go:21,107` w.Write
- [ ] `internal/cli/env_test.go:148,158,167,168` os.Unsetenv/Setenv
- [ ] `internal/config/config_test.go:14,15` os.Chdir
- [ ] `internal/config/schema_test.go:74` json.Unmarshal
- [ ] `internal/input/loader_extra_test.go:48` os.MkdirAll
- [ ] `internal/knowledge/glossary_test.go:157` os.WriteFile
- [ ] `internal/knowledge/loader_test.go:284,285,292` os.MkdirAll/WriteFile
- [ ] `internal/ocr/ocr_test.go:148,157,172,177,220,232` json.Encoder.Encode/fmt.Sscanf
- [ ] `internal/provider/failover_extra_test.go:133` chain.Translate
- [ ] `internal/provider/failover_test.go:253` chain.Translate
- [ ] `internal/provider/gemini_test.go:164` w.Write
- [ ] `internal/provider/ollama_test.go:35,80` json.Decoder.Decode
- [ ] `internal/reader/debug_test.go:58` f.Close
- [ ] `internal/rebuild/rebuild_test.go:28,46,65,81,302` os.Chtimes/MkdirAll/Remove
- [ ] `internal/workspace/workspace_test.go:93` unchecked return

---

## Priority 5 — Complexity (production code only)

High-complexity functions worth refactoring (gocognit > 30 or cyclop in real code):

- [ ] `internal/pipeline/read.go:93` readOneInput — cognitive 103 (!!)
- [ ] `internal/pipeline/translate.go:95` translateOneInput — cognitive 65
- [ ] `internal/cli/make.go:22` newAllCmd — cognitive 65
- [ ] `internal/pipeline/ocr.go:87` ocrOneInput — cognitive 60
- [ ] `internal/cli/clean.go:135` newCleanCmd — cognitive 50
- [ ] `internal/cli/auto.go:68` runPrerequisites — cognitive 52
- [ ] `internal/pipeline/write.go:71` writeOneInput — cognitive 41
- [ ] `internal/pipeline/solve.go:66` solveOneInput — cognitive 30
- [ ] `internal/cli/translate.go:16` newTranslateCmd — cognitive 35
- [ ] `internal/cli/write.go:15` newWriteCmd — cognitive 29
- [ ] `internal/rebuild/rebuild.go:36` NewestMtime — cognitive 32
- [ ] `internal/pipeline/layout.go:104` layoutOneInput — cognitive 46
- [ ] `internal/cli/read.go:20` newReadCmd — cognitive 35
- [ ] `internal/cli/root.go:29` NewRootCmd — cognitive 28

---

## Priority 6 — Complexity (test code, lower priority)

- [ ] Various test functions with cyclop/gocognit violations — accept or split test cases

---

## Priority 7 — Function length (funlen)

- [ ] `internal/cli/env_test.go:9` TestParseEnvLines — 130 lines (test, low priority)
- [ ] `internal/reader/region_reader.go:54` ReadRegionPage — 88 lines
- [ ] `internal/reader/region_reader.go:163` ReadRegionPageWithOCR — 90 lines
