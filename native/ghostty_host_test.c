#include "ghostty_host.h"

#include <stdio.h>
#include <stdlib.h>
#include <string.h>

typedef struct {
    uint8_t bytes[128];
    size_t len;
    unsigned calls;
} callback_capture;

static void capture_input(void *userdata, const uint8_t *bytes, size_t len) {
    callback_capture *capture = (callback_capture *)userdata;
    if (capture == NULL || bytes == NULL || len > sizeof(capture->bytes)) return;
    memcpy(capture->bytes, bytes, len);
    capture->len = len;
    capture->calls++;
}

static int fail(const char *message) {
    fprintf(stderr, "ghostty_host_test: %s\n", message);
    return 1;
}

int main(void) {
    callback_capture capture = {0};
    tlg_ghostty_host *host = NULL;
    if (tlg_ghostty_host_create(&host, NULL, capture_input, &capture) != TLG_GHOSTTY_OK ||
        host == NULL) return fail("create failed");

    static const uint8_t output[] =
        "bridge UTF-8: \xe4\xb8\xad\xe6\x96\x87 "
        "\x1b[1;32mgreen\x1b[0m\r\n\x1b[5n";
    if (tlg_ghostty_host_feed_output(host, output, sizeof(output) - 1) != TLG_GHOSTTY_OK)
        return fail("feed_output rejected valid bytes");
    if (capture.calls == 0 || capture.len == 0)
        return fail("Ghostty write_pty callback was not forwarded");

    uint16_t cols = 0, rows = 0;
    size_t cell_count = 0, text_bytes = 0;
    tlg_terminal_cursor cursor = {0};
    int snapshot_result = tlg_ghostty_host_snapshot(host, NULL, 0, NULL, 0,
        &cols, &rows, &cell_count, &text_bytes, &cursor);
    if (snapshot_result != TLG_GHOSTTY_ERROR_BUFFER_TOO_SMALL)
        return fail("snapshot sizing call did not request caller buffers");
    if (cols != 80 || rows != 24 || cell_count != (size_t)cols * rows || text_bytes == 0)
        return fail("snapshot sizing returned invalid dimensions or lengths");
    tlg_terminal_cell *cells = (tlg_terminal_cell *)calloc(cell_count, sizeof(*cells));
    uint8_t *text = (uint8_t *)calloc(text_bytes + 1, 1);
    if (cells == NULL || text == NULL) return fail("snapshot test allocation failed");
    if (tlg_ghostty_host_snapshot(host, cells, cell_count, text, text_bytes,
        &cols, &rows, &cell_count, &text_bytes, &cursor) != TLG_GHOSTTY_OK)
        return fail("snapshot copy call failed");
    if (cursor.visible && (cursor.x >= cols || cursor.y >= rows))
        return fail("snapshot cursor position is outside the viewport");
    text[text_bytes] = 0;
    int found_chinese = 0, found_bold_green = 0, found_green_color = 0;
    for (size_t i = 0; i < cell_count; i++) {
        if (cells[i].text_length == 3 &&
            memcmp(text + cells[i].text_offset, "\xe4\xb8\xad", 3) == 0)
            found_chinese = 1;
        if (cells[i].text_length == 1 && text[cells[i].text_offset] == 'g') {
            if ((cells[i].style_flags & 1u) != 0) found_bold_green = 1;
            if (cells[i].foreground.g > cells[i].foreground.r &&
                cells[i].foreground.g > cells[i].foreground.b) found_green_color = 1;
        }
    }
    free(text);
    free(cells);
    if (!found_chinese) return fail("snapshot lost UTF-8 grapheme content");
    if (!found_bold_green) return fail("snapshot lost ANSI bold style");
    if (!found_green_color) return fail("snapshot lost resolved ANSI foreground color");

    if (tlg_ghostty_host_resize(host, 120, 40, 9, 18) != TLG_GHOSTTY_OK)
        return fail("valid resize failed");
    snapshot_result = tlg_ghostty_host_snapshot(host, NULL, 0, NULL, 0,
        &cols, &rows, &cell_count, &text_bytes, &cursor);
    if (snapshot_result != TLG_GHOSTTY_ERROR_BUFFER_TOO_SMALL ||
        cols != 120 || rows != 40 || cell_count != 4800)
        return fail("snapshot dimensions did not follow terminal resize");
    if (tlg_ghostty_host_resize(host, 0, 40, 9, 18) != TLG_GHOSTTY_ERROR_INVALID_ARGUMENT)
        return fail("zero-column resize was not rejected");
    if (tlg_ghostty_host_resize(host, 65536, 40, 9, 18) != TLG_GHOSTTY_ERROR_INVALID_ARGUMENT)
        return fail("oversized resize was not rejected");
    if (tlg_ghostty_host_feed_output(host, NULL, 1) != TLG_GHOSTTY_ERROR_INVALID_ARGUMENT)
        return fail("NULL input with nonzero length was not rejected");

    tlg_ghostty_host_destroy(host);
    tlg_ghostty_host_destroy(NULL);
    puts("Ghostty host bridge lifecycle, VT feed, callback, and resize: OK");
    return 0;
}