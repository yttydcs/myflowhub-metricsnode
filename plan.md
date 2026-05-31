# Android Notify Heads-Up Channel Follow-Up Plan

## Project Goal

Make Android NotifyNode user notifications more likely to appear as heads-up notifications after the first FGS crash fix, while keeping the foreground service status notification quiet.

## Current State

- Repo: `D:/project/MyFlowHub3/repo/MyFlowHub-MetricsNode`
- Active worktree: `D:/project/MyFlowHub3/worktrees/android-notify-heads-up-channel`
- Branch: `fix/android-notify-heads-up-channel`
- Base branch: `main`
- Base commit: `72ef23518d0a80cb81a72d44df524ae89a4d5cf9`
- Current stage: `4`

## Stage 1 - Requirements Analysis

### Goal

Android NotifyNode notifications should request interruptive notification behavior using platform-supported notification channel and notification attributes.

### Scope

Must:

- Keep foreground service status notification low importance.
- Keep NotifyNode user notifications separate from foreground service status notification.
- Make NotifyNode channel high importance with explicit alert behavior.
- Preserve runtime behavior when notification permission is denied.

Not doing:

- TopicBus protocol changes.
- Go runtime queue changes.
- Full-screen intent or alarm/call-style notification behavior.
- OEM-specific private APIs.

### Use Cases

- Android user receives a NotifyNode event while the device permits heads-up notifications and sees a banner/pop-up instead of only a drawer entry.

### Functional Requirements

- NotifyNode user channel must be created with high importance.
- Channel must explicitly enable vibration and default notification sound.
- Notification must mark itself as a high-priority message notification.
- Existing old channel settings must not block this attempt.

### Non-Functional Requirements

- Small Android-only change.
- No extra polling, threads, or I/O.
- Build validation required.

### Inputs / Outputs

- Input: `NotifyEvent` dequeued by Android `NodeService`.
- Output: Android notification posted through an alert-capable NotifyNode channel.

### Boundary Exceptions

- If `POST_NOTIFICATIONS` is denied, no notification is posted.
- If user/system/OEM disables channel banners/floating notifications, code cannot force heads-up display.
- If DND suppresses interruption, notification may still only appear in the drawer.

### Acceptance Criteria

- NotifyNode notification channel ID is versioned again to avoid immutable prior channel settings.
- Channel has `IMPORTANCE_HIGH`, vibration enabled, vibration pattern set, and default notification sound.
- Notification has high or max priority, message category, default sound/vibration fallback, and public visibility.
- Android debug build passes.

### Risks

- A more interruptive channel may be too noisy for high-frequency topics.
- Re-versioning channel creates another channel entry in Android settings.

## Stage 2 - Architecture Design

### Overall Approach

Keep the fix inside `NodeService.kt`:

- Use a new channel ID `myflowhub_notify_v3`.
- Configure `NotificationChannel` with high importance, vibration, and default notification sound.
- Configure `NotificationCompat.Builder` with message category, high priority, default sound/vibration, and public visibility.

Alternative considered:

- Full-screen intent. Rejected because NotifyNode messages are not calls/alarms and Android treats full-screen notifications as highly intrusive.

### Module Responsibilities

- `NodeService.kt`: Android foreground service status and user notification posting.

### Data / Call Flow

1. Notify poller dequeues events.
2. `postUserNotification()` checks notification permission.
3. `createNotifyChannelIfNeeded()` creates `myflowhub_notify_v3` with explicit alert behavior.
4. Notification is posted through `NotificationManager.notify()`.

### Interface Draft

- No public API changes.
- Internal channel ID changes from `myflowhub_notify_v2` to `myflowhub_notify_v3`.

### Error And Safety

- Keep permission guard.
- Do not crash or disconnect runtime on notification display failure.
- Do not change foreground service status notification.

### Performance And Test Strategy

- No runtime performance impact beyond one-time channel creation.
- Run `git diff --check`.
- Run `cd android; .\gradlew.bat :app:assembleDebug`.

### Extensibility

- If users need quiet topics later, add user-configurable notification severity/channel mapping instead of weakening this default channel.

## Stage 3.1 - Planning

Requirements impact: none
Specs impact: none
Related requirements: `docs/requirements/notify-node.md`
Related specs: `docs/specs/notify-node.md`
Related lessons: `docs/lessons/android-fgs-notification-behavior.md`

## Executable Checklist

- [x] T1: Strengthen NotifyNode Android channel alert behavior.
- [x] T2: Validate and update workflow docs.

## Tasks

### T1 - Android Notify Channel

- Files:
  - `android/app/src/main/java/com/myflowhub/metricsnode/NodeService.kt`
- Acceptance:
  - User notification channel uses `myflowhub_notify_v3`.
  - Channel explicitly enables vibration and default notification sound.
  - Notification uses message category, high interruptiveness attributes, and default sound/vibration fallback.
- Tests:
  - `git diff --check`
  - Android debug build
- Rollback:
  - Restore `myflowhub_notify_v2` and remove explicit sound/vibration/category additions.

### T2 - Validation And Archive

- Files:
  - `plan.md`
  - `docs/change/2026-05-31_android-notify-heads-up-channel.md`
  - `docs/change/README.md`
  - `docs/lessons/android-fgs-notification-behavior.md`
- Acceptance:
  - Validation result is recorded.
  - Lesson mentions channel settings/OEM/DND limits.
- Tests:
  - Docs diff review.
- Rollback:
  - Revert this workflow commit/change set.

## Dependencies / Risks / Notes

- Official Android docs state heads-up is tied to high-importance channels on Android 8+ and high priority plus ringtone/vibration on Android 7.1 and lower; users can still modify channel settings.
- No Android device is attached to this workstation, so real heads-up behavior cannot be observed locally.
- Parallelism assessment: not parallelizable. The code change is a small single-file edit in `NodeService.kt`.

阻塞：否
进入 3.2
