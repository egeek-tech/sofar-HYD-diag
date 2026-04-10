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
