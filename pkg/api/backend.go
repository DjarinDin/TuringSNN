package api

import "github.com/DjarinDin/TuringSNN/pkg/comms"

// NeuralBackend defines the interface for the neural network backend.
// It abstracts the communication between the UI and the core logic.
type NeuralBackend interface {
	// GetChannels returns the channels used for communication.
	GetChannels() *BackendChannels
	// Start starts the backend processing.
	Start()
	// Stop stops the backend processing.
	Stop()
}

// BackendChannels holds the external control channels for remote clients.
type BackendChannels struct {
	SimControlCh       chan comms.IntMsg
	CortexControlCh    chan comms.ControlMsg
	CortexResetCh      chan bool
	CortexSoftResetCh  chan bool
	MainPanelControlCh chan comms.ControlMsg
}
