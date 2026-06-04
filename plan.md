# Plan - MetricsNode Reconnect And Android Service Restore

## Workflow Information

- Repo: `D:/project/MyFlowHub3/repo/MyFlowHub-MetricsNode`
- Branch: `fix/metricsnode-reconnect-android-keepalive`
- Base: `origin/main` at `5cedf562ec643c758e573432fc7591f9c582afbd`
- Worktree: `D:/project/MyFlowHub3/worktrees/fix-metricsnode-android-background-keepalive`
- Current Stage: `4`

## Stage Records

### Initialization

- `guide.md`: not present in the worktree root.
- Base/worktree confirmation:
  - Implementation work is confined to `D:/project/MyFlowHub3/worktrees/fix-metricsnode-android-background-keepalive`.
  - Main repo path is treated as control-plane only.
  - Participating repo: `MyFlowHub-MetricsNode`.
  - Participating modules: `core/runtime`, `android/app`, `docs`.
- Context search:
  - ACE `search_context` was attempted during implementation planning and timed out more than once. The workflow continued with direct file and documentation review.

### Stage 1 - Requirements Analysis

#### Goal

MetricsNode must recover from broken transport sessions across platforms by reconnecting and re-login on the active runtime session. Android must also recover the foreground service after sticky service restart when the process/service is recreated without the original intent extras.

#### Scope

Must:

- Detect SDK session errors and mark the current session disconnected/logged-out.
- Reconnect to the last successful address after unexpected transport loss.
- Re-login with the persisted durable identity before restoring connection-scoped behavior.
- Restore NotifyNode TopicBus subscriptions after reconnect because TopicBus subscriptions are connection-scoped.
- Restore MetricsNode reporting state by keeping reporting collectors alive and forcing current values to republish after re-login.
- Treat persisted `auth_snapshot.json` identity as durable, but treat `logged_in` as a live session flag that must not survive fresh process startup.
- On Android, persist the desired service run snapshot and use it when `onStartCommand()` receives a null or unknown intent after sticky restart.
- Keep explicit stop/disconnect actions from being undone by restore logic.

Optional:

- Keep foreground notification text informative enough to distinguish running, connected, notify, and disconnected states.
- Unit-test pure Android restore decisions separately from Android service framework behavior.

Not doing:

- Server, Proto, SDK, or TopicBus wire protocol changes.
- OEM-specific Android battery exemption prompts or private keepalive APIs.
- WorkManager scheduling or periodic background polling.
- Offline backlog/replay for NotifyNode.
- A new user-facing UI workflow.

#### Use Cases

- A Windows or Android MetricsNode runtime loses its TCP session while previously connected/logged in; it reconnects and re-login without the user pressing Connect/Login again.
- NotifyNode was running before session loss; after reconnect/re-login, TopicBus subscriptions are restored.
- Metrics reporting was running before session loss; after reconnect/re-login, the runtime republishes the latest known values instead of waiting only for future changes.
- Android foreground service is killed/recreated by the system; if the user previously wanted connect/reporting/notify active, the service rehydrates from the saved run snapshot.
- User explicitly taps disconnect/stop; service restore must not restart it.

#### Functional Requirements

- Runtime reconnect only starts after a successful connect/login establishes user intent to keep the transport alive.
- Unexpected SDK session errors must clear live login state and save `logged_in=false`.
- Reconnect attempts use bounded exponential backoff.
- Reconnect recovery must close the broken transport before dialing again.
- Reconnect recovery must fresh-login using saved `device_id` and `node_id`.
- Login success must resubscribe NotifyNode topics when NotifyNode is running.
- Login success must force MetricsNode variable republish when reporting is running.
- Runtime startup must load saved identity but clear stale `logged_in=true`.
- Android service restore must validate that a desired snapshot has an address before trying to restore.
- Android service restore must re-run `init -> connect -> login -> startReporting/startNotify` as needed.
- Android stop/disconnect actions must clear the desired run snapshot.

#### Non-functional Requirements

- Keep the change small and localized to runtime reconnect and Android service restoration.
- Avoid blocking SDK receive paths or Android main thread.
- Do not swallow errors silently; expose them through runtime last error or Android foreground state.
- Add tests for reconnect/re-login, NotifyNode resubscribe, stale auth snapshot handling, and Android restore helper behavior.

