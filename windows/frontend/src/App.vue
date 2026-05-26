<script lang="ts" setup>
// 本文件实现 MetricsNode Windows 前端中的 `App` 组件或页面。
import { computed, onBeforeUnmount, onMounted, reactive, ref } from "vue"
import {
  BootstrapGet,
  BootstrapSet,
  ClearAuth,
  Connect,
  Disconnect,
  EnsureKeys,
  Login,
  MetricsSettingsGet,
  MetricsSettingsSet,
  DequeueNotifications,
  NotifySettingsGet,
  NotifySettingsSet,
  Register,
  StartReporting,
  StartNotify,
  Status,
  StopReporting,
  StopNotify,
  ShowNotification
} from "../wailsjs/go/main/App"

type StatusDTO = {
  work_dir?: string
  connected?: boolean
  addr?: string
  reporting?: boolean
  notify?: boolean
  auth?: {
    device_id?: string
    node_id?: number
    hub_id?: number
    role?: string
    logged_in?: boolean
    last_action?: string
    last_message?: string
    last_unix_time?: number
  }
  metrics?: Record<string, string>
  last_error?: string
}

type MetricSettingDTO = {
  metric: string
  var_name: string
  enabled: boolean
  writable: boolean
}

type NotifyTopicSettingDTO = {
  topic: string
  enabled: boolean
}

type NotificationEventDTO = {
  id?: string
  topic?: string
  name?: string
  title?: string
  body?: string
  ts?: number
  payload?: any
}

const busy = ref(false)
const pollMs = 1000
let pollTimer: number | undefined

const page = ref<"connect" | "settings" | "notify">("connect")

const form = reactive({
  addr: "",
  deviceId: "",
  nodeId: 0
})

const status = reactive<StatusDTO>({
  work_dir: "",
  connected: false,
  addr: "",
  reporting: false,
  notify: false,
  auth: {},
  metrics: {},
  last_error: ""
})

const loggedIn = computed(() => Boolean(status.auth?.logged_in))
const canReport = computed(() => loggedIn.value && Boolean(status.connected))
const canNotify = computed(() => loggedIn.value && Boolean(status.connected))

const settings = ref<MetricSettingDTO[]>([])
const settingsLoaded = ref(false)
const settingsBusy = ref(false)
const settingsSaving = ref(false)
const settingsError = ref("")
let settingsSaveTimer: number | undefined
let settingsSaveRequested = false

const varNameDraft = reactive<Record<string, string>>({})
const varNameError = reactive<Record<string, string>>({})

const controllableMetrics = new Set(["volume_percent", "volume_muted", "brightness_percent", "flashlight_enabled"])
const isControllable = (metric: string) => controllableMetrics.has(String(metric ?? "").trim())

const notifySettings = ref<NotifyTopicSettingDTO[]>([])
const notifyLoaded = ref(false)
const notifyBusy = ref(false)
const notifySaving = ref(false)
const notifyError = ref("")
const notifyTopicDraft = ref("")
const lastNotification = ref<NotificationEventDTO | null>(null)

const isVarNameValid = (name: string) => {
  const trimmed = String(name ?? "").trim()
  if (!trimmed) return false
  return /^[A-Za-z0-9_]+$/.test(trimmed)
}

const metricValue = (s: MetricSettingDTO) => {
  if (!s.enabled) return "-"
  const v = (status.metrics ?? {})[s.metric]
  const text = String(v ?? "").trim()
  return text ? text : "-"
}

const refresh = async () => {
  try {
    const st = (await Status()) as any
    status.work_dir = st?.work_dir ?? ""
    status.connected = Boolean(st?.connected)
    status.addr = st?.addr ?? ""
    status.reporting = Boolean(st?.reporting)
    status.notify = Boolean(st?.notify)
    status.auth = st?.auth ?? {}
    status.metrics = st?.metrics ?? {}
    status.last_error = st?.last_error ?? ""
    if (!form.nodeId && Number(status.auth?.node_id ?? 0) > 0) {
      form.nodeId = Number(status.auth?.node_id ?? 0)
    }
  } catch (err) {
    status.last_error = String(err ?? "status failed")
  }
}

