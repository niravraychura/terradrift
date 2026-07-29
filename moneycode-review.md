# MonkeyCode Repository Review: TerraDrift

## Summary

TerraDrift is a well-architected, security-conscious Go CLI application for Terraform drift detection. The codebase follows a clean pipeline architecture with 15 focused internal packages. This review covers code quality, security, and performance findings.

---

## Checklist

### Code Quality

- [ ] **CQ-01: Refactor oversized CLI entry point**
  - **File:** `cmd/terradrift/main.go` (1535 lines)
  - **Issue:** The `newScanCommand` function contains ~300 lines of flag definitions and a ~300-line `RunE` handler. The CLI layer has too many responsibilities bundled in a single file.
  - **Fix:** Extract the `RunE` handler logic from `newScanCommand` into a dedicated handler struct or a separate file (e.g., `cmd/terradrift/scan.go`). Similarly extract `newScanAllCommand` into its own file. The `writeScanReport`, `deliverNotifications`, `enrichReport`, and `runDeliveries` functions should move to internal packages or be grouped in a separate helper file.

- [ ] **CQ-02: Eliminate duplicated `limitedBuffer` struct**
  - **Files:**
    - `internal/cost/cost.go:85`
    - `internal/policy/policy.go:68`
    - `internal/audit/audit.go:84`
  - **Issue:** The same `limitedBuffer` struct with identical `Write` method is defined in three packages.
  - **Fix:** Extract into a shared `internal/ioutil` or `internal/limits` package and import it from `cost`, `policy`, and `audit`.

- [ ] **CQ-03: Eliminate duplicated `readLimitedFile` function**
  - **Files:**
    - `cmd/terradrift/main.go:83`
    - `internal/terraform/cli_runner.go:175`
  - **Issue:** Two identical implementations of a bounded file reader.
  - **Fix:** Export from one of the internal packages (or a shared `internal/ioutil` package) and import in both places.

- [ ] **CQ-04: Refactor config-to-CLI-flag mapping**
  - **File:** `cmd/terradrift/main.go:516-617`
  - **Issue:** ~100 lines of repetitive per-field `if !cmd.Flags().Changed(...)` checks. Adding a new config field requires code in two places. This pattern is brittle and verbose.
  - **Fix:** Use a config-overlay helper that merges config values into CLI flags using `cmd.Flags().Changed()` checks via a mapping table or reflection-based approach.

- [ ] **CQ-05: Fix index-based merging in `enrichReport`**
  - **File:** `cmd/terradrift/main.go:972-975`
  - **Issue:** Cost and audit enrichment results are merged by slice index: `scanReport.ResourceChanges[i].CostImpact = costReport.ResourceChanges[i].CostImpact`. If either enrichment operation reorders or drops elements, the merge is silently wrong.
  - **Fix:** Merge by `Address` key using a map lookup instead of index-based access.

- [ ] **CQ-06: Replace `panic` with error handling in flag registration**
  - **File:** `cmd/terradrift/main.go:172-174`
  - **Issue:** `cmd.MarkFlagRequired(name)` failure triggers `panic(err)`. While unreachable in normal use, this will crash the process instead of producing a recoverable error.
  - **Fix:** Check the error and wrap it appropriately, or move to an `init()` function that calls `log.Fatal` for a controlled exit.

- [ ] **CQ-07: Add missing test coverage**
  - **Packages with no or minimal tests:**
    - `internal/validation` -- no test file exists
    - `internal/report` -- has `remediation_test.go` but no tests for `ignore.go`, `throttle.go`, `approval.go`
    - `internal/config` -- has `config_test.go` but output showed `[no tests to run]`
    - `internal/notify` -- has test files but all showed `[no tests to run]` in filtered run
  - **Fix:** Add unit tests for all exported functions, especially validation, ignore rules, throttling logic, and notification formatting. Add integration-style tests with mock HTTP servers for notifiers.

- [ ] **CQ-08: Add edge-case tests for the parser**
  - **File:** `internal/parser/parser.go`
  - **Issue:** The parser handles `resource_drift`, `resource_changes`, `prior_state`, and plan mode selection. Edge cases like malformed JSON, empty plans, and null fields should be tested.
  - **Fix:** Add test cases for: empty JSON, `null` resource_drift, missing prior_state, invalid plan mode, no-op only changes, and mixed data/managed resources.

---

### Security

- [ ] **SEC-01: Narrow the sensitive key redaction rule**
  - **File:** `internal/redact/redact.go:79`
  - **Issue:** `isSensitiveKey` matches any query parameter _containing_ the word `"key"`. This false-positively redacts benign parameters such as `?key_id=abc123`, `?pagerduty_key=pd-12345`, or `?key_name=production`.
  - **Fix:** Use whole-word matching or a more specific pattern. For example, match only standalone parameter names like `api_key`, `access_key`, `secret_key`, `aws_access_key_id`, etc., rather than any parameter containing `key` as a substring.

