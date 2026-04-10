// Sofar HYD Diagnostic Tool - Application Logic

'use strict';

const STORAGE_KEY = 'sofar_connection';
var PV_STORAGE_KEY = 'sofar_pv_channels';
var PV_DEFAULT_CHANNELS = 2;

var BAT_INPUTS_KEY = 'sofar_bat_inputs';
var BAT_TOWERS_KEY = 'sofar_bat_towers';
var BAT_PACKS_KEY = 'sofar_bat_packs';
var BAT_DEFAULT_INPUTS = 1;
var BAT_DEFAULT_TOWERS = 2;
var BAT_DEFAULT_PACKS = 10;

// === DOM Helpers ===

function $(sel) { return document.querySelector(sel); }
function $$(sel) { return document.querySelectorAll(sel); }

// === WSClient Class ===

class WSClient {
    constructor() {
        this.ws = null;
        this.reconnectDelay = 1000;
        this.maxReconnectDelay = 30000;
        this.handlers = {};
        this.connected = false;
    }

    connect() {
        const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
        const url = proto + '//' + location.host + '/ws';

        this.ws = new WebSocket(url);

        this.ws.onopen = () => {
            this.connected = true;
            this.reconnectDelay = 1000;
            // Re-subscribe to active section via navigateToSection so PV config
            // and auto-refresh state are synced with the (possibly restarted) backend
            if (App.activeSection) {
                navigateToSection(App.activeSection);
            }
        };

        this.ws.onmessage = (event) => {
            let msg;
            try {
                msg = JSON.parse(event.data);
            } catch (e) {
                return;
            }
            const handler = this.handlers[msg.type];
            if (handler) {
                handler(msg);
            }
        };

        this.ws.onclose = () => {
            this.connected = false;
            this.scheduleReconnect();
        };

        this.ws.onerror = () => {
            // onclose will handle reconnection
        };
    }

    scheduleReconnect() {
        const jitter = 1 + (Math.random() * 0.3);
        const delay = Math.min(this.reconnectDelay * jitter, this.maxReconnectDelay);
        setTimeout(() => this.connect(), delay);
        this.reconnectDelay = Math.min(this.reconnectDelay * 2, this.maxReconnectDelay);
    }

    send(msg) {
        if (this.ws && this.ws.readyState === WebSocket.OPEN) {
            this.ws.send(JSON.stringify(msg));
        }
    }

    on(type, handler) {
        this.handlers[type] = handler;
    }
}

// === App State ===

const App = {
    ws: null,
    activeSection: null,
    autoRefresh: true,
    connectionState: 'disconnected'
};

// === Initialization ===

document.addEventListener('DOMContentLoaded', function () {
    // Create WSClient and register handlers
    App.ws = new WSClient();
    App.ws.on('connection_state', handleConnectionState);
    App.ws.on('section_data', handleSectionData);
    App.ws.on('section_error', handleSectionError);

    // Connect WebSocket to server
    App.ws.connect();

    // Initialize connection form
    initConnectionForm();

    // Setup event listeners
    setupFormHandler();
    setupSectionNav();
    setupSidebarToggle();
    setupAutoRefreshToggle();
    initPVDropdown();
    initTopologyDropdowns();
});

// === Connection Form (CONN-01, CONN-03) ===

function initConnectionForm() {
    // Priority: localStorage > /api/defaults > hardcoded fallbacks
    const saved = loadConnectionSettings();
    if (saved) {
        $('#input-host').value = saved.host || '';
        $('#input-port').value = saved.port || '';
        $('#input-slave').value = saved.slaveId || '';
        return;
    }

    // Fetch from /api/defaults (D-14)
    fetch('/api/defaults')
        .then(function (res) { return res.json(); })
        .then(function (data) {
            if (!$('#input-host').value) $('#input-host').value = data.host || '';
            if (!$('#input-port').value) $('#input-port').value = data.port || '';
            if (!$('#input-slave').value) $('#input-slave').value = data.slave_id || '';
        })
        .catch(function () {
            // Hardcoded fallbacks
            if (!$('#input-host').value) $('#input-host').value = '10.5.99.29';
            if (!$('#input-port').value) $('#input-port').value = '4192';
            if (!$('#input-slave').value) $('#input-slave').value = '1';
        });
}

