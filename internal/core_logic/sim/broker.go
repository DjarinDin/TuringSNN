package sim

import (
	"log"

	"github.com/DjarinDin/TuringSNN/internal/conf"
	"github.com/DjarinDin/TuringSNN/internal/core_logic/cortex"
	"github.com/DjarinDin/TuringSNN/internal/core_logic/hub"
)

// This file turns the cortex's two binary motor outputs (neurons C and D)
// into a trackable, closed-loop credit-assignment signal: an "open" pulse
// starts a discrete engagement window against whatever scalar external
// signal is being published to hub.TopicExternalSignal; a "close" pulse
// ends the oldest open window and scores it by how much that signal moved
// while the window was open. The score feeds back to the cortex as a
// dopamine-style reward via retributionSink, so engagements that turned out
// well are reinforced and ones that didn't are discouraged.
//
// None of this depends on what the external signal actually represents —
// the open-source core drives it purely with the built-in Sim's synthetic
// output. A deployment feeding it something else (e.g. a real external
// value stream) can reinterpret "engagement," "value," and "return" as
// whatever's appropriate for that context; the mechanism itself doesn't
// change.

// Lot represents a discrete "hypothesis" window: opened on a positive
// signal, closed later against the external signal's value at close time.
type Lot struct {
	EntryValue float64 `json:"entryValue"`
	Exposure   float64 `json:"exposure"` // Fractional (e.g. 0.1 for 10%)
	Positive   bool    `json:"positive"` // true = opened betting the value would rise
	EntryTick  int     `json:"entryTick"`
}

// HistoryItem represents a closed lot for the ledger.
type HistoryItem struct {
	Lot
	ExitValue float64 `json:"exitValue"`
	ExitTick  int     `json:"exitTick"`
	Return    float64 `json:"return"` // Normalized return, net of friction
}

// LedgerState is the full serializable engagement-tracking state.
type LedgerState struct {
	OpenLots       []Lot         `json:"openLots"`
	History        []HistoryItem `json:"history"`
	TotalReturn    float64       `json:"totalReturn"`
	CurrentNet     float64       `json:"currentNet"` // Relative to starting baseline value
	AvgHoldTicks   float64       `json:"avgHoldTicks"`
	AvgHoldSeconds float64       `json:"avgHoldSeconds"`
}

// ExecutionMapping names how output spikes map to tracker actions.
type ExecutionMapping string

const (
	// ExecutionMappingMotorBinary enforces binary execution from motor neurons only.
	ExecutionMappingMotorBinary ExecutionMapping = "motor-binary"
)

// PositionSizingMode declares how exposure is increased/reduced per decision.
type PositionSizingMode string

const (
	// PositionSizingDiscreteLots turns each spike into a fixed exposure "lot".
	PositionSizingDiscreteLots PositionSizingMode = "discrete-lots"
)

// ExitMode declares how close signals unwind exposure.
type ExitMode string

const (
	// ExitModeFIFO closes the oldest open lot first to preserve temporal attribution.
	ExitModeFIFO ExitMode = "fifo"
	// ExitModeGlobalFlush closes all open lots on a single close signal.
	ExitModeGlobalFlush ExitMode = "global-flush"
)

// ExecutionPolicy locks the output-mapping and sizing semantics.
type ExecutionPolicy struct {
	Mapping      ExecutionMapping
	SizingMode   PositionSizingMode
	ExitMode     ExitMode
	LotExposure  float64
	MaxLots      int
	MinHoldTicks int
	// Friction is a small constant cost subtracted from every closed lot's
	// return, so a lot that merely breaks even still nets slightly negative.
	Friction float64
	// PenaltyMultiplier scales up the reinforcement signal for lots that
	// close with a negative return, so the cortex learns to avoid
	// low-quality engagements faster than it would from the raw return alone.
	PenaltyMultiplier float64
}

type PositionTracker struct {
	hub            *hub.Hub
	retribution    retributionSink
	running        bool
	lots           []Lot
	history        []HistoryItem
	currentValue   float64
	baselineValue  float64 // Starting reference value (e.g. 100)
	totalReturn    float64
	currentTick    int
	totalHoldTicks int64
	closedTrades   int64
	policy         ExecutionPolicy
}

type retributionSink interface {
	ApplyRetribution(float64)
}

