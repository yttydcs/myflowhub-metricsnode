package com.myflowhub.metricsnode

import org.json.JSONObject

data class NodeState(
    val running: Boolean = false,
    val connected: Boolean = false,
    val addr: String = "",
    val workDir: String = "",
    val reporting: Boolean = false,
    val lastError: String = "",
)

internal object NodeStateJson {
    fun parse(raw: String): NodeState {
        return try {
            val obj = JSONObject(raw)
            NodeState(
                running = obj.optBoolean("running", false),
                connected = obj.optBoolean("connected", false),
                addr = obj.optString("addr", ""),
                workDir = obj.optString("workdir", ""),
                reporting = obj.optBoolean("reporting", false),
                lastError = obj.optString("last_error", ""),
            )
        } catch (t: Throwable) {
            NodeState(running = false, lastError = t.message ?: t.toString())
        }
    }
}

