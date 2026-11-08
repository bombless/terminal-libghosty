#define WIN32_LEAN_AND_MEAN
#include "ssh_terminal.h"

#include <stdio.h>
#include <stdarg.h>
#include <stdlib.h>
#include <string.h>

static void tlg_log_ssh(const char *fmt, ...) {
    va_list ap;
    va_start(ap, fmt);
    SYSTEMTIME st;
    GetLocalTime(&st);
    fprintf(stderr, "[ssh-term %02d:%02d:%02d.%03d] ", st.wHour, st.wMinute, st.wSecond, st.wMilliseconds);
    vfprintf(stderr, fmt, ap);
    fprintf(stderr, "\n");
    fflush(stderr);
    va_end(ap);
}

struct tlg_ssh_terminal {
    tlg_conpty *conpty;
    tlg_ghostty_host *ghostty;
    CRITICAL_SECTION vt_lock;
    LONG closing;
};

static void set_error(uint32_t *out, uint32_t value) { if (out) *out = value; }

static void on_pty_output(void *userdata, const uint8_t *bytes, size_t len) {
    tlg_ssh_terminal *s = (tlg_ssh_terminal *)userdata;
    if (!s || !bytes || !len || InterlockedCompareExchange(&s->closing, 0, 0)) return;
    tlg_log_ssh("on_pty_output: feed %zu bytes to Ghostty VT", len);
    EnterCriticalSection(&s->vt_lock);
    if (!InterlockedCompareExchange(&s->closing, 0, 0) && s->ghostty)
        (void)tlg_ghostty_host_feed_output(s->ghostty, bytes, len);
    LeaveCriticalSection(&s->vt_lock);
}

static void on_ghostty_input(void *userdata, const uint8_t *bytes, size_t len) {
    tlg_ssh_terminal *s = (tlg_ssh_terminal *)userdata;
    if (!s || !s->conpty || InterlockedCompareExchange(&s->closing, 0, 0)) return;
    tlg_log_ssh("on_ghostty_input: Ghostty requested write to ConPTY (%zu bytes)", len);
    /* Called synchronously while vt_lock is held. ConPTY owns a separate
     * write lock, so this callback must not re-enter the VT host. */
    uint32_t err = 0;
    int rc = tlg_conpty_write(s->conpty, bytes, len, &err);
    if (rc != TLG_CONPTY_OK) {
        tlg_log_ssh("on_ghostty_input: write failed: rc=%d err=%lu", rc, (unsigned long)err);
    }
}

int tlg_ssh_terminal_create_argv(tlg_ssh_terminal **out,
    const wchar_t *executable, const wchar_t *const *argv, size_t argc,
    uint16_t cols, uint16_t rows, uint32_t cell_width_px,
    uint32_t cell_height_px, uint32_t *out_error) {
    if (out) *out = NULL;
    set_error(out_error, ERROR_SUCCESS);
    if (!out || !executable || !executable[0] || (argc && !argv) ||
        !cols || !rows || !cell_width_px || !cell_height_px)
        return TLG_CONPTY_ERROR_INVALID_ARGUMENT;

    tlg_ssh_terminal *s = (tlg_ssh_terminal *)calloc(1, sizeof(*s));
    if (!s) return TLG_CONPTY_ERROR_OUT_OF_MEMORY;
    InitializeCriticalSection(&s->vt_lock);

    int rc = tlg_ghostty_host_create(&s->ghostty, NULL, on_ghostty_input, s);
    if (rc != TLG_GHOSTTY_OK) {
        DeleteCriticalSection(&s->vt_lock);
        free(s);
        return rc == TLG_GHOSTTY_ERROR_OUT_OF_MEMORY
            ? TLG_CONPTY_ERROR_OUT_OF_MEMORY : TLG_CONPTY_ERROR_WIN32;
    }
    rc = tlg_ghostty_host_resize(s->ghostty, cols, rows,
                                 cell_width_px, cell_height_px);
    if (rc != TLG_GHOSTTY_OK) {
        tlg_ghostty_host_destroy(s->ghostty);
        DeleteCriticalSection(&s->vt_lock);
        free(s);
        return TLG_CONPTY_ERROR_INVALID_ARGUMENT;
    }

    tlg_conpty_config cfg;
    memset(&cfg, 0, sizeof(cfg));
    cfg.executable = executable;
    cfg.argv = argv;
    cfg.argc = argc;
    cfg.cols = cols;
    cfg.rows = rows;
    cfg.on_output = on_pty_output;
    cfg.userdata = s;
    rc = tlg_conpty_create(&s->conpty, &cfg, out_error);
    if (rc != TLG_CONPTY_OK) {
        tlg_ghostty_host_destroy(s->ghostty);
        DeleteCriticalSection(&s->vt_lock);
        free(s);
        return rc;
    }
    *out = s;
    return TLG_CONPTY_OK;
}

int tlg_ssh_terminal_write(tlg_ssh_terminal *s, const uint8_t *bytes,
                           size_t len, uint32_t *out_error) {
    if (!s || InterlockedCompareExchange(&s->closing, 0, 0))
        return TLG_CONPTY_ERROR_INVALID_ARGUMENT;
    tlg_log_ssh("tlg_ssh_terminal_write: writing %zu bytes", len);
    int rc = tlg_conpty_write(s->conpty, bytes, len, out_error);
    if (rc != TLG_CONPTY_OK) {
        tlg_log_ssh("tlg_ssh_terminal_write: failed with rc=%d", rc);
    }
    return rc;
}