func NewPositionTracker(h *hub.Hub, c *cortex.Cortex) *PositionTracker {
	return &PositionTracker{
		hub:           h,
		retribution:   c,
		baselineValue: 100.0,
		history:       make([]HistoryItem, 0),
		// Binary execution driven by motor neurons only: pre-motor candidate
		// activity is internal and never itself opens or closes a lot.
		policy: ExecutionPolicy{
			Mapping:           ExecutionMappingMotorBinary,
			SizingMode:        PositionSizingDiscreteLots,
			ExitMode:          ExitModeFIFO,
			LotExposure:       0.1,
			MaxLots:           10,
			MinHoldTicks:      500,
			Friction:          0.001,
			PenaltyMultiplier: 2.0,
		},
	}
}

func (pt *PositionTracker) Start() {
	if pt.running {
		return
	}
	pt.running = true
	go pt.listen()
}

func (pt *PositionTracker) Stop() {
	pt.running = false
}

func (pt *PositionTracker) listen() {
	// Resolve open/close topics from the mapping policy.
	var openTopic hub.Topic
	var closeTopic hub.Topic
	switch pt.policy.Mapping {
	case ExecutionMappingMotorBinary:
		// Motor-only execution prevents pre-motor noise from opening/closing lots.
		openTopic = hub.TopicOutputC
		closeTopic = hub.TopicOutputD
	default:
		// Safe fallback: keep motor-only execution until another policy is wired.
		openTopic = hub.TopicOutputC
		closeTopic = hub.TopicOutputD
	}

	// Subscribe to open/close, value, and clock topics for synchronized actions.
	openCh := pt.hub.Subscribe("tracker-signal-a", openTopic)
	closeCh := pt.hub.Subscribe("tracker-signal-b", closeTopic)
	valueCh := pt.hub.Subscribe("tracker-value", hub.TopicExternalSignal)
	tickCh := pt.hub.Subscribe("tracker-tick", hub.TopicCortexTick)

	// Log the active mapping so behavior is explicit in traces.
	log.Printf("tracker: PositionTracker active with mapping=%s", pt.policy.Mapping)

	openActive := false
	closeActive := false

	for pt.running {
		select {
		case <-openCh:
			openActive = true
		case <-closeCh:
			closeActive = true
		case msg := <-valueCh:
			if v, ok := msg.(float64); ok {
				pt.currentValue = v
			}
		case msg := <-tickCh:
			if t, ok := msg.(int); ok {
				pt.currentTick = t
				pt.processTick(openActive, closeActive)
				openActive = false
				closeActive = false
			}
		}
	}
}

func (pt *PositionTracker) processTick(openSignal, closeSignal bool) {
	if openSignal && closeSignal {
		// Conflicting signals in the same tick carry no clear direction, so
		// treat the tick as a full miss rather than guessing.
		log.Printf("tracker: conflicting open/close signals at tick %d — treating as a miss", pt.currentTick)
		pt.applyRetribution(-1.0)
		return
	}

	if openSignal {
		pt.handleOpen()
	}
	if closeSignal {
		pt.handleClose()
	}
}

func (pt *PositionTracker) handleOpen() {
	if !pt.running || pt.currentValue == 0 {
		return
	}

	switch pt.policy.SizingMode {
	case PositionSizingDiscreteLots:
		// Enforce the exposure cap to keep sizing deterministic.
		if len(pt.lots) >= pt.policy.MaxLots {
			return
		}

		// Each motor spike adds a fixed fractional exposure hypothesis.
		lot := Lot{
			EntryValue: pt.currentValue,
			Exposure:   pt.policy.LotExposure,
			Positive:   true,
			EntryTick:  pt.currentTick,
		}

		pt.lots = append(pt.lots, lot)
		log.Printf("tracker: [LOT %d] OPEN @ %.2f (Total Exposure: %.0f%%)",
			len(pt.lots), lot.EntryValue, float64(len(pt.lots))*pt.policy.LotExposure*100.0)

		pt.publishLedger()
	default:
		// Unknown sizing mode: take no action rather than misprice exposure.
		return
	}
}

