# Plan - Lightweight Notify Node

## Workflow Information
- Repo: `MyFlowHub-MetricsNode`
- Branch: `feat/lightweight-notify-node`
- Base: `main` at `3ec5ca6`
- Worktree: `D:\project\MyFlowHub3\worktrees\feat-lightweight-notify-node`
- Current Stage: `4` archived; workflow remains open until user confirms closeout
- Main repo path: `D:\project\MyFlowHub3\repo\MyFlowHub-MetricsNode` is control-plane only.

## Stage Records

### Initialization
- guide.md: read. Worktrees must be under `D:\project\MyFlowHub3\worktrees`; commit messages should use Chinese text after the conventional prefix.
- repos.md: read. `MyFlowHub-MetricsNode` owns independent node apps and is the right implementation repo for a lightweight notification node.
- Worktree confirmation: created `D:\project\MyFlowHub3\worktrees\feat-lightweight-notify-node` on `feat/lightweight-notify-node`.
- Existing dirty state: meta workspace root is dirty with unrelated docs/thesis/runtime files. This workflow must not modify or revert those files.
- Active repo state: `MyFlowHub-MetricsNode` main repo is clean; worktree is clean except this plan.

### Stage 1 - Requirements Analysis
#### Goal
Build a lightweight notification node, modeled after `MetricsNode`, that can connect/login to MyFlowHub, subscribe to user-configured TopicBus topics, and show OS-level notifications when matching messages arrive.

#### Scope
Must:
- Keep implementation in `MyFlowHub-MetricsNode` worktree.
- Add a dedicated NotifyNode runtime surface instead of changing Server/Proto wire contracts.
- Support Windows system notification display from the Windows app.
- Support Android notification display from foreground/background-capable Android service.
- Allow users to configure subscribed topics.
- Decode TopicBus `publish` messages and turn them into notification title/body.
- Re-subscribe after connect/login/start and after subscription settings change.
- Validate topic input, node identity, and platform notification payloads explicitly.
- Add focused tests for parsing, config validation, TopicBus payload handling, and notification event queue behavior.

Optional:
- Basic UI for topic list and notification preview/history.
- Name filtering if it can be added without changing TopicBus contracts.

Not doing:
- No TopicBus protocol changes.
- No Server/SubProto changes.
- No offline backlog or replay.
- No generic application marketplace implementation.
- No wildcard topic matching unless the current TopicBus implementation already supports it. The stable spec says topic strings are exact and case-sensitive.
- No arbitrary script/plugin execution.

#### Use Cases
- User runs NotifyNode on Windows, configures `dev/codex/task`, logs in, and receives a bottom-right Windows notification when Codex publishes a message.
- User runs NotifyNode on Android, configures `home/alert`, starts the service, and receives a top notification when a sensor node publishes an alert.
- User edits topic subscriptions and the node resubscribes without restarting the whole app where practical.
- User reconnects after network loss and subscriptions are restored because TopicBus subscriptions are in-memory and connection-scoped.

#### Functional Requirements
- Connection/auth:
  - Reuse MetricsNode-style connection, key, register, and login flows.
  - Do not duplicate protocol stack logic in platform UI.
- Subscription:
  - Store subscriptions locally as normalized settings.
  - Subscribe via TopicBus `subscribe_batch` after a successful session is available.
  - Reject blank or duplicate topics at the config boundary.
  - Keep exact topic semantics; no wildcard interpretation in NotifyNode MVP.
- Inbound message handling:
  - Handle unmatched frames with `SubProtoTopicBus`.
  - Decode `publish` payload: `topic`, `name`, `ts`, `payload`.
  - Ignore messages for topics not currently enabled.
  - Build notification title/body from message fields:
    - Prefer payload object fields such as `title`, `body`, `summary`, `message` when present.
    - Fall back to `name` and compact JSON payload text.
  - Bound payload/title/body length to avoid notification/UI overload.
- Platform notification:
  - Windows: expose a platform bridge that shows a system notification when runtime queues a NotifyEvent.
  - Android: NodeService consumes NotifyEvent queue and posts notification through `NotificationManager`.
  - Android must use `POST_NOTIFICATIONS` permission already declared; UI/service should handle permission denial gracefully.
