# Pure-Go Windows ConPTY implementation

Date: 2026-11-08

## Implementation

The Windows-only implementation is in:

- `internal/terminal/conpty_windows.go`
- `internal/terminal/conpty_windows_test.go`

It uses the Windows APIs through Go's `syscall` package. It does not use
cgo or add a third-party dependency.

The implementation owns the ConPTY pipe handles, HPCON, child process, and
optional Job Object. It creates inheritable child-side pipe handles, removes
inheritance from the parent-side handles, attaches HPCON through
`PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE`, and starts the executable directly
with `CreateProcessW` (not through a shell). Windows argv quoting follows the
backslash-before-quote and trailing-backslash rules. Job Object setup uses
`JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE` when permitted, but is best-effort like
the original C implementation and does not block startup in restricted or
nested-job environments.

A single output reader copies each native read into Go-owned memory before
calling the output callback. Process exit is independently observable through
`PollExit`. The optional exit callback follows the original C contract and
fires after the output pipe reaches EOF (normally after `ClosePseudoConsole`).
Writes are serialized and loop over short writes. Resize and handle ownership
are protected by a mutex. Close terminates a still-running child, closes HPCON,
joins the reader, then releases the remaining handles; repeated and concurrent
Close calls wait for the same cleanup.

## Tests

Run on Windows 10 1809 or newer:

```powershell
$env:CGO_ENABLED = '0'
go test ./internal/terminal -run 'Test(WindowsCommandLineQuoting|ConPTY)' -count=5 -v
```

The focused tests cover quoting (including embedded quotes and trailing
backslashes), input/output round-trip, resize validation, 5,000 lines of
output, process polling, output-EOF exit notification, failed process creation
cleanup, and concurrent idempotent Close.

## Integration status

The production Go `Session` now uses this ConPTY implementation together with
the Go Ghostty host adapter. The original C implementation remains available
as a compatibility reference and regression baseline; the Go application no
longer loads the project-owned session DLL.
