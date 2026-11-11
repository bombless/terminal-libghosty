# TODO — Windows 内嵌 Ghostty Terminal

> 目标：在现有 Windows SSH Profile Manager 中实现真正的内嵌终端，使用上游 libghostty-vt 处理 VT/终端状态，使用 Windows ConPTY 承载 SSH 子进程，并在 Win32 窗口中渲染终端。
>
> 当前阶段：**GUI Connect 已接入 `internal/terminal.StartSSH()`：通过 `cmd/terminal-host` 创建嵌入 WinForms 面板的 Win32 child window；GDI 读取 Ghostty cell snapshot 绘制前景/背景色、UTF-8 grapheme、selection 和 cursor；支持基础字符/方向键输入、resize、退出画面保留和 WM_CLOSE graceful teardown。SSH 仍经 `BuildArgs()`、ConPTY 和严格 host-key 校验。Windows Release CTest 4/4、Go tests、vet、Windows PowerShell 5 parser 通过。已支持微软拼音 IME（内联组合串渲染、候选窗口定位、提交写入 ConPTY）；鼠标、剪贴板、滚动、DPI/字体及高级样式仍未完成；真实远端认证连接尚未验收。**

## 0. 新上下文接手须知

- 项目目录：D:\mcp-agent-workspace\terminal-libghosty
- Ghostty 源码目录：D:\mcp-agent-workspace\ghostty-main
- 已有 Windows 工具链：
  - Zig：D:\mcp-agent-workspace\tools\zig-0.17.0\zig-x86_64-windows-0.17.0\zig.exe
  - MinGW Clang：C:\Users\bombl\AppData\Local\Microsoft\WinGet\Packages\MartinStorsjo.LLVM-Mingw.UCRT_Microsoft.Winget.Source_8wekyb3d8bbwe\llvm-mingw-20260616-ucrt-x86_64\bin\clang.exe
  - CMake 3.25.1；Go 已安装。
- Ghostty 本地快照元数据：Ghostty 1.4.1-dev、libghostty-vt 1.1.0-dev，最低 Zig 版本为 0.17.0。
- Ghostty 目录没有 .git，因此**不能确认**它对应项目锁定的 revision 5813c9461d5af78b6307a1254d00b1bc190eaccd。开发时记录并评估 API/版本差异，不要假设二者完全一致。
- 不要覆盖、重置、stash、清理或提交用户已有修改；不要执行 git reset、git checkout --、git clean 等破坏性操作。
- 本轮开始前重新检查的 Git 状态：
  - 分支：`feat/windows-ssh-terminal`（跟踪 `origin/feat/windows-ssh-terminal`，ahead 1）。
  - 已存在修改：`Makefile`、`README.md`、`desktop/TerminalLibghosty.ps1`、`docs/implementation-status.md`、`native/README.md`、`native/ghostty_host.h`。
  - 已存在未跟踪文件：`TODO.md`、GUI 启动日志、`native/ABI-DESIGN.md`、`native/CMakeLists.txt`、Ghostty bridge 源码/测试和 smoke test。
  - 本轮新增 ConPTY 的三个源码文件，并修改 `native/CMakeLists.txt`、`native/README.md` 和本 TODO；没有 stage 或创建 Git commit。
- 每次开始新工作前仍需重新运行 `git status --short --branch`；不要把历史状态当成当前状态，也不要重置、stash、清理或覆盖用户工作树。
- 已在 Ghostty 源码目录生成 Zig 缓存/依赖缓存和 zig-out 构建产物；不要误删这些文件，也不要清理整个源码目录。

## 1. 已完成并验证

