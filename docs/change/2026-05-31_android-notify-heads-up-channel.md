# 2026-05-31 Android Notify Heads-Up Channel Follow-Up

## 变更背景 / 目标

Android 14 foreground service camera crash 修复后，NotifyNode 用户消息仍可能只进入通知中心，不显示 heads-up 横幅。本次补强 Android 用户消息 channel 的 alert 属性，并明确设备侧策略边界。

## 具体变更内容

- `android/app/src/main/java/com/myflowhub/metricsnode/NodeService.kt`
  - NotifyNode channel ID 从 `myflowhub_notify_v2` 更新为 `myflowhub_notify_v3`；
  - channel 保持 `IMPORTANCE_HIGH`，增加默认 notification sound、震动启用和震动 pattern；
  - 用户消息 notification 增加默认 sound / vibration、`CATEGORY_MESSAGE` 和 public visibility；
  - 保持 foreground service status channel 为 `IMPORTANCE_LOW`。

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

- `T1`：补强 Android NotifyNode 用户消息 channel 的 alert 属性。
- `T2`：构建、审查、归档和 lesson 更新。

## 经验 / 教训摘要

- `IMPORTANCE_HIGH` 是 heads-up 的必要条件之一，但设备上的 channel 浮动通知、勿扰模式和厂商策略仍可阻止横幅显示。
- Android 8+ channel 属性创建后不可直接升级；语义调整需要使用新的 channel ID。
- foreground service 常驻状态通知应继续保持 low importance，避免每次状态更新都干扰用户。

## 可复用排查线索

- 症状：通知进入消息中心，但没有顶部横幅。
- 关键词：`heads-up`, `floating notification`, `banner`, `IMPORTANCE_HIGH`, `myflowhub_notify_v3`, `CATEGORY_MESSAGE`, `DND`.
- 快速检查：
  - 确认 Android 13+ 已授予 `POST_NOTIFICATIONS`；
  - 确认设备设置里 `MyFlowHub Notify` channel 允许提醒、声音和浮动/横幅；
  - 确认设备未开启勿扰模式；
  - 确认新 APK 已创建 `myflowhub_notify_v3`，而不是继续观察旧 channel。

## 关键设计决策与权衡

- 不使用 full-screen intent。NotifyNode 消息不是呼叫或闹钟，不应绕过正常通知策略。
- 采用显式 sound + vibration + message category，表达普通 heads-up 通知的最大合理意图。
- 版本化 channel ID，避免旧 channel 的不可变属性阻挡修复。

## 测试与验证方式 / 结果

- `git diff --check`：通过。
- `$env:ANDROID_HOME='D:\project\MyFlowHub3\_android-sdk'; $env:ANDROID_SDK_ROOT='D:\project\MyFlowHub3\_android-sdk'; cd android; .\gradlew.bat :app:assembleDebug`：通过。
- worktree 中未放置 `android/app/libs/myflowhub.aar`，因此 Android build 使用 stub bridge 编译。
- 本机没有已连接 Android 设备，无法直接检查设备 channel 设置或观察 heads-up 横幅。

## 潜在影响

- 新安装包会创建 `myflowhub_notify_v3` channel，旧 `myflowhub_notify` 和 `myflowhub_notify_v2` channel 可能仍留在设备设置中。
- 高频 TopicBus 通知会比之前更明显，后续如需要静默 topic，应增加用户可配置的通知级别。

## 回滚方案

- 将 NotifyNode channel ID 恢复为 `myflowhub_notify_v2`；
- 移除显式 sound、vibration、category 和 visibility 配置；
- 回滚本归档和 lesson 更新。

## 子Agent执行轨迹

- 未派发子Agent。原因：实现集中在单一 Kotlin 文件，改动很小。
