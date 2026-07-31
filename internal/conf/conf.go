package conf

import (
	"image/color"
	"log"
	"math"
	"math/rand"
	"os"
	"time"
)

const (
	// Fyne parameters
	FYNE_SCALE = "1.0" //"2.0" //"2.4" // "1.6" //"2.42" fills

	// ArchitectureImageLocation is the file path to the background image file
	ArchitectureImageLocation = "assets/architecture10.png"

	// Screen object sizes
	SmallHeight      = 32
	LargeHeight      = 64
	SuperLargeHeight = 256

	TinyWidth  = 32
	SmallWidth = 64
	MidWidth   = 128
	LargeWidth = 192

	// Simulator parameters
	SimSignalPatternLength = 128 //NumSensors * 1

	// Cortex parameters
	NumSensors             = 128 // minimum 1 -- 0 will cause a fault in the the math.rand library
	NumNeurons             = 128
	OutputNeuronInputWidth = NumSensors + (3 * NumNeurons)
	NumOutputNeurons       = 4   // [0:1] Pre-Motor (Candidates), [2:3] Motor (Conviction)
	NumTotSynapses         = 512 // Quad Architecture: 4 fields of 128 synapses each
	NumCycles              = NumNeurons

	// Quad Architecture field boundaries (512 synapses total, 128 per field)
	// Field 1: Phenomenon [0-127] - Raw sensor inputs
	QuadPhenomenonStart = 0
	QuadPhenomenonEnd   = NumSensors // 128
	// Field 2: Resonance [128-255] - Individual neuron over time (T-1 to T-128)
	QuadResonanceStart = NumSensors     // 128
	QuadResonanceEnd   = NumSensors * 2 // 256
	// Field 3: Zeitgeist [256-383] - Collective state of layer at T-1 (all neurons at T-1)
	QuadZeitgeistStart = NumSensors * 2 // 256
	QuadZeitgeistEnd   = NumSensors * 3 // 384
	// Field 4: Gestalt [384-511] - Unified prediction from higher layers (feedback)
	QuadGestaltStart  = NumSensors * 3 // 384
	QuadGestaltEnd    = NumSensors * 4 // 512
	NumDisplayPeriods = SimSignalPatternLength * 3

	InitialFocusNeuron = 0

	// Core parameters
	HeartbeatRate      = time.Second
	RealTimeTickPeriod = 10  // ms
	SlowTickPeriod     = 200 // ms
	//MonitoringTickPeriod  = 10  // ms
	SimTickRateMultiplier = 1

	// Zeitgeist inhibition modes
	ZeitgeistInhibitionSoft = iota
	ZeitgeistInhibitionHardWTA

	//PotentialAveragingHorizon = 100

	// Network parameteres
	IPTransportProtocol = "udp"
	IPAddress           = "127.0.0.1"
	IPPort              = "50000" //addr, err := net.ResolveUDPAddr("udp", "localhost:"+portString)

	// Cortex
	CoreTickPeriodMin     = 1.0
	CoreTickPeriodMax     = 1000.0
	CoreTickPeriodInitVal = 930 //991

	// period after which gopher gets worried about hearing no heartbeats
	GUIGopherResetPeriod          = time.Millisecond * 300
	GUISpaceBetweenInputAndOutput = 2

	MemSampleKickerPeriod = time.Second * 1
)

