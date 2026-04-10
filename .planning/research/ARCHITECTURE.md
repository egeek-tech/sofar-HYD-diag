# Architecture Patterns

**Domain:** Go single-binary diagnostic web tool with WebSocket + Modbus backend
**Researched:** 2026-04-10

## Recommended Architecture

### Overview: Hub-and-Spoke with Connection Broker

The application has a fundamental constraint: **one Modbus TCP connection, many WebSocket clients**. This drives the entire architecture toward a central broker that serializes Modbus access and fans out results to connected browsers.

```
                      +-------------------+
                      |   Browser (N)     |
                      | JS + WebSocket    |
                      +--------+----------+
                               |
                      WebSocket (JSON frames)
                               |
                      +--------v----------+
                      |   HTTP/WS Server  |
                      |   (net/http +     |
                      |    gorilla/ws)    |
                      +--------+----------+
                               |
                      +--------v----------+
                      |   Hub (fan-out)   |
                      |   - client set    |
                      |   - broadcast     |
                      |   - subscriptions |
                      +--------+----------+
                               |
                      +--------v----------+
                      |  Modbus Broker    |
                      |  - conn lock      |
                      |  - request queue  |
                      |  - retry logic    |
                      +--------+----------+
                               |
                      TCP (Modbus RTU-over-TCP)
                               |
                      +--------v----------+
                      |  Sofar HYD        |
                      |  Inverter         |
                      +-------------------+
```

### Component Boundaries

| Component | Responsibility | Communicates With | Package |
|-----------|---------------|-------------------|---------|
| **Modbus Transport** | TCP/RTU framing, CRC, read/write raw registers | Inverter (TCP) | `modbus` |
| **Modbus Broker** | Connection lifecycle, request serialization, retry, timing delays | Modbus Transport (calls), Hub (results) | `modbus` |
| **Register Definitions** | Maps register addresses to names, types, scales, sections | Broker (queried by), API (section lookup) | `register` |
| **Hub** | WebSocket client registry, fan-out broadcast, subscription tracking | WS Server (clients), Broker (requests) | `hub` |
| **API / WS Server** | HTTP routes, WebSocket upgrade, embedded static files, message routing | Hub (delegates), Browser (serves) | `api` |
| **Frontend** | Section navigation, WebSocket client, DOM rendering | API (WebSocket + HTTP) | `web/` (embedded) |

### Data Flow

**Startup flow:**
1. User launches binary with optional flags (port, default inverter IP)
2. HTTP server starts, serves embedded frontend on `/`
3. No Modbus connection yet -- connection is lazy, initiated from browser

**Connection flow:**
1. Browser sends `{"type":"connect","host":"10.5.99.29","port":4192,"slaveId":1}` via WebSocket
2. Hub routes to Broker
3. Broker dials TCP, stores connection, responds with `{"type":"connected"}` or `{"type":"error","msg":"..."}`
4. Hub broadcasts connection state to all clients

**Section data flow (lazy loading):**
1. Browser navigates to "Grid Connected" tab, sends `{"type":"subscribe","section":"grid"}`
2. Hub registers this client's interest in "grid" section
3. Hub asks Broker to read "grid" registers (looked up from Register Definitions)
4. Broker acquires connection lock, reads registers sequentially (500ms spacing), releases lock
5. Broker returns parsed results to Hub
6. Hub broadcasts `{"type":"data","section":"grid","values":{...}}` to subscribed clients

**Auto-refresh flow:**
1. Browser sends `{"type":"refresh","enabled":true,"interval":3000}`
2. Hub sets up a ticker for this client's active section
3. On each tick: re-read section registers via Broker, broadcast results
4. Client sends `{"type":"refresh","enabled":false}` to stop

**Battery pack selection flow (critical sequence):**
1. Browser sends `{"type":"selectPack","input":0,"tower":1,"pack":3}`
2. Hub routes to Broker as a **priority request** (acquires exclusive lock)
3. Broker: write pack selection to 0x9020 (func 0x10)
4. Broker: wait 1000ms (BMS settle time -- hardware requirement)
5. Broker: read pack data registers (0x9044+, cell voltages, temperatures)
6. Broker: release lock
7. Hub broadcasts pack data to requesting client

