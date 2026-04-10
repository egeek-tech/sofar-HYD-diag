// Sofar HYD Diagnostic Tool - Application Logic

'use strict';

const STORAGE_KEY = 'sofar_connection';

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
            // Re-subscribe to active section if any
            if (App.activeSection) {
                this.send({ type: 'subscribe', section: App.activeSection });
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

    // Update content title
    var title = section.charAt(0).toUpperCase() + section.slice(1);
    $('#content-title').textContent = title;

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
    // Ignore data for non-active section
    if (msg.section !== App.activeSection) {
        return;
    }

    // Build data card using safe DOM API (T-02-11: never use innerHTML with variable data)
    var card = document.createElement('div');
    card.className = 'data-card';

    var data = msg.data || {};
    var keys = Object.keys(data);
    for (var i = 0; i < keys.length; i++) {
        var row = document.createElement('div');
        row.className = 'data-row';

        var keyEl = document.createElement('div');
        keyEl.className = 'data-row__key';
        keyEl.textContent = keys[i].replace(/_/g, ' ').toUpperCase();

        var valEl = document.createElement('div');
        valEl.className = 'data-row__value';
        valEl.textContent = data[keys[i]];

        row.appendChild(keyEl);
        row.appendChild(valEl);
        card.appendChild(row);
    }

    // Replace content body children with data card
    var body = $('#content-body');
    body.textContent = '';
    body.appendChild(card);

    // Update timestamp (D-10)
    if (msg.timestamp) {
        var ts = $('#content-timestamp');
        var d = new Date(msg.timestamp);
        ts.textContent = 'Last updated: ' + d.toLocaleTimeString();
        ts.style.display = '';
    }

    // Trigger green flash (RT-03)
    triggerFlash('success');
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