- Observability:
  - Expose status: connected, logged in, subscribing/running, subscribed topics, last notification, last error.

#### Non-functional Requirements
- Keep changes narrow and compatible with existing MetricsNode metrics reporting behavior.
- Avoid adding heavyweight dependencies unless needed for platform notification.
- Keep config local and explicit.
- Avoid unbounded queues; drop or coalesce when notification queue is full.
- Do not block the SDK receive path on slow platform notification APIs.
- Make reconnect/resubscribe behavior predictable and logged.

#### Inputs / Outputs
- Inputs:
  - Hub address.
  - Device ID.
  - Node ID for login.
  - Subscription topic list.
  - TopicBus publish payloads.
- Outputs:
  - OS notification.
  - UI/status state.
  - Local config updates.
  - Logs/last error for failures.

#### Edge Cases
- Not connected or not logged in: starting NotifyNode should fail explicitly or report not ready.
- Empty topic list: allowed only as stopped/no-subscription state; no subscribe request is sent.
- Duplicate topics: collapsed in normalized config.
- Malformed TopicBus frame: ignored with warning, no crash.
- Payload is not an object: display compact value safely.
- Notification permission denied on Android: service remains connected/subscribed but records notification failure.
- Windows notification unavailable in dev/runtime environment: report a user-visible error and keep runtime alive.
- TopicBus spec has no offline replay: missed messages during disconnect are expected.

#### Acceptance Criteria
- Windows NotifyNode can connect/login, subscribe to at least one configured topic, receive a publish, and show a Windows notification.
- Android NotifyNode service can connect/login/start listening and post a notification when a subscribed publish arrives.
- Topic subscriptions are local-configurable and survive app restart.
- Reconnect/start path restores subscriptions.
- Existing MetricsNode tests/build paths do not regress.
- New behavior is covered by focused tests where platform UI can be mocked.

#### Risks
- Windows native toast implementation may need an external dependency or PowerShell/WinRT fallback; choose the smallest maintainable option during implementation.
- Android notification permission behavior differs across SDK versions.
- Current gomobile bridge is MetricsNode-specific; adding NotifyNode APIs must avoid breaking existing reflected method names.
- TopicBus publish has no ack, so end-to-end tests need a real or fake Hub path.

### Stage 2 - Architecture Design
#### Overall Solution
Implement NotifyNode as a sibling feature inside `MyFlowHub-MetricsNode`, reusing the repo's cross-platform node app structure:
- `core/notify`: subscription config, TopicBus message parsing, notification event shaping, bounded event queue.
- `core/runtime`: add TopicBus client operations and inbound `publish` handling, or add a NotifyRuntime if cleaner during implementation.
- `windows`: expose NotifyNode controls and a Windows notification presenter.
- `android`: extend bridge/service to start notification listening and post Android notifications.

Reasoning:
- `MyFlowHub-MetricsNode` already owns cross-platform dedicated node apps, config storage, Android foreground service patterns, Windows Wails app, and gomobile bridge.
- No wire-level protocol changes are required because TopicBus already provides exact topic subscription and publish delivery.
- A separate top-level repo would add overhead without solving the core runtime reuse problem.

#### Alternatives Considered
- Add notification feature to `MyFlowHub-Win`: rejected because Win is a full management console, while the user asked for a light node similar to MetricsNode and Android support.
- Add a new `MyFlowHub-NotifyNode` repo: viable long-term if product separation becomes important, but not needed for MVP and would duplicate existing cross-platform scaffolding.
- Implement only via Flow: rejected for this task because user specifically wants a machine-local system notification node.

#### Module Responsibilities
- `core/notify`
  - Validate and store subscription model.
  - Decode TopicBus publish messages into `NotificationEvent`.
  - Extract title/body safely.
  - Maintain bounded event queue for platform consumers.
- `core/runtime`
  - Own SDK session and auth state as today.
  - Send TopicBus subscribe requests.
  - Route unmatched TopicBus frames to notify handler.
  - Expose `StartNotify`, `StopNotify`, `NotifySettingsGet/Set`, `DequeueNotifications`.