int tlg_ssh_terminal_resize(tlg_ssh_terminal *s, uint16_t cols, uint16_t rows,
                            uint32_t cell_width_px, uint32_t cell_height_px,
                            uint32_t *out_error) {
    if (!s || !cols || !rows || !cell_width_px || !cell_height_px ||
        InterlockedCompareExchange(&s->closing, 0, 0))
        return TLG_CONPTY_ERROR_INVALID_ARGUMENT;
    tlg_log_ssh("tlg_ssh_terminal_resize: cols=%u rows=%u", cols, rows);
    EnterCriticalSection(&s->vt_lock);
    int rc = tlg_conpty_resize(s->conpty, cols, rows, out_error);
    if (rc == TLG_CONPTY_OK) {
        int vt_rc = tlg_ghostty_host_resize(s->ghostty, cols, rows,
                                            cell_width_px, cell_height_px);
        if (vt_rc != TLG_GHOSTTY_OK) rc = TLG_CONPTY_ERROR_WIN32;
    }
    LeaveCriticalSection(&s->vt_lock);
    return rc;
}

int tlg_ssh_terminal_poll_exit(tlg_ssh_terminal *s, uint32_t *out_code,
                               uint32_t *out_error) {
    if (!s || InterlockedCompareExchange(&s->closing, 0, 0))
        return TLG_CONPTY_ERROR_INVALID_ARGUMENT;
    return tlg_conpty_poll_exit(s->conpty, out_code, out_error);
}

int tlg_ssh_terminal_snapshot(tlg_ssh_terminal *s,
    tlg_terminal_cell *cells, size_t cell_capacity, uint8_t *text,
    size_t text_capacity, uint16_t *out_cols, uint16_t *out_rows,
    size_t *required_cells, size_t *required_text_bytes,
    tlg_terminal_cursor *out_cursor) {
    if (!s || InterlockedCompareExchange(&s->closing, 0, 0))
        return TLG_GHOSTTY_ERROR_INVALID_ARGUMENT;
    EnterCriticalSection(&s->vt_lock);
    int rc = tlg_ghostty_host_snapshot(s->ghostty, cells, cell_capacity,
        text, text_capacity, out_cols, out_rows, required_cells,
        required_text_bytes, out_cursor);
    LeaveCriticalSection(&s->vt_lock);
    return rc;
}

int tlg_ssh_terminal_read_text(tlg_ssh_terminal *s, uint8_t *buffer,
                              size_t capacity, size_t *required_bytes) {
    if (!s || !required_bytes || (!buffer && capacity) ||
        InterlockedCompareExchange(&s->closing, 0, 0))
        return TLG_GHOSTTY_ERROR_INVALID_ARGUMENT;
    EnterCriticalSection(&s->vt_lock);
    uint16_t cols = 0, rows = 0;
    size_t cell_count = 0, text_bytes = 0;
    int rc = tlg_ghostty_host_snapshot(s->ghostty, NULL, 0, NULL, 0,
        &cols, &rows, &cell_count, &text_bytes, NULL);
    if (rc != TLG_GHOSTTY_ERROR_BUFFER_TOO_SMALL) {
        LeaveCriticalSection(&s->vt_lock);
        return rc;
    }
    tlg_terminal_cell *cells = (tlg_terminal_cell *)calloc(cell_count, sizeof(*cells));
    uint8_t *text = (uint8_t *)malloc(text_bytes ? text_bytes : 1);
    if (!cells || !text) {
        free(cells); free(text); LeaveCriticalSection(&s->vt_lock);
        return TLG_GHOSTTY_ERROR_OUT_OF_MEMORY;
    }
    rc = tlg_ghostty_host_snapshot(s->ghostty, cells, cell_count, text,
        text_bytes, &cols, &rows, &cell_count, &text_bytes, NULL);
    size_t needed = text_bytes + cell_count + (size_t)rows * 2;
    *required_bytes = needed;
    if (rc == TLG_GHOSTTY_OK && (!buffer || capacity < needed))
        rc = TLG_GHOSTTY_ERROR_BUFFER_TOO_SMALL;
    if (rc == TLG_GHOSTTY_OK) {
        size_t out = 0, index = 0;
        for (uint16_t y = 0; y < rows; ++y) {
            for (uint16_t x = 0; x < cols; ++x, ++index) {
                const tlg_terminal_cell *c = &cells[index];
                if (c->text_length) {
                    memcpy(buffer + out, text + c->text_offset, c->text_length);
                    out += c->text_length;
                } else {
                    buffer[out++] = ' ';
                }
            }
            buffer[out++] = '\r'; buffer[out++] = '\n';
        }
        *required_bytes = out;
    }
    free(cells); free(text);
    LeaveCriticalSection(&s->vt_lock);
    return rc;
}

void tlg_ssh_terminal_close(tlg_ssh_terminal *s) {
    if (!s) return;
    tlg_log_ssh("tlg_ssh_terminal_close: closing terminal");
    InterlockedExchange(&s->closing, 1);
    /* Do not hold vt_lock while close joins the ConPTY reader, which may be
     * finishing an output callback that needs that lock. */
    tlg_conpty_close(s->conpty);
    s->conpty = NULL;
    EnterCriticalSection(&s->vt_lock);
    tlg_ghostty_host_destroy(s->ghostty);
    s->ghostty = NULL;
    LeaveCriticalSection(&s->vt_lock);
    DeleteCriticalSection(&s->vt_lock);
    free(s);
    tlg_log_ssh("tlg_ssh_terminal_close: closed successfully");
}
