# Android FGS and Notification Behavior Fix Plan

## Project Goal

Fix two Android-only MetricsNode issues:

- Fresh install, entering hub address and tapping Connect crashes on Android 14 / targetSdk 34.
- NotifyNode user messages go directly to the notification drawer instead of showing as heads-up notifications when the runtime environment allows it.

Windows behavior is out of scope and must remain unchanged.

## Current State

- Repo: `D:/project/MyFlowHub3/repo/MyFlowHub-MetricsNode`
- Active worktree: `D:/project/MyFlowHub3/worktrees/android-fgs-notification-behavior`
- Branch: `fix/android-fgs-notification-behavior`
- Base branch: `main`
- Base commit: `0a875422ce1c1bf8f83e28d97c0b49d6a5bc3601`
- Current stage: `4`

## Stage 1 - Requirements Analysis

### Goal

Make Android connect/start behavior stable on fresh installs and make NotifyNode user notifications interruptive enough for heads-up display where Android settings permit it.

### Scope

Must:

- Prevent `ACTION_CONNECT` from starting a foreground service with the `camera` runtime foreground service type.
- Keep Android foreground service status notification separate from NotifyNode user notifications.
- Keep NotifyNode connected/subscribed behavior independent from notification display failure.
- Preserve Windows behavior and shared Go runtime behavior.

Optional:

- Add a reusable troubleshooting lesson for Android 14 foreground service type crashes and notification channel importance.

Not doing:

- TopicBus protocol changes.
- Go runtime notification queue changes.
- New Android permission UX for camera / flashlight.
- Full-screen intent notifications.
- OEM-specific notification policy bypass.

### Use Cases

- User installs Android app, enters hub address, taps Connect, and the app stays alive while the service connects.
- User receives a matching NotifyNode TopicBus publish and Android shows a heads-up notification if `POST_NOTIFICATIONS` is granted, the channel is high importance, and system/user settings allow interruption.

### Functional Requirements

- Connect/register/login/reporting/notify service state updates must not require camera foreground service eligibility.
- NotifyNode user notifications must be posted on a high-importance channel.
- Existing notification permission denial behavior remains best-effort: no crash, no session teardown.

### Non-Functional Requirements

- Smallest safe Android-only change.
- No unnecessary I/O or repeated computation.
- Explicit handling for existing notification channel behavior.
- Build verification for Android.

### Inputs / Outputs

- Input: Android service actions from `MainActivity` (`CONNECT`, `REGISTER`, `LOGIN`, `START_REPORTING`, `START_NOTIFY`, etc.).
- Output: stable foreground service status notification and high-importance user notification posts.

### Boundary Exceptions

- If `POST_NOTIFICATIONS` is denied on Android 13+, NotifyNode events are skipped by platform UI but runtime remains connected.
- Existing Android notification channels cannot have importance raised programmatically after creation, so a new channel ID may be required for already-installed apps.
- Heads-up display is still subject to Android system, user, DND, and OEM settings.

### Acceptance Criteria

- `ACTION_CONNECT` no longer passes `FOREGROUND_SERVICE_TYPE_CAMERA` to `ServiceCompat.startForeground()`.
- Foreground service status notification remains low importance / ongoing.
- NotifyNode user notifications use a high-importance channel and high notification priority for pre-O behavior.
- Android debug build compiles.
- Code review checklist passes.

### Risks

- Removing the runtime camera FGS type from generic service startup may affect future background flashlight behavior if the app later depends on true camera foreground-service semantics.
- Existing installed devices with the old notify channel ID cannot be upgraded in place if the same channel ID is reused.

## Stage 2 - Architecture Design

### Overall Approach

Use one Android-only service fix:

- Declare the foreground service at runtime as `dataSync` for the MetricsNode service status notification. This matches connect/auth/reporting network synchronization and avoids Android 14 camera foreground-service runtime prerequisites during non-camera actions.
- Move NotifyNode user notifications to a high-importance channel, using a new channel ID to avoid immutable existing channel importance.

Alternative considered:

- Dynamically add `FOREGROUND_SERVICE_TYPE_CAMERA` only when camera permission is granted. This still risks Android 14 while-in-use eligibility failures and is unnecessary for connect/auth/notify startup.

### Module Responsibilities

- `android/app/src/main/java/com/myflowhub/metricsnode/NodeService.kt`
  - Owns foreground service status notification.
  - Owns Android NotifyNode user notification posting.

