package com.myflowhub.metricsnode

import android.Manifest
import android.app.ActivityManager
import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.Service
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.IntentFilter
import android.content.pm.PackageManager
import android.hardware.camera2.CameraCharacteristics
import android.hardware.camera2.CameraManager
import android.content.pm.ServiceInfo
import android.media.AudioManager
import android.net.ConnectivityManager
import android.net.NetworkCapabilities
import android.os.BatteryManager
import android.os.Build
import android.os.Handler
import android.os.IBinder
import android.os.Looper
import android.provider.Settings
import androidx.core.app.NotificationCompat
import androidx.core.app.ServiceCompat
import java.io.File

class NodeService : Service() {
    private val bridge: NodeBridge = try {
        GoNodeBridge()
    } catch (_: Throwable) {
        StubNodeBridge()
    }

    @Volatile
    private var running = false

    @Volatile
    private var volumeRunning = false

    @Volatile
    private var controlRunning = false

    @Volatile
    private var brightnessRunning = false

    @Volatile
    private var systemRunning = false

    private var batteryReceiver: BroadcastReceiver? = null
    private var volumeThread: Thread? = null
    private var controlThread: Thread? = null
    private var brightnessThread: Thread? = null
    private var systemThread: Thread? = null

    private var cameraManager: CameraManager? = null
    private var torchCallback: CameraManager.TorchCallback? = null
    private var torchCameraId: String? = null

    @Volatile
    private var flashlightState: Int = -1

