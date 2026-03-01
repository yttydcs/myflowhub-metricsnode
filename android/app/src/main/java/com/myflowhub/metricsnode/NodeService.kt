package com.myflowhub.metricsnode

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.Service
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.IntentFilter
import android.content.pm.ServiceInfo
import android.media.AudioManager
import android.os.BatteryManager
import android.os.Build
import android.os.IBinder
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

    private var batteryReceiver: BroadcastReceiver? = null
    private var volumeThread: Thread? = null
    private var controlThread: Thread? = null

    override fun onBind(intent: Intent?): IBinder? = null

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        when (intent?.action) {
            ACTION_START -> {
                val addr = intent.getStringExtra(EXTRA_ADDR) ?: ""
                val deviceId = intent.getStringExtra(EXTRA_DEVICE_ID) ?: ""
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
        startControlPoller()
    }

    private fun stopObservers() {
        stopBatteryObserver()
        stopVolumePoller()
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
        for (a in actions) {
            when (a.metric) {
                "volume_percent" -> volumePercent = a
                "volume_muted" -> muted = a
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
            ServiceInfo.FOREGROUND_SERVICE_TYPE_DATA_SYNC,
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