- [ ] **SEC-02: Validate GitHub token at config time**
  - **Files:** `cmd/terradrift/main.go:735`, `internal/notify/github.go:73`, `internal/notify/github.go:125-129`
  - **Issue:** `GITHUB_TOKEN` is read from the environment at notification time. If the token is missing or invalid, the error only surfaces when delivering notifications, not during configuration loading.
  - **Fix:** Add a config-level flag for `GITHUB_TOKEN` (or validate the environment variable exists and is non-empty) during config validation so the user gets early feedback.

- [ ] **SEC-03: Consider TOCTOU risk in workspace root validation**
  - **File:** `internal/scanner/scanner.go:100-127`
  - **Issue:** `ValidateDirectory` resolves symlinks and validates against the workspace root at check time, but the directory could be changed (e.g., symlink swapped) between validation and the actual `terraform init`/`plan` execution.
  - **Fix:** While this is a common filesystem limitation, consider adding a secondary check after the scan lock is acquired, or using `openat2` with `RESOLVE_NO_SYMLINKS` on Linux.

- [ ] **SEC-04: Add TLS certificate pinning option for enterprise users**
  - **File:** `internal/notify/webhook.go:120-130`
  - **Issue:** The secure webhook client uses default TLS settings without certificate pinning or custom CA support. Enterprise users deploying behind corporate proxies with custom CAs cannot customize the TLS configuration.
  - **Fix:** Add an optional `--webhook-ca-cert` flag that configures a custom root CA pool on the webhook `http.Transport`.

---

### Performance

- [ ] **PERF-01: Avoid sorting all history files before limiting**
  - **File:** `internal/history/history.go:94-121`
  - **Issue:** `LoadRecent` collects all filenames, sorts them all descending, then reads only up to `limit`. With 1000 history files, this sorts 1000 entries just to read 10.
  - **Fix:** Since filenames use an ISO 8601 timestamp prefix (`20060102T150405...`), sort is already alphabetical. Use a min-heap of size `limit` to avoid sorting the full list, or read directory entries in reverse order using `ReadDir(-1)` and stop early.

- [ ] **PERF-02: Consider caching terraform init between scans**
  - **File:** `internal/terraform/cli_runner.go:51-54`
  - **Issue:** `terraform init` runs on every scan invocation. For large configurations with many providers, this adds significant latency as providers are verified each time.
  - **Fix:** Check if `.terraform` directory and lock file already match before running init. Only re-init when the dependency lock file changes. Use `-lockfile=readonly` approach combined with a hash of the lockfile to determine if re-init is needed.

- [ ] **PERF-03: Parameterize the scan lock for distributed runners**
  - **File:** `internal/scanner/scanner.go:192-212`
  - **Issue:** The scan lock is file-based (O_EXCL on a local lock file). This only prevents concurrent scans within a single host. In distributed CI environments, multiple runners can scan the same directory simultaneously.
  - **Fix:** (Already noted with `// ponytail:` at line 210) -- add support for an external lock provider (e.g., Redis, PostgreSQL advisory lock, or cloud object lock). Accept a `--lock-backend` flag.

- [ ] **PERF-04: Add memory pooling for frequent allocations**
  - **File:** `internal/parser/parser.go:107-127`
  - **Issue:** `relevantChanges` allocates a new `[]report.ResourceChange` slice and new `[]string` slices for `Actions` on every parse. For scans with thousands of resources, this causes GC pressure.
  - **Fix:** Consider using `sync.Pool` for intermediate `ResourceChange` slices, or pre-allocate with a capacity hint from `len(source)`.

- [ ] **PERF-05: Limit concurrent file opens in history loading**
  - **File:** `internal/history/history.go:94-121`
  - **Issue:** `LoadRecent` opens and reads files sequentially. For batch-sized limits (e.g., 100), this is fine. But if used with higher limits, sequential file I/O may become a bottleneck.
  - **Fix:** Consider using a bounded pool of goroutines to read history files concurrently when `limit > 50`.

---

## Remediation Priority

| Priority | Issue ID | Description |
|----------|----------|-------------|
| High | CQ-01 | Oversized CLI file hurts maintainability and testability |
| High | CQ-05 | Index-based merging can produce incorrect reports |
| Medium | CQ-02 | Duplicated limitedBuffer across 3 packages |
| Medium | CQ-03 | Duplicated readLimitedFile across 2 packages |
| Medium | CQ-04 | Brittle config-to-CLI mapping |
| Medium | CQ-07 | Missing test coverage in core packages |
| Medium | SEC-01 | Overly broad redaction rule for query params |
| Medium | PERF-03 | Scan lock limited to single host |
| Low | CQ-06 | panic vs error in flag registration |
| Low | CQ-08 | Missing parser edge-case tests |
| Low | SEC-02 | Late validation of GitHub token |
| Low | SEC-03 | TOCTOU in workspace root validation |
| Low | SEC-04 | No custom CA support for enterprise |
| Low | PERF-01 | Unnecessary full sort in history loading |
| Low | PERF-02 | terraform init on every scan |
| Low | PERF-04 | No memory pooling for parser allocations |
| Low | PERF-05 | Sequential history file reads |