var (
	// ******************************************************************************************** //
	// ******************************************************************************************** //
	// Network parameters
	// BaseThreshold is the starting threshold for neurons.
	BaseThreshold = math.Pow(2, 64.0)
	// ThresholdGrowth is the multiplier applied to the threshold when a neuron fires (desensitization).
	ThresholdGrowth = 1.05 // +5% (Smoothed from 25% to prevent 'Silence' pathology)
	// ThresholdDecay is the multiplier applied to the threshold when a neuron does not fire (sensitization).
	ThresholdDecay = 0.999 // -0.1% (Smoothed from 0.5% to maintain responsiveness)
	// MinThreshold is the floor for the threshold to prevent runaway excitability.
	MinThreshold = math.Pow(2, 60.0)

	// MaxTraceAndWeight represents the maximum trace and weight value.
	// It is calculated as a power of two with a value of 31.
	MaxTraceAndWeight     = powerOfTwo(31)
	MaxTrSquared          = float64(MaxTraceAndWeight) * float64(MaxTraceAndWeight)
	SensorToFeedbackRatio = 4.0
	//TotalWeights                = MaxTraceAndWeight * NumNeurons / 2
	TraceLeakDivisor = 4 // smaller number means bigger leak

	// Dopamine-trace parameters (multi-timescale)
	MidTermTraceGain     = 0.2
	LongTermTraceGain    = 0.5
	MidTermLearningRate  = 0.02
	LongTermLearningRate = 0.05
	// LongEligibilityGate controls when eligibility is promoted to long-term memory.
	LongEligibilityGate = 0.01
	// Eligibility gating to avoid noisy, dense trace accumulation.
	MinEligibilityAmp = 0.0001
	TraceFloor        = float64(MaxTraceAndWeight) * 0.02
	// STDP timing kernel width in ticks (larger = wider eligibility window).
	StdpTauTicks = 10.0
	// Trace half-life targets (seconds)
	ShortTraceHalfLifeSeconds = 0.1
	MidTraceHalfLifeSeconds   = 2.0
	LongTraceHalfLifeSeconds  = 180.0

	ShortTraceDecayFactor       = traceDecayFactor(ShortTraceHalfLifeSeconds)
	MidTraceDecayFactor         = traceDecayFactor(MidTraceHalfLifeSeconds)
	LongTraceDecayFactor        = traceDecayFactor(LongTraceHalfLifeSeconds)
	PotentialPassiveDecayFactor = 0.8
	Dopamine                    = 0.1
	NumRefractionCycles         = 2
	PotentialRefractoryLevel    = 0.0 // - math.Pow(2,94)
	SpontaneousSpikeProbability = 2   // per thousand

	// Zeitgeist inhibition policy
	//If you want k‑WTA instead of strict WTA, set conf.ZeitgeistWTAK to a higher value.
	ZeitgeistInhibitionMode = ZeitgeistInhibitionHardWTA // Winner Takes All
	ZeitgeistWTAK           = 1

	// Human STDP range is approximately +/- 100ms. This tranlates to 10 ticks for 10ms cortex rate.
	// To span 10 doublings/halvings MaxTrace = 2^10 = 1024

	// ******************************************************************************************** //
	// ******************************************************************************************** //

	ForegroundColor = color.NRGBA{0, 100, 100, 255}
	ColorLoYellow   = color.NRGBA{66, 66, 0, 255}
	ColorMidYellow  = color.NRGBA{150, 142, 0, 255}
	ColorHiYellow   = color.NRGBA{255, 255, 0, 255}
	ColorBlack      = color.NRGBA{0, 0, 0, 255}
	ColorGray       = color.NRGBA{32, 32, 32, 255}
	ColorHiBlue     = color.NRGBA{0, 255, 255, 255}
	ColorLoBlue     = color.NRGBA{0, 180, 180, 255}
	ColorHiGreen    = color.NRGBA{0, 255, 0, 255}
	ColorOrange     = color.NRGBA{255, 100, 0, 255}
	ColorRed        = color.NRGBA{255, 0, 0, 255}
	ColorWhite      = color.NRGBA{255, 255, 255, 255}

	// Reinforcement (Phase 4)
	MaxTagAge    = 1000 // In ticks. 1000 ticks @ 10ms = 10 seconds of trade visibility.
	DopamineGain = 5.0  // Multiplier for profit/loss impact on weights (Damped from 50.0).
	// OutputDopamine is the learning rate for the decision layer's internal noise.
	OutputDopamine = 0.001 // 100x lower than global dopamine to dampen gossip (Damped from 0.01).
)