// === Form Submission (CONN-02) ===

function setupFormHandler() {
    $('#connection-form').addEventListener('submit', function (e) {
        e.preventDefault();

        if (App.connectionState === 'connected' || App.connectionState === 'reconnecting' || App.connectionState === 'connecting') {
            // Disconnect
            App.ws.send({ type: 'disconnect' });
            return;
        }

        // Validate form (D-34)
        if (!validateForm()) {
            return;
        }

        var host = $('#input-host').value.trim();
        var port = parseInt($('#input-port').value, 10);
        var slaveId = parseInt($('#input-slave').value, 10);

        // Save to localStorage (CONN-03)
        saveConnectionSettings(host, port, slaveId);

        // Send connect command
        App.userInitiatedConnect = true;  // D-23: flag for auto-navigate
        App.ws.send({
            type: 'connect',
            host: host,
            port: port,
            slave_id: slaveId
        });
    });
}

// === Form Validation (D-34) ===

function validateForm() {
    var valid = true;
    var host = $('#input-host').value.trim();
    var port = $('#input-port').value.trim();
    var slave = $('#input-slave').value.trim();

    // Validate host
    var hostError = $('#error-host');
    var hostInput = $('#input-host');
    if (!host) {
        hostError.textContent = 'IP address is required';
        hostInput.classList.add('form-input--error');
        valid = false;
    } else if (!/^(\d{1,3}\.){3}\d{1,3}$|^[a-zA-Z0-9.-]+$/.test(host)) {
        hostError.textContent = 'Enter a valid IP address or hostname';
        hostInput.classList.add('form-input--error');
        valid = false;
    } else {
        hostError.textContent = '';
        hostInput.classList.remove('form-input--error');
    }

    // Validate port
    var portError = $('#error-port');
    var portInput = $('#input-port');
    var portNum = parseInt(port, 10);
    if (!port || isNaN(portNum) || portNum < 1 || portNum > 65535) {
        portError.textContent = 'Port must be between 1 and 65535';
        portInput.classList.add('form-input--error');
        valid = false;
    } else {
        portError.textContent = '';
        portInput.classList.remove('form-input--error');
    }

    // Validate slave ID
    var slaveError = $('#error-slave');
    var slaveInput = $('#input-slave');
    var slaveNum = parseInt(slave, 10);
    if (!slave || isNaN(slaveNum) || slaveNum < 1 || slaveNum > 247) {
        slaveError.textContent = 'Slave ID must be between 1 and 247';
        slaveInput.classList.add('form-input--error');
        valid = false;
    } else {
        slaveError.textContent = '';
        slaveInput.classList.remove('form-input--error');
    }

    return valid;
}

// === Section Navigation (RT-01, D-17, D-18) ===

function setupSectionNav() {
    $$('.section-nav__item:not(.section-nav__item--disabled)').forEach(function (btn) {
        btn.addEventListener('click', function () {
            var section = btn.getAttribute('data-section');
            navigateToSection(section);
        });
    });
}