const pollNotifications = async () => {
  try {
    const list = (await DequeueNotifications()) as any
    const events = (Array.isArray(list) ? list : []) as NotificationEventDTO[]
    for (const evt of events) {
      lastNotification.value = evt
      try {
        await ShowNotification(evt as any)
      } catch (err) {
        notifyError.value = String(err ?? "show notification failed")
      }
    }
  } catch (err) {
    notifyError.value = String(err ?? "dequeue notifications failed")
  }
}

const loadBootstrap = async () => {
  try {
    const boot = (await BootstrapGet()) as any
    form.addr = String(boot?.addr ?? "")
    form.deviceId = String(boot?.device_id ?? "")
  } catch (err) {
    status.last_error = String(err ?? "bootstrap load failed")
  }
}

const persistBootstrap = async () => {
  await BootstrapSet({ addr: form.addr, device_id: form.deviceId } as any)
}

const connect = async () => {
  if (busy.value) return
  busy.value = true
  try {
    await persistBootstrap()
    await Connect(form.addr)
  } finally {
    busy.value = false
    await refresh()
  }
}

const disconnect = async () => {
  if (busy.value) return
  busy.value = true
  try {
    await Disconnect()
  } finally {
    busy.value = false
    await refresh()
  }
}

const register = async () => {
  if (busy.value) return
  busy.value = true
  try {
    await persistBootstrap()
    await EnsureKeys()
    await Register(form.deviceId)
  } finally {
    busy.value = false
    await refresh()
  }
}

const login = async () => {
  if (busy.value) return
  busy.value = true
  try {
    await persistBootstrap()
    await EnsureKeys()
    await Login(form.deviceId, Number(form.nodeId || 0))
  } finally {
    busy.value = false
    await refresh()
  }
}

const clearAuth = async () => {
  if (busy.value) return
  busy.value = true
  try {
    await ClearAuth()
    form.nodeId = 0
  } finally {
    busy.value = false
    await refresh()
  }
}

const startReporting = async () => {
  if (busy.value) return
  busy.value = true
  try {
    await StartReporting()
  } finally {
    busy.value = false
    await refresh()
  }
}

const stopReporting = async () => {
  if (busy.value) return
  busy.value = true
  try {
    await StopReporting()
  } finally {
    busy.value = false
    await refresh()
  }
}

const normalizeNotifyTopic = (topic: string) => String(topic ?? "").trim()

const loadNotifySettings = async () => {
  if (notifyBusy.value) return
  notifyBusy.value = true
  notifyError.value = ""
  try {
    const list = (await NotifySettingsGet()) as any
    notifySettings.value = (Array.isArray(list) ? list : []) as any
    notifyLoaded.value = true
  } catch (err) {
    notifyError.value = String(err ?? "load notify settings failed")
  } finally {
    notifyBusy.value = false
  }
}

const validateNotifySettings = (): string | null => {
  const seen = new Set<string>()
  for (const item of notifySettings.value) {
    const topic = normalizeNotifyTopic(item.topic)
    if (!topic) return "topic is required"
    if (topic.length > 256) return `topic too long: ${topic.slice(0, 32)}`
    if (seen.has(topic)) return `duplicate topic: ${topic}`
    seen.add(topic)
  }
  return null
}

const saveNotifySettings = async () => {
  if (notifySaving.value) return
  const err = validateNotifySettings()
  if (err) {
    notifyError.value = err
    return
  }
  notifySaving.value = true
  notifyError.value = ""
  try {
    notifySettings.value = notifySettings.value.map((item) => ({
      topic: normalizeNotifyTopic(item.topic),
      enabled: Boolean(item.enabled)
    }))
    await NotifySettingsSet(notifySettings.value as any)
  } catch (saveErr) {
    notifyError.value = String(saveErr ?? "save notify settings failed")
  } finally {
    notifySaving.value = false
  }
}

