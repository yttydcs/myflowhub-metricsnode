# Plan - MyFlowHub-MetricsNode（Windows 内屏亮度：WMI 读写兜底）

> Worktree：`d:\project\MyFlowHub3\worktrees\fix-metricsnode-win-brightness-wmi`  
> 分支：`fix/metricsnode-win-brightness-wmi`  
> 日期：2026-03-02  
>
> 本 workflow 目标：修复 Windows 笔记本内屏在当前实现下亮度读取失败（`brightness_percent=-1`）的问题，并让“写入亮度变量→执行”在内屏场景可用。

---

## 1. 需求分析

### 1.1 目标

- Windows（笔记本内屏）：
  - 亮度采集：不再长期上报 `brightness_percent=-1`；
  - 亮度下行控制：写入亮度变量（默认 `sys_brightness_percent`）后，能实际调整系统亮度（best-effort）。
- 外接显示器等场景不回退已有能力：仍优先使用现有的 DXVA2/Monitor API（DDC/CI）。

### 1.2 背景/现状

当前 Windows 亮度采集使用 `dxva2.dll` 的物理监视器/亮度 API（DDC/CI）。在用户的 **Windows 笔记本内屏** 场景中，日志显示：

- `read brightness failed err="physical monitor handle is 0"`
- `read brightness failed err="An error occurred while transmitting data to the device on the I2C bus."`

导致 MetricsNode 输出 `brightness_percent=-1`（不可读哨兵值）。

### 1.3 范围

- 必须：
  - Windows：为亮度 **读取/写入** 增加 WMI 兜底（`ROOT\\WMI`：`WmiMonitorBrightness` / `WmiMonitorBrightnessMethods`）。
  - 仍保持“优先 DXVA2，失败后 WMI 兜底”的策略。
  - 避免因 DXVA2 在内屏持续失败导致的高频 warn 日志（WMI 成功时不应再输出 DXVA2 的失败日志）。
- 可选：
  - 在亮度采集侧对 DXVA2 失败做短期退避（避免每 2s 触发 I2C 失败）。
- 不做：
  - 不改变 metric/var 名称与 bindings schema（仍是 `brightness_percent` ↔ bindings 的 `var_name`）。
  - 不引入 UI 引导（例如提示用户开启 DDC/CI 或系统授权），本轮先让内屏可用与可观测。

### 1.4 使用场景

1) 用户手动调节 Windows 系统亮度 → MetricsNode 采集到变化并上报到 VarStore（默认 `sys_brightness_percent`）。
2) 其它节点写入 `sys_brightness_percent=30`（owner 指向 MetricsNode）→ MetricsNode 在本机执行亮度调整；随后采集会把 VarStore 纠偏到真实值（例如 clamp 后的 `100`）。

### 1.5 功能需求

- 亮度读取（Windows）：
  - 优先：DXVA2（现有实现：主显示器）；
  - 兜底：WMI（内屏常用）。
- 亮度写入（Windows）：
  - 优先：DXVA2（外接显示器常用）；
  - 兜底：WMI（内屏常用）。
- 值域与语义：
  - `brightness_percent`：字符串整数 `0~100`；不可读时 `-1`。
  - 写入时：整数解析失败则忽略；解析成功 clamp 到 `0~100` 后执行。

### 1.6 非功能需求

- 稳定性：
  - WMI 查询失败不影响其它 metric 上报/控制；
  - COM 初始化严格按线程模型执行（避免二次初始化导致错误）。
- 性能：
  - WMI 属于相对重的调用：建议在 DXVA2 明显失败后进行退避或优先走 WMI（避免每 2s 触发 I2C 失败）。

### 1.7 输入输出

- 输入：
  - Windows 系统亮度（DXVA2 或 WMI）。
  - VarStore 下行 `notify_set`（亮度变量写入）。
- 输出：
  - 上报：`brightness_percent` → bindings → VarStore `set`（默认 `sys_brightness_percent`）。
  - 控制：执行 DXVA2 / WMI 的亮度设置。

### 1.8 边界异常

- 设备/驱动不支持：
  - DXVA2 不可用（内屏/I2C）→ WMI 兜底；
  - WMI 不可用/返回空 → 最终上报 `-1`，并按节流输出 warn。
- 多显示器：
  - DXVA2 负责“主显示器”；
  - WMI 通常只覆盖内屏，作为兜底使用（不替代 DXVA2 正常工作时的主显示器语义）。

### 1.9 验收标准

- Windows 笔记本内屏：
  - MetricsNode 的 `brightness_percent` 不再长期为 `-1`；
  - 手动调节系统亮度时，`sys_brightness_percent` 跟随变化；
  - 写入 `sys_brightness_percent=30`（owner=<MetricsNodeNodeID>）后，系统亮度发生变化。