function navigateToSection(section) {
    // Update active nav item highlight
    $$('.section-nav__item').forEach(function (item) {
        item.classList.remove('section-nav__item--active');
    });
    var activeBtn = $('.section-nav__item[data-section="' + section + '"]');
    if (activeBtn) {
        activeBtn.classList.add('section-nav__item--active');
    }

    // Set active section
    App.activeSection = section;

    // Update content title (per UI-SPEC copywriting contract)
    var sectionTitles = {
        system: 'System',
        grid: 'Grid',
        eps: 'EPS',
        pv: 'PV',
        battery: 'Battery',
        bms: 'BMS',
        stats: 'Statistics'
    };
    var title = sectionTitles[section] || section.charAt(0).toUpperCase() + section.slice(1);
    $('#content-title').textContent = title;

    // Show/hide PV dropdown based on active section (per UI-SPEC)
    var pvSelect = $('#pv-channel-select');
    if (section === 'pv') {
        pvSelect.style.display = '';
    } else {
        pvSelect.style.display = 'none';
    }

    // Show/hide topology dropdowns based on active section (per UI-SPEC)
    var topoControls = $('#topology-controls');
    if (section === 'bms') {
        topoControls.style.display = '';
    } else {
        topoControls.style.display = 'none';
    }

    // Show loading spinner
    showLoading();

    // Show auto-refresh button
    var autoBtn = $('#btn-auto-refresh');
    autoBtn.style.display = '';
    updateAutoRefreshButton();

    // Hide timestamp
    $('#content-timestamp').style.display = 'none';

    // Send subscribe via WebSocket (D-17; auto-unsubscribes previous per D-18; triggers immediate read per D-20)
    App.ws.send({ type: 'subscribe', section: section });

    // Sync PV channel config with backend
    if (section === 'pv') {
        var pvChannels = loadPVChannels() || PV_DEFAULT_CHANNELS;
        App.ws.send({
            type: 'configure',
            section: 'pv',
            config: { channels: pvChannels }
        });
    }

    // Sync BMS topology config with backend
    if (section === 'bms') {
        var batInputs = loadTopologyValue(BAT_INPUTS_KEY) || BAT_DEFAULT_INPUTS;
        var batTowers = loadTopologyValue(BAT_TOWERS_KEY) || BAT_DEFAULT_TOWERS;
        var batPacks = loadTopologyValue(BAT_PACKS_KEY) || BAT_DEFAULT_PACKS;
        App.ws.send({
            type: 'configure',
            section: 'bms',
            config: { bat_inputs: batInputs, bat_towers: batTowers, bat_packs: batPacks }
        });
    }
}

// === Sidebar Toggle ===

function setupSidebarToggle() {
    $('#sidebar-toggle').addEventListener('click', function () {
        var sidebar = $('#sidebar');
        var content = $('#content');
        var icon = $('#sidebar-toggle-icon');

        sidebar.classList.toggle('sidebar--collapsed');
        content.classList.toggle('content--sidebar-collapsed');

        // Use Unicode characters via textContent (safe, no innerHTML)
        if (sidebar.classList.contains('sidebar--collapsed')) {
            icon.textContent = '\u00BB'; // right-pointing double angle quotation mark
        } else {
            icon.textContent = '\u00AB'; // left-pointing double angle quotation mark
        }
    });
}

// === Auto-Refresh Toggle (RT-02, D-35) ===

function setupAutoRefreshToggle() {
    $('#btn-auto-refresh').addEventListener('click', function () {
        App.autoRefresh = !App.autoRefresh;
        updateAutoRefreshButton();

        if (App.activeSection) {
            App.ws.send({
                type: 'auto_refresh',
                section: App.activeSection,
                enabled: App.autoRefresh
            });
        }
    });
}

function updateAutoRefreshButton() {
    var btn = $('#btn-auto-refresh');
    if (App.autoRefresh) {
        btn.textContent = 'Auto (10s)';
        btn.classList.add('btn-auto-refresh--active');
    } else {
        btn.textContent = 'Auto';
        btn.classList.remove('btn-auto-refresh--active');
    }
}

// === Message Handlers ===

function handleConnectionState(msg) {
    App.connectionState = msg.state;
    var dot = $('#status-dot');
    var text = $('#status-text');
    var btn = $('#btn-connect');

    // Remove all state classes from dot
    dot.className = 'status-dot';

    switch (msg.state) {
        case 'connected':
            dot.classList.add('status-dot--connected');
            text.textContent = 'Connected';
            btn.textContent = 'Disconnect';
            btn.className = 'btn btn--disconnect';
            btn.disabled = false;
            setFormInputsDisabled(true);
            // D-23: Auto-navigate to System section on user-initiated connect
            if (App.userInitiatedConnect) {
                App.userInitiatedConnect = false;
                navigateToSection('system');
            }
            break;
        case 'connecting':
            dot.classList.add('status-dot--connecting');
            text.textContent = 'Connecting...';
            btn.textContent = 'Disconnect';
            btn.className = 'btn btn--disconnect';
            btn.disabled = false;
            setFormInputsDisabled(true);
            break;
        case 'reconnecting':
            dot.classList.add('status-dot--reconnecting');
            text.textContent = 'Reconnecting...';
            btn.textContent = 'Disconnect';
            btn.className = 'btn btn--disconnect';
            btn.disabled = false;
            setFormInputsDisabled(true);
            break;
        case 'disconnected':
        case 'dormant':
        default:
            dot.classList.add('status-dot--disconnected');
            text.textContent = 'Disconnected';
            btn.textContent = 'Connect';
            btn.className = 'btn btn--connect';
            btn.disabled = false;
            setFormInputsDisabled(false);
            // Show connection error if present (e.g. connection refused)
            if (msg.error) {
                showConnectionError(msg.error);
                triggerFlash('error');
            }
            break;
    }
}

