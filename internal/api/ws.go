package api

import (
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type WSHub struct {
	clients   map[*websocket.Conn]bool
	broadcast chan []byte
	mutex     sync.Mutex
	upgrader  websocket.Upgrader
}

func NewWSHub(allowedOrigins []string) *WSHub {
	var sanitizedOrigins []string
	for _, origin := range allowedOrigins {
		if origin == "*" {
			continue // Skip wildcards for security
		}
		sanitizedOrigins = append(sanitizedOrigins, origin)
	}

	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			if origin == "" {
				// No origin means not a cross-origin request
				return true
			}

			// Parse the origin string to get host and scheme
			originURL, err := url.Parse(origin)
			if err != nil {
				return false
			}

			// Allow if the origin matches the request host (gorilla's default safe behavior)
			if strings.EqualFold(originURL.Host, r.Host) {
				return true
			}

			// Check against the allowed origins
			for _, allowed := range sanitizedOrigins {
				if allowed == origin {
					return true
				}
			}

			return false
		},
	}

	return &WSHub{
		clients:   make(map[*websocket.Conn]bool),
		broadcast: make(chan []byte),
		upgrader:  upgrader,
	}
}

func (h *WSHub) Run() {
	for {
		message := <-h.broadcast
		h.mutex.Lock()
		for client := range h.clients {
			err := client.WriteMessage(websocket.TextMessage, message)
			if err != nil {
				log.Printf("websocket error: %v", err)
				client.Close()
				delete(h.clients, client)
			}
		}
		h.mutex.Unlock()
	}
}

func (h *WSHub) Broadcast(message []byte) {
	h.broadcast <- message
}

func (h *WSHub) HandleWebSocket(c *gin.Context, tm *TicketManager) {
	ticket := c.Query("ticket")
	if ticket == "" || !tm.ValidateTicket(ticket) {
		c.JSON(401, gin.H{"error": "unauthorized"})
		return
	}

	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("failed to upgrade connection: %v", err)
		return
	}

	h.mutex.Lock()
	h.clients[conn] = true
	h.mutex.Unlock()

	// Handle disconnects
	defer func() {
		h.mutex.Lock()
		delete(h.clients, conn)
		h.mutex.Unlock()
		conn.Close()
	}()

	// Keep connection alive and read messages (even if ignored)
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
}
