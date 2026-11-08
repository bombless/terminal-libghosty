#ifndef TERMINAL_LIBGHOSTY_SSH_TERMINAL_H
#define TERMINAL_LIBGHOSTY_SSH_TERMINAL_H

#include "ghostty_host.h"
#include "windows_conpty.h"

#ifdef __cplusplus
extern "C" {
#endif

#if defined(_WIN32) && defined(TERMINAL_LIBGHOSTY_SESSION_EXPORTS)
#define TLG_SESSION_API __declspec(dllexport)
#elif defined(_WIN32)
#define TLG_SESSION_API __declspec(dllimport)
#else
#define TLG_SESSION_API
#endif

typedef struct tlg_ssh_terminal tlg_ssh_terminal;

/* Owns one ConPTY child process and one Ghostty VT host. argv excludes argv[0].
 * VT feed/snapshot/resize are serialized internally; ConPTY writes use the
 * process module's write lock. close must not race any other public operation.
 * Output callbacks never escape this API. */
TLG_SESSION_API int tlg_ssh_terminal_create_argv(
    tlg_ssh_terminal **out, const wchar_t *executable,
    const wchar_t *const *argv, size_t argc, uint16_t cols, uint16_t rows,
    uint32_t cell_width_px, uint32_t cell_height_px, uint32_t *out_win32_error);

TLG_SESSION_API int tlg_ssh_terminal_write(tlg_ssh_terminal *session,
    const uint8_t *bytes, size_t len, uint32_t *out_win32_error);
TLG_SESSION_API int tlg_ssh_terminal_resize(tlg_ssh_terminal *session,
    uint16_t cols, uint16_t rows, uint32_t cell_width_px,
    uint32_t cell_height_px, uint32_t *out_win32_error);
TLG_SESSION_API int tlg_ssh_terminal_poll_exit(tlg_ssh_terminal *session,
    uint32_t *out_exit_code, uint32_t *out_win32_error);
TLG_SESSION_API int tlg_ssh_terminal_snapshot(tlg_ssh_terminal *session,
    tlg_terminal_cell *cells, size_t cell_capacity, uint8_t *text,
    size_t text_capacity, uint16_t *out_cols, uint16_t *out_rows,
    size_t *required_cells, size_t *required_text_bytes,
    tlg_terminal_cursor *out_cursor);
/* Convenience snapshot for console diagnostics: UTF-8 text grid with CRLF rows. */
TLG_SESSION_API int tlg_ssh_terminal_read_text(tlg_ssh_terminal *session,
    uint8_t *buffer, size_t capacity, size_t *required_bytes);
TLG_SESSION_API void tlg_ssh_terminal_close(tlg_ssh_terminal *session);

#ifdef __cplusplus
}
#endif
#endif
