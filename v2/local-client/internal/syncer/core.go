package syncer

import (
	"log"
	"math"
	"time"

	"videowithyou/v2/local-client/internal/adapter"
	"videowithyou/v2/local-client/internal/model"
	videowithyoupb "videowithyou/v2/proto/gen"
)

type Config struct {
	HardSeekThresholdMS int64
	DeadzoneMS          int64
	SoftRateEnabled     bool
	SoftRateThresholdMS int64
	SoftRateAdjust      float64
	SoftRateMaxMS       int64
}

type Core struct {
	log     *log.Logger
	adapter adapter.Endpoint
	cfg     Config

	softUntil time.Time
}

func NewCore(cfg Config, adapter adapter.Endpoint, logger *log.Logger) *Core {
	if logger == nil {
		logger = log.Default()
	}
	return &Core{
		log:     logger,
		adapter: adapter,
		cfg:     cfg,
	}
}

func (c *Core) UpdateConfig(cfg Config) {
	c.cfg = cfg
}

func (c *Core) UpdateAdapter(adapter adapter.Endpoint) {
	c.adapter = adapter
}

func (c *Core) Align(host *videowithyoupb.HostState, offsetMs int64, localOffsetMs int64) {
	if host == nil || c.adapter == nil {
		return
	}
	if host.Media != nil && host.Media.Attrs != nil {
		if host.Media.Attrs["page_only"] == "1" {
			return
		}
	}

	localState, ok := c.adapter.GetState()
	if !ok {
		return
	}

	nowServerMs := time.Now().UnixMilli() + offsetMs
	base := host.PositionMs + host.OffsetMs + localOffsetMs
	target := int64(base)
	if !host.Paused {
		elapsed := nowServerMs - host.SampleServerTimeMs
		target = int64(float64(base) + float64(elapsed)*host.Rate)
	}

	drift := target - localState.PositionMs
	absDrift := int64(math.Abs(float64(drift)))

	if absDrift < c.cfg.DeadzoneMS {
		if time.Now().Before(c.softUntil) {
			c.applyRate(host.Rate, host.Paused)
			c.softUntil = time.Time{}
		}
		return
	}

	if absDrift >= c.cfg.HardSeekThresholdMS {
		c.log.Printf("drift=%dms seek target=%d", drift, target)
		_ = c.adapter.ApplyState(model.ApplyState{
			PositionMs: target,
			Paused:     host.Paused,
			Rate:       host.Rate,
		})
		c.softUntil = time.Time{}
		return
	}

	if c.cfg.SoftRateEnabled && absDrift >= c.cfg.SoftRateThresholdMS && !host.Paused {
		adjusted := host.Rate
		if drift > 0 {
			adjusted = host.Rate + c.cfg.SoftRateAdjust
		} else {
			adjusted = host.Rate - c.cfg.SoftRateAdjust
		}
		c.log.Printf("drift=%dms soft-rate=%.3f", drift, adjusted)
		_ = c.adapter.ApplyState(model.ApplyState{
			PositionMs: -1,
			Paused:     false,
			Rate:       adjusted,
		})
		c.softUntil = time.Now().Add(time.Duration(c.cfg.SoftRateMaxMS) * time.Millisecond)
		return
	}

	c.applyRate(host.Rate, host.Paused)
}

func (c *Core) applyRate(rate float64, paused bool) {
	if c.adapter == nil {
		return
	}
	_ = c.adapter.ApplyState(model.ApplyState{
		PositionMs: -1,
		Paused:     paused,
		Rate:       rate,
	})
}
