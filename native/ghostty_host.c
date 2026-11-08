#include "ghostty_host.h"

#include <ghostty/vt.h>

#include <stdlib.h>

struct tlg_ghostty_host {
    GhosttyTerminal terminal;
    GhosttyRenderState render_state;
    GhosttyRenderStateRowIterator row_iterator;
    GhosttyRenderStateRowCells row_cells;
    tlg_write_input_fn write_input;
    void *userdata;
};

static tlg_rgb tlg_copy_rgb(GhosttyColorRgb c) {
    tlg_rgb out = {c.r, c.g, c.b};
    return out;
}

static GhosttyResult tlg_walk_cells(tlg_ghostty_host *host,
                                   tlg_terminal_cell *cells, uint8_t *text,
                                   size_t *text_bytes, bool copy) {
    size_t index = 0, offset = 0;
    GhosttyRenderStateColors colors = GHOSTTY_INIT_SIZED(GhosttyRenderStateColors);
    GhosttyResult r = ghostty_render_state_get(host->render_state,
        GHOSTTY_RENDER_STATE_DATA_COLORS, &colors);
    if (r != GHOSTTY_SUCCESS) return r;
    while (ghostty_render_state_row_iterator_next(host->row_iterator)) {
        r = ghostty_render_state_row_get(host->row_iterator,
            GHOSTTY_RENDER_STATE_ROW_DATA_CELLS, &host->row_cells);
        if (r != GHOSTTY_SUCCESS) return r;
        while (ghostty_render_state_row_cells_next(host->row_cells)) {
            GhosttyBuffer g = {.ptr = NULL, .cap = 0, .len = 0};
            r = ghostty_render_state_row_cells_get(host->row_cells,
                GHOSTTY_RENDER_STATE_ROW_CELLS_DATA_GRAPHEMES_UTF8, &g);
            if (r != GHOSTTY_OUT_OF_SPACE && r != GHOSTTY_SUCCESS) return r;
            if (g.len > UINT32_MAX || offset > UINT32_MAX - g.len) return GHOSTTY_OUT_OF_SPACE;
            if (copy) {
                tlg_terminal_cell *c = &cells[index];
                c->text_offset = (uint32_t)offset;
                c->text_length = (uint32_t)g.len;
                GhosttyStyle style;
                r = ghostty_render_state_row_cells_get(host->row_cells,
                    GHOSTTY_RENDER_STATE_ROW_CELLS_DATA_STYLE, &style);
                if (r != GHOSTTY_SUCCESS) return r;
                c->style_flags = (uint32_t)style.flags;
                c->foreground = tlg_copy_rgb(colors.foreground);
                c->background = tlg_copy_rgb(colors.background);
                GhosttyColorRgb color;
                r = ghostty_render_state_row_cells_get(host->row_cells,
                    GHOSTTY_RENDER_STATE_ROW_CELLS_DATA_FG_COLOR, &color);
                if (r == GHOSTTY_SUCCESS) c->foreground = tlg_copy_rgb(color);
                else if (r != GHOSTTY_NO_VALUE) return r;
                r = ghostty_render_state_row_cells_get(host->row_cells,
                    GHOSTTY_RENDER_STATE_ROW_CELLS_DATA_BG_COLOR, &color);
                if (r == GHOSTTY_SUCCESS) c->background = tlg_copy_rgb(color);
                else if (r != GHOSTTY_NO_VALUE) return r;
                bool selected = false;
                r = ghostty_render_state_row_cells_get(host->row_cells,
                    GHOSTTY_RENDER_STATE_ROW_CELLS_DATA_SELECTED, &selected);
                if (r != GHOSTTY_SUCCESS) return r;
                c->selected = selected ? 1 : 0;
                if (g.len) {
                    g.ptr = text + offset;
                    g.cap = g.len;
                    r = ghostty_render_state_row_cells_get(host->row_cells,
                        GHOSTTY_RENDER_STATE_ROW_CELLS_DATA_GRAPHEMES_UTF8, &g);
                    if (r != GHOSTTY_SUCCESS) return r;
                }
            }
            offset += g.len;
            index++;
        }
    }
    *text_bytes = offset;
    return GHOSTTY_SUCCESS;
}

