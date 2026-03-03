package com.myflowhub.metricsnode

import android.Manifest
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.os.Build
import android.os.Bundle
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Button
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Surface
import androidx.compose.material3.Switch
import androidx.compose.material3.Tab
import androidx.compose.material3.TabRow
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.getValue
import androidx.compose.runtime.setValue
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.mutableStateMapOf
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import androidx.core.content.ContextCompat
import kotlinx.coroutines.delay
import kotlinx.coroutines.Job
import kotlinx.coroutines.launch
import java.io.File

class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContent {
            MetricsNodeApp()
        }
    }
}

@Composable
private fun MetricsNodeApp() {
    val context = androidx.compose.ui.platform.LocalContext.current
    val prefs = remember { context.getSharedPreferences("metricsnode", Context.MODE_PRIVATE) }

    var addr by remember { mutableStateOf(prefs.getString("hub_addr", "127.0.0.1:9000") ?: "127.0.0.1:9000") }
    val initialDeviceId = remember { DeviceId.ensure(prefs, "android") }
    var deviceId by remember { mutableStateOf(initialDeviceId) }
    var nodeIdText by remember { mutableStateOf(prefs.getString("node_id", "") ?: "") }

    val bridge: NodeBridge = remember {
        try {
            GoNodeBridge()
        } catch (_: Throwable) {
            StubNodeBridge()
        }
    }

    val workDir = remember { File(context.filesDir, "metricsnode").absolutePath }

    var state by remember { mutableStateOf(bridge.status()) }

    var page by remember { mutableStateOf(0) } // 0=Connect 1=Settings

    val notificationLauncher = rememberLauncherForActivityResult(
        contract = ActivityResultContracts.RequestPermission(),
        onResult = { /* no-op */ },
    )

    val hasNotifPermission = remember {
        if (Build.VERSION.SDK_INT < 33) true else {
            ContextCompat.checkSelfPermission(context, Manifest.permission.POST_NOTIFICATIONS) == PackageManager.PERMISSION_GRANTED
        }
    }

    LaunchedEffect(Unit) {
        runCatching { bridge.init(workDir) }
        while (true) {
            state = bridge.status()
            delay(1000)
        }
    }

    LaunchedEffect(state.auth.nodeId) {
        if (nodeIdText.isBlank() && state.auth.nodeId > 0) {
            nodeIdText = state.auth.nodeId.toString()
            prefs.edit().putString("node_id", nodeIdText).apply()
        }
    }

    MaterialTheme {
        Surface(modifier = Modifier.fillMaxSize(), color = MaterialTheme.colorScheme.background) {
            Column(modifier = Modifier.fillMaxSize().padding(16.dp)) {
                Text("MyFlowHub MetricsNode", style = MaterialTheme.typography.headlineSmall)

                if (Build.VERSION.SDK_INT >= 33 && !hasNotifPermission) {
                    Button(
                        modifier = Modifier.padding(top = 12.dp),
                        onClick = { notificationLauncher.launch(Manifest.permission.POST_NOTIFICATIONS) },
                    ) {
                        Text("Grant Notification Permission")
                    }
                }

                TabRow(selectedTabIndex = page, modifier = Modifier.padding(top = 12.dp)) {
                    Tab(selected = page == 0, onClick = { page = 0 }, text = { Text("Connect") })
                    Tab(selected = page == 1, onClick = { page = 1 }, text = { Text("Settings") })
                }

                Box(modifier = Modifier.fillMaxSize().padding(top = 12.dp)) {
                    if (page == 0) {
                        ConnectPage(
                            context = context,
                            prefs = prefs,
                            addr = addr,
                            onAddrChange = {
                                addr = it
                                prefs.edit().putString("hub_addr", it).apply()
                            },
                            deviceId = deviceId,
                            onDeviceIdChange = {
                                deviceId = it
                                prefs.edit().putString(DeviceId.PrefKey, it).apply()
                            },
                            nodeIdText = nodeIdText,
                            onNodeIdChange = {
                                nodeIdText = it
                                prefs.edit().putString("node_id", it).apply()
                            },
                            state = state,
                        )
                    } else {
                        SettingsPage(
                            bridge = bridge,
                            state = state,
                        )
                    }
                }
            }
        }
    }
}