#### Inputs / Outputs

- Input: SDK `onClientError` callback, saved runtime auth snapshot, runtime config, Android foreground service intents, Android saved run snapshot.
- Output: restored transport session, fresh logged-in auth state, restored NotifyNode subscriptions, forced metrics republish, restored Android foreground service/pollers.

#### Edge Cases

- Missing address in Android run snapshot blocks restore and clears desired state.
- Missing saved identity permits reconnect-only recovery, but reporting/notify recovery requires login identity.
- User stop/disconnect clears desired state and disables runtime reconnect.
- Stale persisted `logged_in=true` is rewritten as `false` on runtime startup.
- Notify topics empty still fails explicit `StartNotify()` and should not send subscribe requests.

#### Acceptance Criteria

- Runtime auto reconnect re-login is covered by a fake TCP hub test.
- NotifyNode resubscribe after reconnect is covered by a fake TCP hub test.
- Loading a stale logged-in auth snapshot requires fresh login and preserves identity fields.
- Android restore helper tests cover desired snapshot decisions, missing address, and foreground text.
- Go runtime tests pass.
- Android unit tests and Kotlin compile pass.
- Broader Go and Android build validation are attempted and recorded.

#### Risks

- Reconnect logic touches shared runtime state and can introduce races if cancellation/close ordering is wrong.
- Android sticky restore can accidentally restart after explicit stop if desired flags are not cleared.
- Android OS/OEM battery policy can still kill or restrict work; this workflow restores service state but does not guarantee indefinite background survival on every device policy.

### Stage 2 - Architecture Design

#### Overall Solution

Add reconnect/re-login to `core/runtime`, because MetricsNode runtime owns auth, config, reporting, NotifyNode subscription state, and platform bridge state. Add Android sticky restart recovery in `NodeService`, because Android process/service recreation is a host lifecycle problem and cannot be solved by SDK reconnect alone.

#### Alternatives Considered

- SDK-level reconnect: rejected for this workflow because the SDK does not own app auth, TopicBus subscriptions, metrics reporting state, or Android service desire state.
- Android OEM keepalive/battery exemption: rejected as the primary fix because it does not address missing cross-platform reconnect/re-login and is not portable.
- Android WorkManager retry loop: rejected because this is an active foreground service use case and the minimal missing behavior is sticky snapshot restore, not scheduled background jobs.
- Persist and trust `logged_in=true`: rejected because login is session-scoped, not durable identity.

#### Module Responsibilities

- `core/runtime/runtime.go`: connection lifecycle, reconnect scheduling, re-login, auth snapshot live/durable split, forced metrics republish.
- `core/runtime/notify.go`: existing NotifyNode subscribe and running-state behavior reused by reconnect.
- `android/app/src/main/java/com/myflowhub/metricsnode/NodeService.kt`: Android service intent handling, stop semantics, snapshot saving, sticky restore orchestration.
- `android/app/src/main/java/com/myflowhub/metricsnode/NodeServicePrefs.kt`: persisted desired run snapshot.
- `android/app/src/main/java/com/myflowhub/metricsnode/NodeServiceSupport.kt`: testable restore decisions and foreground text.
- `docs/requirements`, `docs/specs`, `docs/lessons`, `docs/change`: stable behavior, technical contract, reusable troubleshooting, and workflow archive.

#### Data / Call Flow

Runtime reconnect:

1. SDK invokes `Runtime.onClientError(err)`.
2. Runtime sets connected=false, records last error, marks auth logged out, and schedules reconnect if reconnect is desired.
3. Reconnect loop closes the old client/session, dials the last address, and re-login using saved identity.
4. `Login()` saves fresh auth, resubscribes NotifyNode topics if running, and forces metrics republish if reporting is running.
5. Reconnect loop stops after a successful recovery or continues with bounded backoff on failure.

Android sticky restore:

1. User actions save a merged `NodeRunSnapshot` with address, identity, and desired connect/reporting/notify flags.
2. Explicit stop/disconnect clears the snapshot.
3. Null/unknown service intent loads the snapshot.
4. If desired state exists and the address is valid, service runs `init -> connect -> login -> startReporting/startNotify`.
5. Service starts/stops Android observers and notification poller based on restored state, then updates the foreground notification.

