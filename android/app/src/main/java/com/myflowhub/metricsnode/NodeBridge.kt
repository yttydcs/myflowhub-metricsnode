package com.myflowhub.metricsnode
// 本文件实现 MetricsNode Android 宿主中与 `NodeBridge` 相关的逻辑。

import org.json.JSONArray
import java.lang.reflect.Method

data class NodeConfig(
    val addr: String,
    val deviceId: String,
    val workDir: String,
)

data class NodeAction(
    val metric: String,
    val value: String,
)

data class MetricSetting(
    val metric: String,
    val varName: String,
    val enabled: Boolean,
    val writable: Boolean,
)

data class NotifyTopicSetting(
    val topic: String,
    val enabled: Boolean,
)

data class NotifyEvent(
    val id: String,
    val topic: String,
    val name: String,
    val title: String,
    val body: String,
    val ts: Long,
)

internal object NodeActionJson {
    fun parseList(raw: String): List<NodeAction> {
        return try {
            val arr = JSONArray(raw)
            val out = ArrayList<NodeAction>(arr.length())
            for (i in 0 until arr.length()) {
                val obj = arr.optJSONObject(i) ?: continue
                val metric = obj.optString("metric", "").trim()
                val value = obj.optString("value", "").trim()
                if (metric.isNotEmpty() && value.isNotEmpty()) {
                    out.add(NodeAction(metric = metric, value = value))
                }
            }
            out
        } catch (_: Throwable) {
            emptyList()
        }
    }
}

internal object MetricSettingJson {
    fun parseList(raw: String): List<MetricSetting> {
        return try {
            val arr = JSONArray(raw)
            val out = ArrayList<MetricSetting>(arr.length())
            for (i in 0 until arr.length()) {
                val obj = arr.optJSONObject(i) ?: continue
                val metric = obj.optString("metric", "").trim()
                val varName = obj.optString("var_name", "").trim()
                if (metric.isEmpty() || varName.isEmpty()) continue
                out.add(
                    MetricSetting(
                        metric = metric,
                        varName = varName,
                        enabled = obj.optBoolean("enabled", false),
                        writable = obj.optBoolean("writable", false),
                    )
                )
            }
            out
        } catch (_: Throwable) {
            emptyList()
        }
    }

    fun toJson(settings: List<MetricSetting>): String {
        return try {
            val arr = JSONArray()
            for (s in settings) {
                val metric = s.metric.trim()
                val varName = s.varName.trim()
                if (metric.isEmpty() || varName.isEmpty()) continue
                val obj = org.json.JSONObject()
                obj.put("metric", metric)
                obj.put("var_name", varName)
                obj.put("enabled", s.enabled)
                obj.put("writable", s.writable)
                arr.put(obj)
            }
            arr.toString()
        } catch (_: Throwable) {
            "[]"
        }
    }
}

internal object NotifyTopicSettingJson {
    fun parseList(raw: String): List<NotifyTopicSetting> {
        return try {
            val arr = JSONArray(raw)
            val out = ArrayList<NotifyTopicSetting>(arr.length())
            for (i in 0 until arr.length()) {
                val obj = arr.optJSONObject(i) ?: continue
                val topic = obj.optString("topic", "").trim()
                if (topic.isEmpty()) continue
                out.add(NotifyTopicSetting(topic = topic, enabled = obj.optBoolean("enabled", false)))
            }
            out
        } catch (_: Throwable) {
            emptyList()
        }
    }

    fun toJson(settings: List<NotifyTopicSetting>): String {
        return try {
            val arr = JSONArray()
            for (s in settings) {
                val topic = s.topic.trim()
                if (topic.isEmpty()) continue
                val obj = org.json.JSONObject()
                obj.put("topic", topic)
                obj.put("enabled", s.enabled)
                arr.put(obj)
            }
            arr.toString()
        } catch (_: Throwable) {
            "[]"
        }
    }
}

internal object NotifyEventJson {
    fun parseList(raw: String): List<NotifyEvent> {
        return try {
            val arr = JSONArray(raw)
            val out = ArrayList<NotifyEvent>(arr.length())
            for (i in 0 until arr.length()) {
                val obj = arr.optJSONObject(i) ?: continue
                val topic = obj.optString("topic", "").trim()
                val title = obj.optString("title", "").trim()
                val body = obj.optString("body", "").trim()
                if (topic.isEmpty() || (title.isEmpty() && body.isEmpty())) continue
                out.add(
                    NotifyEvent(
                        id = obj.optString("id", "").trim(),
                        topic = topic,
                        name = obj.optString("name", "").trim(),
                        title = title,
                        body = body,
                        ts = obj.optLong("ts", 0L),
                    )
                )
            }
            out
        } catch (_: Throwable) {
            emptyList()
        }
    }
}

