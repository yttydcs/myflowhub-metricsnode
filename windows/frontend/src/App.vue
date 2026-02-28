<script lang="ts" setup>
import { computed, onBeforeUnmount, onMounted, reactive, ref } from "vue"
import {
  BootstrapGet,
  BootstrapSet,
  ClearAuth,
  Connect,
  Disconnect,
  EnsureKeys,
  Login,
  Register,
  StartReporting,
  Status,
  StopReporting
} from "../wailsjs/go/main/App"

type StatusDTO = {
  work_dir?: string
  connected?: boolean
  addr?: string
  reporting?: boolean
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

const busy = ref(false)
const pollMs = 1000
let pollTimer: number | undefined

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
  auth: {},
  metrics: {},
  last_error: ""
})

const loggedIn = computed(() => Boolean(status.auth?.logged_in))
const canReport = computed(() => loggedIn.value && Boolean(status.connected))

const refresh = async () => {
  try {
    const st = (await Status()) as any
    status.work_dir = st?.work_dir ?? ""
    status.connected = Boolean(st?.connected)
    status.addr = st?.addr ?? ""
    status.reporting = Boolean(st?.reporting)
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

onMounted(async () => {
  await loadBootstrap()
  await refresh()
  pollTimer = window.setInterval(() => void refresh(), pollMs)
})

onBeforeUnmount(() => {
  if (pollTimer) {
    window.clearInterval(pollTimer)
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
      <div class="pill" :class="status.connected ? 'ok' : 'bad'">
        {{ status.connected ? "Connected" : "Disconnected" }}
      </div>
    </header>

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
        <label>
          Node ID (for login)
          <input v-model.number="form.nodeId" class="input" type="number" min="0" />
        </label>
      </div>
      <div class="row">
        <button class="btn" :disabled="busy" @click="connect">Connect</button>
        <button class="btn secondary" :disabled="busy" @click="disconnect">Disconnect</button>
      </div>
    </section>

    <section class="card">
      <h2>Auth</h2>
      <div class="row">
        <button class="btn" :disabled="busy || !status.connected" @click="register">Register</button>
        <button class="btn" :disabled="busy || !status.connected" @click="login">Login</button>
        <button class="btn secondary" :disabled="busy" @click="clearAuth">Clear</button>
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
      <h2>Metrics</h2>
      <div class="row">
        <button class="btn" :disabled="busy || !canReport || status.reporting" @click="startReporting">
          Start Reporting
        </button>
        <button class="btn secondary" :disabled="busy || !status.reporting" @click="stopReporting">
          Stop Reporting
        </button>
        <div class="pill" :class="status.reporting ? 'ok' : 'bad'">
          {{ status.reporting ? "Reporting" : "Stopped" }}
        </div>
      </div>
      <pre class="pre">{{ status.metrics }}</pre>
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
.grid {
  display: grid;
  gap: 12px;
  grid-template-columns: repeat(3, minmax(0, 1fr));
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
  padding: 6px 10px;
  border-radius: 999px;
  font-size: 12px;
  border: 1px solid rgba(255, 255, 255, 0.18);
}
.pill.ok {
  background: rgba(16, 185, 129, 0.18);
}
.pill.bad {
  background: rgba(239, 68, 68, 0.18);
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
code {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New",
    monospace;
}
@media (max-width: 860px) {
  .grid {
    grid-template-columns: 1fr;
  }
}
</style>
