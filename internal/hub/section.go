package hub

import (
	"log/slog"
	"strings"
	"sync/atomic"
	"time"
	"unicode"

	"sofar-hyd-diag/internal/register"
)

const defaultRefreshInterval = 10 * time.Second // D-19

// Probe is an alias for register.Probe used in hub section definitions.
type Probe = register.Probe

// FormatValue is an alias for register.FormatValue used in hub result formatting.
var FormatValue = register.FormatValue

// Section represents a data section with subscribers and auto-refresh timer.
type Section struct {
	Name        string
	Probes      []register.Probe
	subscribers map[*Client]bool
	autoRefresh bool
	ticker      *time.Ticker
	timerCh     chan<- string  // sends section name on tick
	reading     atomic.Bool   // true while a read is in progress (D-24)
	interval    time.Duration
	logger      *slog.Logger
}

// newSection creates a Section with probes and timer forwarding channel.
func newSection(name string, probes []register.Probe, timerCh chan<- string, logger *slog.Logger) *Section {
	return &Section{
		Name:        name,
		Probes:      probes,
		subscribers: make(map[*Client]bool),
		autoRefresh: true,
		timerCh:     timerCh,
		interval:    defaultRefreshInterval,
		logger:      logger.With("section", name),
	}
}

// SetInterval overrides the default refresh interval (for testing).
func (s *Section) SetInterval(d time.Duration) {
	s.interval = d
}

// SubscriberCount returns the number of subscribed clients.
func (s *Section) SubscriberCount() int {
	return len(s.subscribers)
}

// addSubscriber adds a client and starts the timer if this is the first subscriber.
func (s *Section) addSubscriber(c *Client) {
	s.subscribers[c] = true
	if len(s.subscribers) == 1 && s.autoRefresh {
		s.startTimer()
	}
}

// removeSubscriber removes a client and stops the timer if no subscribers remain.
func (s *Section) removeSubscriber(c *Client) {
	delete(s.subscribers, c)
	if len(s.subscribers) == 0 {
		s.stopTimer()
	}
}

// startTimer starts the auto-refresh ticker goroutine.
func (s *Section) startTimer() {
	if s.ticker != nil {
		return // already running
	}
	s.ticker = time.NewTicker(s.interval)
	go func() {
		for range s.ticker.C {
			select {
			case s.timerCh <- s.Name:
			default:
				// timer channel full, skip this tick
			}
		}
	}()
	s.logger.Debug("auto-refresh timer started", "interval", s.interval)
}

// stopTimer stops the auto-refresh ticker.
func (s *Section) stopTimer() {
	if s.ticker != nil {
		s.ticker.Stop()
		s.ticker = nil
		s.logger.Debug("auto-refresh timer stopped")
	}
}

// pauseTimer stops the timer (for connection drop, D-28).
func (s *Section) pauseTimer() {
	s.stopTimer()
}

// resumeTimer restarts the timer if there are subscribers and auto-refresh is on (D-28).
func (s *Section) resumeTimer() {
	if len(s.subscribers) > 0 && s.autoRefresh {
		s.startTimer()
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