const addNotifyTopic = async () => {
  const topic = normalizeNotifyTopic(notifyTopicDraft.value)
  if (!topic) {
    notifyError.value = "topic is required"
    return
  }
  if (notifySettings.value.some((item) => normalizeNotifyTopic(item.topic) === topic)) {
    notifyError.value = `duplicate topic: ${topic}`
    return
  }
  notifySettings.value.push({ topic, enabled: true })
  notifyTopicDraft.value = ""
  await saveNotifySettings()
}

const removeNotifyTopic = async (index: number) => {
  notifySettings.value.splice(index, 1)
  await saveNotifySettings()
}

const setNotifyEnabled = async (item: NotifyTopicSettingDTO, enabled: boolean) => {
  item.enabled = enabled
  await saveNotifySettings()
}

const startNotify = async () => {
  if (busy.value) return
  busy.value = true
  try {
    await saveNotifySettings()
    await StartNotify()
  } finally {
    busy.value = false
    await refresh()
  }
}

const stopNotify = async () => {
  if (busy.value) return
  busy.value = true
  try {
    await StopNotify()
  } finally {
    busy.value = false
    await refresh()
  }
}

const loadSettings = async () => {
  if (settingsBusy.value) return
  settingsBusy.value = true
  settingsError.value = ""
  try {
    const list = (await MetricsSettingsGet()) as any
    settings.value = (Array.isArray(list) ? list : []) as any
    settingsLoaded.value = true
    for (const s of settings.value) {
      varNameDraft[s.metric] = s.var_name ?? ""
      varNameError[s.metric] = ""
    }
  } catch (err) {
    settingsError.value = String(err ?? "load settings failed")
  } finally {
    settingsBusy.value = false
  }
}

const validateSettingsLocal = (): string | null => {
  const seen = new Map<string, string>()
  for (const s of settings.value) {
    const metric = String(s.metric ?? "").trim()
    const varName = String(s.var_name ?? "").trim()
    if (!metric) return "invalid metric in settings"
    if (!isVarNameValid(varName)) return `invalid var_name for ${metric}`
    if (Boolean(s.enabled)) {
      const prev = seen.get(varName)
      if (prev && prev !== metric) return `duplicate enabled var_name: ${varName}`
      seen.set(varName, metric)
    }
  }
  return null
}

const saveSettings = async () => {
  settingsSaveRequested = true
  if (settingsSaving.value) return
  settingsSaving.value = true
  try {
    while (settingsSaveRequested) {
      settingsSaveRequested = false
      const err = validateSettingsLocal()
      if (err) {
        settingsError.value = err
        break
      }
      settingsError.value = ""
      await MetricsSettingsSet(settings.value as any)
    }
  } catch (err) {
    settingsError.value = String(err ?? "save settings failed")
  } finally {
    settingsSaving.value = false
  }
}

const scheduleSaveSettings = () => {
  if (settingsSaveTimer) {
    window.clearTimeout(settingsSaveTimer)
  }
  settingsSaveTimer = window.setTimeout(() => void saveSettings(), 400)
}

const setMetricEnabled = (s: MetricSettingDTO, enabled: boolean) => {
  s.enabled = enabled
  void saveSettings()
}

const setMetricWritable = (s: MetricSettingDTO, writable: boolean) => {
  s.writable = writable
  void saveSettings()
}

const onVarNameInput = (s: MetricSettingDTO) => {
  const metric = String(s.metric ?? "").trim()
  const draft = String(varNameDraft[metric] ?? "")
  const trimmed = draft.trim()
  if (!trimmed) {
    varNameError[metric] = "var_name is required"
    return
  }
  if (!isVarNameValid(trimmed)) {
    varNameError[metric] = "only A-Z a-z 0-9 _"
    return
  }
  varNameError[metric] = ""
  s.var_name = trimmed
  scheduleSaveSettings()
}