- `nodemobile`
  - Reflect stable methods for Android: notify settings, start/stop listening, dequeue notifications.
- `windows`
  - Add Notify page/controls or a simple mode in existing UI.
  - Poll or subscribe to runtime notification queue.
  - Invoke Windows notification presenter.
- `android`
  - Extend `NodeService` with notify listening mode.
  - Post received notification events through Android notification channels.
  - Keep foreground service status notification separate from user message notifications.
- `docs`
  - Add requirement/spec docs if implementation confirms stable behavior.
  - Archive final workflow in `docs/change`.

#### Data / Call Flow
```text
User configures topics
  -> platform UI writes notify settings
  -> runtime validates and stores config
  -> connect/login/start notify
  -> runtime sends topicbus.subscribe_batch(topics)
  -> Hub forwards matching topicbus.publish
  -> SDK unmatched frame callback
  -> runtime detects SubProtoTopicBus publish
  -> notify parser creates NotificationEvent
  -> bounded queue
  -> Windows/Android platform consumer dequeues
  -> OS notification API displays message
```

#### Interface Drafts
Core DTOs:
```go
type NotifyTopicSetting struct {
    Topic   string `json:"topic"`
    Enabled bool   `json:"enabled"`
}

type NotificationEvent struct {
    ID      string          `json:"id"`
    Topic   string          `json:"topic"`
    Name    string          `json:"name"`
    Title   string          `json:"title"`
    Body    string          `json:"body"`
    TS      int64           `json:"ts"`
    Payload json.RawMessage `json:"payload,omitempty"`
}
```

Runtime methods:
```go
NotifySettingsGet() []NotifyTopicSetting
NotifySettingsSet([]NotifyTopicSetting) error
StartNotify() error
StopNotify()
IsNotifyRunning() bool
DequeueNotifications() []NotificationEvent
```

Android bridge mirrors the methods with JSON string inputs/outputs to stay consistent with current gomobile reflection patterns.

#### Error Handling and Safety
- Config validation rejects blank topic names and overlong topic strings.
- Subscribe errors update last error and stop the ready state instead of silently continuing.
- Inbound decode errors are logged and ignored.
- Notification extraction bounds title/body length.
- Queue overflow drops oldest events and records a counter/error hint.
- Android notification permission denial is surfaced as last error.

#### Performance and Testing Strategy
- Topic lookup uses a map of enabled topics.
- Event queue is bounded and non-blocking from receive path.
- No repeated subscribe requests when settings are unchanged and already active.
- Tests:
  - Go unit tests for settings validation.
  - Go unit tests for TopicBus publish decode and title/body extraction.
  - Go unit tests for queue overflow behavior.
  - Existing `go test ./... -count=1 -p 1` in repo root.
  - Windows targeted build/generate only if code touches Wails bindings.
  - Android Gradle compile path only if Kotlin APIs change significantly.

#### Extensibility Design Points
- Notify extraction is isolated so later app-market manifest rules can reuse it.
- Exact-topic MVP keeps parity with current TopicBus spec; wildcard/filtering can be added later above TopicBus without pretending the protocol supports it today.
- Platform notification presentation is behind a small interface, so Windows and Android can diverge without changing TopicBus logic.
- Queue events keep raw payload for future UI history or routing.

### Stage 3.1 - Planning
#### Project Goal and Current State
The repo already has a cross-platform MetricsNode with:
- shared Go runtime for SDK connection/auth/config
- Windows Wails host
- Android foreground service and gomobile bridge

It lacks:
- TopicBus subscription logic in MetricsNode runtime
- notification-specific config/queue
- Windows OS notification display
- Android user-message notification display for TopicBus messages

#### Docs Governance Routing Decision
Using `$m-docs`:
- Requirements impact: add
  - New stable requirement should describe lightweight NotifyNode behavior, exact-topic subscriptions, and platform notification expectations.
- Specs impact: add/clarify
  - New stable spec should describe NotifyNode local config/runtime behavior and explicitly depend on existing TopicBus exact-match semantics.
- Change archive: add at Stage 4.
- Lessons impact: none currently; create only if implementation exposes recurring Wails/gomobile/notification pitfalls.

