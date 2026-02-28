# Plan - MyFlowHub-MetricsNode（MVP：电量/音量 → VarStore + Devices Config 配置）

> Worktree：`d:\project\MyFlowHub3\worktrees\metricsnode-mvp\MyFlowHub-MetricsNode`
> 分支：`feat/metricsnode-mvp`
>
> 本文档是本 workflow 的唯一执行清单；任何实现性改动必须严格按本计划逐项完成、验证、Review、归档。

---

## 0. 项目目标与当前状态

### 0.1 目标（MVP）

实现一个“普通客户端节点（node）”应用，分别提供：

- Windows：独立 Wails App（与 `MyFlowHub-Win` 技术栈一致），提供简单 UI（连接/注册/登录/状态展示/开关上报）。
- Android：独立 App，提供 **前台服务（Foreground Service）** 持续运行。

核心行为：

1) 通过 Auth 子协议 `register/login` 获取本节点 `node_id`；变量 owner=本节点。
2) 采集本机指标（电量、音量），以 VarStore `set` 写入变量池（默认 `visibility=public`，可配置）。
3) 支持通过 “Devices → Config” 远程配置该节点：实现 Management 子协议 `config_list/config_get/config_set`（仅配置类能力，不做其它 management action）。

### 0.2 非目标（本 workflow 不做）

- 市场/插件分发/第三方扩展包。
- 反向控制/执行能力（订阅变量触发本地行为）。
- “每 N 分钟心跳刷新一次” 的强制刷新策略（只在值变化时 set）。
- 除电量/音量以外的其它指标（后续另起 workflow）。

### 0.3 已确认约束

- 平台：Windows + Android。
- node_id：走现有 Auth 注册/登录获取。
- VarStore 默认可见性：`public`。
- 无电池设备：电量固定写入 `-1`（字符串 `"-1"`）。
- 音量语义：
  - Android：媒体音量（STREAM_MUSIC）。
  - Windows：默认输出设备 master volume。
- 变量名校验：仅允许字母/数字/下划线（否则会被 VarStore 拒绝）。
- 绑定配置：允许用一个 Config key 承载 JSON 字符串（不定参数）。
- 权限：按现有系统策略处理，本节点不额外设计权限模型（仍需做输入校验与日志）。

---

## 1. 目录与模块设计（落地到代码的约定）

> 目标：最大化复用 `myflowhub-sdk`（session/await/transport）与 `myflowhub-proto`（协议类型），避免重复实现 TCP/header/等待语义。

建议仓库结构（可在实现阶段微调，但必须同步更新本节）：

- `core/`：跨平台 Go 核心（连接状态机、Auth、VarStore 上报、Config 存储与 mgmt config 响应、bindings 解析）。
- `windows/`：
  - Wails App（`frontend/` + Go 端 bindings），UI/启动参数在此层。
- `android/`：
  - Android Studio/Gradle 工程（Kotlin UI + 前台服务）。
  - 通过 gomobile/aar 方式调用 `core/`（避免 Kotlin 侧重复实现协议栈）。

配置分层：

- Bootstrap（本地必须能启动/连上 Hub 的最小配置）：
  - 例如：`hub.addr`、`auth.device_id`、Android 是否启用前台服务等。
  - Windows：由 Wails UI 写入本地文件。
  - Android：由 App UI 写入 SharedPreferences/文件，并传给 Go core。
- Runtime Config（可通过 Devices Config 远程下发，且可热更新）：
  - 使用 `MapConfig`（key/value string），并落盘。
  - 建议 key：
    - `metrics.bindings_json`（JSON 数组字符串）
    - `metrics.visibility_default`（public/private，默认 public）
    - `metrics.battery.no_battery_value`（默认 -1）

---

## 2. 可执行任务清单（Checklist）

> 约定：每次编码必须标注对应 Task ID；不允许计划外改动。

### T0 - 仓库脚手架与构建最小闭环

- 目标：建立可编译的仓库骨架（Go module + Windows Wails + Android 工程占位），为后续迭代提供稳定入口。
- 涉及模块/文件（预期）：
  - `go.mod` / `go.sum`
  - `windows/`（Wails scaffold）
  - `android/`（Gradle scaffold）
  - `core/`（空包或最小可编译）
- 验收条件：
  - Windows：`wails build -nopackage` 能跑通（哪怕只是空 UI）。
  - Go：`GOWORK=off go test ./... -count=1 -p 1` 通过（至少无测试失败、能编译）。
- 测试点：
  - 本地能启动 Windows UI（不要求功能）。
- 回滚点：
  - 回滚本仓库分支提交即可恢复到空仓库初始状态。

### T1 - Go Core：连接 + Auth（register/login）状态机

- 目标：实现跨平台的 node 核心运行时：connect → register/login → 得到 node_id/hub_id，并对外暴露状态。
- 涉及模块/文件（预期）：
  - `core/runtime/*`
  - `core/auth/*`（密钥生成/保存、login 签名；可参考 `MyFlowHub-Win/internal/services/auth/*` 的实现思路）
- 验收条件：
  - 能成功注册/登录并得到非 0 的 `node_id`、`hub_id`（可先用手工冒烟验证）。
- 测试点：
  - 断线重连后能恢复连接（至少不崩溃，状态可见）。
- 回滚点：
  - 功能隔离在 `core/`，回滚对应提交不影响脚手架。

