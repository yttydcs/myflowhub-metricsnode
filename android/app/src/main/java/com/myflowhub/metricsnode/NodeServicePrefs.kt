package com.myflowhub.metricsnode

// 本文件承载 MetricsNode Android 前台服务恢复中与运行快照持久化相关的逻辑。

import android.content.Context
import org.json.JSONObject

internal data class NodeRunSnapshot(
    val addr: String = "",
    val deviceId: String = "",
    val nodeId: Long = 0,
    val desiredConnected: Boolean = false,
    val desiredReporting: Boolean = false,
    val desiredNotify: Boolean = false,
) {
    fun wantsRestore(): Boolean = desiredConnected || desiredReporting || desiredNotify
}

internal object NodeServicePrefs {
    private const val PREFS = "metricsnode"
    private const val KEY_RUN_SNAPSHOT = "service_run_snapshot"

    fun loadSnapshot(context: Context): NodeRunSnapshot? {
        val raw = context.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
            .getString(KEY_RUN_SNAPSHOT, null)
            ?: return null
        return try {
            val obj = JSONObject(raw)
            NodeRunSnapshot(
                addr = obj.optString("addr", "").trim(),
                deviceId = obj.optString("device_id", "").trim(),
                nodeId = obj.optLong("node_id", 0),
                desiredConnected = obj.optBoolean("desired_connected", false),
                desiredReporting = obj.optBoolean("desired_reporting", false),
                desiredNotify = obj.optBoolean("desired_notify", false),
            )
        } catch (_: Throwable) {
            null
        }
    }

    fun saveMerged(
        context: Context,
        addr: String? = null,
        deviceId: String? = null,
        nodeId: Long? = null,
        desiredConnected: Boolean? = null,
        desiredReporting: Boolean? = null,
        desiredNotify: Boolean? = null,
    ): NodeRunSnapshot {
        val existing = loadSnapshot(context) ?: NodeRunSnapshot()
        val next = existing.copy(
            addr = addr?.trim()?.takeIf { it.isNotEmpty() } ?: existing.addr,
            deviceId = deviceId?.trim()?.takeIf { it.isNotEmpty() } ?: existing.deviceId,
            nodeId = nodeId?.takeIf { it > 0 } ?: existing.nodeId,
            desiredConnected = desiredConnected ?: existing.desiredConnected,
            desiredReporting = desiredReporting ?: existing.desiredReporting,
            desiredNotify = desiredNotify ?: existing.desiredNotify,
        )
        save(context, next)
        return next
    }

    fun clearDesired(context: Context) {
        context.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
            .edit()
            .remove(KEY_RUN_SNAPSHOT)
            .apply()
    }

    private fun save(context: Context, snapshot: NodeRunSnapshot) {
        val obj = JSONObject()
            .put("addr", snapshot.addr.trim())
            .put("device_id", snapshot.deviceId.trim())
            .put("node_id", snapshot.nodeId)
            .put("desired_connected", snapshot.desiredConnected)
            .put("desired_reporting", snapshot.desiredReporting)
            .put("desired_notify", snapshot.desiredNotify)
        context.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
            .edit()
            .putString(KEY_RUN_SNAPSHOT, obj.toString())
            .apply()
    }
}