#### Interface Drafts

- No public server or wire protocol changes.
- Runtime behavior changes behind existing APIs:
  - `Connect(addr)`
  - `Login(deviceID, nodeID)`
  - `Close()`
  - `StartReporting()`
  - `StartNotify()`
- Android adds internal helpers:
  - `NodeRunSnapshot`
  - `NodeServicePrefs.loadSnapshot/saveMerged/clearDesired`
  - `NodeServiceSupport.shouldRestore/restoreError/foregroundText`

#### Error Handling and Safety

- Blank address/device/node inputs continue to fail explicitly.
- Reconnect logs each failed attempt and records `LastError`.
- Stale persisted login is rewritten to logged out.
- Android missing restore address clears desired state and stops the service.
- Explicit stop/disconnect clears desired Android restore state.
- Notification delivery and metrics update paths must not crash the runtime on transient errors.

#### Performance and Testing Strategy

- Reconnect uses one goroutine per active reconnect loop guarded by `reconnectRunning`.
- Backoff starts at 1 second and caps at 30 seconds in production; tests override delays.
- Android snapshot writes happen only on user lifecycle actions and restore state changes.
- Validation:
  - `GOWORK=off go test ./core/runtime -count=1`
  - `GOWORK=off go test ./... -count=1 -p 1`
  - Android `:app:testDebugUnitTest`, `:app:compileDebugKotlin`, `:app:assembleDebug`
  - `git diff --check`

#### Extensibility Design Points

- Runtime reconnect state stays inside `Runtime`, so other platform hosts inherit reconnect/re-login without Android-specific logic.
- Android restore snapshot is host-owned and can later add user-visible restore diagnostics without changing Go runtime APIs.
- If more connection-scoped features are added, hook their restoration after `Login()` rather than in SDK reconnect.

### Stage 3.1 - Planning

#### Project Goal and Current State

Implement cross-platform runtime auto reconnect/re-login and Android foreground service sticky restore for MetricsNode, preserving explicit user stop semantics and current NotifyNode/reporting behavior.

#### Docs Governance Routing Decision

Using `$m-docs`:

- Stable behavior belongs in `docs/requirements/notify-node.md`.
- Runtime/Android technical behavior belongs in `docs/specs/notify-node.md`.
- Reusable troubleshooting belongs in `docs/lessons/metricsnode-reconnect-android-keepalive.md`.
- Completed workflow record belongs in `docs/change/2026-06-03_metricsnode-reconnect-android-keepalive.md`.

#### Related Requirements / Specs / Lessons

- Requirements impact: clarify
- Specs impact: clarify
- Related requirements:
  - `docs/requirements/notify-node.md`
- Related specs:
  - `docs/specs/notify-node.md`
- Related lessons:
  - `docs/lessons/android-fgs-notification-behavior.md`
  - `D:/project/MyFlowHub3/repo/MyFlowHub-Android/docs/lessons/android-hub-service-restart.md`
  - `D:/project/MyFlowHub3/repo/MyFlowHub-ClipboardNode/docs/lessons/startup-subscribe-timeout-half-connected.md`

#### Executable Task List

- [x] RUNTIME-1: Add runtime reconnect/re-login and stale-login snapshot handling.
- [x] RUNTIME-2: Add runtime reconnect tests for re-login, NotifyNode resubscribe, and stale auth snapshot.
- [x] ANDROID-1: Add Android foreground service desired-state snapshot and sticky restore flow.
- [x] ANDROID-2: Add Android restore helper unit tests and JUnit dependency.
- [x] DOCS-1: Update stable requirements/specs and reusable lesson.
- [x] VERIFY-1: Run broad validation and complete code review.
- [x] ARCHIVE-1: Create change archive and update indexes.

#### Task Details

##### RUNTIME-1 - Runtime Reconnect And Fresh Login

