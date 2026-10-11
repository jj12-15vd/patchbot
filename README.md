# PatchBot

PatchBot is a local CLI prototype that finds known vulnerabilities in an npm project's installed dependencies and explains how to prioritize their review.

It combines advisory information with dependency paths, production/development context, and static import evidence. Its goal is to help individual developers and small teams understand **what was found, where it appears, and why it deserves attention**.

PatchBot provides evidence for review. It does not automatically patch dependencies or determine whether a vulnerability is exploitable.

## Core features

| Feature | What the prototype provides |
| --- | --- |
| Vulnerability detection | OSV lookups for exact installed package versions, advisory details, severity evidence, references, and supplied fix information |
| Dependency context | Direct/transitive relationships, representative dependency paths, production/development paths, and optional/peer information |
| Import evidence | Supported static imports, re-exports, and literal unshadowed `require()` calls with file names and line numbers |
| Explained prioritization | Review priorities, deterministic ordering, rule IDs, reasons, and explicit limitations |
| Reporting | Terminal and JSON output with scan status, stage status, coverage counts, warnings, and errors |

The scanner preserves mixed dependency contexts and separate installations of different package versions. A package used through both production and development paths is not labelled development-only.

## Requirements

To build and install PatchBot:

- **Go 1.25 or newer**.
- **Node.js 24 or newer** and npm.
- `make` and a POSIX shell for the supplied build and installation commands.
- Network access to download the tool's dependencies during setup.

To run an already-built distribution, Go is not required, but Node.js is still required. Live vulnerability lookups need access to `https://api.osv.dev`.

The target project must contain:

- A valid `package.json`.
- An npm `package-lock.json` with `lockfileVersion` **2 or 3** and a package inventory.
- An existing `node_modules` directory with the project's dependencies installed.

PatchBot scans the **installed dependency inventory**, not just declared version ranges. It does not install dependencies or execute the target project's source code or lifecycle scripts. Installation/lockfile discrepancies are reported as coverage limitations.

## Quick start

Run these commands from the PatchBot repository root:

```sh
make setup
make install
patchbot --version
patchbot --help
```

`make setup` downloads the Go dependencies and installs the Node helper's dependencies. `make install` builds the distribution and installs:

```text
~/.local/bin/patchbot                  command shortcut
~/.local/share/patchbot/patchbot       Go executable
~/.local/share/patchbot/analyzer/      Node helper and its dependencies
```

If `patchbot` is not found, add this line to your shell configuration—for example, `~/.zshrc` when using zsh—and open a new terminal:

```sh
export PATH="$HOME/.local/bin:$PATH"
```

`PATH` is the list of directories your shell searches for commands. The installer uses your user directory, so administrator access is not required.

For a custom installation location:

```sh
make install PREFIX=/absolute/custom/prefix
```

Add that prefix's `bin` directory to your PATH. Run `make install` again after changing PatchBot's code to update the installed version.

### Run without installing

```sh
make setup
make build
./dist/patchbot --help
./dist/patchbot scan --timeout 60s /path/to/npm-project
```

Keep `dist/patchbot` and `dist/analyzer` together when copying the distribution. Copying only the Go binary leaves out the analysis helper.

## Usage

```text
patchbot scan [--format text|json] [--timeout 30s] <path>
patchbot --help
patchbot --version
```

Put scan options **before** the project path. Exactly one path is required; `.` means the current directory.

### Scan the current project

From an npm project with dependencies already installed:

```sh
patchbot scan .
```

If dependencies are missing, prepare the project separately using its existing supported lockfile—for example, `npm ci --ignore-scripts`—then scan it.

### Scan another project

```sh
patchbot scan --timeout 60s /path/to/npm-project
```

Quote paths containing spaces:

```sh
patchbot scan --timeout 60s "/path/to/my project"
```

### Save a JSON report

```sh
patchbot scan --format json --timeout 60s . > report.json
```

For scan reports, stdout contains only the requested report; command diagnostics go to stderr. Invalid command syntax produces diagnostics rather than a scan report. See the [JSON contract](docs/json-contract.md) for report fields.

### Set the scan deadline

| Option | Default | Purpose |
| --- | --- | --- |
| `--format` | `text` | Select `text` or `json` output |
| `--timeout` | `30s` | Set a positive total scan deadline, such as `60s` or `2m` |

`--timeout` limits the scan's work; it is not a delay before starting. Expiration or Ctrl+C cancels pending work. The report marks the scan incomplete or failed, preserves available findings where possible, and returns exit code `2`.

## Understand the results

Inspect the overall scan status before interpreting its findings:

| Status | Meaning |
| --- | --- |
| `complete` | All stages completed within the supported prototype scope |
| `partial` | Useful evidence is available, but some analysis, coverage, or lookups are incomplete |
| `failed` | Target validation or the analysis helper could not establish an inventory |

An empty findings list from a partial or failed scan is **not** a clean result. A complete scan without findings means no known vulnerabilities were found in the queried dependencies; it does not establish that the project is secure.

### Exit codes

| Code | Meaning |
| --- | --- |
| `0` | Complete scan with no active findings; also successful help/version commands |
| `1` | Complete scan with active findings |
| `2` | Invalid input, tool failure, cancellation, or incomplete coverage |

To inspect the code, run `echo $?` immediately after the command. Code `2` takes precedence even when an incomplete scan found vulnerabilities.

### Review priorities

Advisory severity and PatchBot's review priority are separate fields. The `prototype-v1` policy uses these bands:

