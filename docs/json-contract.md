# JSON contract, version 1

The Node helper prints exactly one JSON object. It contains `schema_version: 1`, `inventory_mode: "installed"`, dependency/source `stages`, `packages`, `edges`, `imports`, `coverage`, and `warnings`. Logs belong on stderr. Go rejects incompatible schemas, missing arrays, and dangling evidence references.

A package ID is its project-relative installation location, such as `node_modules/demo-parent/node_modules/demo-parser`. The root application's ID is `.` in dependency edges and paths; it is not queried as a dependency. Two installations keep separate node IDs even when names or versions match.

| Helper field | Meaning |
| --- | --- |
| `packages[].name`, `version` | Real package name and exact installed version |
| `packages[].direct` | A root dependency edge points to this node |
| `packages[].context` | Production/development paths, optional/peer evidence, unknown flag |
| `packages[].dependency_path` | One deterministic representative path, starting at `.` |
| `packages[].queryable` | A supported standard npm package/version identity |
| `edges[].from`, `to` | Dependency node IDs; empty `to` means missing dependency |
| `edges[].type`, `valid`, `error` | Declared relationship type and resolution evidence |
| `imports[].node_id` | Resolved installation ID, or empty when unknown |
| `imports[].file`, `line` | Project-relative source file and one-based line |
| `imports[].type_only` | Type evidence rather than runtime import evidence |
| `stages.*.status` | `complete`, `partial`, or `failed` |
| `warnings[]` | Stage, machine-readable code, human-readable coverage limitation |

The Go report wraps analysis with report/rules versions, scan timestamp, target, overall status, findings, and errors. It adds the vulnerability stage and query counts. Stages that never started are `skipped`.

Each finding includes the package node, normalized advisory and raw severity evidence, matched imports, import state, unknown affected-function reachability, priority, context order, rule IDs, reasons, and limitations. Advisory ranges preserve OSV events; a fixed boundary is information to review, not a universal upgrade recommendation.

Import state is `detected` for at least one runtime import, `not_detected_in_scanned_files` for complete supported source coverage without runtime evidence, and `unknown` when evidence is absent and source coverage is incomplete. Type-only imports remain visible but do not count as runtime use. All three leave affected-function reachability unknown.

Overall `complete` requires all supported stages to complete. `partial` retains useful evidence despite coverage/lookup errors. `failed` means target validation or the analysis helper could not establish an inventory. The CLI returns 2 for both partial and failed reports, even when findings exist.

Do not decide success from `findings: []` alone: always inspect `status`, stage statuses, coverage, warnings, and errors. Read `internal/model/model.go` for the exact Go field definitions and `internal/analyzer/analyzer.go` for protocol checks.