    override fun onBind(intent: Intent?): IBinder? = null

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        when (intent?.action) {
            ACTION_START -> {
                val addr = intent.getStringExtra(EXTRA_ADDR) ?: ""
                val prefs = getSharedPreferences("metricsnode", Context.MODE_PRIVATE)
                var deviceId = intent.getStringExtra(EXTRA_DEVICE_ID)?.trim().orEmpty()
                if (deviceId.isBlank()) {
                    deviceId = DeviceId.ensure(prefs, "android")
                } else {
                    prefs.edit().putString(DeviceId.PrefKey, deviceId).apply()
                }
                val workDir = File(filesDir, "metricsnode").absolutePath

                startForegroundWithState("Starting…")
                Thread {
                    val st = bridge.start(NodeConfig(addr = addr, deviceId = deviceId, workDir = workDir))
                    running = st.running
                    startForegroundWithState(if (st.connected) "Running" else "Stopped")
                    if (st.running) {
                        startObservers()
                    }
                }.start()
            }
            ACTION_STOP -> {
                stopObservers()
                running = false
                bridge.stop()
                stopForeground(STOP_FOREGROUND_REMOVE)
                stopSelf()
            }
            else -> {
                // no-op
            }
        }
        return START_STICKY
    }

    override fun onDestroy() {
        stopObservers()
        super.onDestroy()
    }

    fun getState(): NodeState = bridge.status()

    private fun startObservers() {
        startBatteryObserver()
        startVolumePoller()
        startBrightnessPoller()
        startSystemPoller()
        startFlashlightObserver()
        startControlPoller()
    }

    private fun stopObservers() {
        stopBatteryObserver()
        stopVolumePoller()
        stopBrightnessPoller()
        stopSystemPoller()
        stopFlashlightObserver()
        stopControlPoller()
    }

    private fun startBatteryObserver() {
        if (batteryReceiver != null) {
            return
        }
        val receiver = object : BroadcastReceiver() {
            override fun onReceive(context: Context?, intent: Intent?) {
                if (!running) return
                if (intent == null) return
                val level = intent.getIntExtra(BatteryManager.EXTRA_LEVEL, -1)
                val scale = intent.getIntExtra(BatteryManager.EXTRA_SCALE, -1)
                val percent = if (level >= 0 && scale > 0) (level * 100 / scale) else -1
                bridge.updateBatteryPercent(percent)

                val plugged = intent.getIntExtra(BatteryManager.EXTRA_PLUGGED, -1)
                val onAC = when {
                    plugged < 0 -> -1
                    plugged == 0 -> 0
                    else -> 1
                }
                bridge.updateBatteryOnAC(onAC)
                // Per spec: 插电即 charging=1（即便满电）。
                bridge.updateBatteryCharging(onAC)
            }
        }
        registerReceiver(receiver, IntentFilter(Intent.ACTION_BATTERY_CHANGED))
        batteryReceiver = receiver
    }

    private fun stopBatteryObserver() {
        val receiver = batteryReceiver ?: return
        batteryReceiver = null
        runCatching { unregisterReceiver(receiver) }
    }

    private fun startVolumePoller() {
        if (volumeThread != null) {
            return
        }
        val audio = getSystemService(Context.AUDIO_SERVICE) as AudioManager
        volumeRunning = true
        val t = Thread {
            var lastPercent: Int? = null
            var lastMuted: Boolean? = null
            while (volumeRunning) {
                if (!running) {
                    Thread.sleep(200)
                    continue
                }
                val max = audio.getStreamMaxVolume(AudioManager.STREAM_MUSIC)
                val vol = audio.getStreamVolume(AudioManager.STREAM_MUSIC)
                val percent = if (max > 0) (vol * 100 / max) else 0
                val muted = audio.isStreamMute(AudioManager.STREAM_MUSIC) || vol == 0
                if (lastPercent != percent || lastMuted != muted) {
                    bridge.updateVolume(percent, muted)
                    lastPercent = percent
                    lastMuted = muted
                }
                Thread.sleep(1000)
            }
        }
        t.isDaemon = true
        t.start()
        volumeThread = t
    }

    private fun stopVolumePoller() {
        volumeRunning = false
        val t = volumeThread ?: return
        volumeThread = null
        runCatching { t.join(1200) }
    }

    private fun startBrightnessPoller() {
        if (brightnessThread != null) {
            return
        }
        brightnessRunning = true
        val t = Thread {
            var lastPercent: Int? = null
            while (brightnessRunning) {
                if (!running) {
                    Thread.sleep(200)
                    continue
                }
                val percent = readBrightnessPercent()
                if (lastPercent != percent) {
                    bridge.updateBrightnessPercent(percent)
                    lastPercent = percent
                }
                Thread.sleep(1000)
            }
        }
        t.isDaemon = true
        t.start()
        brightnessThread = t
    }

    private fun stopBrightnessPoller() {
        brightnessRunning = false
        val t = brightnessThread ?: return
        brightnessThread = null
        runCatching { t.join(1200) }
    }

    private fun startSystemPoller() {
        if (systemThread != null) {
            return
        }
        val am = getSystemService(Context.ACTIVITY_SERVICE) as ActivityManager
        val cm = getSystemService(Context.CONNECTIVITY_SERVICE) as ConnectivityManager
        val cpuSampler = ProcStatCpuSampler()
        systemRunning = true
        val t = Thread {
            var nextNetAt = 0L
            while (systemRunning) {
                if (!running) {
                    Thread.sleep(200)
                    continue
                }
                val cpu = cpuSampler.readPercent()
                bridge.updateCPUPercent(cpu)

                val mem = readMemPercent(am)
                bridge.updateMemPercent(mem)

                val now = System.currentTimeMillis()
                if (now >= nextNetAt) {
                    val (online, netType) = readNetStatus(cm)
                    bridge.updateNetOnline(online)
                    bridge.updateNetType(netType)
                    nextNetAt = now + 5000
                }

                Thread.sleep(2000)
            }
        }
        t.isDaemon = true
        t.start()
        systemThread = t
    }

    private fun stopSystemPoller() {
        systemRunning = false
        val t = systemThread ?: return
        systemThread = null
        runCatching { t.join(1200) }
    }

    private fun readMemPercent(am: ActivityManager): Int {
        return runCatching {
            val info = ActivityManager.MemoryInfo()
            am.getMemoryInfo(info)
            val total = info.totalMem
            val avail = info.availMem
            if (total <= 0) {
                -1
            } else {
                val used = (total - avail).coerceAtLeast(0)
                ((used * 100) / total).toInt().coerceIn(0, 100)
            }
        }.getOrDefault(-1)
    }

    private fun readNetStatus(cm: ConnectivityManager): Pair<Int, String> {
        return try {
            val network = cm.activeNetwork ?: return 0 to "none"
            val caps = cm.getNetworkCapabilities(network) ?: return 0 to "none"
            val hasInternet = caps.hasCapability(NetworkCapabilities.NET_CAPABILITY_INTERNET)
            if (!hasInternet) {
                return 0 to "none"
            }
            val netType = when {
                caps.hasTransport(NetworkCapabilities.TRANSPORT_WIFI) -> "wifi"
                caps.hasTransport(NetworkCapabilities.TRANSPORT_ETHERNET) -> "ethernet"
                caps.hasTransport(NetworkCapabilities.TRANSPORT_CELLULAR) -> "cellular"
                else -> "unknown"
            }
            1 to netType
        } catch (_: SecurityException) {
            -1 to "-1"
        } catch (_: Throwable) {
            -1 to "-1"
        }
    }

    private fun startFlashlightObserver() {
        if (torchCallback != null) {
            return
        }
        val granted = checkSelfPermission(Manifest.permission.CAMERA) == PackageManager.PERMISSION_GRANTED
        if (!granted) {
            flashlightState = -1
            bridge.updateFlashlightEnabled(-1)
            return
        }
        if (!packageManager.hasSystemFeature(PackageManager.FEATURE_CAMERA_FLASH)) {
            flashlightState = -1
            bridge.updateFlashlightEnabled(-1)
            return
        }

        val mgr = getSystemService(Context.CAMERA_SERVICE) as CameraManager
        val cameraId = findTorchCameraId(mgr)
        if (cameraId.isNullOrBlank()) {
            flashlightState = -1
            bridge.updateFlashlightEnabled(-1)
            return
        }

        cameraManager = mgr
        torchCameraId = cameraId
        flashlightState = -1
        bridge.updateFlashlightEnabled(-1)

        val cb = object : CameraManager.TorchCallback() {
            override fun onTorchModeChanged(id: String, enabled: Boolean) {
                if (id != cameraId) return
                val v = if (enabled) 1 else 0
                if (flashlightState != v) {
                    flashlightState = v
                    bridge.updateFlashlightEnabled(v)
                }
            }

            override fun onTorchModeUnavailable(id: String) {
                if (id != cameraId) return
                if (flashlightState != -1) {
                    flashlightState = -1
                    bridge.updateFlashlightEnabled(-1)
                }
            }
        }
        torchCallback = cb

        runCatching {
            mgr.registerTorchCallback(cb, Handler(Looper.getMainLooper()))
        }.onFailure {
            flashlightState = -1
            bridge.updateFlashlightEnabled(-1)
            torchCallback = null
        }
    }

    private fun stopFlashlightObserver() {
        val mgr = cameraManager
        val cb = torchCallback
        torchCallback = null
        cameraManager = null
        torchCameraId = null
        if (mgr != null && cb != null) {
            runCatching { mgr.unregisterTorchCallback(cb) }
        }
    }

    private fun findTorchCameraId(mgr: CameraManager): String? {
        return runCatching {
            var first: String? = null
            var back: String? = null
            for (id in mgr.cameraIdList) {
                val chars = mgr.getCameraCharacteristics(id)
                val hasFlash = chars.get(CameraCharacteristics.FLASH_INFO_AVAILABLE) == true
                if (!hasFlash) continue
                if (first == null) {
                    first = id
                }
                val facing = chars.get(CameraCharacteristics.LENS_FACING)
                if (facing == CameraCharacteristics.LENS_FACING_BACK) {
                    back = id
                }
            }
            back ?: first
        }.getOrNull()
    }

    private fun readBrightnessPercent(): Int {
        val raw = runCatching {
            Settings.System.getInt(contentResolver, Settings.System.SCREEN_BRIGHTNESS, -1)
        }.getOrDefault(-1)
        if (raw < 0) {
            return -1
        }
        val clamped = raw.coerceIn(0, 255)
        val percent = (clamped * 100 + 127) / 255
        return percent.coerceIn(0, 100)
    }

    private fun startControlPoller() {
        if (controlThread != null) {
            return
        }
        val audio = getSystemService(Context.AUDIO_SERVICE) as AudioManager
        controlRunning = true
        val t = Thread {
            while (controlRunning) {
                if (!running) {
                    Thread.sleep(200)
                    continue
                }
                val actions = bridge.dequeueActions()
                if (actions.isNotEmpty()) {
                    applyControlActions(audio, actions)
                }
                Thread.sleep(250)
            }
        }
        t.isDaemon = true
        t.start()
        controlThread = t
    }

    private fun stopControlPoller() {
        controlRunning = false
        val t = controlThread ?: return
        controlThread = null
        runCatching { t.join(1200) }
    }

    private fun applyControlActions(audio: AudioManager, actions: List<NodeAction>) {
        var volumePercent: NodeAction? = null
        var muted: NodeAction? = null
        var brightnessPercent: NodeAction? = null
        var flashlightEnabled: NodeAction? = null
        for (a in actions) {
            when (a.metric) {
                "volume_percent" -> volumePercent = a
                "volume_muted" -> muted = a
                "brightness_percent" -> brightnessPercent = a
                "flashlight_enabled" -> flashlightEnabled = a
            }
        }

        volumePercent?.let { act ->
            val percent = act.value.toIntOrNull()?.coerceIn(0, 100) ?: return@let
            val max = audio.getStreamMaxVolume(AudioManager.STREAM_MUSIC)
            val idx = if (max > 0) ((percent * max) + 50) / 100 else 0
            val clamped = idx.coerceIn(0, max)
            val wasMuted = audio.isStreamMute(AudioManager.STREAM_MUSIC)
            runCatching { audio.setStreamVolume(AudioManager.STREAM_MUSIC, clamped, 0) }
            if (wasMuted) {
                runCatching { audio.adjustStreamVolume(AudioManager.STREAM_MUSIC, AudioManager.ADJUST_MUTE, 0) }
            }
        }
        muted?.let { act ->
            val v = act.value.trim().lowercase()
            val wantMuted = v == "1" || v == "true" || v == "yes" || v == "y" || v == "on"
            val direction = if (wantMuted) AudioManager.ADJUST_MUTE else AudioManager.ADJUST_UNMUTE
            runCatching { audio.adjustStreamVolume(AudioManager.STREAM_MUSIC, direction, 0) }
        }
        brightnessPercent?.let { act ->
            val percent = act.value.toIntOrNull()?.coerceIn(0, 100) ?: return@let
            if (!Settings.System.canWrite(this)) {
                return@let
            }
            val raw = ((percent * 255) + 50) / 100
            runCatching {
                Settings.System.putInt(contentResolver, Settings.System.SCREEN_BRIGHTNESS, raw.coerceIn(0, 255))
            }
        }
        flashlightEnabled?.let { act ->
            val want = parseBoolish(act.value) ?: return@let
            val mgr = cameraManager ?: run {
                bridge.updateFlashlightEnabled(-1)
                return@let
            }
            val id = torchCameraId ?: run {
                bridge.updateFlashlightEnabled(-1)
                return@let
            }
            val granted = checkSelfPermission(Manifest.permission.CAMERA) == PackageManager.PERMISSION_GRANTED
            if (!granted) {
                bridge.updateFlashlightEnabled(-1)
                return@let
            }
            runCatching {
                mgr.setTorchMode(id, want)
            }.onFailure {
                val correct = if (flashlightState < 0) -1 else flashlightState
                bridge.updateFlashlightEnabled(correct)
            }
        }
    }

    private fun parseBoolish(raw: String): Boolean? {
        val v = raw.trim().lowercase()
        return when (v) {
            "1", "true", "yes", "y", "on" -> true
            "0", "false", "no", "n", "off" -> false
            else -> null
        }
    }

    private fun startForegroundWithState(text: String) {
        createChannelIfNeeded()
        val notification: Notification = NotificationCompat.Builder(this, CHANNEL_ID)
            .setContentTitle("MyFlowHub MetricsNode")
            .setContentText(text)
            .setSmallIcon(android.R.drawable.stat_sys_upload)
            .setOngoing(true)
            .build()
        ServiceCompat.startForeground(
            this,
            NOTIFICATION_ID,
            notification,
            ServiceInfo.FOREGROUND_SERVICE_TYPE_DATA_SYNC or ServiceInfo.FOREGROUND_SERVICE_TYPE_CAMERA,
        )
    }

    private fun createChannelIfNeeded() {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.O) {
            return
        }
        val nm = getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
        val existing = nm.getNotificationChannel(CHANNEL_ID)
        if (existing != null) {
            return
        }
        val ch = NotificationChannel(
            CHANNEL_ID,
            "MyFlowHub MetricsNode",
            NotificationManager.IMPORTANCE_LOW,
        )
        nm.createNotificationChannel(ch)
    }

    companion object {
        const val ACTION_START = "com.myflowhub.metricsnode.action.START"
        const val ACTION_STOP = "com.myflowhub.metricsnode.action.STOP"

        const val EXTRA_ADDR = "addr"
        const val EXTRA_DEVICE_ID = "device_id"

        private const val CHANNEL_ID = "myflowhub_metricsnode"
        private const val NOTIFICATION_ID = 1
    }
}

private class ProcStatCpuSampler {
    private data class CpuTimes(val idle: Long, val total: Long)

    private var prev: CpuTimes? = null

    fun readPercent(): Int {
        val cur = readTimes() ?: return -1
        val p = prev
        prev = cur
        if (p == null) {
            return -1
        }
        val dTotal = cur.total - p.total
        val dIdle = cur.idle - p.idle
        if (dTotal <= 0 || dIdle < 0 || dIdle > dTotal) {
            return -1
        }
        val busy = dTotal - dIdle
        return ((busy * 100) / dTotal).toInt().coerceIn(0, 100)
    }

    private fun readTimes(): CpuTimes? {
        return runCatching {
            val line = File("/proc/stat").bufferedReader().use { it.readLine() } ?: return null
            if (!line.startsWith("cpu")) return null
            val parts = line.trim().split(Regex("\\s+"))
            if (parts.size < 5) return null
            val values = parts.drop(1).mapNotNull { it.toLongOrNull() }
            if (values.size < 4) return null
            val idle = values.getOrElse(3) { 0L } + values.getOrElse(4) { 0L }
            val total = values.sum()
            CpuTimes(idle = idle, total = total)
        }.getOrNull()
    }
}
