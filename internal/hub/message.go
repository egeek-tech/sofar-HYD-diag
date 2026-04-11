package hub

import "time"

// WebSocket message type constants define the JSON protocol contract.
// Inbound types flow from browser client to server.
// Outbound types flow from server to browser client.
const (
	MsgTypeSubscribe   = "subscribe"
	MsgTypeUnsubscribe = "unsubscribe"
	MsgTypeConnect     = "connect"
	MsgTypeDisconnect  = "disconnect"
	MsgTypeRefresh     = "refresh"
	MsgTypeAutoRefresh = "auto_refresh"
	MsgTypeConfigure   = "configure"
	MsgTypeSectionData = "section_data"
	MsgTypeSectionErr  = "section_error"
	MsgTypeState       = "connection_state"
	MsgTypeSelectPack  = "select_pack"
	MsgTypePackData    = "pack_data"
	MsgTypePackError   = "pack_error"
)

// GroupData represents a rendered probe group in the outbound message (D-02).
type GroupData struct {
	Name   string            `json:"name"`
	Layout string            `json:"layout,omitempty"`
	Type   string            `json:"type,omitempty"`   // "bitmap" or "protection" or "" for standard
	Items  map[string]string `json:"items,omitempty"`  // omitempty: not present for bitmap groups
	Bitmap *BitmapData       `json:"bitmap,omitempty"` // populated when Type="bitmap"
}

// BitmapData carries BMS online/offline bitmap state for the topology widget (D-08).
type BitmapData struct {
	Towers           int      `json:"towers"`
	PacksPerTower    int      `json:"packs_per_tower"`
	Online           []uint16 `json:"online"`
	DetectedTopology string   `json:"detected_topology,omitempty"`
	Mismatch         bool     `json:"mismatch"`
}

// FaultEntry represents a single active fault (D-11).
type FaultEntry struct {
	Name string `json:"name"`
}

// ConfigPayload carries section-specific configuration (D-15).
type ConfigPayload struct {
	Channels  int `json:"channels,omitempty"`
	BatInputs int `json:"bat_inputs,omitempty"`
	BatTowers int `json:"bat_towers,omitempty"`
	BatPacks  int `json:"bat_packs,omitempty"`
}

// InboundMessage represents a message from client to server.
// Sent via WebSocket as JSON. Type determines which fields are relevant.
type InboundMessage struct {
	Type    string `json:"type"`
	Section string `json:"section,omitempty"`
	// Connect-specific fields
	Host    string `json:"host,omitempty"`
	Port    int    `json:"port,omitempty"`
	SlaveID int    `json:"slave_id,omitempty"`
	// Auto-refresh toggle
	Enabled *bool `json:"enabled,omitempty"`
	// Configure payload (D-15)
	Config *ConfigPayload `json:"config,omitempty"`
	// Pack selection fields (select_pack)
	Input int `json:"input,omitempty"`
	Tower int `json:"tower,omitempty"`
	Pack  int `json:"pack,omitempty"`
}

// OutboundMessage represents a message from server to client.
// Type + Section identify the message; Data carries pre-formatted key-value pairs.
type OutboundMessage struct {
	Type      string            `json:"type"`
	Section   string            `json:"section,omitempty"`
	Data      map[string]string `json:"data,omitempty"`      // legacy flat sections
	Groups    []GroupData       `json:"groups,omitempty"`     // grouped data (D-02)
	Faults    []FaultEntry      `json:"faults"`               // fault list (D-11); never omit so frontend always renders fault card
	State     string            `json:"state,omitempty"`
	Error     string            `json:"error,omitempty"`
	Timestamp string            `json:"timestamp,omitempty"`
}

// PackDataMessage is the outbound message for pack-level data (select_pack response).
// Separate from OutboundMessage to carry the richer pack payload per the UI-SPEC contract.
type PackDataMessage struct {
	Type      string      `json:"type"`
	Section   string      `json:"section"`
	Input     int         `json:"input"`
	Tower     int         `json:"tower"`
	Pack      int         `json:"pack"`
	Groups    []PackGroup `json:"groups"`
	Timestamp string      `json:"timestamp"`
}

// PackGroup represents a group within a pack data response.
type PackGroup struct {
	Name    string            `json:"name"`
	Layout  string            `json:"layout,omitempty"`
	Type    string            `json:"type,omitempty"`
	Items   map[string]string `json:"items,omitempty"`
	// Cell grid specific (type="cell_grid")
	Cells        []int `json:"cells,omitempty"`           // raw millivolt values for 24 cells
	MaxCell      int   `json:"max_cell,omitempty"`        // register 0x9069 value in mV
	MinCell      int   `json:"min_cell,omitempty"`        // register 0x906A value in mV
	MaxCellIndex int   `json:"max_cell_index,omitempty"`  // 1-based index of max cell
	MinCellIndex int   `json:"min_cell_index,omitempty"`  // 1-based index of min cell
	// Temperature specific
	TempRaw []int `json:"temp_raw,omitempty"` // raw temp values (x10) for color coding
	// Pack status specific (type="pack_status")
	Alarm       int      `json:"alarm,omitempty"`
	Protection  int      `json:"protection,omitempty"`
	Fault       int      `json:"fault,omitempty"`
	Alarm2      int      `json:"alarm2,omitempty"`
	Protection2 int      `json:"protection2,omitempty"`
	Fault2      int      `json:"fault2,omitempty"`
	Decoded     []string `json:"decoded,omitempty"`
	// Balance specific (type="balance")
	BalanceBitmap int `json:"balance_bitmap,omitempty"`
}

// PackErrorMessage is the outbound message for pack read errors.
type PackErrorMessage struct {
	Type    string `json:"type"`
	Section string `json:"section"`
	Input   int    `json:"input"`
	Tower   int    `json:"tower"`
	Pack    int    `json:"pack"`
	Error   string `json:"error"`
}

// NewGroupedSectionData creates a section_data outbound message with grouped data and optional faults.
func NewGroupedSectionData(section string, groups []GroupData, faults []FaultEntry) OutboundMessage {
	return OutboundMessage{
		Type:      MsgTypeSectionData,
		Section:   section,
		Groups:    groups,
		Faults:    faults,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
}

// NewSectionData creates a section_data outbound message.
func NewSectionData(section string, data map[string]string) OutboundMessage {
	return OutboundMessage{
		Type:      MsgTypeSectionData,
		Section:   section,
		Data:      data,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
}

// NewSectionError creates a section_error outbound message.
func NewSectionError(section string, errMsg string) OutboundMessage {
	return OutboundMessage{
		Type:      MsgTypeSectionErr,
		Section:   section,
		Error:     errMsg,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
}

// NewStateMessage creates a connection_state outbound message.
// If errMsg is non-empty, it is included so the client can display the reason for disconnection.
func NewStateMessage(state string, errMsg string) OutboundMessage {
	return OutboundMessage{
		Type:  MsgTypeState,
		State: state,
		Error: errMsg,
	}
}
