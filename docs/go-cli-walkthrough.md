# Build and understand the Go CLI, step by step

The CLI is the adapter between a person typing a command and the scanner doing work. Its responsibilities are arguments, validation, cancellation, output, and exit codes. Dependency analysis and security decisions belong in separate packages.

Read this walkthrough in order, then trace the implementation in `cmd/patchbot/main.go` and `internal/cli/cli.go`. The rest of the scanner is already connected behind the same interface.

## 1. Create the module and entry point

For a fresh project, run `go mod init patchbot`. This repository already contains the module, so do not repeat initialization.

`cmd/patchbot/main.go` contains `package main` and `func main()`: that is Go's executable entry point. Build with `go build -o dist/patchbot ./cmd/patchbot` or use the complete distribution target `make build`.

Keep `main` short. Our entry point creates a cancellation context, calls `cli.Run`, cleans up signal handling, and exits with the returned code. `os.Exit` skips deferred functions, which is why cleanup is explicit before exiting.

Exercise: identify which line forwards command arguments and which line chooses the process exit code.

## 2. Understand the arguments

For this command:

```sh
patchbot scan --format json ./sample-project
```

The program sees approximately:

```go
os.Args // ["patchbot", "scan", "--format", "json", "./sample-project"]
```

`os.Args[1:]` removes the executable name. `Run` checks the first remaining argument to choose the command. Help and version return without starting the scanner.

Common mistake: indexing `args[0]` without first checking `len(args)`. Our implementation checks the length before accessing the first argument.

Exercise: predict the result of `patchbot unknown` and `patchbot --help`.

## 3. Parse the scan options

The essential expert pattern is:

```go
fs := flag.NewFlagSet("scan", flag.ContinueOnError)
fs.StringVar(&opts.Format, "format", "text", "report format: text or json")
fs.DurationVar(&opts.Timeout, "timeout", 30*time.Second, "total scan time limit")
err := fs.Parse(args)
```

`StringVar` stores a parsed value in a struct field; `&` passes its address so the parser can update it. `ContinueOnError` returns errors for our code to handle instead of terminating the process internally. [`FlagSet` documentation](https://pkg.go.dev/flag)

After parsing, `fs.Arg(0)` is the project path. The standard parser stops at the first positional argument, so options must come before that path. The CLI deliberately rejects extra positional arguments rather than ignoring them.

Exercise: compare `scan --format json ./project` with `scan ./project --format json`. Explain why the second is rejected.

## 4. Validate before doing work

Parsing a flag does not establish that its value makes sense. `ParseScan` verifies one path, a supported format, and a positive timeout. The scanner separately checks whether the path is an npm project with a supported lockfile and installed dependencies.

This separation keeps argument mistakes distinct from project setup problems. Project/scanner failures still produce a structured report when `--format json` is selected. Argument syntax errors produce diagnostics on stderr and exit code 2.

Exercise: try `--format yaml` and `--timeout 0s`. Confirm no scan runs.

## 5. Call a scanner through a function

```go
type ScanFunc func(context.Context, string) model.Report
```

This describes a function that takes a cancellation context and project path and returns a report. `Run` accepts this function as a parameter. Production supplies `scan.New().Scan`; tests supply a small fake function.

The gist of dependency injection: pass the behavior a function needs instead of hard-coding how to construct it. We use one function here, not an elaborate framework.

Common mistake: mixing OSV HTTP requests directly into argument parsing. That would make command tests depend on network availability.

Exercise: read `internal/cli/cli_test.go` and locate a fake scanner.

## 6. Add cancellation and a time limit

```go
ctx, cancel := context.WithTimeout(ctx, opts.Timeout)
defer cancel()
result := scan(ctx, opts.Path)
```

The context carries cancellation across the helper process and HTTP requests. `defer cancel()` releases timer resources when `Run` returns. The entry point also cancels on Ctrl+C.

A deadline is a total scan budget, not a guarantee that all packages will be scanned. If time runs out, the report explicitly says the scan is incomplete.

## 7. Print the report and return an exit code

stdout contains the requested report. stderr contains command diagnostics. Keeping these separate lets another program parse JSON without encountering progress messages.

| Code | Meaning |
| --- | --- |
| 0 | Complete scan, no active findings |
| 1 | Complete scan, active findings |
| 2 | Invalid arguments/target, tool error, or incomplete scan |

Code 2 takes precedence even if a partial scan found vulnerabilities. Help/version return 0.

Use the built binary to inspect real exit codes; `go run` itself wraps the program's exit status.

Exercise: run `./dist/patchbot scan --format json /a/missing/project`, then `echo $?`. You should see a failed report and code 2.

## 8. Test behavior without the network

The CLI tests supply in-memory buffers for stdout/stderr and a fake scanner. They check parsed options, cancellation, output format, and exit codes. Run `go test ./internal/cli` after dependencies are set up.

To explain this design in an interview, trace: arguments -> validated options -> scanner function -> report renderer -> exit code. Then explain why this separation allows the same scanner to be tested independently of the terminal.

Your next practice task: add a failing parser test for a negative timeout, then check that the existing validation satisfies it. Keep changes small and explain the expected behavior before changing code.
