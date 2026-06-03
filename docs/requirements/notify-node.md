# NotifyNode Requirement

## Goal

NotifyNode is a lightweight local node feature in `MyFlowHub-MetricsNode`. It connects and authenticates like MetricsNode, subscribes to user-configured TopicBus topics, and displays platform system notifications when matching publish messages arrive.

## Scope

Must support:

- Windows and Android hosts.
- Runtime auto reconnect after unexpected transport loss.
- Fresh re-login on a recovered transport session before restoring connection-scoped behavior.
- Local configuration of subscribed topics.
- TopicBus `subscribe_batch` after login/start.
- Live TopicBus `publish` messages only.
- System notification display using the platform host.
- Android foreground service restart restore from a persisted desired run snapshot.
- Explicit validation for blank topics, duplicate topics, missing login state, and malformed publish payloads.

Out of scope:

- TopicBus protocol changes.
- Server or Proto changes.
- Offline backlog, replay, or delivery acknowledgement.
- Wildcard topic matching.
- Application marketplace packaging or arbitrary plugin execution.

## Behavior

- Topic matching follows existing TopicBus semantics: exact string matching after NotifyNode local input normalization.
- Empty topic settings mean NotifyNode can be stopped, but no subscribe request is sent.
- On connect/login/start, the runtime subscribes all enabled topics.
- When settings change while NotifyNode is running, the runtime re-subscribes the current enabled topic set.
- Inbound `publish` messages are ignored if their topic is not enabled locally.
- Notification title and body are derived from payload fields when available, then from `name`, `topic`, and compact payload text.
- Title and body are bounded before being returned to platform UI.
- Notification delivery failure must not close the SDK session or crash the app.
- Unexpected transport loss clears live login state and triggers reconnect while the runtime still has reconnect intent.
- Reconnect recovery performs fresh login with durable saved identity before restarting connection-scoped behavior.
- Runtime startup preserves durable identity fields from `auth_snapshot.json`, but never trusts persisted `logged_in=true` as a live session.
- Metrics reporting remains enabled across transient session loss and republishes the latest known values after fresh login.
- Android foreground service sticky restart restores only when a saved desired run snapshot requests connected, reporting, or notify state.
- Android explicit stop, stop-all, or disconnect clears desired restore state so sticky restart does not undo the user's stop action.

## Acceptance Criteria

- A Windows host can configure at least one topic, start NotifyNode, receive a matching TopicBus publish, and display a user-visible system notification when the runtime environment supports it.
- An Android host can configure at least one topic, start NotifyNode through the foreground service, receive a matching TopicBus publish, and post an Android notification.
- Topic settings are persisted in local runtime config and survive app restart.
- Reconnect or explicit start restores TopicBus subscriptions because server-side TopicBus subscriptions are connection-scoped and in-memory.
- Reconnect or process restart requires fresh login before NotifyNode subscription or metrics reporting recovery.
- Android service recreation without the original intent restores the last desired connected/reporting/notify state when the snapshot is valid.
- Existing MetricsNode reporting behavior continues to work.