**Disconnect flow:**
1. Browser sends `{"type":"disconnect"}`
2. Broker closes TCP connection, clears state
3. Hub broadcasts `{"type":"disconnected"}` to all clients

### Package Structure

```
modbus_reader/
  main.go                    # Entry point: flags, wire components, start server
  modbus/
    transport.go             # readHoldingRegisters{TCP,RTU}, writeRegister, CRC16
    broker.go                # Connection state, request queue, lock, retry, timing
    types.go                 # RegisterRead, RegisterWrite, response types
  register/
    definitions.go           # All register definitions: address, name, type, scale, unit
    sections.go              # Group registers into sections (system, grid, eps, pv, battery, stats)
  hub/
    hub.go                   # Client registry, broadcast, subscription tracking
    client.go                # Per-client goroutines (readPump, writePump)
    messages.go              # WebSocket message types (JSON)
  api/
    server.go                # HTTP server setup, routes, WebSocket upgrade handler
    handlers.go              # REST endpoints (GET /api/status for connection state)
    embed.go                 # go:embed directives for web/ assets
  web/
    index.html               # Single-page app shell
    css/
      style.css              # Desktop-optimized layout
    js/
      app.js                 # WebSocket client, section navigation, DOM updates
```

## Patterns to Follow

### Pattern 1: Gorilla WebSocket Hub (Fan-Out)

The canonical Go WebSocket pattern. One Hub goroutine owns the client map; clients register/unregister via channels. No mutex on the client map -- the Hub goroutine is the sole accessor.

**What:** Central goroutine that manages all connected WebSocket clients
**When:** Always -- this is the backbone of the real-time architecture

```go
type Hub struct {
    clients    map[*Client]bool
    broadcast  chan []byte
    register   chan *Client
    unregister chan *Client
    broker     *modbus.Broker
}

func (h *Hub) Run() {
    for {
        select {
        case client := <-h.register:
            h.clients[client] = true
        case client := <-h.unregister:
            if _, ok := h.clients[client]; ok {
                delete(h.clients, client)
                close(client.send)
            }
        case message := <-h.broadcast:
            for client := range h.clients {
                select {
                case client.send <- message:
                default:
                    close(client.send)
                    delete(h.clients, client)
                }
            }
        }
    }
}
```

### Pattern 2: Modbus Broker with sync.Mutex (Connection Serialization)

Modbus is serial -- only one request can be in flight at a time. The Broker holds a `sync.Mutex` around all Modbus operations and enforces the 500ms inter-read delay.

**What:** Serialized access to the single Modbus connection
**When:** Every Modbus read/write operation

```go
type Broker struct {
    mu       sync.Mutex
    conn     net.Conn
    addr     string
    slaveID  byte
    useRTU   bool
    connected bool
}

func (b *Broker) ReadSection(section string) (map[string]interface{}, error) {
    b.mu.Lock()
    defer b.mu.Unlock()

    if !b.connected {
        return nil, errors.New("not connected")
    }

    regs := register.ForSection(section)
    results := make(map[string]interface{})
    for _, reg := range regs {
        data, err := b.readWithRetry(reg.Address, reg.Count)
        if err != nil {
            results[reg.Name] = map[string]interface{}{"error": err.Error()}
        } else {
            results[reg.Name] = reg.Parse(data)
        }
        time.Sleep(500 * time.Millisecond)
    }
    return results, nil
}
```

### Pattern 3: Section-Based Subscription (Lazy Loading)

Clients subscribe to sections, not individual registers. The Hub only requests data for sections that have at least one subscriber. This avoids hammering the inverter with reads nobody is looking at.

**What:** Only query registers when a client is viewing that section
**When:** All data fetching -- never read speculatively

