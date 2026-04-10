package hub_test

import (
	"testing"

	"sofar-hyd-diag/internal/broker"
	"sofar-hyd-diag/internal/hub"
)

func TestBrokerSatisfiesInterface(t *testing.T) {
	// Compile-time check that broker.Broker satisfies hub.BrokerInterface
	var _ hub.BrokerInterface = (*broker.Broker)(nil)
}