func init() {
	err := os.Setenv("FYNE_SCALE", FYNE_SCALE)
	if err != nil {
		log.Println("conf: Error setting FYNE_SCALE")
	}

	log.SetPrefix("tg4: ")
	log.SetFlags(0)

	//seed := time.Now().UTC().UnixNano()
	//rand.Seed(time.Now().UTC().UnixNano()) // surprise!
	seed := int64(42)
	log.Println("conf: Seeding rand with", seed)
	rand.Seed(seed) // repeatable
}

func powerOfTwo(exponent int) int {
	result := int(1) // Start with 1 since 2^0 = 1
	for i := int(0); i < exponent; i++ {
		result *= 2 // Multiply by 2 for each power increment
		// Optional: Check for overflow, can be omitted based on the use case
		if result < 1 {
			panic("Overflow detected")
		}
	}
	return result
}

func traceDecayFactor(halfLifeSeconds float64) float64 {
	if halfLifeSeconds <= 0 {
		return 0
	}
	tickSeconds := float64(RealTimeTickPeriod) / 1000.0
	return math.Pow(0.5, tickSeconds/halfLifeSeconds)
}

func recalcTraceDecayFactors() {
	ShortTraceDecayFactor = traceDecayFactor(ShortTraceHalfLifeSeconds)
	MidTraceDecayFactor = traceDecayFactor(MidTraceHalfLifeSeconds)
	LongTraceDecayFactor = traceDecayFactor(LongTraceHalfLifeSeconds)
}

func SetShortTraceHalfLifeSeconds(value float64) {
	if value <= 0 {
		return
	}
	ShortTraceHalfLifeSeconds = value
	recalcTraceDecayFactors()
}

func SetMidTraceHalfLifeSeconds(value float64) {
	if value <= 0 {
		return
	}
	MidTraceHalfLifeSeconds = value
	recalcTraceDecayFactors()
}

func SetLongTraceHalfLifeSeconds(value float64) {
	if value <= 0 {
		return
	}
	LongTraceHalfLifeSeconds = value
	recalcTraceDecayFactors()
}

func SetLongEligibilityGate(value float64) {
	if value < 0 {
		return
	}
	LongEligibilityGate = value
}

/*

 1	                         2
 2	                         4
 3	                         8
 4	                        16
 5	                        32
 6	                        64
 7	                       128
 8 ....................... 256

 9	                       512
10	                     1,024
11	                     2,048
12	                     4,096
13	                     8,192
14	                    16,384
15	                    32,768
16 .................... 65,536

17	                   131,072
18	                   262,144
19	                   524,288
20	                 1,048,576
21	                 2,097,152
22	                 4,194,304
23	                 8,388,608
24 ................ 16,777,216

25	                33,554,432
26	                67,108,864
27	               134,217,728
28	               268,435,456
29	               536,870,912
30	             1,073,741,824
31	             2,147,483,648
32 ............. 4,294,967,296

33	             8,589,934,592
34	            17,179,869,184
35	            34,359,738,368
36	            68,719,476,736
37	           137,438,953,472
38	           274,877,906,944
39	           549,755,813,888
40 ......... 1,099,511,627,776

41	         2,199,023,255,552
42	         4,398,046,511,104
43	         8,796,093,022,208
44	        17,592,186,044,416
45	        35,184,372,088,832
46	        70,368,744,177,664
47	       140,737,488,355,328
48 ....... 281,474,976,710,656

49	       562,949,953,421,312
50	     1,125,899,906,842,620
51	     2,251,799,813,685,250
52	     4,503,599,627,370,500
53	     9,007,199,254,740,990
54	    18,014,398,509,482,000
55	    36,028,797,018,964,000
56 .... 72,057,594,037,927,900

57	   144,115,188,075,856,000
58	   288,230,376,151,712,000
59	   576,460,752,303,423,000
60	 1,152,921,504,606,850,000
61	 2,305,843,009,213,690,000
62	 4,611,686,018,427,390,000
63	 9,223,372,036,854,780,000
64	18,446,744,073,709,600,000

*/
