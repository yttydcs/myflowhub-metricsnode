# 2026-06-03_metricsnode-reconnect-android-keepalive

## 变更背景 / 目标

MetricsNode 在 Android 和其他平台上都存在同一类问题：底层 TCP/Session 断掉后，运行时没有重新登录并恢复连接态行为；Android 前台服务在 `START_STICKY` 重建后，也没有从最近一次实际运行状态恢复。

本次目标是补齐两条链路：

- runtime 断线后自动重连、重新登录、恢复 NotifyNode 订阅与 metrics republish
- Android 前台服务在 sticky restart/null intent 下按保存的运行快照恢复

## 具体变更内容

- `core/runtime/runtime.go`
  - 增加 reconnect intent / reconnect loop / backoff。
  - `onClientError()` 会标记当前登录态失效并调度恢复。
  - 恢复时先关闭旧 session，再连接最后地址并 fresh login。
  - fresh login 后恢复 NotifyNode 订阅并强制 republish 已知 metrics。
  - 加载 `auth_snapshot.json` 时不再信任 persisted `logged_in=true`。
  - 显式关闭时会取消重连意图并停止 reporting / notify。
- `core/runtime/reconnect_test.go`
  - 覆盖 session error 后重新登录。
  - 覆盖 NotifyNode 重新订阅。
  - 覆盖 stale auth snapshot 必须 fresh login。
- `android/app/src/main/java/com/myflowhub/metricsnode/NodeService.kt`
  - 保存/合并运行快照。
  - `ACTION_STOP` / `ACTION_DISCONNECT` / `ACTION_STOP_ALL` 清理 desired restore state。
  - `intent == null` 或未知 action 时尝试 sticky restore。
  - restore flow 按 `init -> connect -> login -> startReporting/startNotify` 执行。
- `android/app/src/main/java/com/myflowhub/metricsnode/NodeServicePrefs.kt`
  - 新增运行快照读写/清理。
- `android/app/src/main/java/com/myflowhub/metricsnode/NodeServiceSupport.kt`
  - 提取可测恢复判断和 foreground 文案逻辑。
- `android/app/src/test/java/com/myflowhub/metricsnode/NodeServiceSupportTest.kt`
  - 覆盖 restore decision、missing addr、foreground text。
- `android/app/build.gradle.kts`
  - 增加 JUnit 本地单测依赖。
- 文档：
  - `docs/requirements/notify-node.md`
  - `docs/specs/notify-node.md`
  - `docs/lessons/metricsnode-reconnect-android-keepalive.md`
  - `docs/lessons/README.md`
  - `docs/change/README.md`

## Requirements impact

updated

补充了跨平台断线重登、fresh login 恢复连接态行为，以及 Android sticky restart 只能按保存的 desired run snapshot 恢复。

## Specs impact

updated

补充了 runtime reconnect/re-login 的状态机、stale auth snapshot 的处理方式，以及 Android 前台服务 restore flow。

## Lessons impact

updated

本次暴露了可复用的排查模式，已新增 lessons 并挂到索引。

## Related requirements

- `docs/requirements/notify-node.md`

## Related specs

- `docs/specs/notify-node.md`

## Related lessons

- `docs/lessons/metricsnode-reconnect-android-keepalive.md`
- `docs/lessons/android-fgs-notification-behavior.md`
- `D:\project\MyFlowHub3\repo\MyFlowHub-Android\docs\lessons\android-hub-service-restart.md`
- `D:\project\MyFlowHub3\repo\MyFlowHub-ClipboardNode\docs\lessons\startup-subscribe-timeout-half-connected.md`

## 对应 plan.md 任务映射

- `RUNTIME-1`
- `RUNTIME-2`
- `ANDROID-1`
- `ANDROID-2`
- `DOCS-1`
- `VERIFY-1`
- `ARCHIVE-1`

## 经验 / 教训摘要

- Persisted identity and live session state are different things; `logged_in` cannot be reused across a fresh process start.
- Connection-scoped features must be restored after fresh login, not merely after transport reconnect.
- Android sticky service restart needs a separate desired run snapshot because intent extras are not guaranteed to survive service recreation.
- Explicit stop/disconnect must clear desired state or sticky restart will undo the user's stop action.

## 可复用排查线索

- 症状：
  - NotifyNode suddenly stops after a network drop.
  - Metrics stop republishing after reconnect.
  - Android service restarts but does not resume reporting or notify.
- 触发条件：
  - session error / TCP disconnect
  - stale `auth_snapshot.json`
  - `intent == null` on Android sticky restart
  - explicit stop/disconnect followed by unexpected restore
- 关键词：
  - `onClientError`
  - `reconnectDesired`
  - `auth_snapshot.json`
  - `logged_in=true`
  - `subscribe_batch`
  - `START_STICKY`
  - `NodeRunSnapshot`
  - `service_run_snapshot`
- 快速检查：
  1. 看 runtime 是否先 fresh login 再恢复 NotifyNode 或 reporting。
  2. 看 `auth_snapshot.json` 是否把 `logged_in` 当成 durable truth。
  3. 看 Android `onStartCommand()` 是否处理了 `intent == null`。
  4. 看 stop/disconnect 是否清掉 desired restore state。

## 关键设计决策与权衡

- reconnect 逻辑放在 `core/runtime`，而不是 SDK 层，因为恢复需要 app auth、NotifyNode 订阅和 metrics republish。
- `auth_snapshot.json` 保留身份字段，但不保留 live session truth。
- Android 只保存 desired run snapshot，不保存 UI 表单状态当作运行事实。
- 退避重连采用指数回退，避免 session loss 后忙等。

## 测试与验证方式 / 结果

- `GOWORK=off go test ./core/runtime -count=1`
- `GOWORK=off go test ./... -count=1 -p 1`
- `GOWORK=off go test . -count=1` in `nodemobile`
- `cd android; :app:testDebugUnitTest :app:assembleDebug`
- `git diff --check`

结果：

- 全部通过。
- Android build 通过，当前环境使用 stub bridge because gomobile AAR is absent.

## 潜在影响与回滚方案

- 影响：
  - reconnect/cancel 逻辑变复杂，若没有测试可能出现 race。
  - Android 会在 sticky restart 后更积极地恢复用户之前的运行意图。
- 回滚：
  - 移除 runtime reconnect state/loop。
  - 恢复 `loadAuthSnapshot()` 对 `logged_in` 的旧处理。
  - 删除 Android 运行快照和 sticky restore 逻辑。
  - 删除新增 docs / lessons / index 条目。

## 子Agent执行轨迹

- 未派发子Agent。
- 原因：
  - runtime reconnect、Android sticky restore、docs archive 共享同一套状态语义，写集耦合。
  - host tool policy only allows sub-agent spawning when the user explicitly asks for delegation/parallel agent work; the user did not.
  - main agent completed implementation, verification, and doc updates directly.
