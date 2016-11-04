package stat

import (
	"fmt"
	"github.com/liuxp0827/Tcppass/common/log"
	"runtime"
	"sync/atomic"
	"time"
)

var TXBytes int64 = 0
var RXBytes int64 = 0
var TXPackets int64 = 0
var RXPackets int64 = 0

type Stats struct {
	name      string
	interval  int
	TXBytes   int64
	RXBytes   int64
	TXPackets int64
	RXPackets int64
}

func NewStats(iface string, interval int) *Stats {
	stat := Stats{
		name:     iface,
		interval: interval,
	}
	go stat.Stat()
	return &stat
}

func (s *Stats) Stat() {
	go func() {
		ticker := time.NewTicker(time.Duration(s.interval) * time.Second)
		defer ticker.Stop()
		var txBytes, txPackets, rxBytes, rxPackets int64
		var oldtxBytes, oldtxPackets, oldrxBytes, oldrxPackets int64

		for {
			select {
			case <-ticker.C:

				txBytes = s.LoadTXBytes()
				txPackets = s.LoadTXPackets()
				rxBytes = s.LoadRXBytes()
				rxPackets = s.LoadRXPackets()

				log.Alertf("[%s STAT] increase TxBytes[%s], TxPackets[%d], RxBytes[%s], RxPackets[%d]",
					s.name,
					bytes(txBytes-oldtxBytes),
					txPackets-oldtxPackets,
					bytes(rxBytes-oldrxBytes),
					rxPackets-oldrxPackets,
				)

				oldtxBytes = txBytes
				oldtxPackets = txPackets
				oldrxBytes = rxBytes
				oldrxPackets = rxPackets
			}
		}
	}()
}

func (s *Stats) AddTXBytes(bytes int64) {
	atomic.AddInt64(&(s.TXBytes), bytes)
}

func (s *Stats) AddRXBytes(bytes int64) {
	atomic.AddInt64(&(s.RXBytes), bytes)
}

func (s *Stats) AddTXPackets(packets int64) {
	atomic.AddInt64(&(s.TXPackets), packets)
}

func (s *Stats) AddRXPackets(packets int64) {
	atomic.AddInt64(&(s.RXPackets), packets)
}

func (s *Stats) LoadTXBytes() int64 {
	return atomic.LoadInt64(&(s.TXBytes))
}

func (s *Stats) LoadRXBytes() int64 {
	return atomic.LoadInt64(&(s.RXBytes))
}

func (s *Stats) LoadTXPackets() int64 {
	return atomic.LoadInt64(&(s.TXPackets))
}

func (s *Stats) LoadRXPackets() int64 {
	return atomic.LoadInt64(&(s.RXPackets))
}

func Stat(interval int) {
	go func() {
		ticker := time.NewTicker(time.Duration(interval) * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				n, _ := runtime.ThreadCreateProfile(nil)
				log.Infof("[PASSSTAT] Goroutine[%d], Thread[%d]", runtime.NumGoroutine(), n)
			}
		}
	}()
}

func bytes(bytes int64) string {
	if bytes < 5*1024 {
		return fmt.Sprintf("%dB", bytes)
	} else if bytes >= 5*1024 && bytes < 5*1024*1024 {
		return fmt.Sprintf("%.2fKB", float32(bytes)/1024.0)
	} else if bytes >= 5*1024*1024 && bytes < 5*1024*1024*1024 {
		return fmt.Sprintf("%.2fMB", float32(bytes)/(1024.0*1024.0))
	} else {
		return fmt.Sprintf("%.2fGB", float32(bytes)/(1024.0*1024.0*1024.0))
	}
}
