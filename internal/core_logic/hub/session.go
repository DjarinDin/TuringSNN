// Package hub - Session management for multi-user support
package hub

import (
	"sync"
	"time"
)

// SessionType represents the type of client connected
type SessionType string

const (
	SessionTypeDesktop SessionType = "desktop"
	SessionTypeBrowser SessionType = "browser"
	SessionTypeMobile  SessionType = "mobile"
	SessionTypeAPI     SessionType = "api"
)

// Permissions defines what a session is allowed to do
type Permissions struct {
	CanReset          bool // Can reset the cortex
	CanChangeDopamine bool // Can modify dopamine levels
	CanControlSim     bool // Can control simulation
	ReadOnly          bool // Can only observe
}

// ViewSettings stores per-session view preferences
type ViewSettings struct {
	FocusNeuron int // Which neuron is focused
	FocusLayer  int // Which layer is focused
}

// Session represents a client connection
type Session struct {
	ID           string
	Type         SessionType
	Permissions  Permissions
	ViewSettings ViewSettings

	// Connection info
	RemoteAddr  string
	ConnectedAt time.Time
	LastSeen    time.Time

	// Statistics
	MessagesSent     uint64
	MessagesReceived uint64

	mu sync.RWMutex
}

// SessionManager manages all active sessions
type SessionManager struct {
	sessions map[string]*Session
	mu       sync.RWMutex
}

// NewSessionManager creates a new session manager
func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions: make(map[string]*Session),
	}
}

// CreateSession creates a new session
func (sm *SessionManager) CreateSession(id string, sessionType SessionType, addr string) *Session {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Default permissions (can be customized later)
	permissions := Permissions{
		CanReset:          true,
		CanChangeDopamine: true,
		CanControlSim:     true,
		ReadOnly:          false,
	}

	session := &Session{
		ID:           id,
		Type:         sessionType,
		Permissions:  permissions,
		ViewSettings: ViewSettings{FocusNeuron: 0, FocusLayer: 1},
		RemoteAddr:   addr,
		ConnectedAt:  time.Now(),
		LastSeen:     time.Now(),
	}

	sm.sessions[id] = session
	return session
}

// GetSession retrieves a session by ID
func (sm *SessionManager) GetSession(id string) (*Session, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session, exists := sm.sessions[id]
	return session, exists
}

// RemoveSession removes a session
func (sm *SessionManager) RemoveSession(id string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	delete(sm.sessions, id)
}

// GetAllSessions returns all active sessions
func (sm *SessionManager) GetAllSessions() []*Session {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	sessions := make([]*Session, 0, len(sm.sessions))
	for _, session := range sm.sessions {
		sessions = append(sessions, session)
	}
	return sessions
}

// UpdateLastSeen updates the last seen timestamp
func (s *Session) UpdateLastSeen() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.LastSeen = time.Now()
}

// IncrementMessagesSent increments the sent message counter
func (s *Session) IncrementMessagesSent() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.MessagesSent++
}

// IncrementMessagesReceived increments the received message counter
func (s *Session) IncrementMessagesReceived() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.MessagesReceived++
}

// SetFocusNeuron sets which neuron the session is focused on
func (s *Session) SetFocusNeuron(neuron int, layer int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ViewSettings.FocusNeuron = neuron
	s.ViewSettings.FocusLayer = layer
}

// GetFocusNeuron gets the focused neuron
func (s *Session) GetFocusNeuron() (neuron int, layer int) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ViewSettings.FocusNeuron, s.ViewSettings.FocusLayer
}

// HasPermission checks if the session has a specific permission
func (s *Session) HasPermission(permType string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	switch permType {
	case "reset":
		return s.Permissions.CanReset
	case "dopamine":
		return s.Permissions.CanChangeDopamine
	case "sim_control":
		return s.Permissions.CanControlSim
	case "read":
		return !s.Permissions.ReadOnly
	default:
		return false
	}
}
