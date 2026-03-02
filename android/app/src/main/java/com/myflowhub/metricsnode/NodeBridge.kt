package com.myflowhub.metricsnode

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

interface NodeBridge {
    fun start(config: NodeConfig): NodeState
    fun stop(): NodeState
    fun status(): NodeState

    fun updateBatteryPercent(percent: Int)
    fun updateVolume(volumePercent: Int, muted: Boolean)
    fun updateBrightnessPercent(percent: Int)

    fun dequeueActions(): List<NodeAction>
}

class StubNodeBridge : NodeBridge {
    private var state = NodeState(running = false)

    override fun start(config: NodeConfig): NodeState {
        state = NodeState(running = true, connected = true, addr = config.addr, workDir = config.workDir, reporting = true)
        return state
    }

    override fun stop(): NodeState {
        state = NodeState(running = false)
        return state
    }

    override fun status(): NodeState = state

    override fun updateBatteryPercent(percent: Int) {}

    override fun updateVolume(volumePercent: Int, muted: Boolean) {}

    override fun updateBrightnessPercent(percent: Int) {}

    override fun dequeueActions(): List<NodeAction> = emptyList()
}

class GoNodeBridge : NodeBridge {
    private val cls: Class<*>

    private val startMethod: Method
    private val stopMethod: Method
    private val statusMethod: Method

    private val updateBatteryMethod: Method
    private val updateVolumePercentMethod: Method
    private val updateVolumeMutedMethod: Method
    private val updateBrightnessMethod: Method?
    private val dequeueActionsMethod: Method

    init {
        cls = GomobileLoader.loadNodeClass()
        startMethod = GoReflect.method(cls, "Start", String::class.java, String::class.java, String::class.java)
        stopMethod = GoReflect.method(cls, "Stop")
        statusMethod = GoReflect.method(cls, "Status")

        updateBatteryMethod = GoReflect.method(cls, "UpdateBatteryPercent", String::class.java)
        updateVolumePercentMethod = GoReflect.method(cls, "UpdateVolumePercent", String::class.java)
        updateVolumeMutedMethod = GoReflect.method(cls, "UpdateVolumeMuted", String::class.java)
        updateBrightnessMethod = runCatching { GoReflect.method(cls, "UpdateBrightnessPercent", String::class.java) }.getOrNull()
        dequeueActionsMethod = GoReflect.method(cls, "DequeueActions")

        // Optional probe to help diagnose missing AAR in runtime.
        runCatching { GoReflect.method(cls, "EnsureLinked").invoke(null) }
    }

    override fun start(config: NodeConfig): NodeState =
        call { startMethod.invoke(null, config.addr, config.deviceId, config.workDir) as String }

    override fun stop(): NodeState =
        call { stopMethod.invoke(null) as String }

    override fun status(): NodeState =
        call { statusMethod.invoke(null) as String }

    override fun updateBatteryPercent(percent: Int) {
        val text = if (percent < 0) "-1" else percent.toString()
        runCatching { updateBatteryMethod.invoke(null, text) }
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

    override fun dequeueActions(): List<NodeAction> {
        val raw = runCatching { dequeueActionsMethod.invoke(null) as String }.getOrNull() ?: return emptyList()
        return NodeActionJson.parseList(raw)
    }

    private fun call(fn: () -> String): NodeState {
        return try {
            NodeStateJson.parse(fn())
        } catch (t: Throwable) {
            NodeState(running = false, lastError = t.message ?: t.toString())
        }
    }
}

