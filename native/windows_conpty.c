#define _WIN32_WINNT 0x0A00
#define WIN32_LEAN_AND_MEAN
#include "windows_conpty.h"

#include <stdio.h>
#include <stdarg.h>
#include <stdlib.h>
#include <string.h>
#include <wchar.h>

#ifndef PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE
#define PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE 0x00020016
#endif

static void tlg_log_conpty(const char *fmt, ...) {
    va_list ap;
    va_start(ap, fmt);
    SYSTEMTIME st;
    GetLocalTime(&st);
    fprintf(stderr, "[conpty %02d:%02d:%02d.%03d] ", st.wHour, st.wMinute, st.wSecond, st.wMilliseconds);
    vfprintf(stderr, fmt, ap);
    fprintf(stderr, "\n");
    fflush(stderr);
    va_end(ap);
}

struct tlg_conpty {
    HANDLE input_write, output_read, process, reader_thread, job;
    HPCON pseudo_console;
    tlg_conpty_output_fn on_output;
    tlg_conpty_exit_fn on_exit;
    void *userdata;
    CRITICAL_SECTION write_lock;
    volatile LONG exit_known;
    DWORD exit_code;
};

static void set_error(uint32_t *out, DWORD error) { if (out) *out = (uint32_t)error; }

static wchar_t *quote_arg(const wchar_t *arg) {
    size_t n = wcslen(arg);
    int needs_quotes = n == 0;
    for (size_t i = 0; i < n; ++i)
        if (arg[i] == L' ' || arg[i] == L'\t' || arg[i] == L'"') needs_quotes = 1;
    if (!needs_quotes) {
        wchar_t *copy = (wchar_t *)calloc(n + 1, sizeof(wchar_t));
        if (copy) memcpy(copy, arg, (n + 1) * sizeof(wchar_t));
        return copy;
    }
    size_t cap = n * 2 + 3, used = 0;
    wchar_t *out = (wchar_t *)calloc(cap, sizeof(wchar_t));
    if (!out) return NULL;
    out[used++] = L'"';
    size_t slashes = 0;
    for (size_t i = 0; i < n; ++i) {
        wchar_t ch = arg[i];
        if (ch == L'\\') { ++slashes; continue; }
        if (ch == L'"') {
            for (size_t j = 0; j < slashes * 2 + 1; ++j) out[used++] = L'\\';
            out[used++] = L'"'; slashes = 0; continue;
        }
        for (size_t j = 0; j < slashes; ++j) out[used++] = L'\\';
        slashes = 0; out[used++] = ch;
    }
    for (size_t j = 0; j < slashes * 2; ++j) out[used++] = L'\\';
    out[used++] = L'"'; out[used] = L'\0';
    return out;
}

static wchar_t *build_command_line(const tlg_conpty_config *cfg) {
    wchar_t *exe = quote_arg(cfg->executable);
    if (!exe) return NULL;
    size_t total = wcslen(exe) + 1;
    wchar_t **args = cfg->argc ? (wchar_t **)calloc(cfg->argc, sizeof(*args)) : NULL;
    if (cfg->argc && !args) { free(exe); return NULL; }
    for (size_t i = 0; i < cfg->argc; ++i) {
        if (!cfg->argv[i] || !(args[i] = quote_arg(cfg->argv[i]))) {
            for (size_t j = 0; j < i; ++j) free(args[j]);
            free(args); free(exe); return NULL;
        }
        total += wcslen(args[i]) + 1;
    }
    wchar_t *line = (wchar_t *)calloc(total, sizeof(wchar_t));
    if (line) {
        wcscpy(line, exe);
        for (size_t i = 0; i < cfg->argc; ++i) { wcscat(line, L" "); wcscat(line, args[i]); }
    }
    for (size_t i = 0; i < cfg->argc; ++i) free(args[i]);
    free(args); free(exe);
    return line;
}

