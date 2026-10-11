# Reproducible prototype demonstration

Set up the helper and Go dependencies first:

```sh
make setup
make test
make build
```

The normal development demo builds a temporary installed project with fictional packages, runs the real dependency/source helper, and injects a local mock OSV server into the real Go scanner. It performs no live advisory requests and removes its temporary project afterward.

```sh
make demo
```

Expected: seven package nodes, seven queried package versions, five fictional findings, and a complete scan. The demo executable returns 1 because findings exist; the Makefile treats that expected result as success. The unknown-severity finding appears in the prominent manual-review group.

Inspect the cases in order:

1. `demo-parser@1.0.0`: direct production dependency, static import at `src/server.ts:1`, high priority, and a fictional fixed boundary.
2. `demo-parser@2.0.0`: a separate transitive installation under `demo-parent`, with a dependency path and no incorrectly copied root import.
3. `demo-build-tool`: development-only dependency that retains high priority.
4. `demo-shared`: shared production/development paths; no direct import does not prove non-use.
5. `@demo/scoped`: subpath import evidence, missing severity, and manual-review priority.

Compare explanations and ordering, not only the number of findings. All affected-function reachability values remain unknown.

After `make demo`, the development executable supports additional cases:

```sh
./dist/demo --format json
./dist/demo --outage
./dist/demo --dynamic
```

The JSON demo returns 1. Outage and dynamic-loading demos return 2. For outage, findings are empty but status is partial and failed lookups are visible. For dynamic loading, findings remain visible but source coverage is incomplete. Neither case is a clean scan.

Production CLI examples:

```sh
./dist/patchbot --version
./dist/patchbot scan --help
./dist/patchbot scan --timeout 60s /path/to/prepared/npm-project
./dist/patchbot scan --format json --timeout 60s /path/to/prepared/npm-project
```

For a live smoke scan, the PatchBot helper itself is a prepared npm project after setup:

```sh
./dist/patchbot scan --timeout 90s ./analyzer
```

This queries live OSV for the helper's real installed dependencies. Results may change as advisories change. A live scan is an integration check, not a deterministic automated test. Inspect status/coverage before interpreting findings.

You can launch the production binary from another working directory. Keep `dist/patchbot` and `dist/analyzer` together; the target's directory is never used to select the helper. Node.js is still required.
