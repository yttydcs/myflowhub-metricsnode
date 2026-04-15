package com.myflowhub.metricsnode
// 本文件实现 MetricsNode Android 宿主中与 `DeviceId` 相关的逻辑。

import android.content.SharedPreferences
import java.util.UUID

internal object DeviceId {
    const val PrefKey = "device_id"

    fun ensure(prefs: SharedPreferences, prefix: String): String {
        val existing = prefs.getString(PrefKey, "")?.trim().orEmpty()
        if (existing.isNotBlank()) {
            return existing
        }
        val p = prefix.trim()
        val id = if (p.isBlank()) UUID.randomUUID().toString() else "${p}-${UUID.randomUUID()}"
        prefs.edit().putString(PrefKey, id).apply()
        return id
    }
}