const commitVarName = (s: MetricSettingDTO) => {
  const metric = String(s.metric ?? "").trim()
  const draft = String(varNameDraft[metric] ?? "")
  const trimmed = draft.trim()
  if (!isVarNameValid(trimmed)) {
    varNameError[metric] = "only A-Z a-z 0-9 _"
    return
  }
  varNameError[metric] = ""
  s.var_name = trimmed
  if (settingsSaveTimer) {
    window.clearTimeout(settingsSaveTimer)
    settingsSaveTimer = undefined
  }
  void saveSettings()
}

const switchPage = async (p: "connect" | "settings" | "notify") => {
  page.value = p
  if (p === "settings" && !settingsLoaded.value) {
    await loadSettings()
  }
  if (p === "notify" && !notifyLoaded.value) {
    await loadNotifySettings()
  }
}

onMounted(async () => {
  await loadBootstrap()
  await refresh()
  pollTimer = window.setInterval(() => {
    void refresh()
    void pollNotifications()
  }, pollMs)
})

onBeforeUnmount(() => {
  if (pollTimer) {
    window.clearInterval(pollTimer)
  }
  if (settingsSaveTimer) {
    window.clearTimeout(settingsSaveTimer)
  }
})
</script>

<template>
  <main class="page">
    <header class="header">
      <div>
        <h1>MyFlowHub MetricsNode</h1>
        <p class="sub">Windows client (Wails). WorkDir: <code>{{ status.work_dir || "-" }}</code></p>
      </div>
      <div class="header-right">
        <div class="pill" :class="status.connected ? 'ok' : 'bad'">
          {{ status.connected ? "Connected" : "Disconnected" }}
        </div>
        <div class="pill" :class="status.reporting ? 'ok' : 'bad'">
          {{ status.reporting ? "Reporting" : "Stopped" }}
        </div>
        <div class="pill" :class="status.notify ? 'ok' : 'bad'">
          {{ status.notify ? "Notify" : "Notify Off" }}
        </div>
      </div>
    </header>

    <nav class="tabs">
      <button class="tab" :class="page === 'connect' ? 'active' : ''" @click="switchPage('connect')">
        Connect
      </button>
      <button class="tab" :class="page === 'settings' ? 'active' : ''" @click="switchPage('settings')">
        Settings
      </button>
      <button class="tab" :class="page === 'notify' ? 'active' : ''" @click="switchPage('notify')">
        Notify
      </button>
      <div class="spacer"></div>
      <button class="btn secondary" :disabled="busy" @click="refresh">Refresh</button>
    </nav>

    <template v-if="page === 'connect'">
      <section class="card">
        <h2>Bootstrap</h2>
        <div class="grid">
          <label>
            Hub Addr
            <input v-model="form.addr" class="input" placeholder="127.0.0.1:9000" />
          </label>
          <label>
            Device ID
            <input v-model="form.deviceId" class="input" placeholder="device-001" />
          </label>
        </div>

        <div class="row">
          <button class="btn" :disabled="busy" @click="connect">Connect</button>
          <button class="btn secondary" :disabled="busy" @click="disconnect">Disconnect</button>
        </div>
      </section>

      <section class="card">
        <h2>Auth</h2>
        <div class="grid auth-grid">
          <label>
            Node ID (for login)
            <input v-model.number="form.nodeId" class="input" type="number" min="0" />
          </label>
        </div>

        <div class="row">
          <button class="btn" :disabled="busy || !status.connected" @click="register">Register</button>
          <button class="btn" :disabled="busy || !status.connected" @click="login">Login</button>
          <button class="btn secondary" :disabled="busy" @click="clearAuth">Clear Auth</button>
        </div>

        <div class="kv">
          <div><span>Logged In</span><b>{{ loggedIn ? "Yes" : "No" }}</b></div>
          <div><span>Node ID</span><b>{{ status.auth?.node_id ?? "-" }}</b></div>
          <div><span>Hub ID</span><b>{{ status.auth?.hub_id ?? "-" }}</b></div>
          <div><span>Role</span><b>{{ status.auth?.role ?? "-" }}</b></div>
          <div><span>Last</span><b>{{ status.auth?.last_action ?? "-" }}</b></div>
          <div class="wide"><span>Msg</span><b>{{ status.auth?.last_message ?? "-" }}</b></div>
        </div>
      </section>

      <section class="card">
        <h2>Reporting</h2>
        <div class="row">
          <button class="btn" :disabled="busy || !canReport || status.reporting" @click="startReporting">
            Start Reporting
          </button>
          <button class="btn secondary" :disabled="busy || !status.reporting" @click="stopReporting">Stop</button>
        </div>
      </section>
    </template>

    <section v-if="page === 'settings'" class="card">
      <div class="card-header">
        <h2>Metrics Settings</h2>
        <div class="row tight">
          <button class="btn secondary" :disabled="settingsBusy" @click="loadSettings">Reload</button>
          <div class="pill" :class="settingsSaving ? 'warn' : 'ok'">
            {{ settingsSaving ? "Saving..." : "Ready" }}
          </div>
        </div>
      </div>

      <div class="table">
        <div class="thead">
          <div>Metric</div>
          <div>Var Name</div>
          <div>Value</div>
          <div class="col-toggle">Enabled</div>
          <div class="col-toggle">Writable</div>
        </div>
        <div v-for="s in settings" :key="s.metric" class="tr">
          <div class="metric">{{ s.metric }}</div>
          <div class="varcol">
            <input
              class="input"
              :class="varNameError[s.metric] ? 'invalid' : ''"
              v-model="varNameDraft[s.metric]"
              :disabled="settingsSaving"
              @input="onVarNameInput(s)"
              @blur="commitVarName(s)"
              @keydown.enter.prevent="commitVarName(s)"
            />
            <div v-if="varNameError[s.metric]" class="hint">{{ varNameError[s.metric] }}</div>
          </div>
          <div><code>{{ metricValue(s) }}</code></div>
          <div class="togglecell">
            <label class="toggle">
              <input
                type="checkbox"
                :checked="s.enabled"
                :disabled="settingsSaving"
                @change="setMetricEnabled(s, ($event.target as HTMLInputElement).checked)"
              />
              <span class="track"></span>
            </label>
          </div>
          <div class="togglecell">
            <template v-if="isControllable(s.metric)">
              <label class="toggle">
                <input
                  type="checkbox"
                  :checked="s.writable"
                  :disabled="settingsSaving"
                  @change="setMetricWritable(s, ($event.target as HTMLInputElement).checked)"
                />
                <span class="track"></span>
              </label>
            </template>
            <template v-else>-</template>
          </div>
        </div>
      </div>
    </section>

    <section v-if="page === 'notify'" class="card">
      <div class="card-header">
        <h2>Notify Topics</h2>
        <div class="row tight">
          <button class="btn secondary" :disabled="notifyBusy" @click="loadNotifySettings">Reload</button>
          <div class="pill" :class="notifySaving ? 'warn' : status.notify ? 'ok' : 'bad'">
            {{ notifySaving ? "Saving..." : status.notify ? "Listening" : "Stopped" }}
          </div>
        </div>
      </div>

      <div class="grid notify-add">
        <label>
          Topic
          <input
            v-model="notifyTopicDraft"
            class="input"
            placeholder="dev/codex/task"
            @keydown.enter.prevent="addNotifyTopic"
          />
        </label>
      </div>

      <div class="row">
        <button class="btn" :disabled="notifySaving" @click="addNotifyTopic">Add Topic</button>
        <button class="btn" :disabled="busy || !canNotify || status.notify || notifySettings.length === 0" @click="startNotify">
          Start Listening
        </button>
        <button class="btn secondary" :disabled="busy || !status.notify" @click="stopNotify">Stop Notify</button>
      </div>

      <div class="table notify-table">
        <div class="thead">
          <div>Topic</div>
          <div class="col-toggle">Enabled</div>
          <div class="col-toggle">Action</div>
        </div>
        <div v-for="(item, index) in notifySettings" :key="item.topic" class="tr">
          <div class="metric">{{ item.topic }}</div>
          <div class="togglecell">
            <label class="toggle">
              <input
                type="checkbox"
                :checked="item.enabled"
                :disabled="notifySaving"
                @change="setNotifyEnabled(item, ($event.target as HTMLInputElement).checked)"
              />
              <span class="track"></span>
            </label>
          </div>
          <div class="togglecell">
            <button class="btn secondary" :disabled="notifySaving" @click="removeNotifyTopic(index)">Remove</button>
          </div>
        </div>
      </div>

      <div v-if="lastNotification" class="kv notify-last">
        <div><span>Last Topic</span><b>{{ lastNotification.topic || "-" }}</b></div>
        <div><span>Last Name</span><b>{{ lastNotification.name || "-" }}</b></div>
        <div class="wide"><span>Title</span><b>{{ lastNotification.title || "-" }}</b></div>
        <div class="wide"><span>Body</span><b>{{ lastNotification.body || "-" }}</b></div>
      </div>
    </section>

    <section v-if="settingsError" class="card warn">
      <h2>Settings Error</h2>
      <pre class="pre">{{ settingsError }}</pre>
    </section>

    <section v-if="notifyError" class="card warn">
      <h2>Notify Error</h2>
      <pre class="pre">{{ notifyError }}</pre>
    </section>

    <section v-if="status.last_error" class="card warn">
      <h2>Last Error</h2>
      <pre class="pre">{{ status.last_error }}</pre>
    </section>
  </main>
