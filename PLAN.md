# PLAN: 将项目自有 C 实现迁移到 Go

> 目标：把 native/ 中项目自己维护的 C 实现及测试逐步迁移为 Go。正式应用不再编译/加载项目自有 C 源码、静态库或 terminal_libghosty_session.dll。Ghostty 官方 VT 引擎仍由上游 libghostty-vt 提供，不重写 VT parser。
>
> 2026-11-09 更新：发布构建已改为通过 cgo 静态链接 `ghostty-vt-static.lib`，发布包不再包含 `ghostty-vt.dll`；`CGO_ENABLED=0` 时保留动态加载作为开发和 ABI 测试回退。下文中“不使用 cgo”的描述属于迁移阶段的历史基线。

## 0. 当前基线与工作区保护

检查时 git status --short 显示以下未跟踪内容：
- TODO.md
- desktop/gui-start.stderr.log
- desktop/gui-start.stdout.log
- desktop/logs/

这些是用户工作区数据，迁移期间不得删除、覆盖、清理或纳入本次迁移。每阶段前后检查 git status --short，只改本计划列出的文件。

当前结构（本轮迁移后）：
- 原生 C 兼容基线保留：native/windows_conpty.c、native/ssh_terminal.c、native/ghostty_host.c 及其头文件和 C 测试。
- Go ConPTY：internal/terminal/conpty_windows.go；Go Ghostty Host：internal/terminal/ghostty_host_windows.go。
- 生产 Session：internal/terminal/session_windows.go，直接组合 Go ConPTY 与 Go Ghostty Host，不再加载项目自有 session DLL。
- Go 等价测试：conpty_windows_test.go、ghostty_host_windows_test.go、session_dsr_windows_test.go、session_windows_test.go、ghostty_abi_windows_test.go。
- go.mod 为 Go 1.22，目前无第三方 Go 依赖；Go 没有 import "C" 或 #cgo。
- 根 Makefile 的 build/test 使用 CGO_ENABLED=0；正式程序旁边部署官方 ghostty-vt.dll。
- native/CMakeLists.txt 继续构建原生 C 参考程序和回归测试，不作为 Go GUI 的构建前置条件。
- build/ 下 CMake 自动生成的 CMakeCCompilerId.c 等文件不是业务源码，不迁移、不手工编辑。

## 1. 目标架构与边界

目标数据流：

    Go GUI / Session API
        +-- Go ConPTY process manager (Windows API)
        +-- Go Ghostty VT adapter
                 +-- official ghostty-vt.dll (upstream C ABI)

- ConPTY、进程启动、管道读写、退出监控、会话生命周期和快照缓冲区管理由 Go 实现。
- Ghostty VT 解析、终端状态机、render-state、行/单元格迭代仍由官方 Ghostty 库提供。
- Go 通过 Windows DLL ABI 调用 Ghostty 官方导出函数；不使用 cgo，不复制/重写 Ghostty 内部结构体或位域。
- 正式程序不再依赖项目自有 terminal_libghosty_session.dll。
- 尽量保持现有 Go API：StartSSH、Start、Write、Resize、PollExit、Snapshot、SnapshotText、Close；GUI 层不应感知底层迁移。
- 明确 Windows 句柄、callback、后台 goroutine 的所有权、关闭顺序和同步策略。

## 2. 第一阶段：验证 Go 与现有 native C 实现兼容

以当前可以运行的 native C 程序、它使用的 ghostty-vt.dll、C 头文件和 C 测试为兼容基准。无需先获取或证明 CMake 中指定的上游 Git commit；重点是 Go 调用同一套已验证 DLL，并复现原 C 程序的行为。

1. 检查实际 ghostty-vt.dll、import library、架构、依赖 DLL 和导出符号；确定实际运行时路径。
2. 对照 native/ghostty_host.c、native/ssh_terminal.c、native/windows_conpty.c 及对应头文件，确认当前 C 程序实际使用的 DLL、导出符号、调用参数、结构体布局和 callback 约定。以原生 C 测试通过的 DLL 为准。
3. 盘点所需 upstream API：terminal create/free、VT feed、resize、render-state create/update/clean、row/cell iterator、文本/颜色/样式/cursor getters、terminal options 和 PTY write callback。
4. 写最小 Windows Go spike：加载官方 DLL、调用版本/创建销毁函数、解析简单 VT 序列并取得最小快照；验证函数签名、调用约定、指针生命周期和 callback 方案。
5. 若同一 DLL 无法通过原 C 程序已有的行为测试，或 Go callback/指针生命周期不安全，暂停并修复兼容问题。不得用其他 VT parser 冒充 Ghostty。

交付：可重复运行的 Go ABI smoke test，以及 Ghostty DLL 版本和部署约定。

## 3. 第二阶段：windows_conpty.c → internal/terminal/conpty_windows.go