No Go, Windows, or protocol modules participate.

### Data / Call Flow

1. `MainActivity` dispatches `ACTION_CONNECT` with hub address.
2. `NodeService.onStartCommand()` calls `startForegroundWithState("Connecting...")`.
3. `startForegroundWithState()` posts an ongoing status notification with `FOREGROUND_SERVICE_TYPE_DATA_SYNC`.
4. Notify poller dequeues events and calls `postUserNotification()`.
5. `postUserNotification()` posts through the high-importance NotifyNode channel.

### Interface Draft

- No public API change.
- Internal constants may change:
  - `NOTIFY_CHANNEL_ID` can be versioned to force high-importance channel creation for existing installs.

### Errors And Safety

- Keep `POST_NOTIFICATIONS` permission check before notification posting.
- Do not throw from channel creation or notification posting beyond existing platform behavior.
- Keep foreground status notification low importance to avoid persistent heads-up noise.

### Performance And Test Strategy

- No additional threads or polling.
- Android build: `cd android; .\gradlew.bat :app:assembleDebug`.
- Static review: inspect relevant diff and `git diff --check`.

### Extensibility

- If true background camera operation is required later, add a dedicated camera action/path with explicit permission UX and foreground eligibility checks instead of coupling it to every service action.

## Stage 3.1 - Planning

Requirements impact: none
Specs impact: none
Related requirements: `docs/requirements/notify-node.md`
Related specs: `docs/specs/notify-node.md`
Related lessons: create `docs/lessons/android-fgs-notification-behavior.md` if investigation remains reusable after implementation.

## Executable Checklist

- [x] T1: Fix foreground service type used by Android `NodeService`.
- [x] T2: Fix Android NotifyNode user notification channel / priority behavior.
- [x] T3: Validate, review, and archive the workflow.

## Tasks

### T1 - Android Foreground Service Type

- Goal: prevent fresh-install Android 14 connect crash caused by using `camera` FGS type for non-camera service startup.
- Files:
  - `android/app/src/main/java/com/myflowhub/metricsnode/NodeService.kt`
- Acceptance:
  - `ServiceCompat.startForeground()` receives `FOREGROUND_SERVICE_TYPE_DATA_SYNC` only for generic MetricsNode service state.
  - Manifest can still declare supported service types, but runtime request avoids camera when not actively required.
- Tests:
  - Android debug build.
  - Diff inspection for no unrelated behavior changes.
- Rollback:
  - Restore previous `dataSync | camera` runtime foreground service type.

### T2 - NotifyNode Heads-Up Channel

- Goal: post user notifications through a high-importance channel so Android can show heads-up notifications.
- Files:
  - `android/app/src/main/java/com/myflowhub/metricsnode/NodeService.kt`
- Acceptance:
  - NotifyNode user channel is created with `NotificationManager.IMPORTANCE_HIGH`.
  - Existing installs are not trapped on an immutable old default-importance channel.
  - Notifications set high priority for Android 7.1 and lower compatibility.
- Tests:
  - Android debug build.
  - Diff inspection for foreground status channel remaining low importance.
- Rollback:
  - Restore previous notify channel ID and `IMPORTANCE_DEFAULT`.

### T3 - Validation / Review / Archive

- Goal: complete code review and workflow documentation.
- Files:
  - `plan.md`
  - `docs/change/2026-05-31_android-fgs-notification-behavior.md`
  - optional `docs/lessons/android-fgs-notification-behavior.md`
  - optional `docs/lessons/README.md`
- Acceptance:
  - `git diff --check` passes.
  - Android debug build result recorded.
  - Required code review checklist passes.
  - Change archive includes task mapping, decisions, validation, risks, rollback, and troubleshooting hints.
- Tests:
  - `cd android; .\gradlew.bat :app:assembleDebug`
- Rollback:
  - Revert this workflow commit/change set.

## Dependencies / Risks / Notes

- Android official docs require camera FGS type users to satisfy `CAMERA` runtime permission and while-in-use eligibility; generic connection work should not request camera type.
- Android notification channel importance is immutable after creation; changing channel ID is the practical migration path for existing installs.
- Parallelism assessment: not parallelizable. Both runtime FGS behavior and notification channel behavior are in `NodeService.kt`, so splitting would create edit conflicts without reducing risk.

阻塞：否
进入 3.2
