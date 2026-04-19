package hub

import (
	"context"
	"log/slog"
	"strings"
	"sync/atomic"
	"unicode"

	"sofar-hyd-diag/internal/register"
)

// Probe is an alias for register.Probe used in hub section definitions.
type Probe = register.Probe

// FormatValue is an alias for register.FormatValue used in hub result formatting.
var FormatValue = register.FormatValue

// FormatRawValue is an alias for register.FormatRawValue used in hub pack metadata.
var FormatRawValue = register.FormatRawValue

// Section represents a data section with subscribers and browser-driven reads.
// No backend timer — all reads are triggered by browser WebSocket messages (REFR-01).
type Section struct {
	Name         string
	Probes       []register.Probe      // flattened from Groups for read requests
	Groups       []register.ProbeGroup // source of truth for grouped sections (D-06)
	BatchPlan    register.BatchPlan    // pre-computed batch read plan (D-02)
	SpanTracker  *SpanTracker          // per-section span degradation tracker (D-01)
	faultSection bool                  // true for "system" section (reads fault registers)
	readOnce     bool                  // D-09: when true, skip re-reads after initial successful read
	hasReadOnce  bool                  // D-09: true after first successful read completes
	subscribers  map[*Client]bool
	readCancel   context.CancelFunc // cancels in-progress read goroutine (D-02)
	reading      atomic.Bool        // true while a read is in progress (D-24)
	logger       *slog.Logger
}

// newSection creates a Section with probes.
func newSection(name string, probes []register.Probe, logger *slog.Logger) *Section {
	return &Section{
		Name:        name,
		Probes:      probes,
		subscribers: make(map[*Client]bool),
		logger:      logger.With("section", name),
	}
}

// newGroupedSection creates a Section backed by ProbeGroups.
// Probes are flattened from the groups for backward-compatible read request generation.
// The faultSection flag is set for "system" to enable fault register reading.
func newGroupedSection(name string, groups []register.ProbeGroup, logger *slog.Logger) *Section {
	sectionLogger := logger.With("section", name)
	return &Section{
		Name:         name,
		Probes:       flattenProbeGroups(groups),
		Groups:       groups,
		BatchPlan:    register.AnalyzeBatchPlan(groups),
		SpanTracker:  NewSpanTracker(DefaultDegradationThreshold, sectionLogger),
		faultSection: name == "system",
		subscribers:  make(map[*Client]bool),
		logger:       sectionLogger,
	}
}

// flattenProbeGroups extracts all probes from groups into a single flat slice.
func flattenProbeGroups(groups []register.ProbeGroup) []register.Probe {
	var probes []register.Probe
	for _, g := range groups {
		probes = append(probes, g.Probes...)
	}
	return probes
}

// SubscriberCount returns the number of subscribed clients.
func (s *Section) SubscriberCount() int {
	return len(s.subscribers)
}

// addSubscriber adds a client to this section.
func (s *Section) addSubscriber(c *Client) {
	s.subscribers[c] = true
}

// removeSubscriber removes a client from this section.
func (s *Section) removeSubscriber(c *Client) {
	delete(s.subscribers, c)
}

// cancelRead cancels any in-progress read goroutine for this section (D-02).
func (s *Section) cancelRead() {
	if s.readCancel != nil {
		s.readCancel()
		s.readCancel = nil
	}
}

// toSnakeCase converts a probe name like "Inverter SN" to "inverter_sn".
// Lowercase, spaces replaced with underscores, non-alphanumeric/underscore stripped.
func toSnakeCase(name string) string {
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, " ", "_")
	var b strings.Builder
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