function handleSectionData(msg) {
    if (msg.section !== App.activeSection) return;

    var body = $('#content-body');
    body.textContent = '';

    // Render grouped data
    if (msg.groups && msg.groups.length > 0) {
        var container = renderGroupedData(msg);
        body.appendChild(container);
    }

    // Update timestamp
    if (msg.timestamp) {
        var ts = $('#content-timestamp');
        var d = new Date(msg.timestamp);
        ts.textContent = 'Last updated: ' + d.toLocaleTimeString();
        ts.style.display = '';
    }

    triggerFlash('success');
}

// === Grouped Data Renderer (Phase 3) ===

function renderGroupedData(msg) {
    var container = document.createElement('div');
    var gridContainer = null;

    var groups = msg.groups || [];
    for (var i = 0; i < groups.length; i++) {
        var group = groups[i];

        // Type-based widget dispatch
        if (group.type === 'bitmap') {
            gridContainer = null;
            container.appendChild(renderBitmapGroup(group));
        } else if (group.type === 'protection') {
            gridContainer = null;
            container.appendChild(renderProtectionGroup(group));
        } else if (group.layout === 'column') {
            if (!gridContainer) {
                gridContainer = document.createElement('div');
                gridContainer.className = 'group-grid';
                container.appendChild(gridContainer);
            }
            gridContainer.appendChild(renderGroupCard(group));
        } else {
            gridContainer = null;
            container.appendChild(renderGroupCard(group));
        }
    }

    // Render faults card (system section only, per D-09, D-11)
    if (Array.isArray(msg.faults)) {
        gridContainer = null;
        container.appendChild(renderFaultCard(msg.faults));
    }

    return container;
}

function renderGroupCard(group) {
    var card = document.createElement('div');
    card.className = 'group-card';

    // Group name heading
    var heading = document.createElement('h3');
    heading.className = 'group-card__name';
    heading.textContent = group.name;
    card.appendChild(heading);

    // Separator
    var sep = document.createElement('hr');
    sep.className = 'group-card__separator';
    card.appendChild(sep);

    // Data rows
    var body = document.createElement('div');
    body.className = 'group-card__body';

    var items = group.items || {};
    var keys = Object.keys(items);
    for (var i = 0; i < keys.length; i++) {
        var row = document.createElement('div');
        row.className = 'data-row-h';

        var keyEl = document.createElement('span');
        keyEl.className = 'data-row-h__key';
        keyEl.textContent = keys[i].toUpperCase();

        var valEl = document.createElement('span');
        valEl.className = 'data-row-h__value';
        valEl.textContent = items[keys[i]];

        row.appendChild(keyEl);
        row.appendChild(valEl);
        body.appendChild(row);
    }

    card.appendChild(body);
    return card;
}

function renderFaultCard(faults) {
    var card = document.createElement('div');

    if (faults && faults.length > 0) {
        // Active faults -- amber warning styling
        card.className = 'fault-card fault-card--active';

        var heading = document.createElement('h3');
        heading.className = 'fault-card__heading';
        heading.textContent = '\u26A0 Faults';  // warning sign
        card.appendChild(heading);

        var list = document.createElement('div');
        list.className = 'fault-card__list';

        for (var i = 0; i < faults.length; i++) {
            var item = document.createElement('div');
            item.className = 'fault-card__item';
            item.textContent = '\u2022 ' + faults[i].name;  // bullet
            list.appendChild(item);
        }

        card.appendChild(list);
    } else {
        // No faults -- green success styling
        card.className = 'fault-card fault-card--clear';

        var heading = document.createElement('h3');
        heading.className = 'fault-card__heading';
        heading.textContent = '\u2713 Faults';  // checkmark
        card.appendChild(heading);

        var clearText = document.createElement('p');
        clearText.className = 'fault-card__clear-text';
        clearText.textContent = 'No active faults';
        card.appendChild(clearText);
    }

    return card;
}

