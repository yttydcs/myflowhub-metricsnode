# 2026-05-31 Android FGS and Notification Behavior

## 变更背景 / 目标

Android 14 / targetSdk 34 下，fresh install 后输入 hub 地址并点击 Connect 会在 `NodeService.startForegroundWithState()` 崩溃。异常显示 generic Connect 流程启动 foreground service 时请求了 `camera` 类型，但此时应用尚不满足 `CAMERA` 运行时权限和 while-in-use eligible state 要求。

同时，NotifyNode 用户消息 channel 使用 `IMPORTANCE_DEFAULT`，消息进入通知中心但不能按预期显示 heads-up 通知。

## 具体变更内容

- `android/app/src/main/java/com/myflowhub/metricsnode/NodeService.kt`
  - generic MetricsNode foreground service runtime type 从 `dataSync|camera` 收窄为 `dataSync`；
  - NotifyNode 用户消息增加 `NotificationCompat.PRIORITY_HIGH`；
  - NotifyNode channel importance 从 `IMPORTANCE_DEFAULT` 提升为 `IMPORTANCE_HIGH`；
  - NotifyNode channel ID 更新为 `myflowhub_notify_v2`，避免已安装设备继续复用不可变的旧 channel importance。

## Requirements impact

none

## Specs impact

none

## Lessons impact

updated

## Related requirements

- `docs/requirements/notify-node.md`

## Related specs

- `docs/specs/notify-node.md`

## Related lessons

- `docs/lessons/android-fgs-notification-behavior.md`

## 对应 plan.md 任务映射

- `T1`：Android foreground service type 修正。
- `T2`：NotifyNode heads-up channel 修正。
- `T3`：Android build、diff 检查、代码审查和归档。

## 经验 / 教训摘要

- Manifest 中声明 service 支持 `camera` 类型，不代表 generic service startup 必须在每次 `startForeground()` 都请求 `camera` runtime type。
- Android 14 会在 foreground service 启动时检查 camera 类型对应的权限和 eligible state，非相机动作不应绑定 camera FGS。
- Android 8+ notification channel importance 创建后不可由应用提升；修正重要性时需要新 channel ID。

## 可复用排查线索

- 症状：
  - Android fresh install 后 Connect 闪退；
  - `SecurityException: Starting FGS with type camera`；
  - NotifyNode 消息只进入消息中心，不显示 heads-up。
- 触发条件：
  - targetSdk 34；
  - generic `startForeground()` 请求 `FOREGROUND_SERVICE_TYPE_CAMERA`；
  - 用户消息 channel 使用 `IMPORTANCE_DEFAULT` 或沿用旧 channel ID。
- 关键词：
  - `FOREGROUND_SERVICE_CAMERA`
  - `FOREGROUND_SERVICE_TYPE_CAMERA`
  - `startForegroundWithState`
  - `eligible state`
  - `IMPORTANCE_HIGH`
  - `myflowhub_notify_v2`
- 快速检查：
  - 查看 `NodeService.startForegroundWithState()` 实际传给 `ServiceCompat.startForeground()` 的 runtime type；
  - 区分低重要性的 foreground status channel 和用户消息 channel；
  - 已安装设备检查 notification channel ID 是否已版本化。

## 关键设计决策与权衡

- 保留 manifest 的 `dataSync|camera` 能力声明，避免移除已有 flashlight 能力；generic runtime request 只使用 `dataSync`。
- 不动态请求 camera FGS type。当前相机只用于 flashlight observer/control，并已有 `CAMERA` 权限检查；把 camera FGS 绑定到 Connect/Auth/Notify 会扩大崩溃面。
- 不使用 full-screen intent。NotifyNode 是普通用户通知，high importance channel 足以表达 heads-up 意图，最终展示仍受系统、用户、DND 和 OEM 设置约束。

## 测试与验证方式 / 结果

- `git diff --check`：通过。
- `$env:ANDROID_HOME='D:\project\MyFlowHub3\_android-sdk'; $env:ANDROID_SDK_ROOT='D:\project\MyFlowHub3\_android-sdk'; cd android; .\gradlew.bat :app:assembleDebug`：通过。
- 构建提示：worktree 中未放置 `android/app/libs/myflowhub.aar`，因此 Android build 使用 stub bridge 编译。Kotlin / manifest 变更已验证，真机端到端仍需安装带 AAR 的 APK 验证。

## 潜在影响

- NotifyNode 用户消息会创建新 channel `myflowhub_notify_v2`。旧 `myflowhub_notify` channel 可能仍留在已安装设备设置中，但不会再用于新通知。
- heads-up 展示仍受 Android 系统设置、用户 channel 设置、勿扰模式和厂商策略影响。
- 如果未来确实需要后台 camera foreground service，应新增专用动作和显式权限 / eligible-state 处理，不要恢复 generic startup 的无条件 camera type。

## 回滚方案

- 将 `NodeService.startForegroundWithState()` runtime type 恢复为 `dataSync|camera`；
- 将 NotifyNode channel ID 恢复为 `myflowhub_notify`；
- 将 channel importance 恢复为 `IMPORTANCE_DEFAULT` 并移除 notification high priority；
- 回滚本归档和 lesson 索引。

## 子Agent执行轨迹

- 未派发子Agent。原因：实现集中在单一 Kotlin 文件，拆分会增加编辑冲突和审查成本。
