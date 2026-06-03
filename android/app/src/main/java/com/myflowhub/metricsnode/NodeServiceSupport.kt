package com.myflowhub.metricsnode

// 本文件承载 MetricsNode Android 前台服务恢复中可单测的纯逻辑。

internal object NodeServiceSupport {
    fun shouldRestore(snapshot: NodeRunSnapshot?): Boolean {
        return snapshot != null && snapshot.wantsRestore()
    }

    fun restoreError(snapshot: NodeRunSnapshot?): String? {
        if (!shouldRestore(snapshot)) {
            return null
        }
        return if (snapshot?.addr?.isBlank() == true) "Restore failed: missing addr" else null
    }

    fun foregroundText(st: NodeState): String {
        return when {
            st.reporting && st.notify -> "Running | Notify"
            st.reporting -> "Running"
            st.connected && st.notify -> "Connected | Notify"
            st.connected -> "Connected"
            st.lastError.isNotBlank() -> "Disconnected: ${summarizeError(st.lastError)}"
            else -> "Disconnected"
        }
    }

    private fun summarizeError(raw: String): String {
        val text = raw.trim()
        return if (text.length <= 72) text else text.take(69) + "..."
    }
}
