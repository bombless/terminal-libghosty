#define WIN32_LEAN_AND_MEAN
#include "ssh_terminal.h"

#include <stdio.h>
#include <stdlib.h>
#include <string.h>

static int vt_query_child(void) {
    static const char query[] = "\x1b[5n";
    HANDLE input = GetStdHandle(STD_INPUT_HANDLE);
    HANDLE output = GetStdHandle(STD_OUTPUT_HANDLE);
    DWORD mode = 0;
    if (!GetConsoleMode(input, &mode) ||
        !SetConsoleMode(input, (mode & ~(ENABLE_LINE_INPUT | ENABLE_ECHO_INPUT |
                                         ENABLE_PROCESSED_INPUT)) |
                               ENABLE_VIRTUAL_TERMINAL_INPUT)) return 19;
    DWORD written = 0;
    if (!WriteFile(output, query, (DWORD)(sizeof(query) - 1), &written, NULL) ||
        written != sizeof(query) - 1) return 20;

    char response[64] = {0};
    size_t used = 0;
    while (used < sizeof(response) - 1) {
        DWORD count = 0;
        if (!ReadFile(input, response + used,
                      (DWORD)(sizeof(response) - 1 - used), &count, NULL) ||
            count == 0) return 21;
        used += count;
        if (memchr(response, 'n', used) != NULL) break;
    }
    if (used < 3 || response[0] != '\x1b' || response[1] != '[' ||
        response[used - 1] != 'n') return 22;

    static const char ok[] = "ghostty-reply-ok\r\n";
    if (!WriteFile(output, ok, (DWORD)(sizeof(ok) - 1), &written, NULL) ||
        written != sizeof(ok) - 1) return 23;
    return 0;
}

static int check_ghostty_reply(void) {
    wchar_t executable[MAX_PATH];
    DWORD n = GetModuleFileNameW(NULL, executable, MAX_PATH);
    if (!n || n >= MAX_PATH) return 10;
    const wchar_t *args[] = {L"--vt-query-child"};
    tlg_ssh_terminal *s = NULL;
    uint32_t error = 0, exit_code = 99;
    int rc = tlg_ssh_terminal_create_argv(&s, executable, args, 1,
        100, 30, 9, 18, &error);
    if (rc != TLG_CONPTY_OK) {
        fprintf(stderr, "VT reply session create failed rc=%d win32=%lu\n",
            rc, (unsigned long)error);
        return 11;
    }
    int exited = 0;
    for (int i = 0; i < 100; ++i) {
        rc = tlg_ssh_terminal_poll_exit(s, &exit_code, &error);
        if (rc == 1) { exited = 1; break; }
        if (rc < 0) break;
        Sleep(50);
    }
    Sleep(100); /* let the reader worker drain the final output bytes */
    size_t required = 0;
    (void)tlg_ssh_terminal_read_text(s, NULL, 0, &required);
    uint8_t *screen = (uint8_t *)calloc(required + 1, 1);
    if (screen) (void)tlg_ssh_terminal_read_text(s, screen, required, &required);
    int found = screen && strstr((char *)screen, "ghostty-reply-ok") != NULL;
    if (!exited || exit_code != 0 || !found) {
        fprintf(stderr, "Ghostty reply round-trip failed: exited=%d code=%lu screen=[%s]\n",
            exited, (unsigned long)exit_code, screen ? (char *)screen : "");
        free(screen); tlg_ssh_terminal_close(s); return 12;
    }
    free(screen);
    tlg_ssh_terminal_close(s);
    return 0;
}