func (pt *PositionTracker) handleClose() {
	if !pt.running || pt.currentValue == 0 || len(pt.lots) == 0 {
		return
	}

	switch pt.policy.SizingMode {
	case PositionSizingDiscreteLots:
		switch pt.policy.ExitMode {
		case ExitModeFIFO:
			// FIFO: Close the oldest lot to preserve temporal attribution.
			pt.closeLotFIFO()
		case ExitModeGlobalFlush:
			// Global flush: Close all open lots on a single close signal.
			pt.closeAllLots()
		default:
			// Unknown exit mode: take no action rather than mis-apply exits.
			return
		}
	default:
		// Unknown sizing mode: take no action rather than misprice exposure.
		return
	}
}

// closeLotFIFO closes a single oldest lot with full accounting and retribution.
func (pt *PositionTracker) closeLotFIFO() {
	// FIFO: Close the oldest lot.
	lot := pt.lots[0]
	// Enforce a minimum hold period so a lot can't be opened and closed
	// again before the external signal has moved meaningfully — otherwise
	// noise alone could be mistaken for a real, evaluable decision.
	if pt.currentTick-lot.EntryTick < pt.policy.MinHoldTicks {
		return
	}
	pt.lots = pt.lots[1:]

	// Compute return for this specific lot to preserve credit assignment.
	ret := (pt.currentValue - lot.EntryValue) / lot.EntryValue
	if !lot.Positive {
		ret = (lot.EntryValue - pt.currentValue) / lot.EntryValue
	}

	// Subtract a small constant friction so a lot that merely broke even
	// still nets slightly negative — this keeps "do nothing" from looking
	// artificially attractive relative to a real, decisive engagement.
	netReturn := ret - pt.policy.Friction

	// Record the closed lot for observability and replay.
	item := HistoryItem{
		Lot:       lot,
		ExitValue: pt.currentValue,
		ExitTick:  pt.currentTick,
		Return:    netReturn,
	}
	pt.history = append(pt.history, item)
	pt.totalReturn += netReturn
	pt.totalHoldTicks += int64(item.ExitTick - item.EntryTick)
	pt.closedTrades++

	log.Printf("tracker: [CLOSE LOT] Return: %.4f%% | Exit: %.2f | Open Lots: %d",
		netReturn*100, pt.currentValue, len(pt.lots))

	// Feed the outcome back to the cortex as a dopamine-style reward.
	if netReturn < 0 {
		// Amplify negative outcomes relative to positive ones of the same
		// magnitude, so the cortex converges away from low-quality
		// engagements faster than symmetric reinforcement would allow.
		log.Printf("tracker: [PENALTY] Amplifying negative outcome (%.4f) to speed convergence away from it", netReturn)
		pt.applyRetribution(netReturn * pt.policy.PenaltyMultiplier)
	} else {
		pt.applyRetribution(netReturn)
	}

	pt.publishLedger()
}

// closeAllLots closes every open lot in sequence to flush exposure.
func (pt *PositionTracker) closeAllLots() {
	// Keep closing the oldest lot until exposure is fully unwound.
	for len(pt.lots) > 0 {
		pt.closeLotFIFO()
		// If the oldest lot is too young, abort to prevent an infinite loop.
		if len(pt.lots) > 0 && pt.currentTick-pt.lots[0].EntryTick < pt.policy.MinHoldTicks {
			return
		}
	}
}

func (pt *PositionTracker) publishLedger() {
	state := LedgerState{
		OpenLots:       pt.lots,
		History:        pt.history,
		TotalReturn:    pt.totalReturn,
		CurrentNet:     pt.baselineValue * (1.0 + pt.totalReturn),
		AvgHoldTicks:   pt.avgHoldTicks(),
		AvgHoldSeconds: pt.avgHoldSeconds(),
	}
	pt.hub.Publish(hub.TopicDecisionLedger, state)
	pt.hub.Publish(hub.TopicOpenEngagements, len(pt.lots))
}

func (pt *PositionTracker) avgHoldTicks() float64 {
	if pt.closedTrades == 0 {
		return 0
	}
	return float64(pt.totalHoldTicks) / float64(pt.closedTrades)
}

func (pt *PositionTracker) avgHoldSeconds() float64 {
	if pt.closedTrades == 0 {
		return 0
	}
	return pt.avgHoldTicks() * float64(conf.RealTimeTickPeriod) / 1000.0
}

func (pt *PositionTracker) applyRetribution(signal float64) {
	if pt.retribution == nil {
		return
	}
	pt.retribution.ApplyRetribution(signal)
}