```go
type Client struct {
    hub     *Hub
    conn    *websocket.Conn
    send    chan []byte
    section string  // Currently subscribed section ("system", "grid", etc.)
    refresh bool    // Auto-refresh enabled
}
```

### Pattern 4: go:embed for Single Binary

Embed the entire `web/` directory into the Go binary at compile time.

**What:** No external file dependencies at runtime
**When:** Always -- this is a project requirement

```go
//go:embed web/*
var webFS embed.FS

func setupRoutes(mux *http.ServeMux) {
    webContent, _ := fs.Sub(webFS, "web")
    mux.Handle("/", http.FileServer(http.FS(webContent)))
}
```

### Pattern 5: Per-Client Read/Write Pumps

Each WebSocket client gets two goroutines: readPump (browser -> server) and writePump (server -> browser). This is the standard gorilla/websocket pattern that handles ping/pong, close frames, and buffered writes correctly.

**What:** Dedicated goroutines per client for clean WebSocket lifecycle
**When:** Every client connection

```go
func (c *Client) readPump() {
    defer func() {
        c.hub.unregister <- c
        c.conn.Close()
    }()
    c.conn.SetReadLimit(maxMessageSize)
    c.conn.SetReadDeadline(time.Now().Add(pongWait))
    c.conn.SetPongHandler(func(string) error {
        c.conn.SetReadDeadline(time.Now().Add(pongWait))
        return nil
    })
    for {
        _, message, err := c.conn.ReadMessage()
        if err != nil {
            break
        }
        c.hub.handleMessage(c, message)
    }
}

func (c *Client) writePump() {
    ticker := time.NewTicker(pingPeriod)
    defer func() {
        ticker.Stop()
        c.conn.Close()
    }()
    for {
        select {
        case message, ok := <-c.send:
            if !ok {
                c.conn.WriteMessage(websocket.CloseMessage, []byte{})
                return
            }
            c.conn.SetWriteDeadline(time.Now().Add(writeWait))
            c.conn.WriteMessage(websocket.TextMessage, message)
        case <-ticker.C:
            c.conn.SetWriteDeadline(time.Now().Add(writeWait))
            if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
                return
            }
        }
    }
}
```

### Pattern 6: Typed Register Definitions (Data-Driven)

Move register addresses, data types, scales, and section assignments into structured definitions. This replaces the hardcoded probe slices in main.go.

**What:** Declarative register map that drives all data reading
**When:** Central to the entire data pipeline

```go
type Register struct {
    Name    string
    Address uint16
    Count   uint16
    Type    DataType  // U16, S16, ASCII, Bitmap
    Unit    string
    Scale   float64
    Section string    // "system", "grid", "eps", "pv", "battery", "stats"
}

type DataType int
const (
    U16 DataType = iota
    S16
    ASCII
    Bitmap
)

func ForSection(section string) []Register { ... }
```

## Anti-Patterns to Avoid

### Anti-Pattern 1: One WebSocket Per Section
**What:** Opening separate WebSocket connections for each section tab
**Why bad:** Wastes connections, complicates state management, browser limits (6 connections per host)
**Instead:** Single WebSocket, multiplex sections via JSON message types

### Anti-Pattern 2: Polling from Frontend
**What:** Using setInterval + fetch() instead of WebSocket push
**Why bad:** Adds latency, wastes bandwidth, loses instant feedback on connection state changes
**Instead:** WebSocket with server-initiated push on data ready

### Anti-Pattern 3: Goroutine-Per-Register Read
**What:** Spawning a goroutine for each register read
**Why bad:** Modbus is serial -- goroutines would all block on the mutex anyway, adding overhead for no parallelism. Worse, without careful ordering you lose the 500ms delay guarantee.
**Instead:** Sequential reads within the Broker, single goroutine per section request

### Anti-Pattern 4: Global Connection Variable
**What:** Storing the `net.Conn` as a package-level variable accessed from multiple goroutines
**Why bad:** Race conditions, unclear ownership, impossible to test
**Instead:** Broker struct owns the connection, accessed only through its methods

