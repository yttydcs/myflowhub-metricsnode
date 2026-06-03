# MetricsNode Reconnect And Android Service Restore

## Summary

MetricsNode can look connected in local UI state while the live Hub session is gone if reconnect does not perform a fresh login and restore connection-scoped behavior. Android foreground services can also lose work after sticky restart when runtime configuration lives only in the original intent extras.

## Lookup Hints

- Symptoms: Android background killed, service restarts but does not resume, NotifyNode stops receiving messages after disconnect, metrics stop updating after network loss, local auth shows node id but Hub does not see the node.
- Keywords: `onClientError`, `reconnectDesired`, `auth_snapshot.json`, `logged_in=true`, `subscribe_batch`, `START_STICKY`, `intent == null`, `NodeRunSnapshot`, `service_run_snapshot`.
- Quick checks:
  - Inspect `auth_snapshot.json`; `logged_in=true` from a previous process must not be trusted.
  - Check whether runtime re-login happens after reconnect before `subscribe_batch`.
  - Check whether Android `onStartCommand()` handles `intent == null`.
  - Check whether explicit stop/disconnect clears desired restore state.

## Symptoms

- NotifyNode works initially, then stops receiving TopicBus publish messages after a network drop.
- Metrics reporting stays marked as running locally but values no longer reach VarStore after reconnect.
- Android service is killed in the background and later restarts with no reporting or notify poller active.
- Android foreground notification may show disconnected or stopped even though the user previously started the service.

## Impact

- Cross-platform users must manually reconnect and login after transient transport loss.
- Android users can lose long-running MetricsNode behavior after system service recreation.
- NotifyNode subscriptions disappear because TopicBus subscriptions are in-memory and connection-scoped.
- Metrics reporting can be half-alive locally while the Hub session is not bound to the node.

## Trigger Conditions

- SDK read loop reports a session error.
- Runtime stores durable identity but has no reconnect/re-login loop.
- Runtime treats saved `LoggedIn` as durable instead of session-scoped.
- Android foreground service returns `START_STICKY` but `onStartCommand()` treats null intent as no-op.
- Android run configuration exists only in intent extras or editable UI form state.

## Root Cause

The SDK transport can reconnect at the socket/session level, but it cannot restore application auth or connection-scoped application state by itself. MetricsNode must fresh-login before restoring NotifyNode TopicBus subscriptions and MetricsNode reporting publish state.

On Android, sticky service restart can recreate the service without the original action and extras. Without a persisted desired run snapshot, the service does not know whether the user wanted connected, reporting, or notify state restored.

## Investigation Trail

1. Trace `Runtime.onClientError` and confirm whether it only records an error or also schedules recovery.
2. Trace recovery ordering: broken client close, connect, login, then connection-scoped restore.
3. Confirm `Runtime.loadAuthSnapshot` clears stale `LoggedIn=true`.
4. Trace `Login()` side effects for NotifyNode resubscribe and metrics republish.
5. On Android, inspect `NodeService.onStartCommand()` null-intent branch.
6. Confirm stop/disconnect actions clear persisted desired state.

## Resolution

- Runtime session errors now mark live auth logged out and schedule reconnect when reconnect is desired.
- Reconnect uses bounded backoff, closes the stale session, reconnects to the last address, and fresh-login with saved durable identity.
- Login success restores NotifyNode subscriptions and forces metrics republish from latest known values.
- Loaded auth snapshots preserve identity but rewrite stale `logged_in=true` to `false`.
- Android `NodeService` persists `NodeRunSnapshot` and restores from it on null or unknown sticky restart intents.
- Android explicit stop/disconnect clears the desired run snapshot.

## Prevention / Guardrails

- Keep reconnect state in the application runtime when recovery requires auth and app-level subscriptions.
- Never treat persisted live session flags as proof of a current Hub-bound connection.
- Restore connection-scoped features after login, not immediately after transport connect.
- Android foreground services that rely on intent extras must persist a separate desired run snapshot.
- Explicit stop actions must clear desired restore state before stopping the service.
- For future connection-scoped features, add restore hooks after `Login()` and cover them with reconnect tests.

## Related Requirements / Specs / Changes

- Requirement: `docs/requirements/notify-node.md`
- Spec: `docs/specs/notify-node.md`
- Related lesson: `docs/lessons/android-fgs-notification-behavior.md`
- External related lesson: `D:/project/MyFlowHub3/repo/MyFlowHub-Android/docs/lessons/android-hub-service-restart.md`
- External related lesson: `D:/project/MyFlowHub3/repo/MyFlowHub-ClipboardNode/docs/lessons/startup-subscribe-timeout-half-connected.md`
- Change: `docs/change/2026-06-03_metricsnode-reconnect-android-keepalive.md`
