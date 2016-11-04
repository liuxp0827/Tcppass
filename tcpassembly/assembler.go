package tcpassembly

import (
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/liuxp0827/Tcppass/stat"
	"time"
)

type Assembler struct {
	Iface      string
	stat       *stat.Stats
	streamPool *StreamPool
}

func NewAssembler(Iface string, pool *StreamPool) *Assembler {
	pool.mu.Lock()
	pool.users++
	pool.mu.Unlock()
	stat := stat.NewStats(Iface, 10)
	return &Assembler{
		Iface:      Iface,
		stat:       stat,
		streamPool: pool,
	}
}

func (a *Assembler) Assemble(netFlow gopacket.Flow, tcp *layers.TCP, ts time.Time) {
	key := key{netFlow, tcp.TransportFlow()}
	end := tcp.FIN || tcp.RST

	_stream := a.streamPool.getStream(key, a.stat, end || (!tcp.SYN && !tcp.PSH && len(tcp.LayerPayload()) == 0), tcp.SYN && !tcp.ACK, ts)

	if _stream == nil {
		//log.Errorf("key %s, Seq: %d, Ack: %d, FIN: %v, %s", key, tcp.Seq, tcp.Ack, tcp.FIN || tcp.RST, ts.Format("2006-01-02 15:04:05.999999"))
		return
	}
	//log.Debugf("flow 111 key %s, Seq: %d, Ack: %d", key, tcp.Seq, tcp.Ack)
	_stream.handle(key, tcp, ts)
}

func (a *Assembler) FlushOlderThan(t time.Time) time.Duration {
	start := time.Now()
	streams := a.streamPool.allStream()
	for _, stream := range streams {
		if stream.lastSeen.Before(t) {
			stream.close(true)
		}
	}
	return time.Now().Sub(start)
}