将 native/windows_conpty.c 行为迁移为 Go Windows API。可用标准库 syscall；如需 golang.org/x/sys/windows，先确认依赖版本与获取策略。不得用 cgo 规避问题。

实现：
1. 创建 ConPTY 输入/输出管道，正确处理 inheritable flags 和父子端句柄。
2. 调用 CreatePseudoConsole；构造 STARTUPINFOEXW 和 proc-thread attribute list，设置 PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE。
3. 正确实现 Windows 命令行 quoting；executable 与 argv 分开处理，不经 shell 启动。
4. 创建子进程和 Job Object，设置 JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE；覆盖创建进程后加入 Job 失败等部分成功场景。
5. 专用 reader goroutine 读取输出字节并交给 session；EOF 后获取退出码并通知会话。
6. 实现串行写入（正确处理短写）、resize、poll exit、close。
7. Close 幂等并正确处理并发：停止/终止子进程、关闭伪控制台与管道、等待 reader 退出、释放句柄。不能持有 reader 完成所需的锁再等待 reader。
8. 错误同时保留 Win32 error code。

建议新增：
- internal/terminal/conpty_windows.go
- internal/terminal/conpty_windows_test.go

验收：
- 启动 cmd.exe/测试子进程，验证输入输出往返、长输出和分块输出。
- 验证 resize、EOF、退出码。
- 验证路径含空格及包含空格/引号/反斜杠的 argv。
- 验证启动中途失败无句柄泄漏，重复 Close 安全。
- 验证进程退出与 Close 竞态无死锁/goroutine 泄漏。
- Windows 10 1809+ 实机集成测试（ConPTY 最低系统要求）。

此阶段通过后再切换生产调用路径；新旧实现不要长期并行维护。

## 4. 第三阶段：ghostty_host.c → internal/terminal/ghostty_host_windows.go

基于阶段 2 的 DLL 验证，迁移项目自己的 VT Host adapter。

实现：
1. 管理 Ghostty terminal 和 render-state 的创建、销毁及所有权。
2. 配置 GHOSTTY_TERMINAL_OPT_USERDATA 与 GHOSTTY_TERMINAL_OPT_WRITE_PTY，把终端回复字节交给 session 输入通道。
3. 实现 VT output feed、resize、render-state update。
4. 实现快照两阶段流程：先查询 cols/rows、cell 数量和 UTF-8 arena 容量，再分配 Go buffer 并复制。
5. 映射 cell 文本 offset/length、前景/背景 RGB、style flags、selection、cursor 到现有 Go Cell/Cursor 类型。
6. 使用 Ghostty 官方 getter 读取 grapheme、颜色、样式；禁止手工解码不稳定的 GhosttyCell packed bits。
7. upstream iterator/render-state 指针只在锁保护的短生命周期内使用，不把借用指针交给 GUI 或跨帧保存。
8. 尺寸限制为合法非零 uint16；窗口最小化时零尺寸 resize 忽略或延后。
9. 串行化同一 terminal 的 mutation 与 render-state update/read；禁止 callback 中递归调用同一 terminal 的 VT feed。

建议新增：
- internal/terminal/ghostty_host_windows.go
- internal/terminal/ghostty_host_windows_test.go

验收：
- 普通 VT 和 fragmented UTF-8 输入正确。
- ANSI SGR 颜色/样式、光标位置与可见性正确。
- DSR 等终端查询能通过 PTY write callback 回写。
- resize 后快照尺寸匹配。
- sizing pass 不清 dirty state；buffer 不足可安全重试。
- create/destroy 和异常路径无泄漏。
- Go host 与现有 C host 使用同一 Ghostty DLL 时，等价的 ABI/快照/回调测试均通过。

## 5. 第四阶段：ssh_terminal.c → Go Session 编排

重构 internal/terminal/session_windows.go，移除：
- terminal_libghosty_session.dll
- 对项目自有 DLL 的 syscall.LazyDLL 加载
- tlg_ssh_terminal_* 导出函数调用
- 仅为项目 DLL 设置 SetDllDirectoryW 的全局搜索路径逻辑

由 Go Session 组合 ConPTY 与 Ghostty Host。

必须保持：
1. StartSSH 继续调用 ssh.BuildArgs(options)，保留现有 OpenSSH 参数和安全选项。
2. Start 解析 executable（如 ssh.exe），不经 shell 启动。
3. 先创建 Ghostty Host、完成 callback/resize 初始化，再创建 ConPTY；reader 启动前必须保证 Ghostty 同步回复 callback 能访问 ConPTY 输入端。
4. ConPTY reader 将输出交给 Ghostty feed；Ghostty PTY write callback 将回复写入 ConPTY stdin。
5. VT feed/snapshot/resize 串行化；ConPTY pipe write 单独串行化。
6. Close 顺序：标记 closing → 不持 VT 锁地关闭/终止 ConPTY 并等待 reader → 销毁 Ghostty Host → 释放同步对象。不可持有 reader 完成所需的锁再 join。
7. 明确 Close 与 Write/Resize/Snapshot/PollExit 并发调用契约；不能只依赖裸 uintptr 是否为零。
8. 维持 Snapshot、SnapshotText 语义；检查长度溢出、nil buffer 和 sizing pass 边界。