- [x] 找到并使用本地 Ghostty 完整源码树（包含 include/ghostty/vt.h、VT 子头文件、build.zig 和上游 CMake 示例）。
- [x] 使用 Zig 0.17.0 和 MinGW Clang 在 Windows 上配置、编译 libghostty-vt。
- [x] 项目原生构建使用独立目录：build/native-local。
- [x] 使用 CMake 参数 `FETCHCONTENT_SOURCE_DIR_GHOSTTY=D:/mcp-agent-workspace/ghostty-main` 指向本地源码；没有修改项目原有 FetchContent 锁定配置。
- [x] 生成并验证 Ghostty DLL、静态库及 C ABI smoke test。
- [x] Smoke test 输出：`Windows libghostty-vt smoke test: ANSI color works`，并确认终端状态由上游 Ghostty 维护。
- [x] 新增项目自有 `native/ghostty_host.c` opaque-handle bridge：create、feed_output、resize、destroy，以及同步 PTY 回复回调。
- [x] 新增 `native/ghostty_host_test.c`，验证创建/销毁、VT 输入、DSR 触发的 PTY 回复回调、resize 和非法参数。
- [x] 最终构建命令 `cmake --build build/native-local --config Release` 成功；构建目标包括 `zig_build_lib_vt`、`ghostty_vt_smoke`、`terminal_libghosty_host`、`ghostty_host_test`、`terminal_libghosty_conpty`、`windows_conpty_test`。
- [x] 最终 `ctest --test-dir build/native-local -C Release --output-on-failure`：4/4 passed（包含 `ssh_conpty_ghostty_e2e`）。
- [x] 最终 `go test ./...`：通过（`internal/ssh` passed；cmd package 无测试文件）。
- [x] 新增 `native/ABI-DESIGN.md`，记录 ABI 映射、所有权、生命周期、回调和线程约束，以及上游 pin 风险。
- [x] 本轮没有 stage 或 commit；没有 reset、checkout、clean、stash 用户工作树。

## 2. 下一步：先读接口和现有代码（不要急着写实现）

- [x] 重新检查 Git 状态和现有改动，识别用户已有工作（保留所有原有未提交文件）。
- [x] 阅读项目的 native/CMakeLists.txt、native/README.md、native/libghostty_vt_smoke.c 和现有 native/ghostty_host.h。
- [x] 阅读上游 include/ghostty/vt.h 及本阶段涉及的 terminal、render、types、allocator、focus、paste、key/mouse 头文件，确认本地快照公开 API、类型、生命周期、错误处理和回调重入约束。
- [x] 检查上游 example/c-vt-cmake，以实际示例为准，不凭印象猜测函数名或结构体布局。
- [x] 阅读项目 GUI/SSH 启动代码，确认当前 Connect 流程、窗口生命周期、配置模型和安全参数（原 GUI 的 wt.exe 路径已替换为嵌入式 Win32 child host；Go BuildArgs 保留 StrictHostKeyChecking=yes）。
- [x] 写下项目自有 C ABI 与 Ghostty API 的映射设计；确认边界后再开始实现（见 native/ABI-DESIGN.md）。
- [x] 注意 API 快照版本与项目锁定 revision 不一致的风险；已记录于 native/ABI-DESIGN.md，本轮未更改锁定 revision，正式集成前仍须对真实 pin 做兼容验证。

## 3. 原生宿主层：libghostty-vt bridge

- [x] 设计稳定、最小化的项目自有 C ABI（opaque handle + create/destroy/feed/resize/snapshot），避免把上游内部类型泄漏到 Go/GUI 层。
- [x] 新增项目自有 bridge 实现 native/ghostty_host.c，复用现有 native/ghostty_host.h。
- [x] 明确输入字节、PTY 回复回调字节、终端状态更新、尺寸变化和快照缓冲区的所有权与生命周期；快照采用调用方双阶段分配，数据完全复制后返回。
- [x] 对错误码、空指针、异常尺寸和调用序列做防御性处理；同一 host 的并发调用目前明确要求调用方串行化。
- [x] 为 bridge 增加独立 C 测试并接入 CTest：创建/销毁、VT 输入触发回复、resize 和参数边界均已覆盖。
- [x] 扩展 bridge 测试：ANSI 色彩/属性、光标视口边界、屏幕/单元格内容、UTF-8 grapheme 与 resize 后网格尺寸断言。
- [x] 在 `native/ABI-DESIGN.md` 和头文件注释中明确 callback 是同步调用且回调 buffer 仅在 callback 返回前有效；回调方必须复制/排队，不能在 callback 中重入同一个 terminal。一个 host 的 terminal/render state 操作必须由调用方串行化。
- [x] CMake 构建后为 smoke/bridge 测试程序复制运行时 DLL；GUI Connect 将 `TERMINAL_LIBGHOSTY_NATIVE_DIR` 指向 `build/native-local`，并在首次连接时构建 `dist/terminal-host.exe`。
- [x] 保留现有 smoke test，并将 bridge 测试接入 CTest。

## 4. Windows ConPTY 子进程层