function handleSectionError(msg) {
    // Ignore errors for non-active section
    if (msg.section !== App.activeSection) {
        return;
    }

    // Show error text using textContent (safe)
    var body = $('#content-body');
    body.textContent = '';
    var errEl = document.createElement('p');
    errEl.className = 'error-message';
    errEl.textContent = 'Failed to read ' + msg.section + ' data. Check connection and retry.';
    body.appendChild(errEl);

    // Trigger red flash (RT-04)
    triggerFlash('error');
}

// === UI Helpers ===

function showConnectionError(errMsg) {
    var body = $('#content-body');
    body.textContent = '';
    var errEl = document.createElement('p');
    errEl.className = 'error-message';
    // Show a user-friendly message with the technical detail
    var friendly = 'Connection failed';
    if (errMsg.indexOf('refused') !== -1) {
        friendly = 'Connection refused — inverter not reachable on this port';
    } else if (errMsg.indexOf('timeout') !== -1 || errMsg.indexOf('i/o timeout') !== -1) {
        friendly = 'Connection timed out — check IP address';
    }
    errEl.textContent = friendly;
    body.appendChild(errEl);
}

function showLoading() {
    var body = $('#content-body');
    body.textContent = '';

    var loading = document.createElement('div');
    loading.className = 'loading';

    var spinner = document.createElement('div');
    spinner.className = 'loading__spinner';
    loading.appendChild(spinner);

    var text = document.createElement('p');
    text.className = 'loading__text';
    text.textContent = 'Loading...';
    loading.appendChild(text);

    body.appendChild(loading);
}

function triggerFlash(type) {
    var content = $('#content');
    // Remove existing flash classes
    content.classList.remove('content-flash--success', 'content-flash--error');
    // Force reflow to restart transition
    void content.offsetWidth;
    // Add flash class (instant color via 0ms transition)
    content.classList.add('content-flash--' + type);
    // Use double requestAnimationFrame to ensure the browser has painted the flash color
    // before removing the class, which triggers the 1000ms fade-out transition
    requestAnimationFrame(function () {
        requestAnimationFrame(function () {
            content.classList.remove('content-flash--' + type);
        });
    });
}

function setFormInputsDisabled(disabled) {
    $('#input-host').disabled = disabled;
    $('#input-port').disabled = disabled;
    $('#input-slave').disabled = disabled;
}

// === localStorage (CONN-03) ===

function saveConnectionSettings(host, port, slaveId) {
    try {
        localStorage.setItem(STORAGE_KEY, JSON.stringify({ host: host, port: port, slaveId: slaveId }));
    } catch (e) {
        // Private browsing or storage full; silently ignore
    }
}

function loadConnectionSettings() {
    try {
        var raw = localStorage.getItem(STORAGE_KEY);
        if (!raw) return null;
        return JSON.parse(raw);
    } catch (e) {
        return null;
    }
}

// === PV Channel Dropdown (D-14, D-15, D-16) ===

function initPVDropdown() {
    var select = $('#pv-channel-select');
    // Populate options 2-16
    for (var i = 2; i <= 16; i++) {
        var opt = document.createElement('option');
        opt.value = String(i);
        opt.textContent = i + ' channels';
        select.appendChild(opt);
    }

    // Load default: localStorage > /api/defaults > 2
    var stored = loadPVChannels();
    if (stored) {
        select.value = String(stored);
    }

    // Also try /api/defaults if no localStorage value
    if (!stored) {
        fetch('/api/defaults')
            .then(function(res) { return res.json(); })
            .then(function(data) {
                if (data.pv_channels && !loadPVChannels()) {
                    select.value = String(data.pv_channels);
                    PV_DEFAULT_CHANNELS = data.pv_channels;
                }
            })
            .catch(function() {});
    }

    // On change: send configure, save to localStorage
    select.addEventListener('change', function() {
        var channels = parseInt(select.value, 10);
        savePVChannels(channels);
        App.ws.send({
            type: 'configure',
            section: 'pv',
            config: { channels: channels }
        });
    });
}

function loadPVChannels() {
    try {
        var val = localStorage.getItem(PV_STORAGE_KEY);
        if (val) return parseInt(val, 10);
        return null;
    } catch(e) { return null; }
}