| Priority | Initial policy |
| --- | --- |
| `manual_review` | Severity or dependency context is unknown; shown in a prominent group |
| `high` | Advisory severity is critical or high |
| `review` | Advisory severity is medium, low, or none |

Within known severity bands, production-path findings with detected runtime imports appear before other production-path findings, followed by development-only findings. High-severity development dependencies retain high priority. Transitive dependencies and packages without detected imports remain visible.

Every finding includes the applicable rule IDs, reasons, and limitations. These rules are a review heuristic, not a measured probability of exploitation. Severity is normalized from recognized labels and supported CVSS 3.0, 3.1, and 4.0 base vectors while preserving the original evidence. Unsupported severity remains unknown.

### Import evidence

- `detected`: at least one supported static runtime import was found.
- `not_detected_in_scanned_files`: supported source analysis completed without a runtime import for that installation.
- `unknown`: no runtime evidence was found and source coverage is incomplete.

Type-only imports remain visible but do not count as runtime imports. An import does not prove that vulnerable code executes, and no detected import does not prove safety—especially for transitive dependencies. Affected-function reachability remains `unknown` in this prototype.

## Try the prototype

### Repeatable demonstration

After `make setup`, run from the repository root:

```sh
make demo
```

The demo creates a temporary project containing **fictional packages and advisories**, runs the real dependency/source analyzer, and uses a local mock OSV server with the real Go scanner. It does not make live advisory requests.

Expected output: seven package nodes, seven queried versions, and five findings demonstrating direct, transitive, development-only, mixed-context, and unknown-severity cases. The demo executable returns `1` because findings exist; `make demo` treats that expected result as success.

After building the demo, try:

```sh
./dist/demo --format json
./dist/demo --outage
./dist/demo --dynamic
```

The JSON demo returns `1`. The outage and dynamic-loading cases return `2` and show incomplete scans. See the [demo guide](docs/demo.md) for the findings to inspect.

### Live smoke scan

After setup, PatchBot's helper is itself a prepared npm project. From the repository root:

```sh
patchbot scan --timeout 90s ./analyzer
```

This queries real OSV data for the helper's installed dependencies. Findings can change as advisory data changes; a live scan is not a deterministic test.

## How it works

```text
Go CLI: arguments, target validation, cancellation
    -> Node helper: installed dependency graph + static import evidence
    -> JSON response validated by Go
    -> Go OSV client: version queries + advisory details
    -> Go priority rules: review bands, ordering, explanations
    -> Terminal or JSON report
```

Source analysis happens locally. OSV receives queried package names, the npm ecosystem identifier, and exact versions; application source code is not sent to OSV.

## Technology stack

| Technology | Responsibility |
| --- | --- |
| Go + standard library | CLI, subprocess control, cancellation, HTTP requests, JSON, orchestration, and reporting |
| Go `flag` | Command option parsing |
| JavaScript + Node.js | Analysis helper runtime |
| `@npmcli/arborist` 9.9.2 | Installed npm dependency graph and relationships |
| `ts-morph` 28.0.0 | JavaScript/TypeScript static import evidence |
| OSV API | Known vulnerability information |
| `github.com/pandatix/go-cvss` 0.6.4 | Supported CVSS vector interpretation |
| Custom Go rules | Explained prioritization and deterministic ordering |
| Go `testing` + Node test runner | Unit and integration checks |
| Makefile + shell installer | Setup, build, test, demo, and user installation |

## Prototype scope and limitations

- One npm project at a time; workspaces, Yarn, pnpm, and other package ecosystems are not supported.
- Standard npm registry package identities are queried. Unsupported sources, missing/invalid dependency edges, and lockfile discrepancies make coverage incomplete.
- Extraneous installed packages are counted but not queried; their presence makes coverage incomplete.
- Source coverage includes `.js`, `.jsx`, `.ts`, `.tsx`, `.mjs`, and `.cjs` application files. It excludes dependency directories, designated generated-output directories, `.d.ts` files, and symlinks.
- Dynamic loading, package import aliases, unresolved imports, parse failures, and unsupported/shadowed `require()` calls produce coverage limitations.
- Dependency metadata describes declared production/development paths, not proof of what is deployed or executed.
- No full execution tracing, affected-function reachability analysis, automatic patching, AI explanations, dashboard, scheduled monitoring, or CI integration is included.

## Development and troubleshooting

Run the automated tests from the repository root:

```sh
make test
```

Tests cover CLI behavior, dependency relationships, duplicate versions, static import evidence, priority rules, OSV pagination/failures, and the complete workflow. Integration tests use local HTTP servers and controlled fixtures rather than live OSV data.

| Problem | What to check |
| --- | --- |
| `patchbot: command not found` | Run `make install` and ensure the installation's `bin` directory is on PATH |
| Missing Node.js or helper dependencies | Install Node.js 24+, then run `make setup` and `make install` from the repository |
| Missing target `node_modules` | Prepare the target project's dependencies before scanning |
| Unsupported lockfile or workspace | Choose a single npm project with a version 2 or 3 lockfile |
| Deadline exceeded or OSV unavailable | Inspect stage errors; check connectivity and consider a longer deadline |
| Partial source/dependency coverage | Read the warnings; do not interpret missing evidence as safety |

Learn the implementation step by step in the [Go CLI walkthrough](docs/go-cli-walkthrough.md). The [implementation plan](PROTOTYPE_PLAN.md), [JSON contract](docs/json-contract.md), and [demo guide](docs/demo.md) explain the architecture, interfaces, and validation cases.
