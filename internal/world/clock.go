package world

import (
	"sync"
	"time"
)

type Clock interface {
	Now() time.Time
	Advance(d time.Duration)
}

type ManualClock struct {
	mu  sync.Mutex
	now time.Time
}

func NewManualClock(now time.Time) *ManualClock {
	return &ManualClock{now: now.UTC()}
}

func (c *ManualClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *ManualClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

type ScaledClock struct {
	mu         sync.Mutex
	originWall time.Time
	originSim  time.Time
	scale      time.Duration
}

func NewScaledClock(originSim time.Time, scale time.Duration) *ScaledClock {
	return NewScaledClockAt(originSim, time.Now(), scale)
}

func NewScaledClockAt(originSim, originWall time.Time, scale time.Duration) *ScaledClock {
	if scale <= 0 {
		scale = time.Hour
	}
	return &ScaledClock{
		originWall: originWall,
		originSim:  originSim.UTC(),
		scale:      scale,
	}
}

func (c *ScaledClock) Scale() time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.scale
}

func (c *ScaledClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	elapsed := time.Since(c.originWall)
	sim := time.Duration(float64(c.scale) * elapsed.Seconds())
	return c.originSim.Add(sim)
}

func (c *ScaledClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.originSim = c.originSim.Add(d)
}
