# NotifyNode Spec

## Protocol Dependency

NotifyNode depends on TopicBus `SubProto=4` and does not change wire contracts. It sends `subscribe_batch` with enabled topics and handles inbound `publish` envelopes.

The TopicBus server contract remains:

- topic strings are matched exactly by TopicBus;
- subscriptions are in-memory and connection-scoped;
- publish has no response or acknowledgement;
- no offline backlog or replay is provided.

## Local Configuration

Runtime config key:

- `notify.topics_json`: JSON array of `NotifyTopicSetting`.

```json
[
  { "topic": "dev/codex/task", "enabled": true }
]
```

Rules:

- trim user input before storage;
- reject blank topics;
- collapse duplicate topics;
- preserve exact case;
- only enabled topics are subscribed.

## Runtime API

The runtime exposes:

- `NotifySettingsGet() []notify.TopicSetting`
- `NotifySettingsSet([]notify.TopicSetting) error`
- `StartNotify() error`
- `StopNotify()`
- `IsNotifyRunning() bool`
- `DequeueNotifications() []notify.Event`

Android gomobile mirrors these with JSON string inputs/outputs.

## Runtime Session Recovery

Runtime reconnect is implemented above the SDK transport layer because MetricsNode owns auth, metrics reporting, and NotifyNode subscription state.

Recovery rules:

- `Runtime.Connect(addr)` records reconnect intent only after a successful connect.
- `Runtime.Login(deviceID, nodeID)` records reconnect intent after successful login and refreshes connection-scoped state.
- SDK session errors mark the runtime disconnected and mark auth `LoggedIn=false`.
- Reconnect uses the last successful address and bounded exponential backoff.
- Each reconnect attempt closes the broken client/session before dialing again.
- If durable `device_id` and `node_id` are available, recovery performs fresh `Login` on the new transport session.
- If NotifyNode is running, fresh login sends `subscribe_batch` again.
- If metrics reporting is running, fresh login clears the publish de-dupe cache and republishes latest known metric values from runtime config.
- Explicit `Close` cancels reconnect intent, stops reporting and NotifyNode, marks auth logged out, and closes the client.

Auth snapshot rules:

- `auth_snapshot.json` stores durable identity and recent auth diagnostics.
- `logged_in` is a live session flag, not durable truth.
- Loading a snapshot with `logged_in=true` rewrites it as `false` while preserving identity fields.

## Inbound Handling

`Runtime.onUnmatchedFrame` routes TopicBus frames to a NotifyNode handler after VarStore and Management checks.

The handler accepts TopicBus `publish` payloads carried in command or message frames, decodes `topicbus.PublishReq`, checks the local enabled-topic map, shapes a bounded notification event, and enqueues it into a bounded queue.

Malformed frames are logged and ignored.

## Notification Event

```go
type Event struct {
    ID      string          `json:"id"`
    Topic   string          `json:"topic"`
    Name    string          `json:"name"`
    Title   string          `json:"title"`
    Body    string          `json:"body"`
    TS      int64           `json:"ts"`
    Payload json.RawMessage `json:"payload,omitempty"`
}
```

Queue behavior:

- bounded FIFO queue;
- non-blocking enqueue from SDK receive path;
- oldest event is dropped when full.

## Platform Integration

Windows:

- Wails app exposes notify settings/start/stop/dequeue methods.
- Vue UI polls queued events and invokes the Wails `ShowNotification` bridge.
- `notify.presenter` is stored in Windows bootstrap config and accepts:
  - `script`: default legacy mode; displays a system tray balloon through PowerShell and `System.Windows.Forms.NotifyIcon`.
  - `toast`: Windows Toast mode; registers local app metadata through `go-toast`, writes the embedded app logo to the local config assets directory, and pushes a ToastGeneric notification with that logo as `appLogoOverride`.
- Notification bridge errors are surfaced in UI state while the runtime stays connected.

Android:

- `NodeService` starts NotifyNode separately from metrics reporting.
- A dedicated notification channel posts user message notifications.
- Foreground service status notification remains separate.
- `POST_NOTIFICATIONS` denial leaves runtime connected/subscribed but prevents Android notification display.
- `NodeService` persists a desired run snapshot with address, device id, node id, desired connected state, desired reporting state, and desired notify state.
- `START_STICKY` restart with a null or unknown intent restores from the desired run snapshot instead of relying on original intent extras.
- Restore flow is `init -> connect -> login -> startReporting/startNotify`, skipping reporting/notify if the restored runtime state does not support them.
- Explicit disconnect, stop, and stop-all actions clear the desired run snapshot before stopping foreground work.