#### Related Requirements / Specs / Lessons
- Requirements:
  - No existing NotifyNode requirement found.
  - Existing `docs/requirements/auth-controlled-admission.md` is relevant for auth boundary only.
- Specs:
  - `D:\project\MyFlowHub3\repo\MyFlowHub-Server\docs\specs\topicbus.md`
  - `D:\project\MyFlowHub3\docs\specs\protocol_map.md`
- Lessons:
  - `D:\project\MyFlowHub3\docs\lessons\wails-bindings-cross-project.md`
  - `D:\project\MyFlowHub3\docs\lessons\frontend-worktree-wailsjs-missing.md`
  - Memory notes about MetricsNode Wails binding repair.

#### Executable Task List
- [x] DOCS-1: Add stable NotifyNode requirement/spec docs.
- [x] CORE-1: Add notify config, parser, event queue, and tests.
- [x] CORE-2: Integrate TopicBus subscribe/publish handling into runtime.
- [x] WIN-1: Add Windows UI/control surface and notification presenter.
- [x] ANDROID-1: Add Android bridge/service notification listener and notification posting.
- [x] VERIFY-1: Run focused tests/build checks and code review.
- [x] ARCHIVE-1: Archive docs/change and update indexes.

#### Task Details
##### DOCS-1 - Stable NotifyNode Requirement And Spec
- Owner: main agent
- Worktree: `D:\project\MyFlowHub3\worktrees\feat-lightweight-notify-node`
- Plan Path: `D:\project\MyFlowHub3\worktrees\feat-lightweight-notify-node\plan.md`
- Goal: Document stable behavior before implementation completes.
- Files / Modules:
  - `docs/requirements/notify-node.md`
  - `docs/requirements/README.md`
  - `docs/specs/notify-node.md`
  - `docs/specs/README.md`
- Write Set: docs only.
- Acceptance: Requirement/spec state exact-topic semantics, no offline replay, platform notification behavior, and no Server/Proto change.
- Test Points: Markdown links and index entries exist.
- Rollback: Remove new docs and index entries.

##### CORE-1 - Notify Config, Parser, Event Queue
- Owner: main agent
- Worktree: `D:\project\MyFlowHub3\worktrees\feat-lightweight-notify-node`
- Plan Path: `D:\project\MyFlowHub3\worktrees\feat-lightweight-notify-node\plan.md`
- Goal: Add platform-neutral notify model and tests.
- Files / Modules:
  - `core/notify/*`
  - `core/notify/*_test.go`
- Write Set: new package preferred.
- Acceptance: Validates topic settings, decodes TopicBus publish payload, extracts bounded title/body, queues events safely.
- Test Points: `go test ./core/notify -count=1`.
- Rollback: Delete `core/notify`.

##### CORE-2 - Runtime TopicBus Subscription Integration
- Owner: main agent
- Worktree: `D:\project\MyFlowHub3\worktrees\feat-lightweight-notify-node`
- Plan Path: `D:\project\MyFlowHub3\worktrees\feat-lightweight-notify-node\plan.md`
- Goal: Reuse SDK session to subscribe and receive TopicBus messages.
- Files / Modules:
  - `core/runtime/runtime.go`
  - `core/runtime/config.go`
  - optional `core/runtime/notify.go`
  - `core/runtime/*_test.go`
- Write Set: runtime package.
- Acceptance: Start/stop notify, settings get/set, subscribe batch, inbound publish handling, resubscribe after start/connect.
- Test Points: runtime unit tests with fake/decoded frames where possible; full `go test ./... -count=1 -p 1`.
- Rollback: Revert runtime notify changes.

##### WIN-1 - Windows Notification Surface
- Owner: main agent
- Worktree: `D:\project\MyFlowHub3\worktrees\feat-lightweight-notify-node`
- Plan Path: `D:\project\MyFlowHub3\worktrees\feat-lightweight-notify-node\plan.md`
- Goal: Provide a lightweight Windows control surface and system notification display.
- Files / Modules:
  - `windows/app.go`
  - `windows/main.go` if notification initialization requires it
  - `windows/frontend/src/App.vue`
  - generated `windows/frontend/wailsjs/**` if Wails bindings must be regenerated
