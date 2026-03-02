# 2026-03-02 MetricsNode：Windows 内屏亮度 WMI 读写兜底

## 背景 / 目标

当前 `MyFlowHub-MetricsNode` 在 Windows 上使用 DXVA2（DDC/CI）方式读取/设置“主显示器”亮度。该方式在部分 **笔记本内屏** 场景不可用，日志表现为：

- `physical monitor handle is 0`
- `An error occurred while transmitting data to the device on the I2C bus.`

导致 `brightness_percent` 长期为 `-1`，同时“写入亮度变量 → 执行”也无法生效。

本次目标：

1) Windows 笔记本内屏：亮度采集不再长期上报 `-1`；
2) Windows 笔记本内屏：下行写入亮度变量后可 best-effort 调整系统亮度；
3) 外接显示器场景不回退原能力：仍优先尝试 DXVA2，失败再走 WMI 兜底；
4) 减少 DXVA2 在 I2C 失败时的无效频繁调用与日志噪音。

## 具体变更内容

### Windows：亮度读取（metrics collector）

- `brightnessLoop` 增加 WMI 兜底读取（`ROOT\\WMI` / `WmiMonitorBrightness.CurrentBrightness`）：
  - 仍优先尝试 DXVA2；
  - DXVA2 失败时使用 WMI 读取；
  - 当 WMI 可用且 DXVA2 失败时，对 DXVA2 启用短期 backoff（默认 30 秒），避免反复触发 I2C 错误。
- 将 WMI 调用固定在同一 goroutine/OS thread：
  - `LockOSThread` + `CoInitialize/CoUninitialize`；
  - 对 `CoInitialize` 的 `S_FALSE`（已初始化）做兼容处理。
- 增加 WMI 调用的空值防护与资源释放修正：
  - 避免 `oleutil` 返回 `*VARIANT == nil` 时触发空指针；
  - 对返回的 `*IDispatch` 使用 `AddRef + VARIANT.Clear()`，避免返回已释放对象或双重释放。
- 错误日志仍保留节流（按“错误文本变化或 60s”输出一次），并在 WMI+DXVA2 双失败时输出合并错误文本。

涉及文件：

- `core/metrics/collectors_windows.go`

### Windows：亮度写入（actuator）

- `SetPrimaryMonitorBrightnessPercent` 增加 WMI 兜底执行（`ROOT\\WMI` / `WmiMonitorBrightnessMethods.WmiSetBrightness`）：
  - 仍优先尝试 DXVA2（DDC/CI）；
  - DXVA2 失败时再尝试 WMI；
  - 若 WMI 成功则返回成功，避免上层记录 DXVA2 失败日志。
- WMI 执行使用 `LockOSThread` + `CoInitialize/CoUninitialize`，并兼容 `S_FALSE`。

涉及文件：

- `core/actuator/brightness_windows.go`

## plan.md 任务映射

- T1：亮度读取增加 WMI 兜底 + DXVA2 退避
- T2：亮度写入增加 WMI 兜底
- T3：回归验证
- T4：Code Review + 归档

## 关键设计决策与权衡

1) **DXVA2 优先，WMI 兜底**
   - DXVA2 对外接显示器（DDC/CI）更通用；WMI 对内屏更可靠。
   - 通过“失败时 backoff”在不改变优先级的前提下，避免 I2C 失败被持续触发。

2) **COM 初始化的鲁棒性**
   - `go-ole` 将 `CoInitialize` 的 `S_FALSE` 视为 error；本次显式兼容，避免在“线程已初始化”时直接退出 loop/执行路径。

3) **资源释放与稳定性**
   - WMI 使用 `oleutil` + `VARIANT.Clear()` 做严格清理：对 `VT_DISPATCH` 由 `VARIANT.Clear()` 负责 Release，避免双重释放；降低长期运行泄漏风险。

## 测试与验证

### Go（已执行）

- `GOWORK=off go test ./... -count=1 -p 1`

结果：通过。

### Windows module（已执行）

- `cd windows/frontend; npm ci; npm run build`
- `cd ..; GOWORK=off go test ./... -count=1 -p 1`

结果：通过。

### 手工（建议）

1) 启动 MetricsNode（Windows）并 `Start Reporting`
2) 观察 `Metrics` 中 `brightness_percent` 不再长期为 `-1`
3) 在任意可写 VarStore 的客户端对 owner=<MetricsNodeNodeID> 写入：
   - `sys_brightness_percent=30`
   - 期望：系统亮度变化；随后采集上报纠偏为真实值

## 潜在影响与回滚方案

潜在影响：

- WMI 查询属于相对重的调用：当前仍为 2s 轮询；后续如发现耗电/卡顿，可再优化（降低频率/事件化/更强缓存）。
- WMI 与 DXVA2 覆盖范围不同：外接显示器依赖 DDC/CI；内屏优先由 WMI 覆盖。

回滚方案：

- 回滚本次提交即可恢复为“仅 DXVA2 读写亮度”的实现（亮度在部分内屏仍会 `-1`）。
