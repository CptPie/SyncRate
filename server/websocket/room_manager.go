package websocket

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Client represents a connected user in a rating room
type Client struct {
	ID       string          // User ID
	Username string          // Username for display
	Conn     *websocket.Conn // WebSocket connection
	RoomID   string          // Which room they're in
	LastSeen time.Time       // For cleanup
}

// Room represents an active rating room with connected clients
type Room struct {
	ID            string             // Room code
	CreatorID     string             // Creator user ID
	Clients       map[string]*Client // Connected clients by user ID
	CurrentSongID *uint              // Current song being rated
	VideoTime     float64            // Current video position in seconds
	IsPlaying     bool               // Video play state
	LastActivity  time.Time          // For cleanup
	Mutex         sync.RWMutex       // Thread safety
}

// RoomManager manages all active rating rooms
type RoomManager struct {
	rooms   map[string]*Room // Active rooms by room ID
	clients map[string]*Client // All connected clients by user ID
	mutex   sync.RWMutex      // Thread safety
}

// Message types for WebSocket communication
type MessageType string

const (
	MsgJoinRoom      MessageType = "join_room"
	MsgLeaveRoom     MessageType = "leave_room"
	MsgSongChange    MessageType = "song_change"
	MsgVideoSync     MessageType = "video_sync"
	MsgVoteUpdate    MessageType = "vote_update"
	MsgUserUpdate    MessageType = "user_update"
	MsgNextSong      MessageType = "next_song"
	MsgRoomSettings  MessageType = "room_settings"
	MsgError         MessageType = "error"
)

// WebSocket message structure
type WSMessage struct {
	Type      MessageType     `json:"type"`
	Data      json.RawMessage `json:"data"`
	Timestamp time.Time       `json:"timestamp"`
}

// Specific message data structures
type JoinRoomData struct {
	RoomID   string `json:"room_id"`
	UserID   string `json:"user_id"`
	Username string `json:"username"`
}

type ArtistData struct {
	ArtistID       uint   `json:"ArtistID"`
	NameOriginal   string `json:"NameOriginal"`
	NameEnglish    string `json:"NameEnglish"`
	PrimaryColor   string `json:"PrimaryColor"`
	SecondaryColor string `json:"SecondaryColor"`
}

type UnitData struct {
	UnitID         uint   `json:"UnitID"`
	NameOriginal   string `json:"NameOriginal"`
	NameEnglish    string `json:"NameEnglish"`
	PrimaryColor   string `json:"PrimaryColor"`
	SecondaryColor string `json:"SecondaryColor"`
}

type AlbumData struct {
	AlbumID      uint   `json:"AlbumID"`
	NameOriginal string `json:"NameOriginal"`
	NameEnglish  string `json:"NameEnglish"`
	Type         string `json:"Type"`
}

type SongChangeData struct {
	SongID            uint              `json:"song_id"`
	SongTitleOriginal string            `json:"song_title_original"`
	SongTitleEnglish  string            `json:"song_title_english"`
	EmbedURL          string            `json:"embed_url"`
	ThumbnailURL      string            `json:"thumbnail_url"`
	Artists           []ArtistData      `json:"artists"`
	Units             []UnitData        `json:"units"`
	Albums            []AlbumData       `json:"albums"`
	Category          string            `json:"category"`
	IsCover           bool              `json:"is_cover"`
	ExistingVotes     []VoteUpdateData  `json:"existing_votes"`
}

type VideoSyncData struct {
	Time      float64 `json:"time"`
	IsPlaying bool    `json:"is_playing"`
}

type VoteUpdateData struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Rating   int    `json:"rating"`
	Comment  string `json:"comment"`
}

type UserUpdateData struct {
	Users []UserInfo `json:"users"`
}

type UserInfo struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}

type RoomSettingsData struct {
	VideoSyncEnabled bool `json:"video_sync_enabled"`
}

// NewRoomManager creates a new room manager
func NewRoomManager() *RoomManager {
	return &RoomManager{
		rooms:   make(map[string]*Room),
		clients: make(map[string]*Client),
	}
}