interface NodeBridge {
    fun init(workDir: String): NodeState
    fun connect(addr: String): NodeState
    fun disconnect(): NodeState
    fun register(deviceId: String): NodeState
    fun login(deviceId: String, nodeId: Long): NodeState
    fun startReporting(): NodeState
    fun stopReporting(): NodeState
    fun stopAll(): NodeState

    fun start(config: NodeConfig): NodeState
    fun stop(): NodeState
    fun status(): NodeState

    fun metricsSettingsGet(): String
    fun metricsSettingsSet(raw: String): NodeState
    fun notifySettingsGet(): String
    fun notifySettingsSet(raw: String): NodeState
    fun startNotify(): NodeState
    fun stopNotify(): NodeState

    fun updateBatteryPercent(percent: Int)
    fun updateBatteryCharging(charging: Int)
    fun updateBatteryOnAC(onAC: Int)
    fun updateVolume(volumePercent: Int, muted: Boolean)
    fun updateBrightnessPercent(percent: Int)
    fun updateNetOnline(online: Int)
    fun updateNetType(netType: String)
    fun updateCPUPercent(percent: Int)
    fun updateMemPercent(percent: Int)
    fun updateFlashlightEnabled(enabled: Int)

    fun dequeueActions(): List<NodeAction>
    fun dequeueNotifications(): List<NotifyEvent>
}

class StubNodeBridge(
    initError: String = "",
) : NodeBridge {
    private val err = initError.trim().ifBlank { "gomobile bridge unavailable" }
    private var state = NodeState(running = false, lastError = err)

    override fun init(workDir: String): NodeState {
        state = state.copy(workDir = workDir.trim())
        return state
    }

    override fun connect(addr: String): NodeState {
        state = state.copy(addr = addr.trim())
        return state
    }

    override fun disconnect(): NodeState = state

    override fun register(deviceId: String): NodeState = state

    override fun login(deviceId: String, nodeId: Long): NodeState = state

    override fun startReporting(): NodeState = state

    override fun stopReporting(): NodeState = state

    override fun stopAll(): NodeState = state

    override fun start(config: NodeConfig): NodeState {
        state = state.copy(addr = config.addr.trim(), workDir = config.workDir.trim())
        return state
    }

    override fun stop(): NodeState = state

    override fun status(): NodeState = state

    override fun metricsSettingsGet(): String = "[]"

    override fun metricsSettingsSet(raw: String): NodeState = state

    override fun notifySettingsGet(): String = "[]"

    override fun notifySettingsSet(raw: String): NodeState = state

    override fun startNotify(): NodeState = state

    override fun stopNotify(): NodeState = state

    override fun updateBatteryPercent(percent: Int) {}

    override fun updateBatteryCharging(charging: Int) {}

    override fun updateBatteryOnAC(onAC: Int) {}

    override fun updateVolume(volumePercent: Int, muted: Boolean) {}

    override fun updateBrightnessPercent(percent: Int) {}

    override fun updateNetOnline(online: Int) {}

    override fun updateNetType(netType: String) {}

    override fun updateCPUPercent(percent: Int) {}

    override fun updateMemPercent(percent: Int) {}

    override fun updateFlashlightEnabled(enabled: Int) {}

    override fun dequeueActions(): List<NodeAction> = emptyList()

    override fun dequeueNotifications(): List<NotifyEvent> = emptyList()
}

class GoNodeBridge : NodeBridge {
    private val cls: Class<*>

    private val initMethod: Method
    private val connectMethod: Method
    private val disconnectMethod: Method
    private val registerMethod: Method
    private val loginMethod: Method
    private val startReportingMethod: Method
    private val stopReportingMethod: Method
    private val stopAllMethod: Method

    private val startMethod: Method
    private val stopMethod: Method
    private val statusMethod: Method
    private val metricsSettingsGetMethod: Method
    private val metricsSettingsSetMethod: Method
    private val notifySettingsGetMethod: Method?
    private val notifySettingsSetMethod: Method?
    private val startNotifyMethod: Method?
    private val stopNotifyMethod: Method?

