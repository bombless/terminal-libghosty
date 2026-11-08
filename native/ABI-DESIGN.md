# Native host ABI design

Status: design based on the checked-out local Ghostty headers (libghostty-vt
1.1.0 headers). This is not a claim that the local source matches the CMake
pin `5813c9461d5af78b6307a1254d00b1bc190eaccd`.

## Boundary

Keep `tlg_ghostty_host` opaque. Go and the Win32 UI must not include Ghostty
headers or depend on Ghostty-owned layouts. `ghostty_host.h` is the only ABI
visible to the application. Return project-owned integer status values and
never return borrowed Ghostty pointers across the ABI.

## Ownership and lifecycle

- `create` allocates one host, one `GhosttyTerminal`, and one
  `GhosttyRenderState`; any partially created objects are unwound in reverse
  order on failure. The default Ghostty allocator is used consistently.
- The host owns the terminal and render state until `destroy`. Destroy is
  idempotent only for a NULL handle; callers must clear their handle after the
  first destroy and must not race destroy against any other operation.
- Input byte buffers are borrowed only for the duration of a call. The host
  never retains pointers supplied by the caller.
- `tlg_ghostty_host_snapshot` updates the owned render state and copies a full
  viewport into caller-owned `tlg_terminal_cell[]` and UTF-8 byte-arena buffers.
  Call once with NULL/zero buffers to query dimensions and required capacities,
  then call again with allocated buffers. Each cell contains byte offset/length,
  resolved foreground/background RGB, style flags, and selection state. Cursor
  viewport position/visibility/blink/style is copied into a project-owned struct.
- Snapshot buffers remain valid independently of Ghostty and the next update;
  Ghostty row/cell iterator data is never exposed. A successful full copy calls
  `ghostty_render_state_clean`; a sizing/undersized-buffer call does not clear
  dirty state, so callers can retry safely.

## Ghostty mapping verified in the local headers

| Project operation | Upstream API | Contract |
|---|---|---|
| create terminal | `ghostty_terminal_new(NULL, &terminal, cols, rows)` | cols/rows are non-zero `uint16_t`; returns `GhosttyResult` |
| receive PTY output | `ghostty_terminal_vt_write(terminal, bytes, len)` | void; input bytes are parsed synchronously; malformed external input is tolerated |
| resize | `ghostty_terminal_resize(terminal, cols, rows, cell_width_px, cell_height_px)` | dimensions are cells; pixel arguments are *cell metrics*, not total surface size |
| update frame | `ghostty_render_state_update(state, terminal)` | consumes terminal dirty state; render state is then read until next update |
| copy frame | `ghostty_render_state_get`, row iterator/cell getters | project ABI performs a sizing pass then copies UTF-8, resolved RGB, flags, selection and cursor into caller buffers; no upstream view escapes |
| destroy | `ghostty_render_state_free`, `ghostty_terminal_free` | free render state before terminal |
| allocate/free returned buffers | `ghostty_alloc` / `ghostty_free` | same allocator and exact allocation length must be used |

Rendering should use the public render-state API: query viewport cols/rows,
obtain the preallocated row iterator through
`GHOSTTY_RENDER_STATE_DATA_ROW_ITERATOR`, populate reusable row-cells objects
through `GHOSTTY_RENDER_STATE_ROW_DATA_CELLS`, then read grapheme UTF-8,
resolved foreground/background colors, and style through the cell getters.
Never decode packed `GhosttyCell` bits manually; the header explicitly says
those bit positions are not ABI-stable. A frame visitor must finish before
the next update.

## PTY replies and threading

Install `GHOSTTY_TERMINAL_OPT_USERDATA` and
`GHOSTTY_TERMINAL_OPT_WRITE_PTY`. Ghostty calls the write callback
synchronously while processing VT output; callback bytes are borrowed only
until return. The callback must enqueue/copy those bytes for the ConPTY input
pump and must never call `ghostty_terminal_vt_write` recursively on the same
terminal (the upstream API explicitly says the terminal is non-reentrant).

Until a deliberate lock/ownership model is implemented, serialize all
terminal mutation and render-state update/read operations on one host-owned
thread. ConPTY reader/writer workers communicate through bounded queues and
never call Ghostty directly. This keeps the GUI message thread free of
blocking pipe I/O and avoids concurrent terminal access.

## Resize and validation

Clamp cell columns/rows to `[1, UINT16_MAX]`; ignore or defer zero-sized
minimized surfaces. Debounce rapid resize events before calling both
`ResizePseudoConsole` and `ghostty_terminal_resize`, using the same cell grid
and measured cell pixel dimensions. Reject invalid handles and inconsistent
arguments before entering Ghostty. Project tests should cover lifecycle,
fragmented UTF-8/VT input, ANSI state, resize, callback queueing, and repeated
NULL-safe cleanup.

## Version gate

The checked-out `include/ghostty/vt/version.h` reports 1.1.0. The project CMake
file pins a different upstream Git revision, while the local Ghostty tree has
no `.git` metadata. Keep the existing pin unchanged; before shipping, build
against the actual pinned revision and compile/test every used symbol there.
Do not treat a successful local-source build as proof of pin compatibility.

## Combined SSH session adapter

`ssh_terminal.h/.c` composes the opaque Ghostty host and ConPTY process into
one opaque session exported by `terminal_libghosty_session.dll`.

- Create the Ghostty host first, install its PTY write callback, resize it to
  the requested cell grid, then create the ConPTY child.
- Publish the ConPTY handle to the session before starting the reader worker.
  This prevents an early SSH banner/query from reaching Ghostty before its
  synchronous PTY reply callback can find the input pipe.
- The ConPTY reader worker enters the session VT critical section and feeds
  output bytes to `tlg_ghostty_host_feed_output`. Ghostty's write callback
  synchronously calls `tlg_conpty_write`; the borrowed Ghostty bytes are
  consumed before the callback returns and are never retained or passed back
  into the VT parser. This write happens on the reader worker, not the UI
  thread.
- Snapshot and resize use the same VT critical section as output feed. The
  ConPTY module separately serializes pipe writes. Callers must not race close
  against any other operation on that session.
- Close marks the session as closing, closes ConPTY without holding the VT
  critical section (the reader may need that lock to finish), waits for the
  reader worker, then destroys the Ghostty host and its synchronization state.
- The Go Windows wrapper calls `ssh.BuildArgs`, resolves `ssh.exe`, marshals
  UTF-16 argv, and invokes the stable DLL ABI through `syscall`; no cgo or
  gccgo dependency is introduced. The session DLL and `ghostty-vt.dll` must
  be beside the application or in the configured native DLL directory.

The end-to-end test verifies normal ConPTY input/output, a DSR query answered
by Ghostty and delivered to the child process, resize, process exit/close, and
the actual Windows OpenSSH executable. The Go runtime test also exercises
`StartSSH` against a closed loopback port to verify the validated argv and
captured SSH error output. Remote authentication and a GUI renderer remain
outside this bridge test's scope.