- Owner: main agent
- Worktree: `D:/project/MyFlowHub3/worktrees/fix-metricsnode-android-background-keepalive`
- Plan Path: `D:/project/MyFlowHub3/worktrees/fix-metricsnode-android-background-keepalive/plan.md`
- Goal: Recover unexpected SDK session loss by reconnecting, fresh-login, resubscribing NotifyNode, and forcing metrics republish.
- Files / Modules:
  - `core/runtime/runtime.go`
  - existing `core/runtime/notify.go`
  - existing `core/runtime/config.go`
- Write Set:
  - `core/runtime/runtime.go`
- Acceptance:
  - Unexpected session error marks connected=false and auth logged out.
  - Reconnect loop is desired only after successful connect/login and stops on explicit close.
  - Recovery reuses last address and saved durable identity.
  - Recovery calls login before restoring connection-scoped behavior.
  - Startup clears persisted `LoggedIn=true`.
- Test Points:
  - Runtime fake hub tests.
  - `go test ./core/runtime -count=1`.
- Rollback:
  - Remove reconnect fields/functions and restore previous `Connect`, `Close`, `Login`, and `loadAuthSnapshot` behavior.

##### RUNTIME-2 - Runtime Reconnect Tests

- Owner: main agent
- Worktree: `D:/project/MyFlowHub3/worktrees/fix-metricsnode-android-background-keepalive`
- Plan Path: `D:/project/MyFlowHub3/worktrees/fix-metricsnode-android-background-keepalive/plan.md`
- Goal: Cover reconnect re-login, NotifyNode resubscribe, and stale auth snapshot handling.
- Files / Modules:
  - `core/runtime/reconnect_test.go`
- Write Set:
  - `core/runtime/reconnect_test.go`
- Acceptance:
  - Fake hub observes a second login after session loss.
  - Fake hub observes a second subscribe after NotifyNode reconnect.
  - Loaded `auth_snapshot.json` preserves identity and clears `LoggedIn`.
- Test Points:
  - `go test ./core/runtime -count=1`.
- Rollback:
  - Remove `core/runtime/reconnect_test.go`.

##### ANDROID-1 - Android Sticky Service Restore

- Owner: main agent
- Worktree: `D:/project/MyFlowHub3/worktrees/fix-metricsnode-android-background-keepalive`
- Plan Path: `D:/project/MyFlowHub3/worktrees/fix-metricsnode-android-background-keepalive/plan.md`
- Goal: Persist desired Android service state and restore it on null/unknown sticky restart intent.
- Files / Modules:
  - `android/app/src/main/java/com/myflowhub/metricsnode/NodeService.kt`
  - `android/app/src/main/java/com/myflowhub/metricsnode/NodeServicePrefs.kt`
  - `android/app/src/main/java/com/myflowhub/metricsnode/NodeServiceSupport.kt`
- Write Set:
  - `android/app/src/main/java/com/myflowhub/metricsnode/NodeService.kt`
  - `android/app/src/main/java/com/myflowhub/metricsnode/NodeServicePrefs.kt`
  - `android/app/src/main/java/com/myflowhub/metricsnode/NodeServiceSupport.kt`
- Acceptance:
  - Connect/register/login/start/stop actions save merged desired snapshot.
  - Stop/disconnect/stop-all clear desired snapshot.
  - Null/unknown intent restores from snapshot or stops when no desired state exists.
  - Restore starts observers and notify poller only after successful runtime state.
- Test Points:
  - Kotlin compile.
  - Android unit tests for helper logic.
- Rollback:
  - Remove new helper files and revert `NodeService.kt` intent restore/snapshot changes.

##### ANDROID-2 - Android Restore Tests

- Owner: main agent
- Worktree: `D:/project/MyFlowHub3/worktrees/fix-metricsnode-android-background-keepalive`
- Plan Path: `D:/project/MyFlowHub3/worktrees/fix-metricsnode-android-background-keepalive/plan.md`
- Goal: Add focused unit coverage for testable restore support logic.
- Files / Modules:
  - `android/app/build.gradle.kts`
  - `android/app/src/test/java/com/myflowhub/metricsnode/NodeServiceSupportTest.kt`
- Write Set:
  - `android/app/build.gradle.kts`
  - `android/app/src/test/java/com/myflowhub/metricsnode/NodeServiceSupportTest.kt`
- Acceptance:
  - JUnit dependency is available for local unit tests.
  - Tests cover restore decision, missing address, and foreground text.
