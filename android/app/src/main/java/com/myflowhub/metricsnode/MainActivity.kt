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
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.border
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.verticalScroll
import androidx.compose.foundation.BorderStroke
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Switch
import androidx.compose.material3.Surface
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
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.style.TextAlign
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
        } catch (t: Throwable) {
            StubNodeBridge(t.message ?: t.toString())
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
                val bridgeKind = remember(bridge) { if (bridge is StubNodeBridge) "Stub" else "Go" }
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(12.dp),
                ) {
                    Column(modifier = Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(4.dp)) {
                        Text("MyFlowHub MetricsNode", style = MaterialTheme.typography.headlineSmall)
                        Text(
                            "Android client. Bridge: $bridgeKind. WorkDir: $workDir",
                            style = MaterialTheme.typography.bodySmall,
                        )
                    }
                    Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                        StatusPill(text = if (state.connected) "Connected" else "Disconnected", ok = state.connected)
                        StatusPill(text = if (state.reporting) "Reporting" else "Stopped", ok = state.reporting)
                    }
                }

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
    val canReport = state.connected && state.auth.loggedIn
    Column(
        modifier = Modifier.fillMaxSize().verticalScroll(rememberScrollState()),
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        Card {
            Column(modifier = Modifier.fillMaxWidth().padding(16.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
                Text("Bootstrap", style = MaterialTheme.typography.titleSmall)

                BoxWithConstraints(modifier = Modifier.fillMaxWidth()) {
                    val wide = maxWidth >= 720.dp
                    if (wide) {
                        Row(modifier = Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                            OutlinedTextField(
                                modifier = Modifier.weight(1f),
                                value = addr,
                                onValueChange = onAddrChange,
                                label = { Text("Hub Addr") },
                                singleLine = true,
                            )
                            OutlinedTextField(
                                modifier = Modifier.weight(1f),
                                value = deviceId,
                                onValueChange = onDeviceIdChange,
                                label = { Text("Device ID") },
                                singleLine = true,
                            )
                        }
                    } else {
                        OutlinedTextField(
                            modifier = Modifier.fillMaxWidth(),
                            value = addr,
                            onValueChange = onAddrChange,
                            label = { Text("Hub Addr") },
                            singleLine = true,
                        )
                        OutlinedTextField(
                            modifier = Modifier.fillMaxWidth().padding(top = 12.dp),
                            value = deviceId,
                            onValueChange = onDeviceIdChange,
                            label = { Text("Device ID") },
                            singleLine = true,
                        )
                    }
                }

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

                    OutlinedButton(
                        modifier = Modifier.weight(1f),
                        enabled = state.connected,
                        onClick = {
                            val intent = Intent(context, NodeService::class.java).apply {
                                action = NodeService.ACTION_DISCONNECT
                            }
                            ContextCompat.startForegroundService(context, intent)
                        },
                    ) { Text("Disconnect") }
                }
            }
        }

        Card {
            Column(modifier = Modifier.fillMaxWidth().padding(16.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
                Text("Auth", style = MaterialTheme.typography.titleSmall)

                OutlinedTextField(
                    modifier = Modifier.fillMaxWidth(),
                    value = nodeIdText,
                    onValueChange = onNodeIdChange,
                    label = { Text("Node ID (for login)") },
                    singleLine = true,
                )

                Row(modifier = Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                    Button(
                        modifier = Modifier.weight(1f),
                        enabled = state.connected,
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

                    Button(
                        modifier = Modifier.weight(1f),
                        enabled = state.connected,
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
                }

                Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
                    KeyValueRow("Logged In", if (state.auth.loggedIn) "Yes" else "No")
                    KeyValueRow("Node ID", state.auth.nodeId.toString())
                    KeyValueRow("Hub ID", state.auth.hubId.toString())
                    KeyValueRow("Role", state.auth.role.ifBlank { "-" })
                    KeyValueRow("Last", state.auth.lastAction.ifBlank { "-" })
                    if (state.auth.lastMessage.isNotBlank()) {
                        KeyValueRow("Msg", state.auth.lastMessage)
                    }
                }
            }
        }

        Card {
            Column(modifier = Modifier.fillMaxWidth().padding(16.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
                Text("Reporting", style = MaterialTheme.typography.titleSmall)

                Row(modifier = Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                    Button(
                        modifier = Modifier.weight(1f),
                        enabled = canReport && !state.reporting,
                        onClick = {
                            val intent = Intent(context, NodeService::class.java).apply {
                                action = NodeService.ACTION_START_REPORTING
                            }
                            ContextCompat.startForegroundService(context, intent)
                        },
                    ) { Text("Start Reporting") }

                    OutlinedButton(
                        modifier = Modifier.weight(1f),
                        enabled = state.reporting,
                        onClick = {
                            val intent = Intent(context, NodeService::class.java).apply {
                                action = NodeService.ACTION_STOP_REPORTING
                            }
                            ContextCompat.startForegroundService(context, intent)
                        },
                    ) { Text("Stop") }
                }

                OutlinedButton(
                    modifier = Modifier.fillMaxWidth(),
                    onClick = {
                        val intent = Intent(context, NodeService::class.java).apply {
                            action = NodeService.ACTION_STOP_ALL
                        }
                        context.startService(intent)
                    },
                ) { Text("Stop All") }
            }
        }

        if (state.lastError.isNotBlank()) {
            Card(colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.errorContainer)) {
                Column(modifier = Modifier.fillMaxWidth().padding(16.dp), verticalArrangement = Arrangement.spacedBy(6.dp)) {
                    Text("LastError", style = MaterialTheme.typography.titleSmall, color = MaterialTheme.colorScheme.onErrorContainer)
                    Text(state.lastError, color = MaterialTheme.colorScheme.onErrorContainer)
                }
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
    val enabledCount = settings.count { it.enabled }
    val writeableCount = settings.count { it.writable && controllable.contains(it.metric) }

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
            if (!isVarNameValid(varName)) return "invalid mapping for ${s.metric}"
            if (s.enabled) {
                if (!seen.add(varName)) return "duplicate enabled mapping: $varName"
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
        verticalArrangement = Arrangement.spacedBy(10.dp),
    ) {
        Card {
            Column(modifier = Modifier.fillMaxWidth().padding(16.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    verticalAlignment = Alignment.Top,
                    horizontalArrangement = Arrangement.spacedBy(12.dp),
                ) {
                    Column(modifier = Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(2.dp)) {
                        Text("Metrics Settings", style = MaterialTheme.typography.titleSmall)
                        Text(
                            "Manage metric mapping and report controls.",
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                        )
                    }
                    StatusPill(text = if (saving) "Saving…" else "Ready", ok = !saving, warn = saving)
                }

                BoxWithConstraints(modifier = Modifier.fillMaxWidth()) {
                    val compactSummary = maxWidth < 680.dp
                    if (compactSummary) {
                        Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                            Row(
                                modifier = Modifier.fillMaxWidth(),
                                horizontalArrangement = Arrangement.spacedBy(8.dp),
                            ) {
                                MetricSummaryChip("Total", settings.size.toString(), Modifier.weight(1f))
                                MetricSummaryChip("Enabled", enabledCount.toString(), Modifier.weight(1f))
                                MetricSummaryChip("Writeable", writeableCount.toString(), Modifier.weight(1f))
                            }
                            OutlinedButton(
                                modifier = Modifier.fillMaxWidth(),
                                enabled = !saving,
                                onClick = { load() },
                            ) { Text("Reload") }
                        }
                    } else {
                        Row(
                            modifier = Modifier.fillMaxWidth(),
                            verticalAlignment = Alignment.CenterVertically,
                            horizontalArrangement = Arrangement.spacedBy(8.dp),
                        ) {
                            MetricSummaryChip("Total", settings.size.toString())
                            MetricSummaryChip("Enabled", enabledCount.toString())
                            MetricSummaryChip("Writeable", writeableCount.toString())
                            Spacer(modifier = Modifier.weight(1f))
                            OutlinedButton(enabled = !saving, onClick = { load() }) { Text("Reload") }
                        }
                    }
                }

                if (saving) {
                    LinearProgressIndicator(
                        modifier = Modifier.fillMaxWidth().heightIn(min = 4.dp),
                    )
                }

                if (settingsError.isNotBlank()) {
                    Surface(
                        modifier = Modifier.fillMaxWidth(),
                        shape = RoundedCornerShape(10.dp),
                        color = MaterialTheme.colorScheme.errorContainer,
                    ) {
                        Text(
                            "Error: $settingsError",
                            modifier = Modifier.padding(horizontal = 10.dp, vertical = 8.dp),
                            color = MaterialTheme.colorScheme.onErrorContainer,
                            style = MaterialTheme.typography.bodySmall,
                        )
                    }
                }

                if (settings.isEmpty()) {
                    Text("No settings loaded.", style = MaterialTheme.typography.bodySmall)
                } else {
                    BoxWithConstraints(modifier = Modifier.fillMaxWidth()) {
                        val wide = maxWidth >= 920.dp
                        Surface(
                            modifier = Modifier.fillMaxWidth(),
                            shape = RoundedCornerShape(14.dp),
                            tonalElevation = 1.dp,
                            border = BorderStroke(1.dp, MaterialTheme.colorScheme.outlineVariant.copy(alpha = 0.6f)),
                        ) {
                            val listArrangement = if (wide) Arrangement.spacedBy(6.dp) else Arrangement.spacedBy(10.dp)
                            Column(
                                modifier = Modifier.fillMaxWidth().padding(10.dp),
                                verticalArrangement = listArrangement,
                            ) {
                                if (wide) {
                                    SettingsHeaderRow()
                                    HorizontalDivider()
                                }

                                for ((index, s) in settings.withIndex()) {
                                    val isControllable = controllable.contains(s.metric)
                                    val valueText = if (!s.enabled) "-" else (state.metrics[s.metric]?.ifBlank { "-" } ?: "-")
                                    val onEnabledChange: (Boolean) -> Unit = { checked ->
                                        settings = settings.map { if (it.metric == s.metric) it.copy(enabled = checked) else it }
                                        scheduleSave()
                                    }
                                    val onWritableChange: (Boolean) -> Unit = { checked ->
                                        settings = settings.map { if (it.metric == s.metric) it.copy(writable = checked) else it }
                                        scheduleSave()
                                    }
                                    val onVarNameChange: (String) -> Unit = fun(v: String) {
                                        val trimmed = v.trim()
                                        settings = settings.map { if (it.metric == s.metric) it.copy(varName = v) else it }
                                        if (!isVarNameValid(trimmed)) {
                                            varNameErrors[s.metric] = "only A-Z a-z 0-9 _"
                                            return
                                        }
                                        varNameErrors.remove(s.metric)
                                        scheduleSave()
                                    }

                                    if (wide) {
                                        SettingsRow(
                                            setting = s,
                                            valueText = valueText,
                                            saving = saving,
                                            isControllable = isControllable,
                                            varNameError = varNameErrors[s.metric],
                                            onEnabledChange = onEnabledChange,
                                            onWritableChange = onWritableChange,
                                            onVarNameChange = onVarNameChange,
                                        )
                                        if (index != settings.lastIndex) {
                                            HorizontalDivider(modifier = Modifier.padding(vertical = 2.dp))
                                        }
                                    } else {
                                        SettingsCompactRow(
                                            setting = s,
                                            valueText = valueText,
                                            saving = saving,
                                            isControllable = isControllable,
                                            varNameError = varNameErrors[s.metric],
                                            onEnabledChange = onEnabledChange,
                                            onWritableChange = onWritableChange,
                                            onVarNameChange = onVarNameChange,
                                        )
                                    }
                                }
                            }
                        }
                    }
                }
            }
        }

        Spacer(modifier = Modifier.size(8.dp))
    }
}

@Composable
private fun StatusPill(
    text: String,
    ok: Boolean,
    warn: Boolean = false,
) {
    val colors = MaterialTheme.colorScheme
    val container = when {
        warn -> colors.secondaryContainer
        ok -> colors.tertiaryContainer
        else -> colors.errorContainer
    }
    val content = when {
        warn -> colors.onSecondaryContainer
        ok -> colors.onTertiaryContainer
        else -> colors.onErrorContainer
    }
    Surface(color = container, shape = RoundedCornerShape(999.dp)) {
        Text(
            text = text,
            modifier = Modifier.padding(horizontal = 10.dp, vertical = 6.dp),
            color = content,
            style = MaterialTheme.typography.labelMedium,
        )
    }
}

@Composable
private fun KeyValueRow(key: String, value: String) {
    Row(modifier = Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
        Text(key, style = MaterialTheme.typography.bodySmall, modifier = Modifier.weight(1f))
        Text(value, style = MaterialTheme.typography.bodySmall, textAlign = TextAlign.End)
    }
}

@Composable
private fun SettingsHeaderRow() {
    val style = MaterialTheme.typography.labelSmall
    val color = MaterialTheme.colorScheme.onSurfaceVariant
    Row(
        modifier = Modifier.fillMaxWidth().padding(horizontal = 6.dp, vertical = 4.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text("Metric", style = style, modifier = Modifier.weight(0.34f), color = color)
        Text("Mapping", style = style, modifier = Modifier.weight(0.38f), color = color)
        Text("Value", style = style, modifier = Modifier.weight(0.28f), color = color, textAlign = TextAlign.Start)
        Text("Writeable", style = style, modifier = Modifier.width(86.dp), color = color, textAlign = TextAlign.End)
        Text("Enabled", style = style, modifier = Modifier.width(86.dp), color = color, textAlign = TextAlign.End)
    }
}

@Composable
private fun SettingsRow(
    setting: MetricSetting,
    valueText: String,
    saving: Boolean,
    isControllable: Boolean,
    varNameError: String?,
    onEnabledChange: (Boolean) -> Unit,
    onWritableChange: (Boolean) -> Unit,
    onVarNameChange: (String) -> Unit,
) {
    val colors = MaterialTheme.colorScheme
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(10.dp),
        color = colors.surface,
        border = BorderStroke(1.dp, colors.outlineVariant.copy(alpha = 0.55f)),
    ) {
        Row(
            modifier = Modifier.fillMaxWidth().padding(horizontal = 10.dp, vertical = 8.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            Column(modifier = Modifier.weight(0.34f), verticalArrangement = Arrangement.spacedBy(4.dp)) {
                Text(
                    setting.metric,
                    style = MaterialTheme.typography.bodySmall,
                    fontFamily = FontFamily.Monospace,
                )
                if (isControllable) {
                    CapabilityTag(text = "Control")
                }
            }

            Column(modifier = Modifier.weight(0.38f), verticalArrangement = Arrangement.spacedBy(4.dp)) {
                CompactVarNameField(
                    modifier = Modifier.fillMaxWidth(),
                    value = setting.varName,
                    enabled = !saving,
                    isError = !varNameError.isNullOrBlank(),
                    onValueChange = onVarNameChange,
                )
                if (!varNameError.isNullOrBlank()) {
                    Text(varNameError, style = MaterialTheme.typography.labelSmall, color = colors.error)
                }
            }

            Box(modifier = Modifier.weight(0.28f), contentAlignment = Alignment.CenterStart) {
                MetricValuePill(valueText = valueText, enabled = setting.enabled)
            }

            Box(modifier = Modifier.width(86.dp), contentAlignment = Alignment.CenterEnd) {
                if (isControllable) {
                    CompactSwitch(
                        checked = setting.writable,
                        enabled = !saving,
                        onCheckedChange = onWritableChange,
                    )
                } else {
                    CapabilityTag(text = "Read-only")
                }
            }

            Box(modifier = Modifier.width(86.dp), contentAlignment = Alignment.CenterEnd) {
                CompactSwitch(
                    checked = setting.enabled,
                    enabled = !saving,
                    onCheckedChange = onEnabledChange,
                )
            }
        }
    }
}

@Composable
private fun SettingsCompactRow(
    setting: MetricSetting,
    valueText: String,
    saving: Boolean,
    isControllable: Boolean,
    varNameError: String?,
    onEnabledChange: (Boolean) -> Unit,
    onWritableChange: (Boolean) -> Unit,
    onVarNameChange: (String) -> Unit,
) {
    val colors = MaterialTheme.colorScheme
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(12.dp),
        color = colors.surface,
        border = BorderStroke(1.dp, colors.outlineVariant.copy(alpha = 0.6f)),
    ) {
        Column(
            modifier = Modifier.padding(12.dp),
            verticalArrangement = Arrangement.spacedBy(10.dp),
        ) {
            Row(
                modifier = Modifier.fillMaxWidth(),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                Column(modifier = Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(4.dp)) {
                    Text(
                        setting.metric,
                        style = MaterialTheme.typography.bodyMedium,
                        fontFamily = FontFamily.Monospace,
                    )
                    if (isControllable) {
                        CapabilityTag(text = "Control")
                    }
                }
                MetricValuePill(valueText = valueText, enabled = setting.enabled)
            }

            Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
                CompactVarNameField(
                    modifier = Modifier.fillMaxWidth(),
                    value = setting.varName,
                    enabled = !saving,
                    isError = !varNameError.isNullOrBlank(),
                    onValueChange = onVarNameChange,
                )
            }

            if (!varNameError.isNullOrBlank()) {
                Text(varNameError, style = MaterialTheme.typography.labelSmall, color = colors.error)
            }

            Row(
                modifier = Modifier.fillMaxWidth(),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(12.dp),
            ) {
                if (isControllable) {
                    SettingToggle(
                        modifier = Modifier.weight(1f),
                        label = "Writeable",
                        checked = setting.writable,
                        enabled = !saving,
                        onCheckedChange = onWritableChange,
                    )
                } else {
                    Surface(
                        modifier = Modifier.weight(1f),
                        shape = RoundedCornerShape(10.dp),
                        color = colors.surfaceVariant.copy(alpha = 0.4f),
                        border = BorderStroke(1.dp, colors.outlineVariant.copy(alpha = 0.55f)),
                    ) {
                        Text(
                            text = "Writeable: Read-only",
                            modifier = Modifier.padding(horizontal = 10.dp, vertical = 8.dp),
                            style = MaterialTheme.typography.labelSmall,
                            color = colors.onSurfaceVariant,
                            textAlign = TextAlign.Start,
                        )
                    }
                }
                SettingToggle(
                    modifier = Modifier.weight(1f),
                    label = "Enabled",
                    checked = setting.enabled,
                    enabled = !saving,
                    onCheckedChange = onEnabledChange,
                )
            }
        }
    }
}

@Composable
private fun MetricSummaryChip(
    label: String,
    value: String,
    modifier: Modifier = Modifier,
) {
    val colors = MaterialTheme.colorScheme
    Surface(
        modifier = modifier,
        shape = RoundedCornerShape(10.dp),
        color = colors.secondaryContainer.copy(alpha = 0.45f),
        border = BorderStroke(1.dp, colors.outlineVariant.copy(alpha = 0.55f)),
    ) {
        Row(
            modifier = Modifier.padding(horizontal = 10.dp, vertical = 7.dp),
            horizontalArrangement = Arrangement.spacedBy(8.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Text(label, style = MaterialTheme.typography.labelSmall, color = colors.onSurfaceVariant)
            Text(value, style = MaterialTheme.typography.titleSmall)
        }
    }
}

@Composable
private fun CapabilityTag(text: String) {
    val colors = MaterialTheme.colorScheme
    Surface(
        shape = RoundedCornerShape(999.dp),
        color = colors.tertiaryContainer.copy(alpha = 0.65f),
    ) {
        Text(
            text = text,
            modifier = Modifier.padding(horizontal = 8.dp, vertical = 3.dp),
            style = MaterialTheme.typography.labelSmall,
            color = colors.onTertiaryContainer,
        )
    }
}

@Composable
private fun MetricValuePill(
    valueText: String,
    enabled: Boolean,
) {
    val colors = MaterialTheme.colorScheme
    val isInactive = !enabled || valueText == "-"
    val container = if (isInactive) colors.surfaceVariant.copy(alpha = 0.45f) else colors.primaryContainer.copy(alpha = 0.75f)
    val content = if (isInactive) colors.onSurfaceVariant else colors.onPrimaryContainer
    Surface(
        shape = RoundedCornerShape(999.dp),
        color = container,
    ) {
        Text(
            text = valueText,
            modifier = Modifier.padding(horizontal = 10.dp, vertical = 5.dp),
            style = MaterialTheme.typography.labelMedium,
            color = content,
            fontFamily = FontFamily.Monospace,
        )
    }
}

@Composable
private fun SettingToggle(
    label: String,
    checked: Boolean,
    enabled: Boolean,
    onCheckedChange: (Boolean) -> Unit,
    modifier: Modifier = Modifier,
) {
    val colors = MaterialTheme.colorScheme
    Surface(
        modifier = modifier,
        shape = RoundedCornerShape(10.dp),
        color = colors.surfaceVariant.copy(alpha = 0.35f),
        border = BorderStroke(1.dp, colors.outlineVariant.copy(alpha = 0.55f)),
    ) {
        Row(
            modifier = Modifier.fillMaxWidth().padding(horizontal = 10.dp, vertical = 6.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            Text(label, style = MaterialTheme.typography.labelSmall, modifier = Modifier.weight(1f))
            CompactSwitch(
                checked = checked,
                enabled = enabled,
                onCheckedChange = onCheckedChange,
            )
        }
    }
}

@Composable
private fun CompactVarNameField(
    value: String,
    enabled: Boolean,
    isError: Boolean,
    onValueChange: (String) -> Unit,
    modifier: Modifier = Modifier,
) {
    val colors = MaterialTheme.colorScheme
    val borderColor = when {
        isError -> colors.error
        enabled -> colors.outline
        else -> colors.outline.copy(alpha = 0.6f)
    }
    val textColor = if (enabled) colors.onSurface else colors.onSurfaceVariant
    Box(
        modifier = modifier
            .heightIn(min = 34.dp)
            .clip(RoundedCornerShape(8.dp))
            .border(1.dp, borderColor, RoundedCornerShape(8.dp))
            .padding(horizontal = 8.dp, vertical = 6.dp),
        contentAlignment = Alignment.CenterStart,
    ) {
        BasicTextField(
            value = value,
            enabled = enabled,
            singleLine = true,
            onValueChange = onValueChange,
            textStyle = MaterialTheme.typography.bodySmall.copy(color = textColor),
            cursorBrush = SolidColor(colors.primary),
            modifier = Modifier.fillMaxWidth(),
        )
    }
}

@Composable
private fun CompactSwitch(
    checked: Boolean,
    enabled: Boolean,
    onCheckedChange: (Boolean) -> Unit,
) {
    Switch(
        checked = checked,
        onCheckedChange = onCheckedChange,
        enabled = enabled,
    )
}
