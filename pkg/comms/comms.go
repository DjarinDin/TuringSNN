package comms

//import "github.com/DjarinDin/TuringSNN/internal/conf"

// type LedPanelMessage struct {
// 	Type  int
// 	Value int
// }

// type MemPanelMessage struct {
// 	Type  int
// 	Value string
// }

// type CPUPanelMessage struct {
// 	Type  int
// 	Value string
// }

//	RIGHT STATS CODES:
//
// 0: signals received during this last heartbeat
// 1: total signals accumulated since start or reset
// 2: collisions during this last heartbeat
// type StringMsg struct {
type StringMsg struct {
	Type  int
	Value string
}

// Cortex control message type constants (ControlMsg.Type).
const (
	CortexControlTickRate             = 0
	CortexControlDopamine             = 1
	CortexControlPause                = 2
	CortexControlFocus                = 3
	CortexControlStep                 = 4
	CortexControlSpontaneousFire      = 5
	CortexControlResetAccumulator     = 6
	CortexControlReset                = 7
	CortexControlSoftReset            = 8
	CortexControlShortTraceHalfLifeMs = 9
	CortexControlMidTraceHalfLifeMs   = 10
	CortexControlLongTraceHalfLifeMs  = 11
	CortexControlLongEligibilityGate  = 12
	CortexControlTraceDisplayMode     = 13 // 0=Short, 1=Mid, 2=Long
)

// Trace display mode constants for CortexControlTraceDisplayMode.
const (
	TraceDisplayShort = 0
	TraceDisplayMid   = 1
	TraceDisplayLong  = 2
)

// Sim control message type constants (IntMsg.Type).
const (
	SimControlNoiseRun         = 0
	SimControlSignal1Run       = 1
	SimControlSignal2Run       = 2
	SimControlRandomWalkRun    = 3
	SimControlTSGen0Run        = 4
	SimControlTSGen1Run        = 5
	SimControlTSGen2Run        = 6
	SimControlTSGen3Run        = 7
	SimControlNoiseVolume      = 8
	SimControlSignal1Volume    = 9
	SimControlSignal2Volume    = 10
	SimControlRandomWalkVolume = 11
	SimControlTSGen0Volume     = 12
	SimControlTSGen1Volume     = 13
	SimControlTSGen2Volume     = 14
	SimControlTSGen3Volume     = 15
	SimControlSimRun           = 80
	SimControlStep             = 81
	SimControlCortexSync       = 82
	SimControlRealDataMode     = 83
	SimControlTickRate         = 90
	SimControlReset            = 99
)

//	MIDDLE STATS CODES:
//
// 0: total signals received since start or reset
// 1: signals accumulated since start or reset
// 2: collisions (signals dropped) [% dropped]
// type CumulativeStatsMessage struct {
// 	Type  int
// 	Value string
// }

//	SIM CONTROL CODES:
//
// 0: noise (1: run, else not)
// 1: signal (1: run, else not)
// 2: stealth signal (1: run, else not)
// 5: noise level
// 6: signal level
// 7: stealth signal level
// 90: rate control
// 99: initialize
// type SimControlMessage struct {
// 	Type  int
// 	Value int
// }

//	GUI PANEL CONTROL CODES:
//
// 0: run/stop display (stop:0, run:1)
// 1: sweep/scroll display (sweep:0, scroll:1)
// 2: clear (any value)
type IntMsg struct {
	Type  int
	Value int
}

//	WEIGHTS PANEL CONTROL CODES:
//
// 0:
// 1:
// 2:
type ControlMsg struct {
	Type      int
	BoolValue bool
	IntValue  int
	Layer     int
	Cycle     int
}

type BoolArrayMsg struct {
	Cycle  int
	Spikes []bool
}

type BoolMsg struct {
	Pot    float64
	Spikes bool
}

type PotentialsAndSpikesMsg struct {
	Pots   []float64
	Spikes []bool
}

// HubDropStats captures per-topic hub drop counts.
type HubDropStats struct {
	Total   uint64            `json:"total"`
	ByTopic map[string]uint64 `json:"byTopic"`
}

// LearningStateMsg captures high-level learning dynamics for observability.
type LearningStateMsg struct {
	Tick                      int
	Dopamine                  float64
	Adaptability              float64
	SpontaneousFire           float64
	ShortTraceHalfLifeSeconds float64
	MidTraceHalfLifeSeconds   float64
	LongTraceHalfLifeSeconds  float64
	LongEligibilityGate       float64
	ResidualAbsMeanFocus      float64
	ResidualAbsMaxFocus       float64
}