- Test Points:
  - `:app:testDebugUnitTest`
- Rollback:
  - Remove test and JUnit dependency.

##### DOCS-1 - Stable Docs And Lessons

- Owner: main agent
- Worktree: `D:/project/MyFlowHub3/worktrees/fix-metricsnode-android-background-keepalive`
- Plan Path: `D:/project/MyFlowHub3/worktrees/fix-metricsnode-android-background-keepalive/plan.md`
- Goal: Record stable behavior and reusable troubleshooting knowledge.
- Files / Modules:
  - `docs/requirements/notify-node.md`
  - `docs/specs/notify-node.md`
  - `docs/lessons/metricsnode-reconnect-android-keepalive.md`
  - `docs/lessons/README.md`
- Write Set:
  - `docs/requirements/notify-node.md`
  - `docs/specs/notify-node.md`
  - `docs/lessons/metricsnode-reconnect-android-keepalive.md`
  - `docs/lessons/README.md`
- Acceptance:
  - Requirements clarify reconnect/re-login and Android restore boundary.
  - Specs document runtime reconnect flow and Android snapshot restore flow.
  - Lesson has symptoms, triggers, quick checks, root cause, resolution, and related docs.
- Test Points:
  - Docs diff review.
- Rollback:
  - Revert docs changes.

##### VERIFY-1 - Validation And Code Review

- Owner: main agent
- Worktree: `D:/project/MyFlowHub3/worktrees/fix-metricsnode-android-background-keepalive`
- Plan Path: `D:/project/MyFlowHub3/worktrees/fix-metricsnode-android-background-keepalive/plan.md`
- Goal: Validate implementation and complete mandatory code review.
- Files / Modules:
  - `plan.md`
  - validation outputs
- Write Set:
  - `plan.md`
- Acceptance:
  - Required checks are run or explicitly recorded if blocked.
  - Stage 3.3 checklist marks each item `通过` or `不通过`.
  - Any failed review item returns workflow to 3.2.
- Test Points:
  - `GOWORK=off go test ./... -count=1 -p 1`
  - Android unit/Kotlin/build checks.
  - `git diff --check`
- Rollback:
  - Fix implementation or revert the failing task's change set before leaving Stage 3.

##### ARCHIVE-1 - Change Archive

- Owner: main agent
- Worktree: `D:/project/MyFlowHub3/worktrees/fix-metricsnode-android-background-keepalive`
- Plan Path: `D:/project/MyFlowHub3/worktrees/fix-metricsnode-android-background-keepalive/plan.md`
- Goal: Archive completed workflow and update change index.
- Files / Modules:
  - `docs/change/2026-06-03_metricsnode-reconnect-android-keepalive.md`
  - `docs/change/README.md`
  - `plan.md`
- Write Set:
  - `docs/change/2026-06-03_metricsnode-reconnect-android-keepalive.md`
  - `docs/change/README.md`
  - `plan.md`
- Acceptance:
  - Archive includes required sections, task mapping, validation, rollback, and sub-agent trace.
  - User is asked whether to end workflow after archive is complete.
- Test Points:
  - Docs diff review.
- Rollback:
  - Remove archive/index edits.

#### Dependencies

- Android build depends on local SDK path `D:/project/MyFlowHub3/_android-sdk`.
- Go commands in this worktree require `GOWORK=off` because the parent `go.work` does not include the worktree.

#### Risks and Notes

- PowerShell may append unrelated `conda-script.py ... invalid choice: ''` / `Invoke-Expression ... empty string` noise after successful commands.
- Android gomobile AAR may be missing locally; the app build has an existing stub bridge fallback and logs that condition.
- No Android device is attached, so real OS kill/restart behavior cannot be observed locally.

#### Parallelism Assessment

- Sub-agent use: not used.
- Reason:
  - Runtime reconnect and Android restore are tightly coupled by saved identity/session semantics.
  - Host tool policy only allows spawning sub-agents when the user explicitly asks for delegation/parallel agents; the user did not.
  - Main-agent review keeps task mapping and docs/archive consistency simpler.

#### Issue List

- None.

阻塞：否
进入 3.2

### Stage 3.2 - Implementation

#### File-Level Change Summary

