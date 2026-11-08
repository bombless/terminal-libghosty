#ifndef TERMINAL_LIBGHOSTY_WINDOWS_CONPTY_H
#define TERMINAL_LIBGHOSTY_WINDOWS_CONPTY_H

#include <stddef.h>
#include <stdint.h>

#ifdef _WIN32
#ifndef _WIN32_WINNT
#define _WIN32_WINNT 0x0A00 /* Windows 10 1809+: CreatePseudoConsole / HPCON. */
#endif
#include <windows.h>
#else
#error "windows_conpty is only available on Windows"
#endif

#ifdef __cplusplus
extern "C" {
#endif

typedef struct tlg_conpty tlg_conpty;

typedef void (*tlg_conpty_output_fn)(void *userdata, const uint8_t *bytes, size_t len);
typedef void (*tlg_conpty_exit_fn)(void *userdata, uint32_t exit_code);

typedef struct {
    const wchar_t *executable; /* Absolute or PATH-resolvable UTF-16 executable path. */
    const wchar_t *const *argv; /* Arguments excluding argv[0]; each is UTF-16. */
    size_t argc;
    uint16_t cols;
    uint16_t rows;
    tlg_conpty_output_fn on_output; /* Called on the reader worker thread. */
    tlg_conpty_exit_fn on_exit;     /* Called once after output reaches EOF. */
    void *userdata;                 /* Borrowed until tlg_conpty_close returns. */
} tlg_conpty_config;

enum {
    TLG_CONPTY_OK = 0,
    TLG_CONPTY_ERROR_INVALID_ARGUMENT = -1,
    TLG_CONPTY_ERROR_OUT_OF_MEMORY = -2,
    TLG_CONPTY_ERROR_WIN32 = -3,
    TLG_CONPTY_ERROR_THREAD = -4
};

/* Creates a ConPTY and starts the requested process. Handles are owned by the
 * session. The caller must serialize write/resize/close for a given session. */
int tlg_conpty_create(tlg_conpty **out, const tlg_conpty_config *config,
                      uint32_t *out_win32_error);

/* Writes raw terminal input bytes to the child stdin pipe. */
int tlg_conpty_write(tlg_conpty *session, const uint8_t *bytes, size_t len,
                     uint32_t *out_win32_error);

/* Resizes the pseudo console. Zero dimensions are rejected; callers should
 * defer resize while the containing window is minimized. */
int tlg_conpty_resize(tlg_conpty *session, uint16_t cols, uint16_t rows,
                      uint32_t *out_win32_error);

/* Returns 1 when the process has exited, 0 while running, or a negative error. */
int tlg_conpty_poll_exit(tlg_conpty *session, uint32_t *out_exit_code,
                         uint32_t *out_win32_error);

/* Terminates a still-running child, closes the pseudo console, joins the output
 * worker, and releases all handles. Safe for NULL; do not call concurrently. */
void tlg_conpty_close(tlg_conpty *session);

#ifdef __cplusplus
}
#endif
#endif
