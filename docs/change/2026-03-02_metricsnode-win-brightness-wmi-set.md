# 2026-03-02 MetricsNode：修复 Windows WMI 亮度设置“类型不匹配”

## 背景 / 目标

在 Windows 笔记本内屏场景中，MetricsNode 亮度下行控制链路为：

`notify_set (brightness var)` → 控制队列 → `SetPrimaryMonitorBrightnessPercent` → DXVA2（失败）→ WMI fallback

当前出现的问题是：DXVA2 常因 I2C/DDC 失败，而 WMI fallback 在调用 `WmiMonitorBrightnessMethods.WmiSetBrightness` 时返回：

- `发生意外。 (类型不匹配 )`

导致“写入亮度变量 → 实际调节亮度”失败。

本次目标：在不改变协议与 bindings 的前提下，修复 WMI 设置亮度的调用参数类型，避免 `DISP_E_TYPEMISMATCH`，使内屏场景可 best-effort 生效。

## 具体变更内容

### Windows：WMI 亮度写入参数类型修复

WMI 方法签名为 `WmiSetBrightness(Timeout, Brightness)`，但部分 WMI provider 对传入的 VARIANT 类型较敏感：

- 旧实现使用 `uint32` / `uint8`（更接近 CIM 的 unsigned 类型），在部分设备上触发 `类型不匹配`；
- 新实现默认改用 `int32`（`VT_I4`）传参以提升兼容性，并保留旧 unsigned 传参作为 fallback。

涉及文件：

- `core/actuator/brightness_windows.go`

## plan.md 任务映射

- T1：Windows：修复 WMI 设置亮度类型不匹配
- T2：回归验证
- T3：Code Review + 归档

## 关键设计决策与权衡

1) **默认使用 `VT_I4`（int32）传参**
   - VBScript/PowerShell 等常见 WMI 调用路径通常以“Variant Integer”传递参数；
   - 在部分设备上比 `VT_UI1/VT_UI4` 更不易触发 `DISP_E_TYPEMISMATCH`。

2) **保留旧 unsigned 调用作为兜底**
   - 避免某些 provider 仅接受 unsigned 变体类型的潜在兼容问题；
   - 失败时返回组合错误，便于定位。

## 测试与验证

### Go（已执行）

- `GOWORK=off go test ./... -count=1 -p 1`

结果：通过。

### Windows module（已执行）

- `cd windows/frontend; npm ci; npm run build`
- `cd ..; GOWORK=off go test ./... -count=1 -p 1`

结果：通过。

### 手工（建议）

在 WMI 可设置亮度的笔记本内屏机器上：

1) 启动 MetricsNode 并开始 Reporting
2) 从其它节点对 owner=<MetricsNodeNodeID> 写入：
   - `sys_brightness_percent=32`
3) 期望：亮度变化；日志不再出现 `wmi: ... 类型不匹配`

## 潜在影响与回滚方案

潜在影响：

- 仍属于 best-effort：少数设备/驱动的 WMI provider 可能不支持写入亮度或行为异常，失败时会回落为错误日志提示。

回滚方案：

- 回滚本次提交即可恢复到旧的 WMI 调用参数类型（可能重新出现 `类型不匹配`）。

