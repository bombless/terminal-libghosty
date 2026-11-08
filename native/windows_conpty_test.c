#define _WIN32_WINNT 0x0A00
#include "windows_conpty.h"
#include <stdio.h>
#include <string.h>

typedef struct { char output[8192]; size_t used; volatile LONG exited; uint32_t exit_code; } test_state;
static void on_output(void *userdata, const uint8_t *bytes, size_t len) {
    test_state *s = (test_state *)userdata;
    size_t remaining = sizeof(s->output) - s->used - 1;
    size_t copy = len < remaining ? len : remaining;
    memcpy(s->output + s->used, bytes, copy); s->used += copy; s->output[s->used] = 0;
}
static void on_exit(void *userdata, uint32_t code) {
    test_state *s = (test_state *)userdata; s->exit_code = code; InterlockedExchange(&s->exited, 1);
}
int main(void) {
    tlg_conpty *session = NULL; test_state state = {0};
    const wchar_t *args[] = {L"/d", L"/q"};
    tlg_conpty_config cfg = {L"C:\\Windows\\System32\\cmd.exe", args, 2, 80, 24, on_output, on_exit, &state};
    uint32_t error = 0, exit_code = 0;
    int rc = tlg_conpty_create(&session, &cfg, &error);
    if (rc != TLG_CONPTY_OK) { fprintf(stderr, "ConPTY create failed: rc=%d win32=%lu\n", rc, (unsigned long)error); return 1; }
    if (tlg_conpty_resize(session, 100, 30, &error) != TLG_CONPTY_OK ||
        tlg_conpty_resize(session, 0, 30, &error) != TLG_CONPTY_ERROR_INVALID_ARGUMENT) {
        fprintf(stderr, "ConPTY resize validation failed: win32=%lu\n", (unsigned long)error);
        tlg_conpty_close(session); return 2;
    }
    static const uint8_t input[] = "echo conpty-input-ok\r\nexit\r\n";
    if (tlg_conpty_write(session, input, sizeof(input) - 1, &error) != TLG_CONPTY_OK) {
        fprintf(stderr, "ConPTY input write failed: win32=%lu\n", (unsigned long)error);
        tlg_conpty_close(session); return 3;
    }
    for (int i = 0; i < 100; ++i) { if (tlg_conpty_poll_exit(session, &exit_code, &error) == 1) break; Sleep(50); }
    for (int i = 0; i < 40 && !InterlockedCompareExchange(&state.exited, 0, 0); ++i) Sleep(25);
    int ok = strstr(state.output, "conpty-input-ok") != NULL && exit_code == 0;
    if (!ok) fprintf(stderr, "unexpected output/exit: [%s], exit=%lu\n", state.output, (unsigned long)exit_code);
    tlg_conpty_close(session);
    return ok ? 0 : 4;
}