static void tlg_on_write_pty(GhosttyTerminal terminal, void *userdata,
                             const uint8_t *bytes, size_t len) {
    (void)terminal;
    tlg_ghostty_host *host = (tlg_ghostty_host *)userdata;
    if (host != NULL && host->write_input != NULL && bytes != NULL && len != 0) {
        /* The upstream buffer is borrowed only for this callback. The client
         * must copy/enqueue it and must not re-enter the Ghostty terminal. */
        host->write_input(host->userdata, bytes, len);
    }
}

int tlg_ghostty_host_create(tlg_ghostty_host **out, void *native_window,
                            tlg_write_input_fn write_input, void *userdata) {
    if (out == NULL) return TLG_GHOSTTY_ERROR_INVALID_ARGUMENT;
    *out = NULL;
    (void)native_window;

    tlg_ghostty_host *host = (tlg_ghostty_host *)calloc(1, sizeof(*host));
    if (host == NULL) return TLG_GHOSTTY_ERROR_OUT_OF_MEMORY;
    host->write_input = write_input;
    host->userdata = userdata;

    GhosttyResult result = ghostty_render_state_new(NULL, &host->render_state);
    if (result == GHOSTTY_SUCCESS)
        result = ghostty_render_state_row_iterator_new(NULL, &host->row_iterator);
    if (result == GHOSTTY_SUCCESS)
        result = ghostty_render_state_row_cells_new(NULL, &host->row_cells);
    if (result != GHOSTTY_SUCCESS || !host->render_state || !host->row_iterator || !host->row_cells) {
        ghostty_render_state_row_cells_free(host->row_cells);
        ghostty_render_state_row_iterator_free(host->row_iterator);
        ghostty_render_state_free(host->render_state);
        free(host);
        return result == GHOSTTY_OUT_OF_MEMORY ? TLG_GHOSTTY_ERROR_OUT_OF_MEMORY : TLG_GHOSTTY_ERROR_UPSTREAM;
    }
    result = ghostty_terminal_new(NULL, &host->terminal, 80, 24);
    if (result != GHOSTTY_SUCCESS || host->terminal == NULL) {
        ghostty_render_state_row_cells_free(host->row_cells);
        ghostty_render_state_row_iterator_free(host->row_iterator);
        ghostty_render_state_free(host->render_state);
        free(host);
        return result == GHOSTTY_OUT_OF_MEMORY
                   ? TLG_GHOSTTY_ERROR_OUT_OF_MEMORY
                   : TLG_GHOSTTY_ERROR_UPSTREAM;
    }

    result = ghostty_terminal_set(host->terminal, GHOSTTY_TERMINAL_OPT_USERDATA,
                                  host);
    if (result == GHOSTTY_SUCCESS) {
        result = ghostty_terminal_set(host->terminal,
                                     GHOSTTY_TERMINAL_OPT_WRITE_PTY,
                                     (const void *)tlg_on_write_pty);
    }
    if (result != GHOSTTY_SUCCESS) {
        ghostty_terminal_free(host->terminal);
        ghostty_render_state_row_cells_free(host->row_cells);
        ghostty_render_state_row_iterator_free(host->row_iterator);
        ghostty_render_state_free(host->render_state);
        free(host);
        return TLG_GHOSTTY_ERROR_UPSTREAM;
    }

    *out = host;
    return TLG_GHOSTTY_OK;
}

int tlg_ghostty_host_feed_output(tlg_ghostty_host *host,
                                 const uint8_t *bytes, size_t len) {
    if (host == NULL || (bytes == NULL && len != 0))
        return TLG_GHOSTTY_ERROR_INVALID_ARGUMENT;
    if (len != 0) ghostty_terminal_vt_write(host->terminal, bytes, len);
    return TLG_GHOSTTY_OK;
}

int tlg_ghostty_host_resize(tlg_ghostty_host *host, uint32_t cols,
                            uint32_t rows, uint32_t cell_width_px,
                            uint32_t cell_height_px) {
    if (host == NULL || cols == 0 || rows == 0 || cols > UINT16_MAX ||
        rows > UINT16_MAX || cell_width_px == 0 || cell_height_px == 0)
        return TLG_GHOSTTY_ERROR_INVALID_ARGUMENT;
    GhosttyResult result = ghostty_terminal_resize(
        host->terminal, (uint16_t)cols, (uint16_t)rows,
        cell_width_px, cell_height_px);
    return result == GHOSTTY_SUCCESS ? TLG_GHOSTTY_OK
                                     : TLG_GHOSTTY_ERROR_UPSTREAM;
}

