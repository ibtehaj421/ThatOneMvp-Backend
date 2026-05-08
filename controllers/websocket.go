package controllers

import (
	"log"
	"net/http"
	"sync"
	"strconv"

	"health/anam/backend/database"
	"health/anam/backend/models"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// Allow all origins for local testing
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true 
	},
}

// Client represents a single connected user
type Client struct {
	Conn          *websocket.Conn
	UserID        uint
	AppointmentID uint
}

// Global state to keep track of active connections
var (
	clients = make(map[*Client]bool)
	mutex   = sync.Mutex{} // Prevents race conditions when multiple users connect/disconnect
)

func HandleWebSocket(c *gin.Context) {
	// For WebSockets, we typically pass data via query parameters since browsers 
	// can't easily send standard HTTP Auth headers in a new WebSocket() call.
	apptIDStr := c.Query("appointment_id")
	userIDStr := c.Query("user_id") // Note: In production, extract this from the token instead!

	apptID, _ := strconv.ParseUint(apptIDStr, 10, 32)
	userID, _ := strconv.ParseUint(userIDStr, 10, 32)

	// 1. Upgrade the HTTP connection to a WebSocket
	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("WebSocket Upgrade Error:", err)
		return
	}
	defer ws.Close()

	// 2. Register the new client
	client := &Client{
		Conn:          ws,
		UserID:        uint(userID),
		AppointmentID: uint(apptID),
	}

	mutex.Lock()
	clients[client] = true
	mutex.Unlock()

	log.Printf("User %d connected to Appointment %d", client.UserID, client.AppointmentID)

	// 3. Listen for incoming messages in an infinite loop
	for {
		var incomingMsg struct {
			Message string `json:"message"`
		}

		err := ws.ReadJSON(&incomingMsg)
		if err != nil {
			log.Printf("User %d disconnected", client.UserID)
			mutex.Lock()
			delete(clients, client)
			mutex.Unlock()
			break
		}

		// Save message to database
		dbMsg := models.AppointmentMessage{
			AppointmentID: client.AppointmentID,
			SenderID:      client.UserID,
			Message:       incomingMsg.Message,
		}
		
		if err := database.DB.Create(&dbMsg).Error; err != nil {
			log.Println("Failed to save message:", err)
			continue
		}

		// Preload sender info so the frontend knows who sent it
		database.DB.Preload("Sender").First(&dbMsg, dbMsg.ID)

		// 4. Broadcast the message to everyone in THIS appointment room
		mutex.Lock()
		for c := range clients {
			if c.AppointmentID == client.AppointmentID {
				err := c.Conn.WriteJSON(dbMsg)
				if err != nil {
					log.Printf("Error sending message to user %d: %v", c.UserID, err)
					c.Conn.Close()
					delete(clients, c)
				}
			}
		}
		mutex.Unlock()
	}
}