- `core/runtime/runtime.go`
  - Added reconnect desired/running state, reconnect loop, recover-once flow, close/cancel handling, and stale-login snapshot clearing.
  - `Login()` now restores connection-scoped NotifyNode subscriptions and forces reporting republish after fresh login.
- `core/runtime/reconnect_test.go`
  - Added fake TCP hub tests for reconnect login, NotifyNode resubscribe, and stale auth snapshot load.
- `android/app/src/main/java/com/myflowhub/metricsnode/NodeService.kt`
  - Added desired snapshot saves on lifecycle actions, clears on explicit stops, and restore/refresh handling for null or unknown intents.
- `android/app/src/main/java/com/myflowhub/metricsnode/NodeServicePrefs.kt`
  - Added persisted `NodeRunSnapshot` load/save/clear helpers.
- `android/app/src/main/java/com/myflowhub/metricsnode/NodeServiceSupport.kt`
  - Added testable restore decision and foreground text helpers.
- `android/app/src/test/java/com/myflowhub/metricsnode/NodeServiceSupportTest.kt`
  - Added Android local unit tests.
- `android/app/build.gradle.kts`
  - Added JUnit local unit test dependency.

#### Design Notes

- Runtime reconnect lives above SDK transport because it must perform application auth and restore connection-scoped app behavior.
- Persisted identity fields are durable; persisted `LoggedIn` is not.
- Android sticky restore uses a service run snapshot rather than UI form state.
- Explicit stop/disconnect remains authoritative and clears desired restore state.

#### Validation Completed So Far

- `gofmt -w core\runtime\runtime.go core\runtime\reconnect_test.go`: passed.
- `GOWORK=off go test ./core/runtime -count=1`: passed.
- Android `:app:testDebugUnitTest :app:compileDebugKotlin`: passed with expected missing gomobile AAR stub-bridge lifecycle message.

### Stage 3.3 - Code Review

#### Checklist

- 需求覆盖: 通过
- 架构合理性: 通过
- 性能风险（N+1 / 重复计算 / 多余 I/O / 锁竞争）: 通过
- 可读性与一致性: 通过
- 可扩展性与配置化: 通过
- 稳定性与安全: 通过
- 测试覆盖情况: 通过
- 子Agent治理与审计（任务映射、上下文完整性、文件所有权、结果复核、冲突处理、记录完整性）: 通过

#### Review Notes

- Runtime reconnect lives above SDK transport and preserves application-level auth and connection-scoped behavior.
- Android sticky restore uses a separate desired run snapshot and clears it on explicit stop/disconnect.
- Validation passed after the final reconnect backoff refinement.

#### Issue List

- None.

### Stage 4 - Change Archive

#### Archive Status

- `docs/change/2026-06-03_metricsnode-reconnect-android-keepalive.md`: completed
- `docs/change/README.md`: updated
- `docs/lessons/metricsnode-reconnect-android-keepalive.md`: created
- `docs/lessons/README.md`: updated

#### Outcome

- Workflow archive is complete and awaiting the user's end-workflow confirmation.

阻塞：否
进入 4

## Follow-up - Android Background Notify

### Trigger

User reported on 2026-06-04 that Android background notifications are completely non-functional after the reconnect/service restore workflow.

### Current Worktree

- Repo: `D:/project/MyFlowHub3/repo/MyFlowHub-MetricsNode`
- Branch: `fix/metricsnode-android-background-notify`
- Base: `main` at `cfd46be`
- Worktree: `D:/project/MyFlowHub3/worktrees/fix-metricsnode-android-background-notify`
- Current Stage: `3.2` follow-up implementation

### Requirements / Specs / Lessons Impact

- Requirements impact: clarify
- Specs impact: clarify
- Related requirements: `docs/requirements/notify-node.md`
- Related specs: `docs/specs/notify-node.md`
- Related lessons:
  - `docs/lessons/metricsnode-reconnect-android-keepalive.md`
  - `docs/lessons/android-fgs-notification-behavior.md`

### Requirements Analysis

- Goal: Android NotifyNode must actually post user notifications while the service is in the background when the user has enabled NotifyNode and Android notification permission allows display.
- Must:
  - request/check `POST_NOTIFICATIONS` before Android explicit NotifyNode start on Android 13+;
  - avoid relying on a one-time Compose permission value;
  - restart host-side NotifyNode poller when service refresh sees live runtime `notify=true`;
  - keep explicit stop semantics unchanged.