- Write Set: Windows host and frontend.
- Acceptance: User can configure topics/start listening and receive OS notification from queued events.
- Test Points: targeted Go compile or Wails generation/build per repo scripts if feasible.
- Rollback: Revert Windows host/frontend changes.

##### ANDROID-1 - Android Notification Service Integration
- Owner: main agent
- Worktree: `D:\project\MyFlowHub3\worktrees\feat-lightweight-notify-node`
- Plan Path: `D:\project\MyFlowHub3\worktrees\feat-lightweight-notify-node\plan.md`
- Goal: Extend Android service to listen for notifications and post user-visible notifications.
- Files / Modules:
  - `nodemobile/nodemobile.go`
  - `android/app/src/main/java/com/myflowhub/metricsnode/NodeBridge.kt`
  - `android/app/src/main/java/com/myflowhub/metricsnode/NodeService.kt`
  - `android/app/src/main/java/com/myflowhub/metricsnode/MainActivity.kt` if UI control is needed
  - Android resources/manifest only if extra channel strings or permissions are needed
- Write Set: bridge and Android host.
- Acceptance: Service has a notification-listening mode and posts a distinct message notification for received TopicBus events.
- Test Points: Kotlin/Gradle compile if feasible; otherwise targeted static validation and note limitation.
- Rollback: Revert Android bridge/service changes.

##### VERIFY-1 - Validation And Review
- Owner: main agent
- Worktree: `D:\project\MyFlowHub3\worktrees\feat-lightweight-notify-node`
- Plan Path: `D:\project\MyFlowHub3\worktrees\feat-lightweight-notify-node\plan.md`
- Goal: Validate and review changed behavior.
- Files / Modules: no new feature files unless fixes are needed.
- Write Set: test outputs are not committed.
- Acceptance: Required tests pass or failures are clearly blocked with exact cause.
- Test Points:
  - `$env:GOWORK='off'; go test ./... -count=1 -p 1`
  - Windows build script if Wails binding/API changed.
  - Android compile path if bridge/service changed.
- Rollback: Revert failing task changes or return to Stage 3.2.

##### ARCHIVE-1 - Change Archive
- Owner: main agent
- Worktree: `D:\project\MyFlowHub3\worktrees\feat-lightweight-notify-node`
- Plan Path: `D:\project\MyFlowHub3\worktrees\feat-lightweight-notify-node\plan.md`
- Goal: Archive implementation, validation, decisions, and rollback.
- Files / Modules:
  - `docs/change/2026-05-26_lightweight-notify-node.md`
  - `docs/change/README.md`
  - lessons docs only if needed
- Write Set: docs archive.
- Acceptance: Stage 4 archive is complete and indexed.
- Test Points: links and task mappings present.
- Rollback: Remove archive/index updates.

#### Dependencies
- CORE-2 depends on CORE-1.
- WIN-1 and ANDROID-1 depend on CORE-2.
- VERIFY-1 depends on implementation tasks.
- ARCHIVE-1 depends on VERIFY-1.

#### Risks and Notes
- Windows toast implementation choice is the main uncertainty. During Stage 3.2, choose the least risky repo-consistent presenter and document fallback behavior.
- Android service currently uses metrics-specific naming. Keep MVP integrated in the repo without broad renaming; avoid a cosmetic rename unless required.
- TopicBus has no permission control or replay; NotifyNode must document that it only receives live messages while connected/subscribed.
- Generated Wails bindings are easy to drift; if Wails API changes, regenerate using the repo script and verify exports.

#### Parallelism Assessment
- Do not use sub-agents for this workflow.
- Reason: write sets overlap through shared runtime and bridge APIs; splitting Windows/Android before the core API stabilizes would increase integration risk.
- Main agent owns all tasks and must keep changes task-mapped.

#### Issue List
- Windows system notification implementation technique is not locked until Stage 3.2 code inspection.
- Android notification permission prompt/control may require UI adjustment; if this expands beyond current task, return to Stage 3.1.