int tlg_ghostty_host_snapshot(tlg_ghostty_host *host,
                              tlg_terminal_cell *cells, size_t cell_capacity,
                              uint8_t *text, size_t text_capacity,
                              uint16_t *out_cols, uint16_t *out_rows,
                              size_t *required_cells, size_t *required_text_bytes,
                              tlg_terminal_cursor *out_cursor) {
    if (!host || !out_cols || !out_rows || !required_cells || !required_text_bytes ||
        (!cells && cell_capacity) || (!text && text_capacity))
        return TLG_GHOSTTY_ERROR_INVALID_ARGUMENT;
    GhosttyResult r = ghostty_render_state_update(host->render_state, host->terminal);
    if (r != GHOSTTY_SUCCESS) return TLG_GHOSTTY_ERROR_UPSTREAM;
    uint16_t cols = 0, rows = 0;
    if (ghostty_render_state_get(host->render_state, GHOSTTY_RENDER_STATE_DATA_COLS, &cols) != GHOSTTY_SUCCESS ||
        ghostty_render_state_get(host->render_state, GHOSTTY_RENDER_STATE_DATA_ROWS, &rows) != GHOSTTY_SUCCESS)
        return TLG_GHOSTTY_ERROR_UPSTREAM;
    *out_cols = cols; *out_rows = rows;
    *required_cells = (size_t)cols * (size_t)rows;
    *required_text_bytes = 0;
    if (out_cursor) {
        GhosttyRenderStateCursor cursor = GHOSTTY_INIT_SIZED(GhosttyRenderStateCursor);
        r = ghostty_render_state_get(host->render_state, GHOSTTY_RENDER_STATE_DATA_CURSOR, &cursor);
        if (r != GHOSTTY_SUCCESS) return TLG_GHOSTTY_ERROR_UPSTREAM;
        out_cursor->x = cursor.viewport_has_value ? cursor.viewport_x : 0;
        out_cursor->y = cursor.viewport_has_value ? cursor.viewport_y : 0;
        out_cursor->visible = cursor.visible && cursor.viewport_has_value ? 1 : 0;
        out_cursor->blinking = cursor.blinking ? 1 : 0;
        out_cursor->visual_style = (uint8_t)cursor.visual_style;
    }
    r = ghostty_render_state_get(host->render_state, GHOSTTY_RENDER_STATE_DATA_ROW_ITERATOR, &host->row_iterator);
    if (r != GHOSTTY_SUCCESS) return TLG_GHOSTTY_ERROR_UPSTREAM;
    size_t needed = 0;
    r = tlg_walk_cells(host, NULL, NULL, &needed, false);
    if (r != GHOSTTY_SUCCESS) return TLG_GHOSTTY_ERROR_UPSTREAM;
    *required_text_bytes = needed;
    if (!cells || cell_capacity < *required_cells ||
        (needed && (!text || text_capacity < needed)))
        return TLG_GHOSTTY_ERROR_BUFFER_TOO_SMALL;
    r = ghostty_render_state_get(host->render_state, GHOSTTY_RENDER_STATE_DATA_ROW_ITERATOR, &host->row_iterator);
    if (r != GHOSTTY_SUCCESS) return TLG_GHOSTTY_ERROR_UPSTREAM;
    size_t copied = 0;
    r = tlg_walk_cells(host, cells, text, &copied, true);
    if (r != GHOSTTY_SUCCESS || copied != needed) return TLG_GHOSTTY_ERROR_UPSTREAM;
    r = ghostty_render_state_clean(host->render_state);
    return r == GHOSTTY_SUCCESS ? TLG_GHOSTTY_OK : TLG_GHOSTTY_ERROR_UPSTREAM;
}

void tlg_ghostty_host_destroy(tlg_ghostty_host *host) {
    if (host == NULL) return;
    ghostty_render_state_row_cells_free(host->row_cells);
    ghostty_render_state_row_iterator_free(host->row_iterator);
    ghostty_render_state_free(host->render_state);
    ghostty_terminal_free(host->terminal);
    host->terminal = NULL;
    free(host);
}