- Not doing:
  - OEM battery exemption UX;
  - WorkManager background scheduling;
  - TopicBus protocol changes;
  - guaranteed heads-up/banner display beyond existing notification channel behavior.
- Acceptance:
  - Start Notify requests permission and refuses to silently start display-less notification mode when permission is missing.
  - Null/unknown service refresh restarts `startNotifyPoller()` from live runtime status when notify is active.
  - Android unit tests and Kotlin compile pass.

### Architecture Analysis

- `MainActivity` owns user-visible permission flow and passes live permission state into `NotifyPage`.
- `NodeService` owns host-side poller lifecycle because Go runtime notify state and Java/Kotlin polling thread state are separate.
- `NodeServiceSupport` keeps restore/refresh decisions pure and unit-testable.
- Docs routing:
  - stable behavior clarification -> `docs/requirements/notify-node.md`;
  - technical contract clarification -> `docs/specs/notify-node.md`;
  - reusable troubleshooting -> `docs/lessons/metricsnode-reconnect-android-keepalive.md`;
  - archive -> `docs/change/2026-06-04_metricsnode-android-background-notify.md`.

### Executable Tasks

##### FUP-ANDROID-NOTIFY-1 - Permission-Gated Notify Start

- Owner: main agent
- Worktree: `D:/project/MyFlowHub3/worktrees/fix-metricsnode-android-background-notify`
- Plan Path: `D:/project/MyFlowHub3/worktrees/fix-metricsnode-android-background-notify/plan.md`
- Goal: Make Android 13+ notification permission state live and block explicit Notify start when user notifications cannot be posted.
- Files / Modules:
  - `android/app/src/main/java/com/myflowhub/metricsnode/MainActivity.kt`
- Write Set:
  - `android/app/src/main/java/com/myflowhub/metricsnode/MainActivity.kt`
- Acceptance:
  - `Start Notify` requests permission if needed.
  - stale permission state is rechecked against OS state before blocking.
- Test Points:
  - Kotlin compile.
- Rollback:
  - Revert the `hasNotifPermission` and `NotifyPage` parameter changes.

##### FUP-ANDROID-NOTIFY-2 - Refresh Restarts Notify Poller

- Owner: main agent
- Worktree: `D:/project/MyFlowHub3/worktrees/fix-metricsnode-android-background-notify`
- Plan Path: `D:/project/MyFlowHub3/worktrees/fix-metricsnode-android-background-notify/plan.md`
- Goal: Restart Android host-side pollers when service refresh sees live Go runtime work.
- Files / Modules:
  - `android/app/src/main/java/com/myflowhub/metricsnode/NodeService.kt`
  - `android/app/src/main/java/com/myflowhub/metricsnode/NodeServiceSupport.kt`
  - `android/app/src/test/java/com/myflowhub/metricsnode/NodeServiceSupportTest.kt`
- Write Set:
  - `android/app/src/main/java/com/myflowhub/metricsnode/NodeService.kt`
  - `android/app/src/main/java/com/myflowhub/metricsnode/NodeServiceSupport.kt`
  - `android/app/src/test/java/com/myflowhub/metricsnode/NodeServiceSupportTest.kt`
- Acceptance:
  - refresh with `notify=true` starts `startNotifyPoller()`;
  - refresh with `reporting=true` starts metric observers;
  - idle refresh stops the service.
- Test Points:
  - `:app:testDebugUnitTest`.
- Rollback:
  - Revert the no-snapshot branch and helper/test changes.

##### FUP-DOCS-1 - Follow-up Archive And Lessons

- Owner: main agent
- Worktree: `D:/project/MyFlowHub3/worktrees/fix-metricsnode-android-background-notify`
- Plan Path: `D:/project/MyFlowHub3/worktrees/fix-metricsnode-android-background-notify/plan.md`
- Goal: Record the follow-up root cause and searchable Android notification checks.
- Files / Modules:
  - `docs/requirements/notify-node.md`
  - `docs/specs/notify-node.md`
  - `docs/lessons/metricsnode-reconnect-android-keepalive.md`
  - `docs/lessons/README.md`
  - `docs/change/2026-06-04_metricsnode-android-background-notify.md`
  - `docs/change/README.md`
