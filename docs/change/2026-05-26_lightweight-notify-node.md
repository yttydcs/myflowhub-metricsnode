# 2026-05-26_lightweight-notify-node

## 变更背景 / 目标

本次目标是在 `MyFlowHub-MetricsNode` 内新增一个轻量 NotifyNode 能力：节点登录后订阅用户配置的 TopicBus topic，收到匹配 `publish` 后在本机弹出系统通知。实现范围覆盖 Windows Wails 宿主和 Android 前台服务，不修改 Server、Proto 或 TopicBus 线协议。

## 具体变更内容

- 新增 `core/notify`：
  - `TopicSetting` 配置校验、去重、启用 topic 提取；
  - TopicBus `publish` envelope 解析；
  - 从 payload/name/topic 提取并裁剪通知 title/body；
  - bounded FIFO notification queue。
- 扩展 `core/runtime`：
  - 新增 `notify.topics_json` runtime config；
  - 新增 `NotifySettingsGet/Set`、`StartNotify`、`StopNotify`、`IsNotifyRunning`、`DequeueNotifications`；
  - `onUnmatchedFrame` 增加 TopicBus `publish` 处理；
  - connect/login/settings 更新后恢复或刷新订阅。
- 扩展 Windows：
  - Wails App 暴露 Notify 配置、启动停止、出队和 `ShowNotification`；
  - Vue UI 增加 `Notify` tab；
  - 顶部状态胶囊保持单行显示，避免 `Notify Off` 被压缩换行；
  - Windows 通知通过 PowerShell + `System.Windows.Forms.NotifyIcon` 展示。
- 扩展 Android：
  - gomobile bridge 增加 notify JSON API；
  - `NodeService` 增加 Start/Stop Notify action、通知轮询线程和独立通知 channel；
  - Compose UI 增加 `Notify` tab，支持 topic 增删、启停监听。
- 新增稳定文档：
  - `docs/requirements/notify-node.md`
  - `docs/specs/notify-node.md`

## Requirements impact

updated

新增 NotifyNode 稳定需求，明确 exact-topic、实时消息、无离线重放、跨 Windows/Android 系统通知边界。

## Specs impact

updated

新增 NotifyNode 技术规格，记录 runtime config key、runtime API、TopicBus 入站处理、队列策略和平台接入方式。

## Lessons impact

none

本次没有暴露新的可复用故障模式。Wails binding 已按现有脚本处理，不新增 lessons。

## Related requirements

- `docs/requirements/notify-node.md`

## Related specs

- `docs/specs/notify-node.md`
- `D:\project\MyFlowHub3\repo\MyFlowHub-Server\docs\specs\topicbus.md`

## Related lessons

- none

## 对应 plan.md 任务映射

- `DOCS-1`：新增 requirements/specs 和索引入口。
- `CORE-1`：新增 `core/notify` 配置、解析、事件队列和单测。
- `CORE-2`：runtime 接入 TopicBus subscribe/publish 处理。
- `WIN-1`：Windows Wails API、Notify 页和系统通知 presenter。
- `ANDROID-1`：Android bridge/service/UI 通知链路。
- `VERIFY-1`：Go、Windows、Android 构建验证和代码审查。
- `ARCHIVE-1`：本 change archive。

## 经验 / 教训摘要

- TopicBus 当前是 exact topic + in-memory subscription，NotifyNode 必须在登录、重连和配置变化后主动重新订阅。
- Windows 通知不依赖浏览器 `Notification` API，改由 Wails Go bridge 调用本机 `NotifyIcon`，更贴近系统通知需求。
- Android 通知必须与 foreground service status notification 分开，避免用户消息和服务常驻状态互相覆盖。

## 可复用排查线索

- 症状：NotifyNode 已启动但收不到消息。
- 触发条件：未登录、topic 列表为空、TopicBus 断线后未重订阅、发布 topic 与订阅 topic 大小写或空白不一致。
- 关键词：`notify.topics_json`, `StartNotify`, `subscribe_batch`, `DequeueNotifications`, `topicbus publish`, `myflowhub_notify`.
- 快速检查：
  - 查看 UI 状态是否 `Connected`、`Logged In`、`Notify`；
  - 检查 `runtime_config.json` 的 `notify.topics_json`；
  - 确认发布端 topic 与订阅 topic 精确一致；
  - Android 13+ 检查 `POST_NOTIFICATIONS` 权限。

## 关键设计决策与权衡

- 保持在 `MyFlowHub-MetricsNode` 内实现，复用已有跨平台 runtime、Wails、gomobile 和 Android foreground service。
- 不改 TopicBus 协议，不做 wildcard/filter/offline replay，避免把应用市场方向提前固化到协议层。
- 通知事件队列采用 bounded FIFO，SDK 收包路径只做解析和入队，不阻塞系统通知 API。
- Windows 通知使用 PowerShell/NotifyIcon，无新增 Go 依赖；代价是依赖 Windows Forms 可用性。

## 测试与验证方式 / 结果

- `GOWORK=off go test ./... -count=1 -p 1`：通过。
- `GOWORK=off go test . -count=1` in `nodemobile`：通过。
- `npm run build` in `windows/frontend`：通过。
- `powershell -ExecutionPolicy Bypass -File .\scripts\build-windows.ps1`：通过，产物 `windows/build/bin/windows.exe`。
- UI 预览修正后再次运行 `npm run build` 和 `scripts/build-windows.ps1`：通过。
- `ANDROID_HOME=D:\project\MyFlowHub3\_android-sdk; ANDROID_SDK_ROOT=D:\project\MyFlowHub3\_android-sdk; .\gradlew.bat :app:assembleDebug`：通过。
- `git diff --check`：通过。

## 潜在影响

- Windows 每条通知会启动一个短生命周期 PowerShell 子进程；正常低频通知场景可接受，高频 topic 后续应增加节流或原生 toast presenter。
- Android 无 AAR 时仍可 stub 编译，但真机通知链路需要重新构建并放置 gomobile AAR。
- TopicBus publish 无 ack，通知展示只能做到 best-effort。

## 回滚方案

- 删除 `core/notify` 和 `core/runtime/notify.go`；
- 移除 runtime notify 配置与 `onUnmatchedFrame` TopicBus 分支；
- 移除 Windows App/Vue/Wails notify 方法和 UI；
- 移除 Android bridge/service/UI notify 逻辑；
- 删除 `docs/requirements/notify-node.md`、`docs/specs/notify-node.md` 和本归档。

## 子Agent执行轨迹

- 未派发子Agent。原因：runtime、Wails binding、Android bridge/service 共享 API，拆分会增加集成风险。