static int check_conpty_round_trip(void) {
    wchar_t system_dir[MAX_PATH];
    UINT n = GetSystemDirectoryW(system_dir, MAX_PATH);
    if (!n || n >= MAX_PATH) return 1;
    wcscat(system_dir, L"\\cmd.exe");
    const wchar_t *args[] = {L"/d", L"/q"};
    tlg_ssh_terminal *s = NULL;
    uint32_t error = 0, exit_code = 99;
    if (tlg_ssh_terminal_create_argv(&s, system_dir, args, 2,
        80, 24, 9, 18, &error) != TLG_CONPTY_OK) return 2;
    if (tlg_ssh_terminal_resize(s, 100, 30, 9, 18, &error) != TLG_CONPTY_OK) {
        tlg_ssh_terminal_close(s); return 3;
    }
    static const uint8_t input[] = "echo bridge-round-trip-ok\r\nexit\r\n";
    if (tlg_ssh_terminal_write(s, input, sizeof(input) - 1, &error) != TLG_CONPTY_OK) {
        tlg_ssh_terminal_close(s); return 4;
    }
    int exited = 0;
    for (int i = 0; i < 120; ++i) {
        int rc = tlg_ssh_terminal_poll_exit(s, &exit_code, &error);
        if (rc == 1) { exited = 1; break; }
        if (rc < 0) break;
        Sleep(50);
    }
    Sleep(100);
    size_t required = 0;
    (void)tlg_ssh_terminal_read_text(s, NULL, 0, &required);
    uint8_t *screen = (uint8_t *)calloc(required + 1, 1);
    if (screen) (void)tlg_ssh_terminal_read_text(s, screen, required, &required);
    int found = screen && strstr((char *)screen, "bridge-round-trip-ok") != NULL;
    if (!exited || exit_code != 0 || !found) {
        fprintf(stderr, "ConPTY/VT output round-trip failed: exited=%d code=%lu screen=[%s]\n",
            exited, (unsigned long)exit_code, screen ? (char *)screen : "");
        free(screen); tlg_ssh_terminal_close(s); return 5;
    }
    free(screen);
    tlg_ssh_terminal_close(s);
    return 0;
}

static int check_ssh_version(void) {
    wchar_t system_dir[MAX_PATH];
    UINT n = GetSystemDirectoryW(system_dir, MAX_PATH);
    if (!n || n >= MAX_PATH) return 1;
    wcscat(system_dir, L"\\OpenSSH\\ssh.exe");
    const wchar_t *args[] = {L"-V"};
    tlg_ssh_terminal *s = NULL;
    uint32_t error = 0, exit_code = 99;
    int rc = tlg_ssh_terminal_create_argv(&s, system_dir, args, 1,
        100, 30, 9, 18, &error);
    if (rc != TLG_CONPTY_OK) {
        fprintf(stderr, "SSH/ConPTY/VT create failed rc=%d win32=%lu\n",
            rc, (unsigned long)error);
        return 2;
    }
    if (tlg_ssh_terminal_resize(s, 120, 36, 9, 18, &error) != TLG_CONPTY_OK) {
        tlg_ssh_terminal_close(s); return 3;
    }
    int exited = 0;
    for (int i = 0; i < 100; ++i) {
        rc = tlg_ssh_terminal_poll_exit(s, &exit_code, &error);
        if (rc == 1) { exited = 1; break; }
        if (rc < 0) break;
        Sleep(50);
    }
    Sleep(100);
    size_t required = 0;
    (void)tlg_ssh_terminal_read_text(s, NULL, 0, &required);
    uint8_t *screen = (uint8_t *)calloc(required + 1, 1);
    if (screen) (void)tlg_ssh_terminal_read_text(s, screen, required, &required);
    int found = screen && strstr((char *)screen, "OpenSSH") != NULL;
    if (!exited || exit_code != 0 || !found) {
        fprintf(stderr, "SSH bridge failed: exited=%d code=%lu screen=[%s]\n",
            exited, (unsigned long)exit_code, screen ? (char *)screen : "");
        free(screen); tlg_ssh_terminal_close(s); return 4;
    }
    free(screen);
    tlg_ssh_terminal_close(s);
    return 0;
}

int main(int argc, char **argv) {
    if (argc > 1 && strcmp(argv[1], "--vt-query-child") == 0)
        return vt_query_child();
    int rc = check_conpty_round_trip();
    if (rc) return rc;
    rc = check_ghostty_reply();
    if (rc) return rc;
    rc = check_ssh_version();
    if (rc) return rc;
    puts("SSH -> ConPTY -> Ghostty VT bridge end-to-end: OK");
    return 0;
}