- Write Set:
  - same as files/modules.
- Acceptance:
  - change archive exists and indexes link it;
  - lesson contains permission and poller restart quick checks.
- Test Points:
  - docs diff review.
- Rollback:
  - remove the follow-up archive and revert docs/index edits.

##### FUP-VERIFY-1 - Validation And Review

- Owner: main agent
- Worktree: `D:/project/MyFlowHub3/worktrees/fix-metricsnode-android-background-notify`
- Plan Path: `D:/project/MyFlowHub3/worktrees/fix-metricsnode-android-background-notify/plan.md`
- Goal: Validate Android follow-up and complete code review.
- Files / Modules:
  - `plan.md`
- Write Set:
  - `plan.md`
- Acceptance:
  - Android unit tests and Kotlin compile pass;
  - `git diff --check` passes;
  - review checklist is recorded.
- Test Points:
  - `cd android; :app:testDebugUnitTest :app:compileDebugKotlin`
  - `git diff --check`
- Rollback:
  - return to implementation task for any failed check.

### Parallelism Assessment

- Sub-agent use: not used.
- Reason: Android permission state, service poller lifecycle, and docs archive share one narrow write set and do not benefit from delegation.

阻塞：否
进入 3.2

### Follow-up Stage 3.2 - Implementation Summary

- `FUP-ANDROID-NOTIFY-1`: completed.
  - `MainActivity` now computes notification permission from live OS state, updates mutable state on permission result, and rechecks stale state before blocking `Start Notify`.
  - `NotifyPage` requests permission and returns a clear error instead of silently starting notification delivery without Android permission.
- `FUP-ANDROID-NOTIFY-2`: completed.
  - `NodeService.handleRestoreOrRefresh()` now restarts metric observers and `startNotifyPoller()` when `bridge.status()` reports live work and no persisted snapshot restore is required.
  - `NodeServiceSupport` exposes pure live-work/poller decisions covered by `NodeServiceSupportTest`.
  - Notify foreground status text uses `NodeServiceSupport.foregroundText()`.
- `FUP-DOCS-1`: completed.
  - Requirements/specs clarify Android permission gating and service refresh poller restart.
  - Lessons and change archive include reusable checks for `POST_NOTIFICATIONS` and `startNotifyPoller`.

### Follow-up Stage 3.3 - Code Review

#### Checklist

- 需求覆盖: 通过
- 架构合理性: 通过
- 性能风险（N+1 / 重复计算 / 多余 I/O / 锁竞争）: 通过
- 可读性与一致性: 通过
- 可扩展性与配置化: 通过
- 稳定性与安全: 通过
- 测试覆盖情况: 通过
- 子Agent治理与审计（任务映射、上下文完整性、文件所有权、结果复核、冲突处理、记录完整性）: 通过

#### Review Notes

- Android permission state is read from the OS before user-visible Notify start, so stale Compose state cannot keep the app in a false denial state.
- Host poller lifecycle is restored at the Android service layer because Go runtime `notify=true` does not imply the Java/Kotlin poller thread survived service destruction.
- No shared runtime reconnect behavior was changed in this follow-up.

### Follow-up Stage 4 - Change Archive

Using `$m-docs`:

- Requirements impact: updated
- Specs impact: updated
- Lessons impact: updated
- Related requirements: `docs/requirements/notify-node.md`
- Related specs: `docs/specs/notify-node.md`
- Related lessons:
  - `docs/lessons/metricsnode-reconnect-android-keepalive.md`
  - `docs/lessons/android-fgs-notification-behavior.md`
- Archive: `docs/change/2026-06-04_metricsnode-android-background-notify.md`

#### Validation

- `cd android; :app:testDebugUnitTest :app:compileDebugKotlin`: passed.
- `git diff --check`: passed.
- Note: Android validation used the existing stub bridge fallback because local `android/app/libs/myflowhub.aar` is absent.

#### Outcome

- Follow-up archive is complete and awaiting the user's end-workflow confirmation.

阻塞：否
进入 4
