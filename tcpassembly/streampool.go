package tcpassembly

import (
	"github/liuxp0827/tcppass/common/log"
	"github/liuxp0827/tcppass/stat"
	"sync"
	"time"
)

const initialAllocSize = 8192

type StreamPool struct {
	streams            map[key]*stream
	users              int
	mu                 *sync.RWMutex
	free               []*stream
	all                [][]stream
	nextAlloc          int
	newConnectionCount int64
}

func NewStreamPool() *StreamPool {
	return &StreamPool{
		streams:   make(map[key]*stream, initialAllocSize),
		free:      make([]*stream, 0, initialAllocSize),
		nextAlloc: initialAllocSize,
		mu:        &sync.RWMutex{},
	}
}

func (sp *StreamPool) grow() {
	streams := make([]stream, sp.nextAlloc)
	sp.all = append(sp.all, streams)
	for i, _ := range streams {
		sp.free = append(sp.free, &streams[i])
	}

	log.Info("StreamPool: created", sp.nextAlloc, "new conns")
	sp.nextAlloc *= 2
}

func (sp *StreamPool) allStream() []*stream {
	sp.mu.RLock()
	defer sp.mu.RUnlock()
	streams := make([]*stream, 0, len(sp.streams))
	for _, stream := range sp.streams {
		streams = append(streams, stream)
	}
	return streams
}

func (sp *StreamPool) newStream(k key, stat *stat.Stats, ts time.Time) (stream *stream) {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	sp.newConnectionCount++
	if sp.newConnectionCount&0x7FFF == 0 {
		log.Info("StreamPool:", sp.newConnectionCount, "requests,", len(sp.streams), "used,", len(sp.free), "free")
	}

	if len(sp.free) == 0 {
		sp.grow()
	}
	index := len(sp.free) - 1
	stream, sp.free = sp.free[index], sp.free[:index]
	stream.reset(sp, k, stat, ts)

	sp.streams[k] = stream
	return stream
}

func (sp *StreamPool) getStream(k key, stat *stat.Stats, end bool, SYN bool, ts time.Time) *stream {
	sp.mu.RLock()
	stream := sp.streams[k]
	if stream == nil {
		stream = sp.streams[k.Reverse()]
	}
	sp.mu.RUnlock()

	if end || !SYN || stream != nil {
		return stream
	}

	log.Infof("created the bidirectional stream %s at %s", k, ts.Format("2006-01-02 15:04:05.999999"))

	stream = sp.newStream(k, stat, ts)
	return stream
}

func (sp *StreamPool) remove(_s *stream) {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	if _s != nil {
		delete(sp.streams, _s.key)
		sp.free = append(sp.free, _s)
	}
}
