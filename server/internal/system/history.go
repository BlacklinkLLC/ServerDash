package system

import (
	"context"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
	psnet "github.com/shirou/gopsutil/v3/net"
)

// Sample is one point of the rolling metrics history the overview's charts
// draw from. Network figures are rates (bytes/sec) over the sample interval.
type Sample struct {
	At         time.Time `json:"at"`
	CPUPercent float64   `json:"cpu"`
	MemPercent float64   `json:"mem"`
	Load1      float64   `json:"load1"`
	NetRxBps   float64   `json:"rx"`
	NetTxBps   float64   `json:"tx"`
}

// Sampler records a Sample every interval into a fixed-size ring, so charts
// have history the moment the page loads instead of starting empty.
type Sampler struct {
	interval time.Duration
	size     int

	mu      sync.RWMutex
	samples []Sample
}

func NewSampler(interval time.Duration, size int) *Sampler {
	return &Sampler{interval: interval, size: size}
}

func (s *Sampler) Start(ctx context.Context) {
	go func() {
		var lastRx, lastTx uint64
		var lastAt time.Time
		t := time.NewTicker(s.interval)
		defer t.Stop()
		for {
			now := time.Now()
			sample := Sample{At: now}
			if pct, err := cpu.Percent(0, false); err == nil && len(pct) > 0 {
				sample.CPUPercent = pct[0]
			}
			if vm, err := mem.VirtualMemory(); err == nil {
				sample.MemPercent = vm.UsedPercent
			}
			if avg, err := load.Avg(); err == nil {
				sample.Load1 = avg.Load1
			}
			if rx, tx, ok := netTotals(); ok {
				if !lastAt.IsZero() && rx >= lastRx && tx >= lastTx {
					secs := now.Sub(lastAt).Seconds()
					sample.NetRxBps = float64(rx-lastRx) / secs
					sample.NetTxBps = float64(tx-lastTx) / secs
				}
				lastRx, lastTx, lastAt = rx, tx, now
			}
			s.mu.Lock()
			s.samples = append(s.samples, sample)
			if len(s.samples) > s.size {
				s.samples = s.samples[len(s.samples)-s.size:]
			}
			s.mu.Unlock()

			select {
			case <-ctx.Done():
				return
			case <-t.C:
			}
		}
	}()
}

// History returns a copy of the recorded samples, oldest first. The first
// sample's network rates are always zero (there's no prior reading to diff).
func (s *Sampler) History() []Sample {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Sample, len(s.samples))
	copy(out, s.samples)
	return out
}

// netTotals sums bytes across every non-loopback interface.
func netTotals() (rx, tx uint64, ok bool) {
	counters, err := psnet.IOCounters(true)
	if err != nil {
		return 0, 0, false
	}
	for _, c := range counters {
		if c.Name == "lo" {
			continue
		}
		rx += c.BytesRecv
		tx += c.BytesSent
	}
	return rx, tx, true
}