static DWORD WINAPI output_worker(void *arg) {
    tlg_conpty *s = (tlg_conpty *)arg;
    uint8_t buffer[16 * 1024];
    tlg_log_conpty("output_worker: reader thread started");
    for (;;) {
        DWORD count = 0;
        BOOL ok = ReadFile(s->output_read, buffer, (DWORD)sizeof(buffer), &count, NULL);
        if (!ok || count == 0) {
            DWORD err = GetLastError();
            tlg_log_conpty("output_worker: ReadFile exited (ok=%d, count=%lu, err=%lu)", ok, (unsigned long)count, (unsigned long)err);
            break;
        }
        if (s->on_output) s->on_output(s->userdata, buffer, (size_t)count);
    }
    tlg_log_conpty("output_worker: waiting for child process to exit");
    WaitForSingleObject(s->process, INFINITE);
    DWORD code = 1;
    if (GetExitCodeProcess(s->process, &code)) {
        s->exit_code = code; InterlockedExchange(&s->exit_known, 1);
        tlg_log_conpty("output_worker: child process exited with code %lu", (unsigned long)code);
        if (s->on_exit) s->on_exit(s->userdata, (uint32_t)code);
    }
    return 0;
}

static void close_handle(HANDLE *h) {
    if (*h && *h != INVALID_HANDLE_VALUE) CloseHandle(*h);
    *h = NULL;
}

