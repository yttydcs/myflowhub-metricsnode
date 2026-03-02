# Plan - MyFlowHub-MetricsNode（Windows 亮度下行：WMI 类型不匹配修复）

> Worktree：`d:\project\MyFlowHub3\worktrees\fix-metricsnode-win-brightness-wmi-set`  
> 分支：`fix/metricsnode-win-brightness-wmi-set`  
> 日期：2026-03-02  
>
> 本 workflow 目标：修复 Windows 在“写入亮度变量 → 执行”时，WMI fallback 报 `类型不匹配` 导致亮度设置失败的问题（读取已 OK）。

---

## 1. 需求分析

### 1.1 目标

- Windows（笔记本内屏为主）：
  - 亮度下行控制：写入亮度变量（默认 `sys_brightness_percent`）后，DXVA2 失败时能通过 WMI 成功设置系统亮度（best-effort）。
- 不回退已有能力：
  - DXVA2 可用时仍优先 DXVA2（外接显示器 DDC/CI 常见）。

### 1.2 背景/现状

当前日志（写入亮度变量时）：

- DXVA2：`An error occurred while transmitting data to the device on the I2C bus.`
- WMI：`发生意外。 (类型不匹配 )`

同时已确认 WMI 在本机可用（PowerShell `Invoke-CimMethod WmiSetBrightness` 可成功改变亮度），因此问题更可能是 **Go 侧调用 `WmiSetBrightness` 的参数 VARIANT 类型不兼容**。

### 1.3 范围

- 必须：
  - Windows：修复 `WmiSetBrightness(Timeout,Brightness)` 的参数类型传递，避免 `DISP_E_TYPEMISMATCH`。
  - 保持“DXVA2 优先，失败后 WMI fallback”的策略不变。
- 不做：
  - 不修改协议/子协议与 bindings schema；
  - 不新增 UI 配置项（本轮只修复执行链路）。

### 1.4 使用场景

1) 其它节点对 owner=<MetricsNodeNodeID> 写入亮度变量（如 `sys_brightness_percent=32`）。  
2) MetricsNode 收到 `notify_set` → 入队控制动作 → Windows control worker 执行亮度设置。  
3) 期望：内屏亮度变更为对应百分比（clamp 后）。

### 1.5 功能需求

- 仍遵循既有语义：
  - `brightness_percent`：字符串整数 `0~100`；非数字忽略；写入时 clamp 到 `0~100`。
- WMI fallback：
  - DXVA2 失败时，WMI 设置应尽最大可能成功；
  - 若 WMI 不可用/失败，返回错误并由上层 warn（不崩溃）。

### 1.6 非功能需求

- 稳定性：
  - COM/VARIANT 生命周期正确，不出现偶发崩溃或资源泄漏；
  - 即使 WMI 失败也不影响其它控制动作与 metrics 上报。
- 性能：
  - 仅修复参数类型，不引入额外轮询或高开销操作。

### 1.7 输入输出

- 输入：VarStore 下行 `notify_set`（亮度变量写入）。
- 输出：Windows 亮度实际变化（DXVA2 或 WMI）。

### 1.8 边界异常

- 设备/驱动不支持 WMI 设置：继续返回 error（best-effort）。
- 多显示器：仍以 DXVA2“主显示器”为主；WMI 作为失败兜底（通常影响内屏）。

### 1.9 验收标准

- 在 Windows 笔记本内屏（WMI 可设置）机器上：
  - 写入 `sys_brightness_percent=32`（owner=<MetricsNodeNodeID>）后，亮度发生变化；
  - 日志不再出现 `wmi: ... 类型不匹配`。
- 回归命令通过：
  - Go：`GOWORK=off go test ./... -count=1 -p 1`
  - Windows module：`cd windows/frontend; npm ci; npm run build` 后 `cd ..; GOWORK=off go test ./... -count=1 -p 1`

### 1.10 风险

- 不同设备的 WMI provider 行为可能不同：部分机器 WMI 仍可能无法设置亮度（属于 best-effort 边界）。

---

## 2. 架构设计（分析）

### 2.1 总体方案（选型与对比）

问题聚焦在 **WMI 调用参数类型**。备选：

- 方案 A（采用）：`oleutil.CallMethod` 改用 `int32`（VT_I4）参数传递（`Timeout`/`Brightness`），避免 `VT_UI1/VT_UI4` 引发的类型不匹配。
- 方案 B：手工构造 `ole.VARIANT` 并显式设置 VT（更复杂，收益有限）。
- 方案 C：通过启动 `powershell` 执行 `Invoke-CimMethod`（引入子进程与权限/性能/安全问题，不采用）。

### 2.2 模块职责

- `core/actuator/brightness_windows.go`
  - 亮度设置：DXVA2 优先；失败后 WMI fallback；
  - 本次仅修复 WMI `WmiSetBrightness` 的参数传递类型。

### 2.3 数据/调用流

`notify_set` → `Runtime.enqueueControlAction(brightness_percent)` → `controlWorker` → `actuator.SetPrimaryMonitorBrightnessPercent()` →  
DXVA2 fail → `setBrightnessPercentWMI()` → `WmiSetBrightness(int32(0), int32(percent))`

### 2.4 错误与安全

- 错误：保留现有返回/日志策略；不 panic。
- 安全：仍仅接受 owner 匹配的下行写入（既有逻辑）。

### 2.5 性能与测试策略

- 性能：仅参数类型修正，不新增额外开销。
- 测试：
  - 编译/单测回归；
  - 手工：在内屏机器上写入亮度变量验证实际生效。

---

## 3.1 计划拆分（Checklist）

### T0 - 基线确认

- 目标：worktree/分支正确，工作区干净。
- 验收：`git status --porcelain` 为空。

### T1 - Windows：修复 WMI 设置亮度类型不匹配

- 目标：WMI fallback 不再因 `类型不匹配` 失败。
- 涉及文件：
  - `core/actuator/brightness_windows.go`
- 验收：
  - `wmi: ... 类型不匹配` 不再出现；
  - 在 WMI 可设置机器上写入亮度变量可生效。
- 回滚点：
  - 回滚该文件改动即可恢复现状（仍可能 WMI set 失败）。

### T2 - 回归验证

- Go：`GOWORK=off go test ./... -count=1 -p 1`
- Windows module：
  - `cd windows/frontend; npm ci; npm run build`
  - `cd ..; GOWORK=off go test ./... -count=1 -p 1`

### T3 - Code Review（阶段 3.3）+ 归档（阶段 4）

- 归档输出：
  - `docs/change/2026-03-02_metricsnode-win-brightness-wmi-set.md`

