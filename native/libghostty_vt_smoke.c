#include <ghostty/vt.h>

#include <stdio.h>
#include <string.h>

int main(void) {
    GhosttyTerminal terminal = NULL;
    GhosttyFormatter formatter = NULL;
    uint8_t *output = NULL;
    size_t output_len = 0;

    GhosttyResult result = ghostty_terminal_new(NULL, &terminal, 80, 24);
    if (result != GHOSTTY_SUCCESS || terminal == NULL) {
        fprintf(stderr, "ghostty_terminal_new failed: %d\n", (int)result);
        return 1;
    }

    static const char input[] =
        "Windows libghostty-vt smoke test: "
        "\x1b[1;32mANSI color works\x1b[0m\r\n"
        "Terminal state is maintained by upstream Ghostty.\r\n";
    ghostty_terminal_vt_write(terminal, (const uint8_t *)input, strlen(input));

    result = ghostty_formatter_terminal_new(NULL, &formatter, terminal);
    if (result != GHOSTTY_SUCCESS || formatter == NULL) {
        fprintf(stderr, "ghostty_formatter_terminal_new failed: %d\n", (int)result);
        ghostty_terminal_free(terminal);
        return 2;
    }

    result = ghostty_formatter_format_alloc(formatter, NULL, &output, &output_len);
    if (result != GHOSTTY_SUCCESS || output == NULL) {
        fprintf(stderr, "ghostty_formatter_format_alloc failed: %d\n", (int)result);
        ghostty_formatter_free(formatter);
        ghostty_terminal_free(terminal);
        return 3;
    }

    fwrite(output, 1, output_len, stdout);
    ghostty_free(NULL, output, output_len);
    ghostty_formatter_free(formatter);
    ghostty_terminal_free(terminal);
    return 0;
}