- [x] 新建独立的 Windows ConPTY 模块：`native/windows_conpty.h/.c`，不与 Ghostty VT bridge 混杂。
- [x] 使用 `CreatePseudoConsole` 创建伪控制台。
- [x] 创建输入/输出 pipe；明确 PTY 端与宿主端方向、继承属性、关闭顺序和失败回滚。
- [x] 使用 `InitializeProcThreadAttributeList`、`UpdateProcThreadAttribute(PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE)` 和 `CreateProcessW` 启动 UTF-16 子进程参数。
- [x] 修复 Go SSH 参数校验：正确拒绝 target/identity path 中的 CR、LF、NUL，拒绝显式端口 0 和超出 1–65535 的端口；增加普通 hostname（含 r/n/0 字符）及危险输入回归测试。
- [x] 保持 SSH 参数中的 `StrictHostKeyChecking=yes`，没有降低 host key 检查要求。
- [x] 新增 `native/ssh_terminal.h/.c` session adapter，统一拥有 ConPTY 与 Ghostty host；ConPTY reader → `ghostty_terminal_vt_write`，Ghostty `WRITE_PTY` callback → ConPTY input。
- [x] 新增 Windows Go wrapper `internal/terminal.StartSSH()`，复用 `ssh.BuildArgs()` 和 OpenSSH `ssh.exe`，不经 shell，保留 StrictHostKeyChecking=yes。
- [x] 增加 `ssh_conpty_ghostty_e2e`：本地 cmd.exe I/O、Ghostty DSR 回复回传、resize、真实 `ssh.exe -V` 均通过。
- [ ] 使用真实 SSH profile/凭据连接远端并验收认证、host key 和断网行为（需要用户提供测试目标/凭据）。
- [x] 使用独立 reader worker 异步读取输出并回调；输入写入由独立 API 完成，不阻塞 GUI 消息线程（调用方仍需避免在 UI 线程执行可能阻塞的大块写入）。
- [x] 实现 `ResizePseudoConsole`；拒绝零尺寸，最小化窗口时由调用方延迟 resize；快速 resize 合并/防抖仍待窗口层实现。
- [x] 实现进程退出查询、关闭时终止仍运行的子进程、关闭伪控制台、等待 reader worker 并释放 pipe/线程/进程句柄；NULL handle 可安全关闭。
- [x] native session close 标记 closing，先停止/等待 ConPTY reader，再销毁 VT host；调用方仍不得与 close 并发调用其他 session API。
- [x] 增加本机交互式 `cmd.exe` smoke test：验证输入、输出、退出码、有效 resize 和零尺寸拒绝。
- [ ] 后续补充认证失败/远端退出、快速关闭压力、长输出、Unicode、断网/管道异常及句柄泄漏测试。

## 5. Win32 终端窗口与渲染

- [x] 明确终端子窗口的创建/销毁与 GUI 主窗口生命周期（完全由 Go 原生 Win32 窗口管理，移除 PowerShell WinForms 依赖）。
- [x] 实现最小 Win32 child window 和消息/绘制循环（`internal/gui`）。
- [x] 使用 Direct2D + DirectWrite 硬件加速渲染 Ghostty cell snapshot，替代原有 GDI 双缓冲。
- [x] 从 libghostty-vt RenderState 获取可渲染的屏幕/单元格状态；bridge 快照 API 已复制 UTF-8 grapheme、样式、解析颜色、选区和光标，不重复解析 VT。
- [x] 支持基础行列数计算、ConPTY/Ghostty resize、Direct2D render target resize、重绘刷新。
- [x] Direct2D 绘制 UTF-8 grapheme、单元格前景/背景色、选区和基础光标。
- [ ] 支持字体选择、宽字符/组合字符精确布局、DPI、滚动区域/scrollback、粗体/斜体/下划线等完整样式。
- [ ] 完整处理光标形状和闪烁。
- [x] WM_DESTROY 时关闭 session；Disconnect 使用 WM_CLOSE 请求宿主执行 native cleanup，避免正常关闭遗留 SSH 子进程。

## 6. 键盘、鼠标与剪贴板

- [x] 基础 Win32 WM_CHAR 输入（可打印字符、Enter、Backspace、Tab）。
- [x] 方向键、Home/End、Insert/Delete 映射为 VT escape sequence。
- [ ] 完整支持 PageUp/PageDown、功能键及 Ctrl/Alt/Shift 组合键。
- [ ] 按终端模式处理鼠标报告；没有启用鼠标模式时保留本地选择/滚动行为。
- [ ] 实现文本选择、复制和粘贴；遵守 Windows 剪贴板格式和 Unicode 规则。
- [ ] 对 bracketed paste、焦点事件等模式按上游 VT 状态决定是否发送。
- [x] 微软拼音 IME：内联组合串、候选窗口定位、提交文本写入 ConPTY。
- [ ] 验证 Unicode 粘贴和高 DPI 下的鼠标坐标。

## 7. GUI / SSH 集成

