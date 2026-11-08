#ifndef TERMINAL_LIBGHOSTY_GHOSTTY_HOST_H
#define TERMINAL_LIBGHOSTY_GHOSTTY_HOST_H

#include <stddef.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

/* Stable project-owned ABI between Go/Win32 and the native host. The public
 * Ghostty types never cross this boundary. All calls for one host must be
 * serialized by the caller; callback bytes are borrowed only until return. */
typedef struct tlg_ghostty_host tlg_ghostty_host;
typedef void (*tlg_write_input_fn)(void *userdata, const uint8_t *bytes, size_t len);

typedef struct { uint8_t r, g, b; } tlg_rgb;
typedef struct {
    uint32_t text_offset, text_length, style_flags;
    tlg_rgb foreground, background;
    uint8_t selected;
    uint8_t reserved[3];
} tlg_terminal_cell;
typedef struct {
    uint16_t x, y;
    uint8_t visible, blinking, visual_style;
    uint8_t reserved[3];
} tlg_terminal_cursor;

enum {
    TLG_GHOSTTY_OK = 0,
    TLG_GHOSTTY_ERROR_INVALID_ARGUMENT = -1,
    TLG_GHOSTTY_ERROR_OUT_OF_MEMORY = -2,
    TLG_GHOSTTY_ERROR_UPSTREAM = -3,
    TLG_GHOSTTY_ERROR_BUFFER_TOO_SMALL = -4
};

/* Creates an 80x24 terminal. native_window is reserved for the Win32 renderer
 * and is not retained or dereferenced by the VT bridge. */

int tlg_ghostty_host_create(tlg_ghostty_host **out, void *native_window,
                            tlg_write_input_fn write_input, void *userdata);
int tlg_ghostty_host_feed_output(tlg_ghostty_host *host,
                                 const uint8_t *bytes, size_t len);
int tlg_ghostty_host_resize(tlg_ghostty_host *host, uint32_t cols,
                            uint32_t rows, uint32_t cell_width_px,
                            uint32_t cell_height_px);
/* Query with NULL buffers/capacity 0, allocate required sizes, then call
 * again. All cell data and UTF-8 bytes are copied into caller-owned buffers. */
int tlg_ghostty_host_snapshot(tlg_ghostty_host *host,
                              tlg_terminal_cell *cells, size_t cell_capacity,
                              uint8_t *text, size_t text_capacity,
                              uint16_t *out_cols, uint16_t *out_rows,
                              size_t *required_cells, size_t *required_text_bytes,
                              tlg_terminal_cursor *out_cursor);
/* NULL is accepted. A non-NULL handle must be destroyed exactly once. */
void tlg_ghostty_host_destroy(tlg_ghostty_host *host);

#ifdef __cplusplus
}
#endif
#endif
