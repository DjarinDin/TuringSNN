package service

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"nhooyr.io/websocket"

	"github.com/DjarinDin/TuringSNN/internal/core_logic"
	"github.com/DjarinDin/TuringSNN/internal/core_logic/hub"
	"github.com/DjarinDin/TuringSNN/pkg/comms"
)

// Server hosts a headless cortex service with WebSocket telemetry and HTTP controls.
type Server struct {
	backend *core_logic.Backend
	hub     *hub.Hub
}

// NewServer wires the backend and hub into a service for remote GUI clients.
func NewServer(backend *core_logic.Backend) *Server {
	return &Server{
		backend: backend,
		hub:     backend.GetHub(),
	}
}

// Start launches the HTTP server for control and WebSocket telemetry.
func (s *Server) Start(addr string) error {
	// Control endpoints provide a clean, non-WS path for commands.
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/control", s.handleControl)
	mux.HandleFunc("/sim-control", s.handleSimControl)
	mux.HandleFunc("/stream", s.handleStream)

	// The headless service should log startup explicitly for ops visibility.
	log.Printf("service: cortex headless listening on %s", addr)
	return http.ListenAndServe(addr, mux)
}

// handleHealth provides a quick liveness probe for remote clients.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

// ControlRequest maps incoming JSON commands to cortex/sim control channels.
type ControlRequest struct {
	Channel   string `json:"channel"` // "cortex" or "sim"
	Type      int    `json:"type"`
	BoolValue bool   `json:"boolValue,omitempty"`
	IntValue  int    `json:"intValue,omitempty"`
	Layer     int    `json:"layer,omitempty"`
	Cycle     int    `json:"cycle,omitempty"`
}

// handleControl applies commands to the cortex control channel.
func (s *Server) handleControl(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req ControlRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	// Route to cortex control channel for deterministic remote manipulation.
	if req.Channel == "" || req.Channel == "cortex" {
		if req.Type == comms.CortexControlReset {
			s.backend.GetChannels().CortexResetCh <- true
			w.WriteHeader(http.StatusAccepted)
			return
		}
		if req.Type == comms.CortexControlSoftReset {
			s.backend.GetChannels().CortexSoftResetCh <- true
			w.WriteHeader(http.StatusAccepted)
			return
		}
		s.backend.GetChannels().CortexControlCh <- comms.ControlMsg{
			Type:      req.Type,
			BoolValue: req.BoolValue,
			IntValue:  req.IntValue,
			Layer:     req.Layer,
			Cycle:     req.Cycle,
		}
		w.WriteHeader(http.StatusAccepted)
		return
	}

	http.Error(w, "unsupported channel", http.StatusBadRequest)
}

// handleSimControl applies commands to the sim control channel.
func (s *Server) handleSimControl(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req ControlRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	// Route to sim control for deterministic signal orchestration.
	if req.Channel == "" || req.Channel == "sim" {
		s.backend.GetChannels().SimControlCh <- comms.IntMsg{
			Type:  req.Type,
			Value: req.IntValue,
		}
		w.WriteHeader(http.StatusAccepted)
		return
	}

	http.Error(w, "unsupported channel", http.StatusBadRequest)
}

// handleStream upgrades the request to a WebSocket and streams hub topics.
func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	// Accept the WebSocket connection with default options for simplicity.
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "stream closed")

	// Subscribe to all hub topics so the GUI can remain fully decoupled.
	topics := allTopics()
	outCh := make(chan hubMessage, 256)

	// Fan-in all topic subscriptions into a single outbound stream.
	for _, topic := range topics {
		topic := topic
		sub := s.hub.Subscribe("service-"+string(topic), topic)
		go func() {
			for msg := range sub {
				outCh <- newHubMessage(topic, msg)
			}
		}()
	}

	// Emit messages until the client disconnects or context is canceled.
	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-outCh:
			// Bound the write time to prevent a slow client from blocking the loop.
			writeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			if err := conn.Write(writeCtx, websocket.MessageText, msg.toJSON()); err != nil {
				cancel()
				return
			}
			cancel()
		}
	}
}

// hubMessage is a typed envelope for hub telemetry over the WebSocket stream.
type hubMessage struct {
	SchemaVersion int         `json:"schemaVersion"`
	Topic         string      `json:"topic"`
	PayloadType   string      `json:"payloadType"`
	Cycle         int         `json:"cycle,omitempty"`
	Payload       interface{} `json:"payload"`
}

func (msg hubMessage) toJSON() []byte {
	encoded, _ := json.Marshal(msg)
	return encoded
}

