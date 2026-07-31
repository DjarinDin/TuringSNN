package cortex

import (
	"github.com/DjarinDin/TuringSNN/pkg/comms"
	"log"
)

// Feedback is a struct that holds the feedback event count, control message, and event message
type Feedback struct {
	feedbackEventCount uint64
	ctrlMsg            comms.ControlMsg
	eventMsg           comms.IntMsg
	lastInValue        int
	lastOutValue       int
}

const (
	feedbackCtrlLog  = 0
	feedbackCtrlIn   = 10
	feedbackCtrlOut  = 11
	feedbackCtrlInit = 99

	feedbackEventDelta = 0
	feedbackEventRaw   = 1

	// feedbackDopamineScale converts int feedback units to dopamine pulses.
	feedbackDopamineScale = 0.001
)

func (feedback *Feedback) initialize() {
	feedback.feedbackEventCount = 0
	feedback.lastInValue = 0
	feedback.lastOutValue = 0
}

// RunFeedback runs the feedback loop
func (feedback *Feedback) RunFeedback(
	feedbackControlCh chan comms.ControlMsg,
	eventCh chan comms.IntMsg,
	dopamineOutCh chan float64) {

	feedback.initialize()

	// Run the feedback loop in the caller's goroutine for deterministic ordering.
	feedback.runFeedback(feedbackControlCh, eventCh, dopamineOutCh)
}

func (feedback *Feedback) runFeedback(
	feedbackControlCh chan comms.ControlMsg,
	eventCh chan comms.IntMsg,
	dopamineOutCh chan float64) {
	for {
		select {
		case feedback.ctrlMsg = <-feedbackControlCh:
			// Control messages update the reference values and reset state.
			switch feedback.ctrlMsg.Type {
			case feedbackCtrlLog:
				log.Println("feedback control message:", feedback.ctrlMsg)
			case feedbackCtrlIn:
				// Capture the "entry" value for delta-based reward.
				feedback.lastInValue = feedback.ctrlMsg.IntValue
			case feedbackCtrlOut:
				// Capture the "exit" value for delta-based reward.
				feedback.lastOutValue = feedback.ctrlMsg.IntValue
			case feedbackCtrlInit:
				log.Println("feedback initialization message:", feedback.ctrlMsg)
				feedback.initialize()
			}
		case feedback.eventMsg = <-eventCh:
			// Event messages trigger dopamine pulses based on configured semantics.
			feedback.feedbackEventCount++
			switch feedback.eventMsg.Type {
			case feedbackEventDelta:
				// Delta mode: reward is derived from the latest in/out values.
				delta := feedback.lastOutValue - feedback.lastInValue
				dopamineOutCh <- float64(delta) * feedbackDopamineScale
			case feedbackEventRaw:
				// Raw mode: reward uses the event's signed value directly.
				dopamineOutCh <- float64(feedback.eventMsg.Value) * feedbackDopamineScale
			default:
				// Unknown mode: log and ignore to avoid undefined reward.
				log.Println("feedback event (ignored):", feedback.eventMsg)
			}
		}
	}
}
