package com.myflowhub.metricsnode
// Context: This file belongs to the MetricsNode application layer around NodeState.

import org.json.JSONObject

data class AuthState(
    val deviceId: String = "",
    val nodeId: Long = 0,
    val hubId: Long = 0,
    val role: String = "",
    val loggedIn: Boolean = false,
    val lastAction: String = "",
    val lastMessage: String = "",
    val lastUnixTime: Long = 0,
)

data class NodeState(
    val running: Boolean = false,
    val connected: Boolean = false,
    val addr: String = "",
    val workDir: String = "",
    val reporting: Boolean = false,
    val auth: AuthState = AuthState(),
    val metrics: Map<String, String> = emptyMap(),
    val lastError: String = "",
)

internal object NodeStateJson {
    fun parse(raw: String): NodeState {
        return try {
            val obj = JSONObject(raw)
            val authObj = obj.optJSONObject("auth")
            val auth = AuthState(
                deviceId = authObj?.optString("device_id", "")?.trim().orEmpty(),
                nodeId = authObj?.optLong("node_id", 0) ?: 0,
                hubId = authObj?.optLong("hub_id", 0) ?: 0,
                role = authObj?.optString("role", "")?.trim().orEmpty(),
                loggedIn = authObj?.optBoolean("logged_in", false) ?: false,
                lastAction = authObj?.optString("last_action", "")?.trim().orEmpty(),
                lastMessage = authObj?.optString("last_message", "")?.trim().orEmpty(),
                lastUnixTime = authObj?.optLong("last_unix_time", 0) ?: 0,
            )
            val metricsObj = obj.optJSONObject("metrics")
            val metrics = LinkedHashMap<String, String>()
            if (metricsObj != null) {
                val it = metricsObj.keys()
                while (it.hasNext()) {
                    val k = it.next()
                    metrics[k] = metricsObj.optString(k, "")
                }
            }
            NodeState(
                running = obj.optBoolean("running", false),
                connected = obj.optBoolean("connected", false),
                addr = obj.optString("addr", ""),
                workDir = obj.optString("workdir", ""),
                reporting = obj.optBoolean("reporting", false),
                auth = auth,
                metrics = metrics,
                lastError = obj.optString("last_error", ""),
            )
        } catch (t: Throwable) {
            NodeState(running = false, lastError = t.message ?: t.toString())
        }
    }
}