</template>

<style scoped>
.page {
  max-width: 980px;
  margin: 0 auto;
  padding: 24px;
  text-align: left;
}
.header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 16px;
}
.header-right {
  display: flex;
  gap: 10px;
  align-items: center;
}
h1 {
  margin: 0;
  font-size: 22px;
}
.sub {
  margin: 6px 0 0;
  opacity: 0.8;
  font-size: 12px;
}
.card {
  background: rgba(255, 255, 255, 0.06);
  border: 1px solid rgba(255, 255, 255, 0.12);
  border-radius: 12px;
  padding: 16px;
  margin: 12px 0;
}
.warn {
  border-color: rgba(255, 180, 0, 0.6);
}
h2 {
  margin: 0 0 12px;
  font-size: 14px;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  opacity: 0.9;
}
.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
}
.card-header h2 {
  margin: 0;
}
.tabs {
  display: flex;
  align-items: center;
  gap: 10px;
  margin: 12px 0;
}
.tab {
  height: 34px;
  padding: 0 12px;
  border-radius: 8px;
  border: 1px solid rgba(255, 255, 255, 0.18);
  background: rgba(255, 255, 255, 0.06);
  color: white;
  cursor: pointer;
}
.tab.active {
  background: rgba(255, 255, 255, 0.14);
}
.spacer {
  flex: 1;
}
.grid {
  display: grid;
  gap: 12px;
  grid-template-columns: repeat(3, minmax(0, 1fr));
}
.grid.auth-grid {
  grid-template-columns: minmax(0, 360px);
}
label {
  display: grid;
  gap: 6px;
  font-size: 12px;
  opacity: 0.9;
}
.input {
  height: 34px;
  border-radius: 8px;
  border: 1px solid rgba(255, 255, 255, 0.18);
  background: rgba(0, 0, 0, 0.2);
  color: white;
  padding: 0 10px;
  outline: none;
}
.row {
  display: flex;
  gap: 10px;
  align-items: center;
  margin-top: 12px;
  flex-wrap: wrap;
}
.row.tight {
  margin-top: 0;
}
.between {
  justify-content: space-between;
}
.btn {
  height: 34px;
  padding: 0 12px;
  border-radius: 8px;
  border: 1px solid rgba(255, 255, 255, 0.18);
  background: rgba(255, 255, 255, 0.14);
  color: white;
  cursor: pointer;
}
.btn.secondary {
  background: rgba(255, 255, 255, 0.06);
}
.btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
.pill {
  display: inline-flex;
  align-items: center;
  flex-shrink: 0;
  padding: 6px 10px;
  border-radius: 999px;
  font-size: 12px;
  white-space: nowrap;
  border: 1px solid rgba(255, 255, 255, 0.18);
}
.pill.ok {
  background: rgba(16, 185, 129, 0.18);
}
.pill.bad {
  background: rgba(239, 68, 68, 0.18);
}
.pill.warn {
  background: rgba(255, 180, 0, 0.18);
}
.kv {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
  margin-top: 12px;
}
.kv > div {
  display: flex;
  justify-content: space-between;
  gap: 10px;
  font-size: 12px;
  opacity: 0.9;
}
.kv .wide {
  grid-column: 1 / -1;
}
.pre {
  margin: 12px 0 0;
  padding: 12px;
  border-radius: 10px;
  background: rgba(0, 0, 0, 0.25);
  border: 1px solid rgba(255, 255, 255, 0.12);
  overflow: auto;
  font-size: 12px;
}
.table {
  display: grid;
  gap: 10px;
}
.thead,
.tr {
  display: grid;
  grid-template-columns: 220px 1fr 120px 90px 90px;
  gap: 10px;
  align-items: start;
}
.notify-table .thead,
.notify-table .tr {
  grid-template-columns: 1fr 120px 120px;
}
.notify-add {
  grid-template-columns: minmax(0, 1fr);
}
.notify-last {
  margin-top: 16px;
}
.thead {
  font-size: 12px;
  opacity: 0.8;
  padding-bottom: 8px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.12);
}
.tr {
  padding: 8px 0;
  border-bottom: 1px solid rgba(255, 255, 255, 0.06);
}
.metric {
  font-size: 12px;
  opacity: 0.95;
  word-break: break-word;
}
.col-toggle,
.togglecell {
  display: flex;
  justify-content: flex-end;
}
.varcol {
  display: grid;
  gap: 6px;
}
.input.invalid {
  border-color: rgba(239, 68, 68, 0.6);
}
.hint {
  font-size: 11px;
  opacity: 0.9;
  color: rgba(255, 180, 0, 0.95);
}
code {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New",
    monospace;
}
.toggle {
  --toggle-w: 38px;
  --toggle-h: 20px;
  display: inline-flex;
  align-items: center;
  cursor: pointer;
}
.toggle input {
  position: absolute;
  opacity: 0;
  width: 0;
  height: 0;
}
.toggle .track {
  width: var(--toggle-w);
  height: var(--toggle-h);
  border-radius: var(--toggle-h);
  border: 1px solid rgba(255, 255, 255, 0.18);
  background: rgba(255, 255, 255, 0.08);
  position: relative;
  transition:
    background 120ms ease,
    border-color 120ms ease;
}
.toggle .track::after {
  content: "";
  position: absolute;
  top: 2px;
  left: 2px;
  width: calc(var(--toggle-h) - 4px);
  height: calc(var(--toggle-h) - 4px);
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.92);
  transition: transform 120ms ease;
}
.toggle input:checked + .track {
  background: rgba(16, 185, 129, 0.35);
  border-color: rgba(16, 185, 129, 0.55);
}
.toggle input:checked + .track::after {
  transform: translateX(calc(var(--toggle-w) - var(--toggle-h)));
}
.toggle input:disabled + .track {
  opacity: 0.55;
  cursor: not-allowed;
}
.toggle input:focus-visible + .track {
  outline: 2px solid rgba(255, 255, 255, 0.35);
  outline-offset: 2px;
}
@media (max-width: 860px) {
  .grid {
    grid-template-columns: 1fr;
  }
  .thead,
  .tr {
    grid-template-columns: 1fr;
  }
}
</style>