function savePVChannels(channels) {
    try {
        localStorage.setItem(PV_STORAGE_KEY, String(channels));
    } catch(e) {}
}

// === Topology Dropdowns (BAT-06, D-10, D-11, D-12) ===

function initTopologyDropdowns() {
    var inputsSel = $('#bat-inputs-select');
    var towersSel = $('#bat-towers-select');
    var packsSel = $('#bat-packs-select');

    // Populate options
    for (var i = 1; i <= 2; i++) {
        var opt = document.createElement('option');
        opt.value = String(i);
        opt.textContent = String(i);
        inputsSel.appendChild(opt);
    }
    for (var i = 1; i <= 4; i++) {
        var opt = document.createElement('option');
        opt.value = String(i);
        opt.textContent = String(i);
        towersSel.appendChild(opt);
    }
    for (var i = 4; i <= 10; i++) {
        var opt = document.createElement('option');
        opt.value = String(i);
        opt.textContent = String(i);
        packsSel.appendChild(opt);
    }

    // Load defaults: localStorage > /api/defaults > hardcoded
    var storedInputs = loadTopologyValue(BAT_INPUTS_KEY);
    var storedTowers = loadTopologyValue(BAT_TOWERS_KEY);
    var storedPacks = loadTopologyValue(BAT_PACKS_KEY);

    if (storedInputs) inputsSel.value = String(storedInputs);
    if (storedTowers) towersSel.value = String(storedTowers);
    if (storedPacks) packsSel.value = String(storedPacks);

    // Also try /api/defaults if no localStorage values
    if (!storedInputs || !storedTowers || !storedPacks) {
        fetch('/api/defaults')
            .then(function(res) { return res.json(); })
            .then(function(data) {
                if (!loadTopologyValue(BAT_INPUTS_KEY) && data.bat_inputs) {
                    inputsSel.value = String(data.bat_inputs);
                    BAT_DEFAULT_INPUTS = data.bat_inputs;
                }
                if (!loadTopologyValue(BAT_TOWERS_KEY) && data.bat_towers) {
                    towersSel.value = String(data.bat_towers);
                    BAT_DEFAULT_TOWERS = data.bat_towers;
                }
                if (!loadTopologyValue(BAT_PACKS_KEY) && data.bat_packs) {
                    packsSel.value = String(data.bat_packs);
                    BAT_DEFAULT_PACKS = data.bat_packs;
                }
            })
            .catch(function() {});
    }

    // On change: save to localStorage and send configure
    inputsSel.addEventListener('change', sendTopologyConfigure);
    towersSel.addEventListener('change', sendTopologyConfigure);
    packsSel.addEventListener('change', sendTopologyConfigure);
}

function sendTopologyConfigure() {
    var inputs = parseInt($('#bat-inputs-select').value, 10);
    var towers = parseInt($('#bat-towers-select').value, 10);
    var packs = parseInt($('#bat-packs-select').value, 10);
    saveTopologyValue(BAT_INPUTS_KEY, inputs);
    saveTopologyValue(BAT_TOWERS_KEY, towers);
    saveTopologyValue(BAT_PACKS_KEY, packs);
    App.ws.send({
        type: 'configure',
        section: 'bms',
        config: { bat_inputs: inputs, bat_towers: towers, bat_packs: packs }
    });
}

function loadTopologyValue(key) {
    try {
        var val = localStorage.getItem(key);
        if (val) return parseInt(val, 10);
        return null;
    } catch(e) { return null; }
}

function saveTopologyValue(key, val) {
    try { localStorage.setItem(key, String(val)); }
    catch(e) {}
}

// === Bitmap Grid Renderer (BAT-05, D-06, D-07, D-08) ===

