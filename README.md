# PatchBot CLI prototype

PatchBot scans one installed npm project, queries known dependency vulnerabilities through OSV, collects dependency and static import evidence, and explains its review priorities. It does not patch dependencies or determine exploitability.

The prototype uses a Go CLI with a Node.js analysis helper. Requires **Go 1.25+** for building and **Node.js 24+ with npm** for setup and scanning. This is a development prototype, not a complete security assessment.

Start with the [Go CLI walkthrough](docs/go-cli-walkthrough.md) to learn the command layer step by step. The [implementation plan](PROTOTYPE_PLAN.md) describes the milestones and boundaries; the [demo guide](docs/demo.md) provides repeatable fictional findings.

```sh
make setup
make test
make build
./dist/patchbot --help
./dist/patchbot scan --format json --timeout 60s /path/to/npm-project
```

The target must already have `package.json`, a version 2 or 3 `package-lock.json`, and installed `node_modules`. PatchBot does not install or execute the target's dependencies. Options go before the project path. Distribute the entire `dist` directory: the Go binary needs its adjacent helper and Node.js.

Exit codes: **0** = complete scan without active findings, **1** = complete scan with findings, **2** = invalid input, failed stage, or incomplete coverage. Complete means complete within supported prototype scope; a clean lookup is not proof that a project is secure. Help/version return 0.

## Run `patchbot` from any directory

From this repository, run `make install`. It builds and copies the complete bundle to `~/.local/share/patchbot`, then creates the command `~/.local/bin/patchbot`. No administrator access is needed. Run `make install` again after source changes to update that installation.

If `~/.local/bin` is not already on your shell's PATH, add `export PATH="$HOME/.local/bin:$PATH"` to `~/.zshrc`, then start a new terminal. PATH is the list of directories the shell searches for commands.

```sh
patchbot --version
patchbot --help
# In an npm project with dependencies already installed:
patchbot scan --timeout 60s .
patchbot scan --format json --timeout 60s .
```

Use `make install PREFIX=/absolute/custom/prefix` for a custom location, and put that prefix's `bin` directory on PATH. The command is a symlink; the helper is located beside the resolved executable, as recommended when `os.Executable` can return a link. [Go executable-path documentation](https://pkg.go.dev/os#Executable)

## Implemented functionality

- Installed dependency inventory using exact versions, root dependency relationships, representative paths, and production/development context, including mixed paths.
- Batched OSV queries, advisory details, pagination, bounded retries, withdrawn advisory handling, and preserved partial results.
- Static JavaScript/TypeScript imports and re-exports, type-only distinctions, and literal unshadowed CommonJS `require()` evidence with source locations.
- Versioned, explicit review rules and deterministic ordering. Missing severity requires manual review; development-only or transitive packages are never assumed safe.
- Terminal and JSON reports with stage status, coverage counts, warnings, and errors. JSON stdout contains only the report.

The [JSON contract](docs/json-contract.md) documents the data passed between Go and Node and the final report.

## Prototype boundaries

One npm project; no workspaces, Yarn/pnpm, automatic upgrades, AI explanations, dashboard, continuous monitoring, or proof of affected-function reachability. Standard npm registry package identities are queried; unsupported sources and installation discrepancies make coverage incomplete. Extraneous packages are counted but not queried and make the scan incomplete.

Source analysis covers application `.js`, `.jsx`, `.ts`, `.tsx`, `.mjs`, and `.cjs` files, excluding generated output, dependency directories, declaration files, and symlinks. It does not trace execution inside dependencies. Dynamic loading, import aliases, unresolved imports, and parse failures produce coverage warnings. Absence of a static import does not establish non-use.

The `prototype-v1` policy keeps critical/high findings in the high queue, medium/low/none in the review queue, and unknown severity/context in a prominent manual-review group. Within known severity bands, production paths with runtime imports are ordered first. Severity is normalized from recognized labels or CVSS 3.0, 3.1, and 4.0 base vectors while preserving the original evidence; unsupported severity remains unknown.

## PatchBot Technology Stack

| Component            | Technology              | Purpose                                                                                               |
| -------------------- | ----------------------- | ----------------------------------------------------------------------------------------------------- |
| Programming Language | Go + JavaScript         | Go implements the CLI, OSV client, reports, and risk engine; JavaScript implements the analysis helper |
| Runtime              | Go executable + Node.js | Runs PatchBot and its analysis helper locally                                                         |
| CLI Framework        | Go `flag`               | Provides commands such as `patchbot scan`                                                             |
| Dependency Analysis  | npm Arborist            | Analyzes dependency trees and identifies direct, transitive, production, and development dependencies |
| Vulnerability Data   | OSV API                 | Retrieves known vulnerability information for installed packages                                      |
| Source Code Analysis | ts-morph                | Analyzes TypeScript/JavaScript source code, imports, and package usage                                |
| Risk Engine          | Custom Go Rules         | Combines advisory severity and project evidence into an explained review queue                       |
| Explanation          | Go templates            | Explains the priority rules; LLM integration is deferred                                               |
| Terminal Interface   | Go text / JSON output   | Provides readable CLI output and structured reports                                                    |
| Testing              | Go testing + Node tests | Tests CLI behavior, analysis, OSV failures, prioritization, and the full workflow                       |

### Core Context-Aware Pipeline

npm Arborist
→ Inventories exact installed versions and dependency paths

OSV API
→ Identifies known vulnerabilities for those versions

ts-morph
→ Examines how dependencies are imported and used by the application

Custom Go Risk Engine
→ Combines the collected evidence to assign a practical risk priority

Go Explanation Templates
→ Explain the evidence, priority rules, and limitations

PatchBot aims to help developers review known vulnerabilities using application context. Its prototype gathers dependency relationships and static import evidence; it does not establish actual exploitability. Many security alerts can be difficult for small teams to prioritize, so findings include evidence and explicit reasons for their ordering.

PatchBot examines whether a vulnerable package is a direct or transitive dependency, whether declared paths place it in development or production context, and whether a static runtime import was detected. These observations support a review queue without proving that the affected function executes.

Instead of simply displaying a “Critical” warning, PatchBot explains why a vulnerability deserves attention or why it may be a lower priority. This gives developers evidence they can review rather than expecting them to blindly trust an automated decision.

PatchBot is also designed as a lightweight CLI tool that fits naturally into developers’ existing workflows. Its key distinction is therefore its combination of vulnerability detection, project-specific context, transparent risk prioritization, and developer-friendly explanations—helping small teams focus their limited security time on the vulnerabilities most likely to matter.