// CreateRoom creates a new rating room
func (rm *RoomManager) CreateRoom(roomID, creatorID, creatorUsername string) *Room {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	room := &Room{
		ID:           roomID,
		CreatorID:    creatorID,
		Clients:      make(map[string]*Client),
		LastActivity: time.Now(),
	}

	rm.rooms[roomID] = room
	log.Printf("Created room %s by user %s", roomID, creatorID)
	return room
}

// JoinRoom adds a client to a room
func (rm *RoomManager) JoinRoom(roomID, userID, username string, conn *websocket.Conn) error {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	// Check if user is already in another room
	if existingClient, exists := rm.clients[userID]; exists {
		// Remove from previous room
		rm.removeClientFromRoom(existingClient)
	}

	// Get or create room
	room, exists := rm.rooms[roomID]
	if !exists {
		return &RoomError{Message: "Room not found"}
	}

	// Create client
	client := &Client{
		ID:       userID,
		Username: username,
		Conn:     conn,
		RoomID:   roomID,
		LastSeen: time.Now(),
	}

	// Add to room and global clients
	room.Mutex.Lock()
	room.Clients[userID] = client
	room.LastActivity = time.Now()
	room.Mutex.Unlock()

	rm.clients[userID] = client

	// Notify other users
	rm.broadcastUserUpdate(room)

	log.Printf("User %s joined room %s", username, roomID)
	return nil
}

// LeaveRoom removes a client from their current room
func (rm *RoomManager) LeaveRoom(userID string) {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	if client, exists := rm.clients[userID]; exists {
		rm.removeClientFromRoom(client)
		delete(rm.clients, userID)
	}
}

// BroadcastToRoom sends a message to all clients in a room
func (rm *RoomManager) BroadcastToRoom(roomID string, message WSMessage) {
	rm.mutex.RLock()
	room, exists := rm.rooms[roomID]
	rm.mutex.RUnlock()

	if !exists {
		return
	}

	room.Mutex.RLock()
	defer room.Mutex.RUnlock()

	messageBytes, _ := json.Marshal(message)

	for _, client := range room.Clients {
		if err := client.Conn.WriteMessage(websocket.TextMessage, messageBytes); err != nil {
			log.Printf("Error sending message to client %s: %v", client.ID, err)
			// Client will be cleaned up by connection handler
		}
	}
}

// GetRoom returns a room by ID
func (rm *RoomManager) GetRoom(roomID string) (*Room, bool) {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()
	room, exists := rm.rooms[roomID]
	return room, exists
}

// CleanupInactiveRooms removes rooms with no active clients
func (rm *RoomManager) CleanupInactiveRooms(timeout time.Duration) {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	now := time.Now()
	for roomID, room := range rm.rooms {
		room.Mutex.RLock()
		clientCount := len(room.Clients)
		lastActivity := room.LastActivity
		room.Mutex.RUnlock()

		if clientCount == 0 && now.Sub(lastActivity) > timeout {
			delete(rm.rooms, roomID)
			log.Printf("Cleaned up inactive room %s", roomID)
		}
	}
}

// Helper functions

func (rm *RoomManager) removeClientFromRoom(client *Client) {
	if room, exists := rm.rooms[client.RoomID]; exists {
		room.Mutex.Lock()
		delete(room.Clients, client.ID)
		room.LastActivity = time.Now()
		room.Mutex.Unlock()

		// Close connection
		client.Conn.Close()

		// Notify other users
		rm.broadcastUserUpdate(room)

		log.Printf("User %s left room %s", client.Username, client.RoomID)
	}
}

func (rm *RoomManager) broadcastUserUpdate(room *Room) {
	room.Mutex.RLock()
	users := make([]UserInfo, 0, len(room.Clients))
	for _, client := range room.Clients {
		users = append(users, UserInfo{
			ID:       client.ID,
			Username: client.Username,
		})
	}
	room.Mutex.RUnlock()

	data, _ := json.Marshal(UserUpdateData{Users: users})
	message := WSMessage{
		Type:      MsgUserUpdate,
		Data:      data,
		Timestamp: time.Now(),
	}

	go rm.BroadcastToRoom(room.ID, message)
}

// Custom error type
type RoomError struct {
	Message string
}

func (e *RoomError) Error() string {
	return e.Message
}