function renderBitmapGroup(group) {
    var card = document.createElement('div');
    card.className = 'group-card';

    // Heading
    var heading = document.createElement('h3');
    heading.className = 'group-card__name';
    heading.textContent = group.name;
    card.appendChild(heading);

    var sep = document.createElement('hr');
    sep.className = 'group-card__separator';
    card.appendChild(sep);

    var body = document.createElement('div');
    body.className = 'bitmap-group';

    var bm = group.bitmap;
    if (!bm) { card.appendChild(body); return card; }

    // Detected topology label (D-14)
    if (bm.detected_topology) {
        var detLabel = document.createElement('div');
        detLabel.className = 'bitmap-detected';
        detLabel.textContent = 'Detected: ' + bm.detected_topology;

        // Mismatch warning
        if (bm.mismatch) {
            var warn = document.createElement('span');
            warn.className = 'bitmap-mismatch';
            warn.textContent = ' \u26A0 Config mismatch';
            detLabel.appendChild(warn);
        }
        body.appendChild(detLabel);
    }

    // Grid rows (one per tower)
    for (var t = 0; t < bm.towers; t++) {
        var rowWrap = document.createElement('div');
        rowWrap.className = 'bitmap-row';

        var rowLabel = document.createElement('span');
        rowLabel.className = 'bitmap-row__label';
        rowLabel.textContent = 'Tower ' + (t + 1);
        rowWrap.appendChild(rowLabel);

        var grid = document.createElement('div');
        grid.className = 'bitmap-grid';
        grid.style.gridTemplateColumns = 'repeat(' + bm.packs_per_tower + ', 28px)';

        var online = bm.online[t] || 0;
        for (var p = 0; p < bm.packs_per_tower; p++) {
            var cell = document.createElement('div');
            var isOnline = (online >> p) & 1;
            cell.className = 'bitmap-cell ' + (isOnline ? 'bitmap-cell--online' : 'bitmap-cell--offline');
            cell.textContent = String(p + 1);
            grid.appendChild(cell);
        }
        rowWrap.appendChild(grid);
        body.appendChild(rowWrap);
    }

    // Legend
    var legend = document.createElement('div');
    legend.className = 'bitmap-legend';

    var onItem = document.createElement('div');
    onItem.className = 'bitmap-legend__item';
    var onSwatch = document.createElement('span');
    onSwatch.className = 'bitmap-legend__swatch bitmap-cell--online';
    onItem.appendChild(onSwatch);
    var onLabel = document.createElement('span');
    onLabel.className = 'bitmap-legend__label';
    onLabel.textContent = 'Online';
    onItem.appendChild(onLabel);
    legend.appendChild(onItem);

    var offItem = document.createElement('div');
    offItem.className = 'bitmap-legend__item';
    var offSwatch = document.createElement('span');
    offSwatch.className = 'bitmap-legend__swatch bitmap-cell--offline';
    offItem.appendChild(offSwatch);
    var offLabel = document.createElement('span');
    offLabel.className = 'bitmap-legend__label';
    offLabel.textContent = 'Offline';
    offItem.appendChild(offLabel);
    legend.appendChild(offItem);

    body.appendChild(legend);
    card.appendChild(body);
    return card;
}

// === Protection & Alarms Renderer (D-04) ===

function renderProtectionGroup(group) {
    var card = document.createElement('div');
    var items = group.items || {};
    var keys = Object.keys(items);

    // Check if any non-zero values
    var hasActive = false;
    for (var i = 0; i < keys.length; i++) {
        if (items[keys[i]] !== '0x0000') {
            hasActive = true;
            break;
        }
    }

    if (hasActive) {
        card.className = 'fault-card fault-card--active';
        var heading = document.createElement('h3');
        heading.className = 'fault-card__heading';
        heading.textContent = '\u26A0 Protection & Alarms';
        card.appendChild(heading);

        var list = document.createElement('div');
        list.className = 'fault-card__list';
        for (var i = 0; i < keys.length; i++) {
            if (items[keys[i]] !== '0x0000') {
                var item = document.createElement('div');
                item.className = 'fault-card__item';
                item.textContent = '\u2022 ' + keys[i] + ': ' + items[keys[i]];
                list.appendChild(item);
            }
        }
        card.appendChild(list);
    } else {
        card.className = 'fault-card fault-card--clear';
        var heading = document.createElement('h3');
        heading.className = 'fault-card__heading';
        heading.textContent = '\u2713 Protection & Alarms';
        card.appendChild(heading);

        var clearText = document.createElement('p');
        clearText.className = 'fault-card__clear-text';
        clearText.textContent = 'No active protections or alarms';
        card.appendChild(clearText);
    }

    return card;
}
