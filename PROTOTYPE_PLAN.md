# PatchBot CLI prototype implementation plan

Status: prototype implementation is present. See README.md for setup, supported scope, and the Go CLI walkthrough; milestone verification is documented in docs/demo.md and the automated tests.

## Outcome and boundaries

Build a local CLI that scans one npm project, identifies known vulnerable dependencies, gathers dependency and static import evidence, and produces an explained review queue.

Use Go for orchestration, OSV requests, priority rules, output, and Go tests. Use a small Node.js helper with Arborist and ts-morph for npm and JavaScript/TypeScript analysis. The helper can be plain JavaScript (`.mjs`) to avoid a second compilation pipeline.

The first version scans installed dependencies. Require `package.json`, an npm `package-lock.json` using version 2 or 3, and an existing `node_modules` directory. The developer prepares dependencies separately. Label results as an installed dependency scan; do not claim the lockfile exactly matches the installation. Detect and report missing/invalid dependency edges and installation discrepancies where available.

Support one project, standard npm registry packages, and `.js`, `.jsx`, `.ts`, and `.tsx` source files. Reject workspaces and unsupported lockfile versions with actionable messages. Report unsupported package identities, links, or source constructs as coverage limitations rather than silently treating them as safe.

Exclude automatic installation/upgrades, exploitability proof, complete reachability analysis, LLM explanations, dashboards, continuous monitoring, other ecosystems/package managers, and complex monorepos. Do not execute the target project's code or lifecycle scripts.

## Architecture

```text
Go CLI: validate target and prerequisites
  -> Node helper: installed dependency graph and source evidence
  -> Go OSV client: query exact package versions and fetch advisories
  -> Go priority engine: apply documented rules
  -> Go reporter: terminal or JSON report
```

Go starts a helper shipped with PatchBot using an argument array, not a shell command string. Resolve the helper relative to the distribution, not the target project's working directory. Use a process timeout and cancellation. The helper emits one versioned JSON object on stdout; diagnostics go to stderr.

Proposed layout:

```text
cmd/patchbot/main.go       CLI entry point
internal/cli/             argument parsing and exit handling
internal/model/           shared report and evidence types
internal/analyzer/        Node helper invocation and JSON validation
internal/osv/             HTTP client and advisory normalization
internal/risk/            pure priority and ordering functions
internal/report/          terminal and JSON rendering
analyzer/package.json     pinned helper dependencies
analyzer/package-lock.json
analyzer/src/index.mjs     helper entry point
analyzer/src/dependencies.mjs
analyzer/src/imports.mjs
testdata/                 controlled project and advisory fixtures
docs/demo.md              reproducible demonstration instructions
```

The prototype distribution contains the Go binary and the helper, and requires Node.js. A Go executable alone will not provide the complete scan.

## Shared data contract

Define this before implementing both languages. It is their agreement about field names, identities, and unknown values.

| Entity | Required information |
| --- | --- |
| Analysis envelope | Schema version, inventory mode, stage statuses, package nodes, dependency edges, import evidence, warnings |
| Package node | Stable node ID based on project-relative installation location, real package name, exact version, location, root relationship, dependency context |
| Dependency edge | From/to node IDs, requested name, dependency type, validity; missing targets remain explicit |
| Dependency context | Production path present, development path present, optional/peer flags, or unknown; preserve mixed usage |
| Import evidence | Module specifier, normalized package name, resolved node ID when known, relative file path, line, runtime/type-only distinction |
| Advisory | ID, aliases, summary, original severity evidence, normalized severity or unknown, affected/fixed ranges, references, withdrawal status |
| Finding | Package node ID plus advisory ID, representative dependency path, evidence, priority, rule IDs, reasons, limitations |
| Scan report | Report/rules versions, scan time, target, inventory/coverage summary, findings, complete/partial/failed status, stage errors |

Do not merge distinct installations simply because they have the same package name. Deduplicate OSV network queries by ecosystem/name/version, then attach the results to every corresponding node. Multiple dependency paths may be summarized with a deterministic representative path and a path count.

Import states are `detected`, `not_detected_in_scanned_files`, and `unknown`. None means exploitability has been established or ruled out. Store affected-function reachability as `unknown` in this prototype.