验收：
- 原有 Go 测试通过。
- ssh.exe -V 能通过同一路径启动。
- closed loopback port 的 SSH 失败输出可从 VT snapshot 读取。
- DSR 往返、输入输出、resize、exit、close 端到端通过。
- 多次启动关闭和竞争场景无死锁、panic、句柄/goroutine 泄漏。

## 6. 第五阶段：把 C 测试迁移为 Go 测试

| 现有 C 文件 | Go 替代建议 | 验收重点 |
|---|---|---|
| native/windows_conpty_test.c | internal/terminal/conpty_windows_test.go | ConPTY 创建、I/O、resize、退出、释放 |
| native/ghostty_host_test.c | internal/terminal/ghostty_host_windows_test.go | Host 生命周期、VT feed、快照、颜色、cursor |
| native/ssh_terminal_test.c | 扩展 internal/terminal/session_windows_test.go | SSH/ConPTY/Ghostty E2E、DSR 回写、错误输出 |
| native/libghostty_vt_smoke.c | internal/terminal/ghostty_abi_windows_test.go | 官方 DLL 导出、ABI、版本、最小 VT smoke |

测试要求：
- Windows-only 测试加 //go:build windows。
- 可将耗时集成测试放在 testing.Short() 保护下。
- 不依赖远端凭据；用本地命令、ssh.exe -V 和 closed loopback port。
- 环境缺少真实 Ghostty DLL 时，明确 skip 或失败原因，不能把未运行说成通过。
- 在支持的平台运行 go test -race；Windows ConPTY 仍需真实 Windows 集成测试。

## 7. 第六阶段：Go 生产构建与 C 兼容基线分离

所有 Go 功能与等价测试通过后：

1. 根 Makefile 使用 CGO_ENABLED=0 执行 go build/test，并将 ghostty-vt.dll 放在正式程序旁边。
2. 正式 Go 应用不得加载或依赖 terminal_libghosty_session.dll。
3. 保留 native/ 下原有 C 源码、头文件、CMake 和 C 测试，作为可运行的兼容基线；不要为了迁移而删除这套已验证程序。
4. CMake 仍可用于构建/运行原生参考程序和对照测试，但不作为 Go GUI 的构建前置步骤。
5. 更新 native/README.md、native/ABI-DESIGN.md、README.md，说明 Go 生产路径与 C 参考路径的边界。
6. 检查启动脚本、Makefile、GUI 启动路径和 CI，确保正式 Go 应用不再查找 terminal_libghosty_session.dll。
7. 保留必要的官方 ghostty-vt.dll，并记录其部署目录和加载方式。

## 8. 最终验收清单

- [x] 所有项目自有 C 实现均有 Go 实现，生产路径实际使用 Go。
- [x] 所有项目自有 C 测试/Smoke test 有 Go 替代。
- [x] 正式程序不需要 cgo、C 编译器、项目自有 C DLL 或 terminal_libghosty_session.dll。
- [x] Ghostty 官方 VT 引擎仍是实际 parser/render-state 来源，没有被其他 parser 替代。
- [x] Go 使用与原生 C 程序相同的 Ghostty DLL；导出、架构和部署约定经过 C/Go 对照测试并写入文档。
- [x] go test ./... 通过；Windows 实机集成测试通过。
- [x] Windows GUI 构建成功，应用能启动 SSH 子进程并显示输出。
- [x] ConPTY I/O、DSR 回写、颜色/样式/cursor、resize、退出和关闭行为通过。
- [x] 多次开关、异常启动、并发关闭无句柄/goroutine 泄漏和死锁。
- [x] git status 确认没有碰触第 0 节列出的未跟踪用户文件。

## 9. 执行纪律与推荐顺序

- 一次迁移一个模块；先读完整实现和所有调用点。
- 每阶段先写/更新 Go 测试，再实现，再运行测试；通过后进入下一阶段。
- 不以扩展名改成 .go 作为完成标准；必须保持原行为、错误路径和资源生命周期。
- 不引入未经验证的 Windows API 结构体布局、Ghostty ABI 签名或 callback 约定。
- 每阶段记录改动文件、测试命令、结果和剩余风险。
- Go 与原生 C 行为对照不一致时先修复兼容问题，不要求先验证远程上游 commit。

建议顺序：以原生 C 程序为基准，完成 ConPTY → Ghostty Host → Go Session → 等价 Go 测试 → Go 生产构建切换。原生 C 源码和 CMake 测试保留为回归基线。