- [x] 找到并替换当前使用 wt.exe 外部启动 OpenSSH 的 Connect 路径（`desktop/TerminalLibghosty.ps1`）。
- [x] 保留现有 SSH profile、Identity、端口配置；通过 `StartSSH()` → `BuildArgs()` 保留严格 host-key 验证和 argv 构造。
- [x] 新增 `cmd/terminal-host` session/window host，将 SSH 子进程接入 ConPTY。
- [x] 将单个终端子窗口嵌入现有 GUI；当前策略为每次 Connect 一个活动 session。
- [x] 提供 Connect、BACK/DISCONNECT、子进程退出轮询和最终 VT 屏幕保留。
- [x] 正常关闭通过 WM_CLOSE/WM_DESTROY 调用 native session Close；异常情况下有超时强制退出兜底。
- [ ] 完善连接中、已连接、断开、认证失败状态提示；真实远端认证尚未验收。
- [ ] 扩展多连接/标签页策略；当前不是多 session 管理器。

## 8. 构建、测试与验收

- [x] 提供可重复的 Windows 原生构建命令（明确 Zig、Clang、CMake 和本地 Ghostty 源码路径）。
- [x] 生成 `terminal_libghosty_session.dll`，依赖 `ghostty-vt.dll`；应用部署时两者需与 EXE 同目录或位于 DLL 搜索路径。
- [x] CTest 全部通过（本轮 Windows Release：4/4，包含 SSH/ConPTY/Ghostty 双向 E2E）。
- [x] `go test ./...` 和 `go vet ./...` 通过；Windows native wrapper 的 `ssh.exe -V` 和 `StartSSH()` loopback refused runtime tests 通过（设置 `TERMINAL_LIBGHOSTY_NATIVE_DIR`）。
- [x] 在真实 Windows GUI 中触发 Connect，确认它启动内嵌 Win32 host 并通过 `StartSSH()` 发起 loopback SSH 测试；Go cell snapshot ABI、渲染构建和 PowerShell 5 语法均通过。
- [ ] 使用真实远端凭据验收交互式 shell、中文/IME、完整 ANSI 样式、resize、复制粘贴和断开。
- [ ] 验证 host key 首次确认、known_hosts 不匹配等安全场景没有退化。
- [ ] 验证没有 GUI 卡死、句柄泄漏、孤儿进程或关闭崩溃。
- [x] 更新 README.md、native/README.md、docs/desktop-ui.md 和 docs/implementation-status.md，如实区分已完成/未完成。
- [x] 最终复查 git diff 和 git status；未 stage/commit。全量 `git diff --check` 仅报告本轮开始前已有的 `Makefile:13`、`README.md:7` 行尾空白，本轮未修改这两处用户工作。

## 9. 已知构建命令

在 PowerShell 中：

```powershell
$zig = 'D:\mcp-agent-workspace\tools\zig-0.17.0\zig-x86_64-windows-0.17.0'
$clang = 'C:\Users\bombl\AppData\Local\Microsoft\WinGet\Packages\MartinStorsjo.LLVM-Mingw.UCRT_Microsoft.Winget.Source_8wekyb3d8bbwe\llvm-mingw-20260616-ucrt-x86_64\bin'
$env:PATH = "$zig;$clang;$env:PATH"

Set-Location 'D:\mcp-agent-workspace\terminal-libghosty'

cmake -S native -B build/native-local -G 'MinGW Makefiles' `
  -DCMAKE_C_COMPILER=clang `
  -DFETCHCONTENT_SOURCE_DIR_GHOSTTY='D:/mcp-agent-workspace/ghostty-main'

cmake --build build/native-local --config Release

.\build\native-local\ghostty_vt_smoke.exe
ctest --test-dir build/native-local -C Release --output-on-failure
go test ./...
```

## 10. 实施原则

1. 小步改动工作树，每步先编译/测试；**不要创建 Git commit**，除非用户明确要求。
2. 先核对真实头文件和 sample，再写 API 调用；不猜 API。
3. 业务层、ConPTY、VT bridge、渲染层分层，避免把所有逻辑塞进一个 C 文件。
4. GUI 消息线程不能做阻塞式 pipe 读写或长时间等待。
5. 不牺牲 SSH 安全性：保留严格 host key 校验，不把密码写入日志，不在命令行泄漏秘密。
6. 不宣称“内嵌终端完成”，直到 GUI 中真实 SSH 交互的验收清单全部通过。
7. 每次开始先查看当前 Git 状态；用户现有未提交改动一律视为受保护内容。