## Milestone 1: project skeleton and CLI contract

- Initialize the Go module and Node helper package; select compatible supported runtime versions and pin helper dependencies.
- Implement `patchbot scan [--format text|json] [--timeout 30s] <path>`, `--help`, and `--version`. Put flags before the positional path with the initial parser.
- Use Go's `flag.FlagSet` for the scan subcommand; it supports independent flag sets suitable for subcommands. [Go flag documentation](https://pkg.go.dev/flag)
- Validate paths and prerequisites; distinguish tool setup errors from target-project errors.
- Define schemas and connect Go to a stub helper returning a small fixture.

Done when: Go can validate a target, invoke the helper, decode the contract, and explain missing prerequisites. No empty placeholder result is presented as a completed security scan.

Learning focus: command handling, process boundaries, and structured error handling. Keep `main` small; scanning belongs in testable functions.

## Milestone 2: installed dependency analysis

- Use Arborist's library API and `loadActual()` to inspect installed packages. Its graph edges describe dependency relationships; folder nesting alone does not. [Arborist documentation](https://github.com/npm/cli/tree/latest/workspaces/arborist)
- Traverse root-reachable dependency edges with cycle protection. Collect exact versions and installation locations, plus missing/invalid edges and extraneous-package counts.
- Determine direct relationships from root edges. Preserve production, development, optional, and peer information, including mixed paths; these describe declared dependency context, not deployment proof.
- Return one deterministic representative path for each finding candidate; avoid enumerating every possible graph path.
- Exclude the root application from dependency advisory queries. Mark unsupported package sources/identities as unqueried coverage gaps.
- Add Node helper tests using the built-in test runner and controlled dependency trees.

Done when: fixtures correctly identify direct, transitive, development-only, and mixed-use packages, and keep two installed versions of one package separate.

Learning focus: a dependency tree is logically a graph. Hoisting means the disk layout is not a reliable explanation of who depends on whom.

## Milestone 3: OSV integration