    private val updateBatteryMethod: Method
    private val updateBatteryChargingMethod: Method?
    private val updateBatteryOnACMethod: Method?
    private val updateVolumePercentMethod: Method
    private val updateVolumeMutedMethod: Method
    private val updateBrightnessMethod: Method?
    private val updateNetOnlineMethod: Method?
    private val updateNetTypeMethod: Method?
    private val updateCPUPercentMethod: Method?
    private val updateMemPercentMethod: Method?
    private val updateFlashlightEnabledMethod: Method?
    private val dequeueActionsMethod: Method
    private val dequeueNotificationsMethod: Method?

    init {
        cls = GomobileLoader.loadNodeClass()
        initMethod = GoReflect.method(cls, "Init", String::class.java)
        connectMethod = GoReflect.method(cls, "Connect", String::class.java)
        disconnectMethod = GoReflect.method(cls, "Disconnect")
        registerMethod = GoReflect.method(cls, "Register", String::class.java)
        loginMethod = GoReflect.method(cls, "Login", String::class.java, java.lang.Long.TYPE)
        startReportingMethod = GoReflect.method(cls, "StartReporting")
        stopReportingMethod = GoReflect.method(cls, "StopReporting")
        stopAllMethod = GoReflect.method(cls, "StopAll")

        startMethod = GoReflect.method(cls, "Start", String::class.java, String::class.java, String::class.java)
        stopMethod = GoReflect.method(cls, "Stop")
        statusMethod = GoReflect.method(cls, "Status")
        metricsSettingsGetMethod = GoReflect.method(cls, "MetricsSettingsGet")
        metricsSettingsSetMethod = GoReflect.method(cls, "MetricsSettingsSet", String::class.java)
        notifySettingsGetMethod = runCatching { GoReflect.method(cls, "NotifySettingsGet") }.getOrNull()
        notifySettingsSetMethod = runCatching { GoReflect.method(cls, "NotifySettingsSet", String::class.java) }.getOrNull()
        startNotifyMethod = runCatching { GoReflect.method(cls, "StartNotify") }.getOrNull()
        stopNotifyMethod = runCatching { GoReflect.method(cls, "StopNotify") }.getOrNull()

        updateBatteryMethod = GoReflect.method(cls, "UpdateBatteryPercent", String::class.java)
        updateBatteryChargingMethod = runCatching { GoReflect.method(cls, "UpdateBatteryCharging", String::class.java) }.getOrNull()
        updateBatteryOnACMethod = runCatching { GoReflect.method(cls, "UpdateBatteryOnAC", String::class.java) }.getOrNull()
        updateVolumePercentMethod = GoReflect.method(cls, "UpdateVolumePercent", String::class.java)
        updateVolumeMutedMethod = GoReflect.method(cls, "UpdateVolumeMuted", String::class.java)
        updateBrightnessMethod = runCatching { GoReflect.method(cls, "UpdateBrightnessPercent", String::class.java) }.getOrNull()
        updateNetOnlineMethod = runCatching { GoReflect.method(cls, "UpdateNetOnline", String::class.java) }.getOrNull()
        updateNetTypeMethod = runCatching { GoReflect.method(cls, "UpdateNetType", String::class.java) }.getOrNull()
        updateCPUPercentMethod = runCatching { GoReflect.method(cls, "UpdateCPUPercent", String::class.java) }.getOrNull()
        updateMemPercentMethod = runCatching { GoReflect.method(cls, "UpdateMemPercent", String::class.java) }.getOrNull()
        updateFlashlightEnabledMethod = runCatching { GoReflect.method(cls, "UpdateFlashlightEnabled", String::class.java) }.getOrNull()
        dequeueActionsMethod = GoReflect.method(cls, "DequeueActions")
        dequeueNotificationsMethod = runCatching { GoReflect.method(cls, "DequeueNotifications") }.getOrNull()

        // Optional probe to help diagnose missing AAR in runtime.
        runCatching { GoReflect.method(cls, "EnsureLinked").invoke(null) }
    }

    override fun init(workDir: String): NodeState =
        call { initMethod.invoke(null, workDir.trim()) as String }

    override fun connect(addr: String): NodeState =
        call { connectMethod.invoke(null, addr.trim()) as String }

    override fun disconnect(): NodeState =
        call { disconnectMethod.invoke(null) as String }

    override fun register(deviceId: String): NodeState =
        call { registerMethod.invoke(null, deviceId.trim()) as String }

    override fun login(deviceId: String, nodeId: Long): NodeState =
        call { loginMethod.invoke(null, deviceId.trim(), nodeId) as String }

    override fun startReporting(): NodeState =
        call { startReportingMethod.invoke(null) as String }

    override fun stopReporting(): NodeState =
        call { stopReportingMethod.invoke(null) as String }

    override fun stopAll(): NodeState =
        call { stopAllMethod.invoke(null) as String }

