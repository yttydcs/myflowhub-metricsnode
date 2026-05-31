# Android FGS and Notification Behavior

## Summary

Android 14 / targetSdk 34 rejects foreground service startup when a generic service action requests `FOREGROUND_SERVICE_TYPE_CAMERA` without meeting camera runtime permission and while-in-use eligible-state rules. NotifyNode heads-up behavior also depends on using a high-importance Android notification channel.

## Lookup Hints

- Symptoms: fresh install Connect crash, Android app asks for authorization after crash, notification goes straight to drawer, no heads-up banner.
- Error text: `SecurityException: Starting FGS with type camera`, `FOREGROUND_SERVICE_CAMERA`, `eligible state`, `android.permission.CAMERA`.
- Code keywords: `NodeService.startForegroundWithState`, `ServiceCompat.startForeground`, `FOREGROUND_SERVICE_TYPE_CAMERA`, `NotificationChannel`, `IMPORTANCE_HIGH`, `CATEGORY_MESSAGE`, `myflowhub_notify_v3`.
- Quick check: inspect the runtime foreground service type passed to `startForeground()`, not only the manifest declaration.

## Symptoms

- User enters hub address on Android fresh install and taps Connect.
- App crashes before or during service startup.
- Stack trace points at `NodeService.startForegroundWithState()`.
- NotifyNode user notifications may appear only in the notification center without heads-up display.

## Impact

- Android users cannot complete the initial connect flow.
- NotifyNode messages are less visible than expected.
- Windows host behavior is unaffected.

## Trigger Conditions

- Android targetSdk 34.
- Generic connect/auth/reporting/notify foreground service state uses camera FGS runtime type.
- App does not yet have `CAMERA` runtime permission or is not in a system-eligible state for while-in-use camera access.
- NotifyNode user notification channel is `IMPORTANCE_DEFAULT` or an already-created old channel.
- Device/user settings disable floating/banner notifications for the NotifyNode channel, or DND/OEM policy suppresses interruption.

## Root Cause

The manifest declared `android:foregroundServiceType="dataSync|camera"` after flashlight support was added. `NodeService.startForegroundWithState()` then requested both `DATA_SYNC` and `CAMERA` at runtime for every service state transition, including Connect. Android 14 enforces camera FGS permission and foreground eligibility during `startForeground()`, so non-camera startup crashed.

The NotifyNode channel used `IMPORTANCE_DEFAULT`. Android heads-up notifications generally require high importance, and existing channel importance cannot be raised after the channel is created. Some devices also require the channel to have alert behavior such as sound or vibration, and user/OEM settings can still suppress the banner.

## Investigation Trail

- Crash stack pointed to `ServiceCompat.startForeground()` in `NodeService.startForegroundWithState()`.
- `AndroidManifest.xml` declared `FOREGROUND_SERVICE_CAMERA` and service type `dataSync|camera`.
- `NodeService.kt` unconditionally passed `FOREGROUND_SERVICE_TYPE_DATA_SYNC or FOREGROUND_SERVICE_TYPE_CAMERA`.
- Flashlight observer/control paths already check `CAMERA` permission before camera access.
- NotifyNode user channel was separate from the foreground service status channel, but used default importance.
- Follow-up showed that high importance alone may still land in the drawer on some devices if channel alert behavior or device floating-notification settings do not permit interruption.

## Resolution

- Use `FOREGROUND_SERVICE_TYPE_DATA_SYNC` for generic MetricsNode foreground service status startup.
- Keep the foreground status channel low importance.
- Move NotifyNode user messages to high-priority notifications on a new high-importance channel ID.
- Follow-up: move NotifyNode user messages to `myflowhub_notify_v3`, enable vibration, set default notification sound, and mark notifications as message category with public visibility.

## Prevention / Guardrails

- Do not request sensitive foreground service runtime types on generic service actions.
- Treat manifest service type declarations as allowed capabilities; choose the narrow runtime type per action.
- For Android 8+ notification behavior, version the channel ID when importance semantics need to change.
- Keep user message channels separate from ongoing foreground service status channels.
- When heads-up still does not appear, check app notification settings for the exact channel, floating/banner permission, DND, and OEM notification restrictions before assuming TopicBus delivery failed.

## Related Docs

- `docs/requirements/notify-node.md`
- `docs/specs/notify-node.md`
- `docs/change/2026-03-03_metricsnode-p0-metrics-flashlight.md`
- `docs/change/2026-05-26_lightweight-notify-node.md`
- `docs/change/2026-05-31_android-fgs-notification-behavior.md`
- `docs/change/2026-05-31_android-notify-heads-up-channel.md`