int tlg_conpty_create(tlg_conpty **out, const tlg_conpty_config *cfg, uint32_t *out_error) {
    if (out) *out = NULL;
    set_error(out_error, ERROR_SUCCESS);
    if (!out || !cfg || !cfg->executable || !cfg->executable[0] ||
        (cfg->argc && !cfg->argv) || !cfg->cols || !cfg->rows)
        return TLG_CONPTY_ERROR_INVALID_ARGUMENT;
    tlg_conpty *s = (tlg_conpty *)calloc(1, sizeof(*s));
    if (!s) return TLG_CONPTY_ERROR_OUT_OF_MEMORY;
    s->on_output = cfg->on_output; s->on_exit = cfg->on_exit; s->userdata = cfg->userdata;
    InitializeCriticalSection(&s->write_lock);

    HANDLE child_input = NULL, child_output = NULL;
    SIZE_T attr_size = 0;
    LPPROC_THREAD_ATTRIBUTE_LIST attrs = NULL;
    PROCESS_INFORMATION pi; STARTUPINFOEXW si;
    memset(&pi, 0, sizeof(pi)); memset(&si, 0, sizeof(si));
    int result = TLG_CONPTY_ERROR_WIN32; DWORD error = ERROR_SUCCESS;

    SECURITY_ATTRIBUTES pipe_security = { sizeof(SECURITY_ATTRIBUTES), NULL, TRUE };
    if (!CreatePipe(&child_input, &s->input_write, &pipe_security, 0) ||
        !CreatePipe(&s->output_read, &child_output, &pipe_security, 0)) { error = GetLastError(); goto fail; }
    /* Only the pseudo-console-side ends may be inherited by the console host. */
    if (!SetHandleInformation(s->input_write, HANDLE_FLAG_INHERIT, 0) ||
        !SetHandleInformation(s->output_read, HANDLE_FLAG_INHERIT, 0)) { error = GetLastError(); goto fail; }
    HRESULT hr = CreatePseudoConsole((COORD){(SHORT)cfg->cols, (SHORT)cfg->rows},
                                     child_input, child_output, 0, &s->pseudo_console);
    if (FAILED(hr)) { error = (DWORD)HRESULT_CODE(hr); if (!error) error = ERROR_GEN_FAILURE; goto fail; }
    close_handle(&child_input); close_handle(&child_output);

    InitializeProcThreadAttributeList(NULL, 1, 0, &attr_size);
    attrs = (LPPROC_THREAD_ATTRIBUTE_LIST)malloc(attr_size);
    if (!attrs) { result = TLG_CONPTY_ERROR_OUT_OF_MEMORY; goto fail; }
    if (!InitializeProcThreadAttributeList(attrs, 1, 0, &attr_size) ||
        !UpdateProcThreadAttribute(attrs, 0, PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE,
                                   s->pseudo_console, sizeof(s->pseudo_console), NULL, NULL)) {
        error = GetLastError(); goto fail;
    }
    si.StartupInfo.cb = sizeof(si);
    /* Required by the current Windows cmd.exe smoke-test path so its standard
     * streams are bound to the pseudoconsole-backed process handles. */
    si.StartupInfo.dwFlags |= STARTF_USESTDHANDLES;
    si.StartupInfo.hStdInput = NULL;
    si.StartupInfo.hStdOutput = NULL;
    si.StartupInfo.hStdError = NULL;
    si.lpAttributeList = attrs;
    wchar_t *command_line = build_command_line(cfg);
    if (!command_line) { result = TLG_CONPTY_ERROR_OUT_OF_MEMORY; goto fail; }
    BOOL created = CreateProcessW(cfg->executable, command_line, NULL, NULL, FALSE,
        EXTENDED_STARTUPINFO_PRESENT | CREATE_UNICODE_ENVIRONMENT, NULL, NULL, &si.StartupInfo, &pi);
    free(command_line);
    if (!created) { error = GetLastError(); goto fail; }
    close_handle(&pi.hThread); s->process = pi.hProcess;
    DeleteProcThreadAttributeList(attrs); free(attrs); attrs = NULL;

    /* Bind child process to a Job Object configured with KILL_ON_JOB_CLOSE so
     * that when this host process terminates for any reason, all child and
     * grandchild processes (including conhost and ssh) are automatically terminated. */
    HANDLE job = CreateJobObjectW(NULL, NULL);
    if (job) {
        JOBOBJECT_EXTENDED_LIMIT_INFORMATION jeli;
        memset(&jeli, 0, sizeof(jeli));
        jeli.BasicLimitInformation.LimitFlags = JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE;
        if (SetInformationJobObject(job, JobObjectExtendedLimitInformation, &jeli, sizeof(jeli))) {
            AssignProcessToJobObject(job, pi.hProcess);
            s->job = job;
            tlg_log_conpty("tlg_conpty_create: child process assigned to auto-kill job object");
        } else {
            close_handle(&job);
        }
    }

    /* Publish the session handle before the reader can deliver early child
     * output. The combined SSH/VT callback may synchronously answer an
     * immediate terminal query as soon as the first bytes arrive. */
    *out = s;
    s->reader_thread = CreateThread(NULL, 0, output_worker, s, 0, NULL);
    if (!s->reader_thread) { *out = NULL; error = GetLastError(); goto fail; }
    tlg_log_conpty("tlg_conpty_create: session created successfully (pid=%lu)", (unsigned long)GetProcessId(s->process));
    return TLG_CONPTY_OK;

fail:
    if (attrs) { DeleteProcThreadAttributeList(attrs); free(attrs); }
    close_handle(&child_input); close_handle(&child_output);
    if (s->process) { TerminateProcess(s->process, 1); WaitForSingleObject(s->process, 5000); }
    if (s->pseudo_console) { ClosePseudoConsole(s->pseudo_console); s->pseudo_console = NULL; }
    if (pi.hThread) close_handle(&pi.hThread);
    close_handle(&pi.hProcess); close_handle(&s->input_write); close_handle(&s->output_read);
    close_handle(&s->process); close_handle(&s->reader_thread); close_handle(&s->job);
    DeleteCriticalSection(&s->write_lock); free(s);
    set_error(out_error, error);
    tlg_log_conpty("tlg_conpty_create: failed with error %lu", (unsigned long)error);
    return result;
}

