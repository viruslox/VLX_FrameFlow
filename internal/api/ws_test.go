package api

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewWSHub(t *testing.T) {
	hub := NewWSHub([]string{})
	assert.NotNil(t, hub)
	assert.NotNil(t, hub.clients)
	assert.NotNil(t, hub.broadcast)
}

func TestWSHubBroadcast(t *testing.T) {
	hub := NewWSHub([]string{})
	msg := []byte("test message")

	// Start a goroutine to read from the broadcast channel
	done := make(chan struct{})
	go func() {
		received := <-hub.broadcast
		assert.Equal(t, msg, received)
		close(done)
	}()

	// Send message
	hub.Broadcast(msg)

	// Wait for message to be received, with a timeout
	select {
	case <-done:
		// Success
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for message to be broadcasted")
	}
}

// We cannot easily test Run or HandleWebSocket directly without network overhead
// because they tightly couple to gorilla/websocket network connections,
// and the WSHub explicitly iterates over `*websocket.Conn` keys, writing to the actual network.
//
// However, we can test that calling Broadcast places the message correctly onto the channel.
// The rationale states: "Creating channels and broadcasting mock data can be done easily within a single test without network overhead."