- Implement a Go client with `net/http`, JSON decoding, context cancellation, explicit timeouts, and bounded requests.
- Query unique npm package/version pairs using `/v1/querybatch`; retain input/result correspondence and follow per-query pagination.
- Fetch each returned ID through `/v1/vulns/{id}` and reuse details within the scan. The batch endpoint returns IDs and modification timestamps rather than full advisories. [OSV batch documentation](https://google.github.io/osv.dev/post-v1-querybatch/), [advisory endpoint](https://google.github.io/osv.dev/get-v1-vulns/)
- Preserve raw severity data and its source. Normalize recognized severity labels and supported CVSS vectors through an established, tested Go library selected at implementation time; unsupported or missing severity stays unknown. Preserve advisory aliases and withdrawal information. [OSV schema](https://ossf.github.io/osv-schema/)
- Omit withdrawn advisories from active findings while recording their count. Do not invent fixed versions or recommend an upgrade as universally safe.
- Bound retries for transient failures and rate limits. Keep successful results but mark failed or unfinished lookups explicitly.
- Use a local Go test HTTP server for deterministic success, pagination, malformed response, timeout, and rate-limit scenarios; tests must not depend on live OSV.

Done when: exact versions produce normalized findings, duplicate queries are avoided, and an outage produces a partial/failed report rather than a clean result.

Learning focus: an HTTP integration needs both a success path and a trustworthy failure path.

## Milestone 4: static source evidence

- Scan supported files within the target, excluding `node_modules`, `.git`, generated output directories, declaration-only files, and paths outside the project.
- Use ts-morph for static ES import declarations and re-exports; add literal `require("package")` calls as a separately tested supported case. `getImportDeclarations()` provides static imports and module specifiers. [ts-morph import documentation](https://ts-morph.com/details/imports)
- Distinguish type-only evidence from runtime imports. Normalize scoped packages and package subpaths; ignore relative imports and Node built-ins as external package evidence.
- Match imports to the installed node using the importing file's resolution context. Ambiguous or unsupported resolution remains unknown; do not attach evidence to every installed version of a name.
- Flag dynamic loading and parse failures as limitations. Analysis is application source only; absence of a direct import says little about a transitive package's usage.
- Report file/line evidence and scanned/skipped-file counts. If analysis fails, retain dependency findings with unknown source evidence.

Done when: fixtures identify named, scoped, subpath, type-only, and literal CommonJS imports without confusing duplicate installations or claiming unused packages are safe.

Learning focus: static analysis gathers evidence from source syntax; it does not observe runtime execution.

## Milestone 5: transparent prioritization

- Implement a pure Go function from a finding's evidence to priority, ordering keys, rule IDs, and explanation.
- Version the initial policy as `prototype-v1`. It is a review heuristic, not a probability of exploitation.
- Initial priority bands: critical/high advisory severity -> `high`; medium/low -> `review`; unknown/unrecognized severity -> `manual_review`. Unknown severity is not a low-risk conclusion.
- Within known severity bands, prioritize production-path findings with detected runtime imports, then other production-path findings, then development-only findings. Unknown dependency context requires explicit manual review. Break ties deterministically using package location and advisory ID.
- Keep manual-review items prominent in a separate visible group. Never suppress a finding or reduce its severity because it is transitive, development-only, or lacks a detected import. Development-only high-severity findings retain high priority.
- Explain the base priority and context ordering separately, and show that actual execution and affected-function reachability are unknown.
- Add table-driven Go tests for high-severity development tooling, missing severity, absent import evidence, mixed dependency paths, and stable ordering.

Done when: every finding carries a traceable rule and reason, and changing context changes the explanation/order without inventing safety guarantees.

Learning focus: explicit rules are easier to review and debug than unexplained numeric scores. Stronger context-driven priority changes can follow after evaluating the initial policy.

## Milestone 6: reports and scan orchestration

- Connect analysis, lookup, prioritization, and rendering. Continue after recoverable source/lookup failures so useful evidence survives.
- Terminal output shows package/version, advisory, original severity, priority, dependency path/context, import locations, reasons, available fix information, and limitations.
- JSON includes the same underlying evidence and all coverage/error statuses. JSON stdout contains only the report; progress and diagnostics use stderr.
- Exit codes: `0` for a complete scan with no active findings; `1` for a complete scan with findings; `2` for invalid input, incomplete coverage, or failed stages. Incomplete status takes precedence over finding count. Document that complete means complete within the stated supported scope.
- An empty successful lookup is reported as "no known vulnerabilities found in the queried dependencies," not "project is secure."
- Add end-to-end checks for both output formats, cancellation, missing Node/helper, paths containing spaces, malformed helper JSON, and prerequisite failures.

Done when: one command produces an honest, readable report; text and JSON agree; scan failures cannot be mistaken for clean results.

## Milestone 7: demonstration and delivery

- Create controlled fixtures for a direct production finding, transitive finding, development-only finding, duplicate versions, mixed dependency paths, unknown severity, dynamic usage, and an OSV outage.
- Use clearly labeled fictional advisories and a local mock OSV server for repeatable demonstrations. Keep endpoint injection in test/demo tooling rather than exposing an unnecessary production CLI option.
- Demonstrate the full installed-dependency workflow, including dependency/import collection, rather than replaying only a canned final report.
- Run a separate live OSV smoke scan on a small prepared npm project; record that results may change as advisories change. Do not make live findings an automated-test requirement.
- Provide build/setup instructions, runtime prerequisites, helper packaging, CLI examples, supported scope, rules, exit codes, and known limitations. Update the README to reflect Go plus the Node helper during implementation.
- Verify meaningful Go and helper tests, then check the documented setup from a fresh development distribution. Confirm scans do not modify target files.

Done when: a new developer can follow setup instructions, run the CLI, reproduce the demonstration, and explain every shown priority from its evidence.

## Working sequence and definition of done

Complete milestones in order. Keep each change small and demonstrable; do not start presentation polish before dependency lookup and failure handling work. After each milestone, review its output, explain the underlying concept, and check its completion criterion before continuing.

The prototype is finished when all five core features work together, the distribution includes its helper, controlled demos are repeatable, meaningful tests pass, incomplete scans are unmistakable, and documentation accurately states what the scanner can and cannot conclude.

The first implementation task is milestone 1: agree on the data contract, create the Go/Node skeleton, and demonstrate a stub helper round trip. Avoid implementing every component simultaneously; the contract and small working milestones make integration problems easier to find.