    override fun start(config: NodeConfig): NodeState =
        call { startMethod.invoke(null, config.addr, config.deviceId, config.workDir) as String }

    override fun stop(): NodeState =
        call { stopMethod.invoke(null) as String }

    override fun status(): NodeState =
        call { statusMethod.invoke(null) as String }

    override fun metricsSettingsGet(): String =
        callRaw { metricsSettingsGetMethod.invoke(null) as String }

    override fun metricsSettingsSet(raw: String): NodeState =
        call { metricsSettingsSetMethod.invoke(null, raw) as String }

    override fun notifySettingsGet(): String {
        val method = notifySettingsGetMethod ?: return "[]"
        return callRaw { method.invoke(null) as String }
    }

    override fun notifySettingsSet(raw: String): NodeState {
        val method = notifySettingsSetMethod ?: return NodeState(running = true, lastError = "notify settings bridge unavailable")
        return call { method.invoke(null, raw) as String }
    }

    override fun startNotify(): NodeState {
        val method = startNotifyMethod ?: return NodeState(running = true, lastError = "notify bridge unavailable")
        return call { method.invoke(null) as String }
    }

    override fun stopNotify(): NodeState {
        val method = stopNotifyMethod ?: return status()
        return call { method.invoke(null) as String }
    }

    override fun updateBatteryPercent(percent: Int) {
        val text = if (percent < 0) "-1" else percent.toString()
        runCatching { updateBatteryMethod.invoke(null, text) }
    }

    override fun updateBatteryCharging(charging: Int) {
        val method = updateBatteryChargingMethod ?: return
        val text = boolishText(charging)
        runCatching { method.invoke(null, text) }
    }

    override fun updateBatteryOnAC(onAC: Int) {
        val method = updateBatteryOnACMethod ?: return
        val text = boolishText(onAC)
        runCatching { method.invoke(null, text) }
    }

    override fun updateVolume(volumePercent: Int, muted: Boolean) {
        val percent = volumePercent.coerceIn(0, 100).toString()
        val mutedText = if (muted) "1" else "0"
        runCatching { updateVolumePercentMethod.invoke(null, percent) }
        runCatching { updateVolumeMutedMethod.invoke(null, mutedText) }
    }

    override fun updateBrightnessPercent(percent: Int) {
        val text = if (percent < 0) "-1" else percent.coerceIn(0, 100).toString()
        val method = updateBrightnessMethod ?: return
        runCatching { method.invoke(null, text) }
    }

    override fun updateNetOnline(online: Int) {
        val method = updateNetOnlineMethod ?: return
        val text = boolishText(online)
        runCatching { method.invoke(null, text) }
    }

    override fun updateNetType(netType: String) {
        val method = updateNetTypeMethod ?: return
        runCatching { method.invoke(null, netType.trim()) }
    }

    override fun updateCPUPercent(percent: Int) {
        val method = updateCPUPercentMethod ?: return
        val text = if (percent < 0) "-1" else percent.coerceIn(0, 100).toString()
        runCatching { method.invoke(null, text) }
    }

    override fun updateMemPercent(percent: Int) {
        val method = updateMemPercentMethod ?: return
        val text = if (percent < 0) "-1" else percent.coerceIn(0, 100).toString()
        runCatching { method.invoke(null, text) }
    }

    override fun updateFlashlightEnabled(enabled: Int) {
        val method = updateFlashlightEnabledMethod ?: return
        val text = boolishText(enabled)
        runCatching { method.invoke(null, text) }
    }

    override fun dequeueActions(): List<NodeAction> {
        val raw = runCatching { dequeueActionsMethod.invoke(null) as String }.getOrNull() ?: return emptyList()
        return NodeActionJson.parseList(raw)
    }

    override fun dequeueNotifications(): List<NotifyEvent> {
        val method = dequeueNotificationsMethod ?: return emptyList()
        val raw = runCatching { method.invoke(null) as String }.getOrNull() ?: return emptyList()
        return NotifyEventJson.parseList(raw)
    }

    private fun call(fn: () -> String): NodeState {
        return try {
            NodeStateJson.parse(fn())
        } catch (t: Throwable) {
            NodeState(running = false, lastError = t.message ?: t.toString())
        }
    }

    private fun callRaw(fn: () -> String): String {
        return try {
            fn()
        } catch (_: Throwable) {
            "[]"
        }
    }

    private fun boolishText(n: Int): String {
        return when {
            n < 0 -> "-1"
            n == 0 -> "0"
            else -> "1"
        }
    }
}