- 回归命令通过：
  - Go：`GOWORK=off go test ./... -count=1 -p 1`
  - Windows module：`cd windows/frontend; npm ci; npm run build` 后 `cd ..; GOWORK=off go test ./... -count=1 -p 1`

### 1.10 风险

- WMI 依赖 COM（线程初始化与对象释放必须正确，否则易泄漏或偶发失败）。
- WMI 能力与 DXVA2 覆盖范围不同：WMI 主要用于内屏兜底；需要避免“WMI 成功就永远不再尝试 DXVA2”的死锁策略，防止外接显示器切换主屏时误读内屏亮度。

---

## 2. 架构设计（分析）

### 2.1 总体方案

在 Windows 亮度读写路径上引入“双通道”：

- DXVA2（现有）：主显示器 DDC/CI（外接显示器常用）
- WMI（新增兜底）：`ROOT\\WMI`：
  - 读：`WmiMonitorBrightness.CurrentBrightness`
  - 写：`WmiMonitorBrightnessMethods.WmiSetBrightness(Timeout,Brightness)`

并约定：

- **优先 DXVA2**；仅在 DXVA2 明显失败（handle=0 / I2C 错误 / 无监视器等）时使用 WMI。
- 在采集 loop 内对 DXVA2 失败做退避，避免持续触发 I2C 错误（例如禁用 DXVA2 30s 后再试一次）。

### 2.2 模块职责

- `core/metrics/collectors_windows.go`：
  - 亮度采集逻辑：DXVA2 + WMI 兜底 + 退避；
  - COM 初始化：亮度 loop 使用 `LockOSThread + CoInitialize/CoUninitialize`，保证 WMI 调用稳定。
- `core/actuator/brightness_windows.go`：
  - 亮度设置逻辑：DXVA2 + WMI 兜底（控制 worker 已初始化 COM，可直接调用 WMI）。

### 2.3 数据/调用流

- 采集：
  - `brightnessLoop` → DXVA2 读失败 → WMI 读成功 → `emit(brightness_percent,"N")`
- 控制：
  - `notify_set` → 入队 `brightness_percent` → `controlWorker` → DXVA2 写失败 → WMI 写成功

### 2.4 错误与安全

- WMI 查询/调用失败：返回 error，最终由上层节流记录 warn；不 panic，不阻塞其它指标。
- COM 线程模型：
  - 不能在同一线程二次 `CoInitialize`（go-ole 对 `S_FALSE` 也会当 error），因此必须保证“每个执行 WMI 的 goroutine 只初始化一次”。

### 2.5 性能与测试策略

- 性能：
  - DXVA2 失败退避（减少 I2C 错误与无效调用）。
  - WMI 查询每 2s 轮询可接受；若后续发现耗电/卡顿，再考虑事件驱动或更长间隔。
- 测试：
  - Go 编译/单测覆盖（主要验证不引入编译问题）。
  - 手工：在内屏机器上观察 `brightness_percent` 与写入执行。

### 2.6 可扩展性设计点

- 未来若需更精确地绑定“主显示器”到 WMI instance，可扩展为：
  - DXVA2 获取主显示器设备路径/ID → 映射到 WMI 的 `InstanceName`（本轮不做）。

---

## 3.1 计划拆分（Checklist）

### T0 - 基线确认

- 目标：worktree/分支正确，工作区干净。
- 验收：`git status --porcelain` 为空。

### T1 - Windows：亮度读取增加 WMI 兜底 + DXVA2 退避

- 目标：DXVA2 失败时，能用 WMI 读出内屏亮度并上报 `0~100`。
- 涉及文件：
  - `core/metrics/collectors_windows.go`
- 验收：
  - 内屏不再持续 `brightness_percent=-1`；
  - 日志不再每 2s warn（WMI 成功时不应 warn）。

### T2 - Windows：亮度写入增加 WMI 兜底

- 目标：写入亮度变量时，DXVA2 失败可回退到 WMI 执行。
- 涉及文件：
  - `core/actuator/brightness_windows.go`
- 验收：
  - 内屏写入 `sys_brightness_percent=30` 能改变系统亮度。

### T3 - 回归验证

- Go：`GOWORK=off go test ./... -count=1 -p 1`
- Windows module：
  - `cd windows/frontend; npm ci; npm run build`
  - `cd ..; GOWORK=off go test ./... -count=1 -p 1`

### T4 - Code Review（阶段 3.3）+ 归档（阶段 4）

- 归档输出：
  - `docs/change/2026-03-02_metricsnode-win-brightness-wmi.md`