@Composable
private fun ConnectPage(
    context: Context,
    prefs: android.content.SharedPreferences,
    addr: String,
    onAddrChange: (String) -> Unit,
    deviceId: String,
    onDeviceIdChange: (String) -> Unit,
    nodeIdText: String,
    onNodeIdChange: (String) -> Unit,
    state: NodeState,
) {
    Column(
        modifier = Modifier.fillMaxSize().verticalScroll(rememberScrollState()),
        verticalArrangement = Arrangement.spacedBy(12.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        OutlinedTextField(
            modifier = Modifier.fillMaxWidth(),
            value = addr,
            onValueChange = onAddrChange,
            label = { Text("Hub Addr") },
        )

        OutlinedTextField(
            modifier = Modifier.fillMaxWidth(),
            value = deviceId,
            onValueChange = onDeviceIdChange,
            label = { Text("Device ID") },
        )

        OutlinedTextField(
            modifier = Modifier.fillMaxWidth(),
            value = nodeIdText,
            onValueChange = onNodeIdChange,
            label = { Text("Node ID (for login)") },
        )

        Row(modifier = Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(12.dp)) {
            Button(
                modifier = Modifier.weight(1f),
                onClick = {
                    val intent = Intent(context, NodeService::class.java).apply {
                        action = NodeService.ACTION_CONNECT
                        putExtra(NodeService.EXTRA_ADDR, addr.trim())
                    }
                    ContextCompat.startForegroundService(context, intent)
                },
            ) { Text("Connect") }

            Button(
                modifier = Modifier.weight(1f),
                onClick = {
                    val prefs2 = prefs
                    val id = deviceId.trim().ifBlank { DeviceId.ensure(prefs2, "android") }
                    val intent = Intent(context, NodeService::class.java).apply {
                        action = NodeService.ACTION_REGISTER
                        putExtra(NodeService.EXTRA_DEVICE_ID, id)
                    }
                    ContextCompat.startForegroundService(context, intent)
                },
            ) { Text("Register") }
        }

        Row(modifier = Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(12.dp)) {
            Button(
                modifier = Modifier.weight(1f),
                onClick = {
                    val prefs2 = prefs
                    val id = deviceId.trim().ifBlank { DeviceId.ensure(prefs2, "android") }
                    val nodeId = nodeIdText.trim().toLongOrNull() ?: 0L
                    val intent = Intent(context, NodeService::class.java).apply {
                        action = NodeService.ACTION_LOGIN
                        putExtra(NodeService.EXTRA_DEVICE_ID, id)
                        putExtra(NodeService.EXTRA_NODE_ID, nodeId)
                    }
                    ContextCompat.startForegroundService(context, intent)
                },
            ) { Text("Login") }

            Button(
                modifier = Modifier.weight(1f),
                onClick = {
                    val intent = Intent(context, NodeService::class.java).apply {
                        action = NodeService.ACTION_START_REPORTING
                    }
                    ContextCompat.startForegroundService(context, intent)
                },
            ) { Text("Start") }
        }

        Row(modifier = Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(12.dp)) {
            Button(
                modifier = Modifier.weight(1f),
                onClick = {
                    val intent = Intent(context, NodeService::class.java).apply {
                        action = NodeService.ACTION_STOP_ALL
                    }
                    context.startService(intent)
                },
            ) { Text("Stop All") }
        }

        Column(modifier = Modifier.fillMaxWidth().padding(top = 8.dp), verticalArrangement = Arrangement.spacedBy(4.dp)) {
            Text("Runtime: ${if (state.running) "Yes" else "No"}")
            Text("Connected: ${if (state.connected) "Yes" else "No"}")
            Text("Reporting: ${if (state.reporting) "Yes" else "No"}")
            Text("Node ID: ${state.auth.nodeId}")
            Text("Hub ID: ${state.auth.hubId}")
            if (state.lastError.isNotBlank()) {
                Text("LastError: ${state.lastError}")
            }
        }

        Spacer(modifier = Modifier.size(12.dp))
    }
}

@Composable
private fun SettingsPage(
    bridge: NodeBridge,
    state: NodeState,
) {
    val scope = rememberCoroutineScope()

    var settings by remember { mutableStateOf<List<MetricSetting>>(emptyList()) }
    var settingsError by remember { mutableStateOf("") }
    var saving by remember { mutableStateOf(false) }

    val varNameErrors = remember { mutableStateMapOf<String, String>() }
    var saveJob by remember { mutableStateOf<Job?>(null) }

    val controllable = remember { setOf("volume_percent", "volume_muted", "brightness_percent", "flashlight_enabled") }

    fun isVarNameValid(name: String): Boolean {
        val trimmed = name.trim()
        if (trimmed.isEmpty()) return false
        return trimmed.all { it.isLetterOrDigit() || it == '_' }
    }

    fun load() {
        val raw = bridge.metricsSettingsGet()
        settings = MetricSettingJson.parseList(raw)
        settingsError = ""
        varNameErrors.clear()
    }

    fun validate(): String? {
        val seen = HashSet<String>()
        for (s in settings) {
            val varName = s.varName.trim()
            if (!isVarNameValid(varName)) return "invalid var_name for ${s.metric}"
            if (s.enabled) {
                if (!seen.add(varName)) return "duplicate enabled var_name: $varName"
            }
        }
        return null
    }

    fun saveNow() {
        val err = validate()
        if (err != null) {
            settingsError = err
            return
        }
        saving = true
        settingsError = ""
        val raw = MetricSettingJson.toJson(settings)
        scope.launch {
            runCatching { bridge.metricsSettingsSet(raw) }
                .onFailure { settingsError = it.message ?: it.toString() }
            saving = false
        }
    }

    fun scheduleSave() {
        saveJob?.cancel()
        saveJob = scope.launch {
            delay(400)
            saveNow()
        }
    }

    LaunchedEffect(Unit) {
        load()
    }

    Column(
        modifier = Modifier.fillMaxSize().verticalScroll(rememberScrollState()),
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        Row(modifier = Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(12.dp)) {
            Button(onClick = { load() }) { Text("Reload") }
            if (saving) {
                Text("Saving…", modifier = Modifier.align(Alignment.CenterVertically))
            }
            Spacer(modifier = Modifier.weight(1f))
        }

        if (settingsError.isNotBlank()) {
            Text("Error: $settingsError")
        }

        for (s in settings) {
            val valueText = if (!s.enabled) "-" else (state.metrics[s.metric]?.ifBlank { "-" } ?: "-")
            Column(modifier = Modifier.fillMaxWidth().padding(top = 4.dp), verticalArrangement = Arrangement.spacedBy(6.dp)) {
                Text(s.metric)

                Row(horizontalArrangement = Arrangement.spacedBy(12.dp), verticalAlignment = Alignment.CenterVertically) {
                    Text("Enabled")
                    Switch(
                        checked = s.enabled,
                        enabled = !saving,
                        onCheckedChange = { checked ->
                            settings = settings.map { if (it.metric == s.metric) it.copy(enabled = checked) else it }
                            scheduleSave()
                        },
                    )

                    Spacer(modifier = Modifier.width(12.dp))

                    if (controllable.contains(s.metric)) {
                        Text("Writable")
                        Switch(
                            checked = s.writable,
                            enabled = !saving,
                            onCheckedChange = { checked ->
                                settings = settings.map { if (it.metric == s.metric) it.copy(writable = checked) else it }
                                scheduleSave()
                            },
                        )
                    } else {
                        Text("Writable -")
                    }
                }

                OutlinedTextField(
                    modifier = Modifier.fillMaxWidth(),
                    value = s.varName,
                    enabled = !saving,
                    onValueChange = { v ->
                        val trimmed = v.trim()
                        settings = settings.map { if (it.metric == s.metric) it.copy(varName = v) else it }
                        if (!isVarNameValid(trimmed)) {
                            varNameErrors[s.metric] = "only A-Z a-z 0-9 _"
                            return@OutlinedTextField
                        }
                        varNameErrors.remove(s.metric)
                        scheduleSave()
                    },
                    label = { Text("Var Name") },
                )
                val hint = varNameErrors[s.metric]
                if (!hint.isNullOrBlank()) {
                    Text(hint)
                }

                Text("Value: $valueText")
            }
        }

        Spacer(modifier = Modifier.size(12.dp))
    }
}
