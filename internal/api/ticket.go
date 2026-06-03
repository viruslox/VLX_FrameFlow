package api

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

type TicketManager struct {
	mu      sync.Mutex
	tickets map[string]time.Time
}

func NewTicketManager() *TicketManager {
	return &TicketManager{
		tickets: make(map[string]time.Time),
	}
}

func (tm *TicketManager) GenerateTicket() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	ticket := hex.EncodeToString(b)

	tm.mu.Lock()
	defer tm.mu.Unlock()

	tm.cleanup()
	tm.tickets[ticket] = time.Now().Add(10 * time.Second)

	return ticket, nil
}

func (tm *TicketManager) ValidateTicket(ticket string) bool {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	tm.cleanup()

	exp, exists := tm.tickets[ticket]
	if !exists {
		return false
	}

	delete(tm.tickets, ticket) // single-use

	return time.Now().Before(exp)
}

// cleanup removes expired tickets. Must be called with lock held.
func (tm *TicketManager) cleanup() {
	now := time.Now()
	for t, exp := range tm.tickets {
		if now.After(exp) {
			delete(tm.tickets, t)
		}
	}
}
