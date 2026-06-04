# 2026-06-04 MetricsNode Android Background Notify

## Background

Android NotifyNode could still look completely non-functional in the background after the reconnect/service-restore workflow. Two host-side gaps remained: Android 13+ notification permission was treated as a one-time UI value, and a service refresh with an already-live Go runtime did not restart Java/Kotlin notify poller threads after `onDestroy()` stopped them.

## Changes

- `MainActivity` now keeps notification permission as mutable state, refreshes it from live OS state, and requests `POST_NOTIFICATIONS` before explicit `Start Notify`.
- `NotifyPage` blocks `Start Notify` with a clear error if Android notification permission is missing.
- `NodeService.handleRestoreOrRefresh()` now restarts or stops host-side metric observers and NotifyNode poller from `bridge.status()` when no persisted snapshot restore is needed but the Go runtime still reports live work.
- `NodeService` foreground status text for notify start/stop/connect paths now uses the shared foreground text helper.
- `NodeServiceSupport` exposes pure helper decisions for live runtime work, metric observers, and notify poller restart.
- Android unit coverage now includes the live runtime poller restart decisions.

## Related Plan

- `plan.md`, follow-up task IDs:
  - `FUP-ANDROID-NOTIFY-1`
  - `FUP-ANDROID-NOTIFY-2`
  - `FUP-DOCS-1`
  - `FUP-VERIFY-1`

## Related Requirements

- `docs/requirements/notify-node.md`

## Related Specs

- `docs/specs/notify-node.md`

## Lessons Impact

updated

## Related Lessons

- `docs/lessons/metricsnode-reconnect-android-keepalive.md`
- `docs/lessons/android-fgs-notification-behavior.md`

## Searchable Lessons Summary

- Symptoms: Android background NotifyNode subscribed but no notifications, Android 13+ notifications never appear, service refresh shows foreground status but user notifications do not post.
- Triggers: missing `POST_NOTIFICATIONS`, stale Compose permission state, `onDestroy()` stopped poller threads, null or unknown service intent refresh while Go runtime still reports `notify=true`.
- Keywords: `POST_NOTIFICATIONS`, `remember`, `startNotifyPoller`, `handleRestoreOrRefresh`, `bridge.status().notify`, `START_STICKY`.
- Quick checks: verify notification permission first, then verify the host poller thread restarts when runtime notify state is already true.

## Requirements Impact

updated

Clarified Android notification permission gating and host poller restart behavior for live runtime refresh.

## Specs Impact

updated

Clarified Android `POST_NOTIFICATIONS` handling and null/unknown intent refresh behavior that restarts host-side pollers from live runtime state.

## Validation

- `cd android; :app:testDebugUnitTest :app:compileDebugKotlin`
- `git diff --check`

Result:

- Passed.
- Android build used the existing stub bridge path because `android/app/libs/myflowhub.aar` is absent in the local environment.

## Rollback

- Revert the `MainActivity` notification permission state/gating changes.
- Revert `NodeService.handleRestoreOrRefresh()` host-poller restart logic and foreground text helper usage.
- Remove the new `NodeServiceSupport` helpers and unit test assertions.
- Revert the docs/change, docs/lessons, requirements, and specs edits for this follow-up.