### T2 - Go Core：VarStore 上报（电量/音量）

- 目标：将采集结果以 VarStore `set` 写入（owner=本节点，默认 public，仅在变化时发送）。
- 涉及模块/文件（预期）：
  - `core/varstore/*`（encode message + SendAndAwait/Send）
  - `core/metrics/*`（采集接口 + 去抖/差量判断）
  - 变量名/值规范（常量/工具）
- 验收条件：
  - 指标变化时可在其它客户端用 VarPool `get(owner=<node_id>, name=...)` 读取到值。
  - 无电池时 `sys_battery_percent`（或最终命名）写入 `-1`。
- 测试点：
  - 变量名非法（含 `.`/`-`）会被拒绝（应在本地校验并给出错误日志，而不是一直重试刷屏）。
- 回滚点：
  - 回滚上报模块提交即可停止写入。

### T3 - Go Core：Devices Config 远程配置（Management config_*）

- 目标：实现 `config_list/config_get/config_set` 的请求处理，使 Win/Android 的 “Devices → Edit(Config)” 可直接编辑本节点配置。
- 涉及模块/文件（预期）：
  - `core/mgmtconfig/*`（解码 management message → 操作 MapConfig → 构造 *_resp）
  - `core/configstore/*`（MapConfig + 落盘 + 热更新回调）
- 设计要点：
  - `metrics.bindings_json` 为 JSON 字符串；变更后应触发 bindings 重新加载（不要求立即回写 VarStore）。
  - 记录配置变更日志（至少 key、来源 node_id）。
- 验收条件：
  - 在 `MyFlowHub-Win` 的 Devices 页面编辑该节点 Config：能 list/get/set 成功且重启后不丢。
- 测试点：
  - bindings_json 非法 JSON：拒绝并返回可读错误（config_set_resp code!=1）。
- 回滚点：
  - 回滚 mgmtconfig/configstore 提交即可恢复为“仅本地配置”。

### T4 - Windows：Wails UI（简单前台）

- 目标：提供最小可用 UI：配置 hub addr/device_id、连接/断开、注册/登录、显示 node_id、显示当前指标值与上报状态。
- 涉及模块/文件（预期）：
  - `windows/app.go` / `windows/main.go` / `windows/wails.json`
  - `windows/frontend/*`
  - 与 Go core 的 bindings
- 验收条件：
  - 可通过 UI 完成 connect + register/login，且能看到 node_id。
  - 指标上报后，外部可读取到 VarStore 值。
- 测试点：
  - UI 关闭后程序退出是否应停止上报（本 MVP：退出即停止）。
- 回滚点：
  - UI 与 core 解耦，回滚 windows 目录提交即可。

### T5 - Android：App + 前台服务（后台持续上报）

- 目标：实现 Android 独立 App：UI 负责 bootstrap 配置与启动/停止前台服务；前台服务运行 Go core 并持续上报。
- 涉及模块/文件（预期）：
  - `android/app/`（Kotlin/Compose）
  - `android/*`（gomobile 产物集成脚本/说明）
- 验收条件：
  - 开启服务后保持后台上报；停止服务后不再上报。
  - 通过 Auth 获取 node_id 后，VarStore 可读。
- 测试点：
  - 权限/通知渠道、后台限制下的稳定性（至少能在常见机型运行）。
- 回滚点：
  - 回滚 android 目录提交即可。

### T6 - 测试与验证（关键路径）

- 目标：覆盖关键路径与边界：bindings_json 校验、config_set 行为、varstore set 输入校验。
- 形式：
  - Go 单测（优先）：`core/*` 的纯逻辑模块（解析/校验/差量判断）。
  - 手工冒烟（必须写清步骤）：Win + Android 各一条端到端链路。
- 验收条件：
  - `GOWORK=off go test ./... -count=1 -p 1` 通过。
- 回滚点：
  - 测试仅辅助，不影响运行时逻辑；可单独回滚。

### T7 - Code Review（阶段 3.3）与变更归档（阶段 4）

- 目标：完成强制 Review 清单，并在本 worktree 根目录创建 `docs/change/YYYY-MM-DD_metricsnode-mvp.md` 归档。
- 验收条件：
  - Review 结论：通过。
  - 归档文档包含：背景/目标、变更内容、任务映射、关键权衡、测试结果、回滚方案。

---

## 3. 依赖关系、风险与注意事项

### 3.1 依赖

- 依赖 Hub 侧已启用：
  - Auth 子协议
  - VarStore 子协议
  - Management 子协议（用于 config_* 转发到目标 node）

### 3.2 风险

- Android 后台策略差异：前台服务是必要前提，但仍需考虑厂商杀后台。
- 只在变化时上报：若长时间不变，下游可能误判“离线”（本 MVP 接受，后续再评估心跳策略）。
- config_* 远程写入属于高权限操作：虽然本轮不设计权限模型，但必须确保输入校验与错误信息清晰，避免配置损坏导致无法恢复（至少保留本地 bootstrap）。

### 3.3 本地验收命令（建议）

为避免临时目录权限/并发导致的问题（参考 MyFlowHub3 习惯），可使用：

```powershell
$env:GOTMPDIR='d:\\project\\MyFlowHub3\\.tmp\\gotmp'
New-Item -ItemType Directory -Force -Path $env:GOTMPDIR | Out-Null
GOWORK=off go test ./... -count=1 -p 1
```

