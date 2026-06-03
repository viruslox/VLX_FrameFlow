package api

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTicketManager(t *testing.T) {
	tm := NewTicketManager()

	// Test valid ticket
	ticket, err := tm.GenerateTicket()
	assert.NoError(t, err)
	assert.NotEmpty(t, ticket)

	isValid := tm.ValidateTicket(ticket)
	assert.True(t, isValid)

	// Test single use
	isStillValid := tm.ValidateTicket(ticket)
	assert.False(t, isStillValid)

	// Test invalid ticket
	isInvalidValid := tm.ValidateTicket("invalid-ticket")
	assert.False(t, isInvalidValid)

	// Test expiration
	expTicket, _ := tm.GenerateTicket()
	// manually expire it for test
	tm.mu.Lock()
	tm.tickets[expTicket] = time.Now().Add(-1 * time.Second)
	tm.mu.Unlock()

	isExpiredValid := tm.ValidateTicket(expTicket)
	assert.False(t, isExpiredValid)
}
