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
	MsgTypeSectionData = "section_data"
	MsgTypeSectionErr  = "section_error"
	MsgTypeState       = "connection_state"
)

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
}

// OutboundMessage represents a message from server to client.
// Type + Section identify the message; Data carries pre-formatted key-value pairs.
type OutboundMessage struct {
	Type      string            `json:"type"`
	Section   string            `json:"section,omitempty"`
	Data      map[string]string `json:"data,omitempty"`
	State     string            `json:"state,omitempty"`
	Error     string            `json:"error,omitempty"`
	Timestamp string            `json:"timestamp,omitempty"`
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