int tlg_conpty_write(tlg_conpty *s, const uint8_t *bytes, size_t len, uint32_t *out_error) {
    set_error(out_error, ERROR_SUCCESS);
    if (!s || (!bytes && len)) return TLG_CONPTY_ERROR_INVALID_ARGUMENT;
    tlg_log_conpty("tlg_conpty_write: writing %zu bytes", len);
    EnterCriticalSection(&s->write_lock);
    size_t offset = 0;
    while (offset < len) {
        DWORD chunk = (DWORD)((len - offset) > 0x7fffffffU ? 0x7fffffffU : (len - offset));
        DWORD written = 0;
        if (!WriteFile(s->input_write, bytes + offset, chunk, &written, NULL) || !written) {
            DWORD error = GetLastError(); LeaveCriticalSection(&s->write_lock);
            set_error(out_error, error);
            tlg_log_conpty("tlg_conpty_write: WriteFile failed: err=%lu written=%lu", (unsigned long)error, (unsigned long)written);
            return TLG_CONPTY_ERROR_WIN32;
        }
        offset += written;
    }
    LeaveCriticalSection(&s->write_lock);
    tlg_log_conpty("tlg_conpty_write: successfully wrote %zu bytes", len);
    return TLG_CONPTY_OK;
}

int tlg_conpty_resize(tlg_conpty *s, uint16_t cols, uint16_t rows, uint32_t *out_error) {
    set_error(out_error, ERROR_SUCCESS);
    if (!s || !cols || !rows) return TLG_CONPTY_ERROR_INVALID_ARGUMENT;
    tlg_log_conpty("tlg_conpty_resize: cols=%u rows=%u", cols, rows);
    HRESULT hr = ResizePseudoConsole(s->pseudo_console, (COORD){(SHORT)cols, (SHORT)rows});
    if (FAILED(hr)) {
        set_error(out_error, (DWORD)HRESULT_CODE(hr));
        tlg_log_conpty("tlg_conpty_resize: ResizePseudoConsole failed: hr=0x%08lX", (unsigned long)hr);
        return TLG_CONPTY_ERROR_WIN32;
    }
    return TLG_CONPTY_OK;
}

int tlg_conpty_poll_exit(tlg_conpty *s, uint32_t *out_code, uint32_t *out_error) {
    set_error(out_error, ERROR_SUCCESS);
    if (!s) return TLG_CONPTY_ERROR_INVALID_ARGUMENT;
    if (!InterlockedCompareExchange(&s->exit_known, 0, 0)) {
        DWORD wait = WaitForSingleObject(s->process, 0);
        if (wait == WAIT_TIMEOUT) return 0;
        if (wait != WAIT_OBJECT_0) { set_error(out_error, GetLastError()); return TLG_CONPTY_ERROR_WIN32; }
        DWORD code = 1;
        if (!GetExitCodeProcess(s->process, &code)) { set_error(out_error, GetLastError()); return TLG_CONPTY_ERROR_WIN32; }
        s->exit_code = code; InterlockedExchange(&s->exit_known, 1);
    }
    if (out_code) *out_code = s->exit_code;
    return 1;
}

void tlg_conpty_close(tlg_conpty *s) {
    if (!s) return;
    tlg_log_conpty("tlg_conpty_close: closing session");
    if (s->process && WaitForSingleObject(s->process, 0) == WAIT_TIMEOUT) {
        tlg_log_conpty("tlg_conpty_close: terminating process");
        TerminateProcess(s->process, 1);
    }
    if (s->pseudo_console) { ClosePseudoConsole(s->pseudo_console); s->pseudo_console = NULL; }
    if (s->reader_thread) {
        tlg_log_conpty("tlg_conpty_close: waiting for reader thread");
        WaitForSingleObject(s->reader_thread, 2000);
        close_handle(&s->reader_thread);
    }
    close_handle(&s->input_write); close_handle(&s->output_read); close_handle(&s->process);
    close_handle(&s->job);
    DeleteCriticalSection(&s->write_lock); free(s);
    tlg_log_conpty("tlg_conpty_close: session freed");
}
