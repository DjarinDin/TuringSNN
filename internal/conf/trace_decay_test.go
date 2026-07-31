package conf

import (
	"math"
	"testing"
)

func TestTraceDecayFactorHalfLife(t *testing.T) {
	tickSeconds := float64(RealTimeTickPeriod) / 1000.0
	if tickSeconds <= 0 {
		t.Fatalf("invalid tick seconds: %.6f", tickSeconds)
	}

	cases := []struct {
		name     string
		halfLife float64
		decay    float64
	}{
		{"short", ShortTraceHalfLifeSeconds, ShortTraceDecayFactor},
		{"mid", MidTraceHalfLifeSeconds, MidTraceDecayFactor},
		{"long", LongTraceHalfLifeSeconds, LongTraceDecayFactor},
	}

	for _, c := range cases {
		if c.halfLife <= 0 {
			t.Fatalf("%s half-life must be positive", c.name)
		}
		ticks := int(math.Round(c.halfLife / tickSeconds))
		if ticks <= 0 {
			t.Fatalf("%s half-life yields non-positive ticks", c.name)
		}
		remaining := math.Pow(c.decay, float64(ticks))
		if math.Abs(remaining-0.5) > 0.05 {
			t.Fatalf("%s decay mismatch: remaining=%.4f ticks=%d", c.name, remaining, ticks)
		}
	}
}