## Gate
阻塞：否
Stage 3.2, 3.3, and 4 complete
禁止派发子Agent

## Stage 3.2 - Implementation Record

### File-Level Change Summary
- `docs/README.md`, `docs/requirements/*`, `docs/specs/*`: added governed docs entry points and stable NotifyNode requirement/spec.
- `core/notify/*`: added topic settings validation, TopicBus publish parsing, notification event shaping, bounded queue, and tests.
- `core/runtime/config.go`, `core/runtime/runtime.go`, `core/runtime/notify.go`, `core/runtime/notify_test.go`: added `notify.topics_json`, TopicBus subscribe/publish handling, start/stop/status/dequeue APIs, reconnect/login/settings resubscribe, and tests.
- `nodemobile/nodemobile.go`: exposed notify settings/start/stop/dequeue JSON bridge methods.
- `windows/app.go`, `windows/frontend/src/App.vue`, `windows/frontend/wailsjs/go/*`: added Windows Notify UI, Wails bindings, and PowerShell/NotifyIcon presenter.
- `windows/frontend/src/App.vue`: kept status pills single-line so `Notify Off` does not wrap under header pressure.
- `android/app/src/main/java/com/myflowhub/metricsnode/*`: added Notify state, bridge types/methods, service notification poller/channel, and Compose Notify page.

### Design Notes
- TopicBus semantics remain exact topic matching with no offline replay.
- Notification display is best-effort and decoupled from the SDK receive path through a bounded queue.
- Windows uses a no-new-dependency `System.Windows.Forms.NotifyIcon` bridge.
- Android uses a separate `myflowhub_notify` notification channel and keeps foreground service status separate.

## Stage 3.3 - Code Review

- 需求覆盖：通过。Windows/Android topic 配置、订阅、入站 publish 处理和系统通知均覆盖。
- 架构合理性：通过。新增 `core/notify` 隔离协议解析与事件队列，runtime 只负责 session/subscription。
- 性能风险：通过。topic lookup 使用 map，队列有界，SDK 收包路径不调用平台通知 API。
- 可读性与一致性：通过。沿用 runtime config、Wails method、gomobile JSON bridge 和 Android service 既有模式。
- 可扩展性与配置化：通过。`notify.topics_json` 与 `notify.Event` 保留后续应用市场/规则抽象空间。
- 稳定性与安全：通过。配置校验、登录/连接校验、payload 大小限制、错误显式返回或日志记录。
- 测试覆盖情况：通过。核心配置、解析、队列、runtime 入站处理均有单测；Windows/Android 做构建验证。
- 子Agent治理与审计：通过。未派发子Agent，原因是共享 runtime/bridge 写集重叠。

## Stage 4 - Change Archive

### Docs Governance
- 使用 `$m-docs`。
- Requirements impact: updated.
- Specs impact: updated.
- Lessons impact: none.
- Related requirements: `docs/requirements/notify-node.md`.
- Related specs: `docs/specs/notify-node.md`, `D:\project\MyFlowHub3\repo\MyFlowHub-Server\docs\specs\topicbus.md`.
- Related lessons: none.

### Archive
- `docs/change/2026-05-26_lightweight-notify-node.md`
- `docs/change/README.md`

### Validation Results
- `$env:GOWORK='off'; go test ./... -count=1 -p 1` passed.
- `$env:GOWORK='off'; go test . -count=1` in `nodemobile` passed.
- `npm run build` in `windows/frontend` passed.
- `powershell -ExecutionPolicy Bypass -File .\scripts\build-windows.ps1` passed and produced `windows/build/bin/windows.exe`.
- UI preview follow-up: fixed `Notify Off` pill wrapping, then reran `npm run build` and `scripts/build-windows.ps1`; both passed.
- `$env:ANDROID_HOME='D:\project\MyFlowHub3\_android-sdk'; $env:ANDROID_SDK_ROOT='D:\project\MyFlowHub3\_android-sdk'; .\gradlew.bat :app:assembleDebug` passed.
- `git diff --check` passed.

### Remaining Notes
- Workflow is not closed or merged. Worktree remains at `D:\project\MyFlowHub3\worktrees\feat-lightweight-notify-node`.