func newHubMessage(topic hub.Topic, payload interface{}) hubMessage {
	msg := hubMessage{
		SchemaVersion: 1,
		Topic:         string(topic),
		PayloadType:   payloadType(topic, payload),
		Payload:       payload,
	}
	switch v := payload.(type) {
	case comms.BoolArrayMsg:
		msg.Cycle = v.Cycle
	case *comms.BoolArrayMsg:
		msg.Cycle = v.Cycle
	case int:
		if topic == hub.TopicCortexTick {
			msg.Cycle = v
		}
	}
	return msg
}

func payloadType(topic hub.Topic, payload interface{}) string {
	if override, ok := payloadTypeOverride(topic); ok {
		return override
	}
	switch payload.(type) {
	case bool:
		return "bool"
	case int:
		return "int"
	case float64:
		return "float64"
	case string:
		return "string"
	case []bool:
		return "bool[]"
	case []int:
		return "int[]"
	case []float32:
		return "float32[]"
	case []float64:
		return "float64[]"
	case comms.StringMsg, *comms.StringMsg:
		return "string-msg"
	case comms.BoolMsg, *comms.BoolMsg:
		return "bool-msg"
	case comms.BoolArrayMsg, *comms.BoolArrayMsg:
		return "bool-array-msg"
	case comms.PotentialsAndSpikesMsg, *comms.PotentialsAndSpikesMsg:
		return "potentials-and-spikes"
	case comms.LearningStateMsg, *comms.LearningStateMsg:
		return "learning-state"
	case comms.HubDropStats, *comms.HubDropStats:
		return "hub-drops"
	default:
		return "unknown"
	}
}

func payloadTypeOverride(topic hub.Topic) (string, bool) {
	switch topic {
	case hub.TopicHubDrops:
		return "hub-drops", true
	case hub.TopicDecisionLedger:
		return "decision-ledger", true
	case hub.TopicOpenEngagements:
		return "open-engagements", true
	case hub.TopicDecisionTrackerStatus:
		return "tracker-status", true
	case hub.TopicExternalSignal:
		return "external-signal", true
	default:
		return "", false
	}
}

// allTopics enumerates the hub topics that should stream to remote clients.
func allTopics() []hub.Topic {
	return []hub.Topic{
		hub.TopicCortexHeartbeat,
		hub.TopicSimHeartbeat,
		hub.TopicDensities,
		hub.TopicCortexTick,
		hub.TopicHubDrops,
		hub.TopicLayer1Potentials,
		hub.TopicLayer1Spikes,
		hub.TopicLayer2Potentials,
		hub.TopicLayer2Spikes,
		hub.TopicLayer3Potentials,
		hub.TopicLayer3Spikes,
		hub.TopicLayer2InputSpikes,
		hub.TopicLayer3InputSpikes,
		hub.TopicLayer1InputWeights,
		hub.TopicLayer1FeedbackWeights,
		hub.TopicLayer2InputWeights,
		hub.TopicLayer2FeedbackWeights,
		hub.TopicLayer3InputWeights,
		hub.TopicLayer3FeedbackWeights,
		hub.TopicLayer1InputTraces,
		hub.TopicLayer1FeedbackTraces,
		hub.TopicLayer1OutputTraces,
		hub.TopicLayer2InputTraces,
		hub.TopicLayer2FeedbackTraces,
		hub.TopicLayer2OutputTraces,
		hub.TopicLayer3InputTraces,
		hub.TopicLayer3FeedbackTraces,
		hub.TopicLayer3OutputTraces,
		hub.TopicLayer3FeedbackSpikes,
		hub.TopicSensorSpikes,
		hub.TopicRawSensorSpikes,
		hub.TopicRawSensorBlank,
		hub.TopicOutputA,
		hub.TopicOutputB,
		hub.TopicOutputC,
		hub.TopicOutputD,
		hub.TopicOutputAWeights,
		hub.TopicOutputBWeights,
		hub.TopicOutputCWeights,
		hub.TopicOutputDWeights,
		hub.TopicOutputAHistory,
		hub.TopicOutputBHistory,
		hub.TopicOutputCHistory,
		hub.TopicOutputDHistory,
		hub.TopicFocusPotentialHistory,
		hub.TopicCumulativeStats,
		hub.TopicPerHeartbeatStats,
		hub.TopicMetrics,
		hub.TopicDecisionTrackerStatus,
		hub.TopicExternalSignal,
		hub.TopicOpenEngagements,
		hub.TopicDecisionLedger,
		hub.TopicLearningState,
	}
}