### Anti-Pattern 5: Putting Business Logic in WebSocket Handlers
**What:** Parsing Modbus responses, applying scales, formatting values in the API layer
**Why bad:** Untestable, duplicated logic, tight coupling
**Instead:** Register definitions handle parsing; API layer just marshals JSON

### Anti-Pattern 6: Blocking the Hub Goroutine on Modbus Reads
**What:** Having the Hub directly call Broker.ReadSection() synchronously
**Why bad:** Blocks all client registration/unregistration/broadcast while waiting for Modbus (could be 10+ seconds for a section)
**Instead:** Hub dispatches read requests to a separate goroutine, results come back via channel

## Detailed Component Design

### Modbus Broker -- The Critical Component

The Broker is the most architecturally significant component because it enforces the hardware constraints:

```
Constraints:
- Single TCP connection to inverter
- 500ms minimum between reads (hardware timing)
- 1000ms settle time after BMS pack switch write
- Max 60 registers per read
- 3 retries with reconnect on failure
- 10s timeout per read
```

**State machine:**
```
DISCONNECTED --connect()--> CONNECTING --success--> CONNECTED
CONNECTED --error/close--> DISCONNECTED
CONNECTED --read/write--> BUSY --> CONNECTED (after delay)
```

**Request queue approach:** Rather than a simple mutex, consider a channel-based request queue. This allows the Broker goroutine to enforce timing delays naturally and prioritize write operations (pack selection) over reads.

```go
type request struct {
    section  string
    packSel  *PackSelection  // nil for reads, non-nil for write-then-read
    response chan<- response
}

type response struct {
    section string
    data    map[string]interface{}
    err     error
}
```

### Hub -- Subscription-Aware Broadcasting

The Hub must track which section each client is viewing to implement lazy loading. When no client is subscribed to a section, no registers for that section are read.

**Key behaviors:**
- When client subscribes to a section: immediate read, then periodic refresh if enabled
- When client unsubscribes: cancel any pending refresh timer for that client
- When client disconnects: clean up subscription, cancel timers
- Broadcast is section-scoped: only send data to clients subscribed to that section

### Frontend -- Single Page, Tab Navigation

Six tabs corresponding to backend sections. Only the active tab triggers data fetching.

```
+------------------------------------------------------------------+
| [Connect] IP: [10.5.99.29] Port: [4192] Slave: [1]  [Disconnect] |
+------------------------------------------------------------------+
| System | Grid | EPS | PV | Battery | Statistics |  [Auto-Refresh] |
+------------------------------------------------------------------+
|                                                                    |
|  (Section content with register values)                            |
|  (Green flash on update, red flash on error)                       |
|                                                                    |
+------------------------------------------------------------------+
```

Battery section has sub-navigation:
```
Battery tab:
  [Global] [BMS Global] [Pack Details]
  
Pack Details sub-panel:
  Input: [1] Tower: [2] Pack: [3]  [Load Pack]
  
  Cell Voltages    | Temperatures | Info
  Cell 1: 3.298V   | Temp 1: 22.5C | SN: ...
  Cell 2: 3.301V   | Temp 2: 23.0C | Cycles: 164
  ...
```

## WebSocket Message Protocol

All messages are JSON. Direction indicated as C->S (client to server) or S->C (server to client).

**Connection:**
```json
C->S: {"type":"connect","host":"10.5.99.29","port":4192,"slaveId":1,"mode":"tcp"}
S->C: {"type":"connected"}
S->C: {"type":"error","msg":"connection refused"}
C->S: {"type":"disconnect"}
S->C: {"type":"disconnected"}
```

**Section subscription:**
```json
C->S: {"type":"subscribe","section":"grid"}
S->C: {"type":"data","section":"grid","values":{"Grid frequency":{"value":50.01,"unit":"Hz"},...},"ts":"..."}
S->C: {"type":"data","section":"grid","error":"read timeout"}
C->S: {"type":"unsubscribe"}
```

**Auto-refresh:**
```json
C->S: {"type":"refresh","enabled":true}
C->S: {"type":"refresh","enabled":false}
```

