#ifndef TERMINAL_LIBGHOSTY_GHOSTTY_HOST_H
#define TERMINAL_LIBGHOSTY_GHOSTTY_HOST_H

#include <stddef.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

/* Stable project-owned ABI between gccgo and the native host.
 * Implementations must be compiled against the exact pinned upstream
 * libghostty embedding header. No upstream symbols are guessed here. */
typedef struct tlg_ghostty_host tlg_ghostty_host;
typedef void (*tlg_write_input_fn)(void *userdata, const uint8_t *bytes, size_t len);

int tlg_ghostty_host_create(tlg_ghostty_host **out, void *native_window,
                            tlg_write_input_fn write_input, void *userdata);
int tlg_ghostty_host_feed_output(tlg_ghostty_host *host,
                                 const uint8_t *bytes, size_t len);
int tlg_ghostty_host_resize(tlg_ghostty_host *host, uint32_t cols,
                            uint32_t rows, uint32_t pixel_width,
                            uint32_t pixel_height);
void tlg_ghostty_host_destroy(tlg_ghostty_host *host);

#ifdef __cplusplus
}
#endif
#endif