**Battery pack selection:**
```json
C->S: {"type":"selectPack","input":0,"tower":1,"pack":3}
S->C: {"type":"packData","input":0,"tower":1,"pack":3,"values":{...}}
```

**Configuration (topology):**
```json
C->S: {"type":"config","pvChannels":2,"batteryInputs":1,"towersPerInput":2,"packsPerTower":10}
S->C: {"type":"configAck","config":{...}}
```

## Scalability Considerations

| Concern | 1 Client | 5 Clients | Notes |
|---------|----------|-----------|-------|
| Modbus load | 1 section refresh | Same -- section reads are shared | Multiple clients on same section get same broadcast |
| WebSocket memory | ~50KB | ~250KB | Negligible for desktop tool |
| Refresh conflicts | None | Lock contention | Queue serializes; clients may see slightly stale data |
| Pack selection | Exclusive | Must serialize | Only one pack selection can happen at a time; queue others |

This is a diagnostic tool, not a SaaS product. Practically it will serve 1-2 browser tabs. The architecture handles multiple clients cleanly but does not need to optimize for scale.

## Build Order (Dependencies)

This ordering reflects what depends on what. Each layer can be built and tested before the one above it.

```
Layer 0: modbus/transport.go     -- Extract from main.go, pure protocol code
Layer 0: register/definitions.go -- Pure data, no dependencies
    |
    v
Layer 1: modbus/broker.go       -- Uses transport + register definitions
    |
    v
Layer 2: hub/                    -- Uses broker for data, manages clients
    |
    v
Layer 3: api/                    -- Uses hub, serves frontend
Layer 3: web/                    -- Frontend (can develop in parallel with hub)
    |
    v
Layer 4: main.go                 -- Wires everything together
```

**Phase implications for roadmap:**

1. **Extract modbus package** (Layer 0) -- Lift transport functions from main.go into `modbus/transport.go`. Pure refactor, testable against real hardware or mock connection. Register definitions can be built in parallel.

2. **Build Broker** (Layer 1) -- Connection management, locking, retry logic, timing. Depends on transport layer. Testable with mock connection.

3. **Build Hub + WebSocket** (Layer 2-3) -- Hub pattern, client management, message routing. API server with WebSocket upgrade. Can stub the Broker for initial development.

4. **Build Frontend** (Layer 3) -- HTML/JS/CSS. Can develop against a mock WebSocket server initially. The battery sub-navigation is the most complex UI piece.

5. **Integration** (Layer 4) -- Wire components in main.go, embed frontend, test end-to-end.

## Key Architectural Decision: gorilla/websocket vs nhooyr/websocket vs stdlib

**Use `github.com/gorilla/websocket`** because:
- Most battle-tested Go WebSocket library (archived but stable, no security issues)
- The Hub pattern (readPump/writePump per client) is well-documented and understood
- Simpler API for this use case than nhooyr/websocket
- Single dependency, no transitive deps
- Note: gorilla/websocket was archived in 2022 but has been revived under community maintenance. Even if using the archived version, it remains production-ready for a local diagnostic tool.

**Confidence: HIGH** -- gorilla/websocket is the de facto standard for Go WebSocket servers and the patterns are well-established.

## Key Architectural Decision: net/http vs chi vs gin

**Use `net/http` (stdlib)** because:
- Only 3-4 routes needed (/, /ws, /api/status, maybe /api/config)
- No middleware complexity warranting a framework
- Zero dependencies beyond gorilla/websocket
- Go 1.22+ has improved routing with method patterns

**Confidence: HIGH** -- This is a simple routing scenario that does not benefit from a framework.

## Sources

- Go `embed` package: standard library, stable since Go 1.16 (HIGH confidence)
- gorilla/websocket Hub pattern: canonical example in gorilla/websocket repo (HIGH confidence)
- Modbus protocol constraints: derived from existing main.go and PROJECT.md (HIGH confidence -- verified against real hardware)
- sync.Mutex for connection serialization: standard Go concurrency pattern (HIGH confidence)
