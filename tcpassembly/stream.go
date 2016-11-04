package tcpassembly

import (
	"flag"
	"math"
	"github.com/liuxp0827/Tcppass/common/cache"
	"github.com/liuxp0827/Tcppass/dpi"
	"github.com/google/gopacket/layers"
	"github.com/liuxp0827/Tcppass/httpassembly"
	"github.com/liuxp0827/Tcppass/common/log"
	"github.com/liuxp0827/Tcppass/stat"
	"sync"
	"time"
)

var timeoutRtt = flag.Int("t", 300000, "timeout for rtt, default 300000 ms")

const timeout time.Duration = time.Minute * 2

type pbody struct {
	key key
	tcp layers.TCP
	ts  time.Time
}

type stream struct {
	pool    *StreamPool
	key     key
	c2s     *conn
	s2c     *conn
	connMap map[key]*conn

	data      chan pbody
	firstSeen time.Time
	lastSeen  time.Time
	closed    bool
	mu        sync.Mutex

	CloseFlag                                  int32
	SYNRTT, MinRTT, MaxRTT, TotalRTT, RttCount int64

	RttCache *cache.RTTCache
	stat     *stat.Stats

	StreamType int

	dpitotal int
	dpier    *dpi.DPIEnginer
	Req      *httpassembly.HTTPRequest
	Resp     *httpassembly.HTTPResponse
}

func (s *stream) reset(pool *StreamPool, k key, stat *stat.Stats, ts time.Time) {
	if s.pool == nil {
		s.pool = pool
	}

	s.key = k

	if s.c2s == nil {
		s.c2s = s.newConn(k, true)
	} else {
		s.c2s.reset(k, true)
	}

	if s.s2c == nil {
		s.s2c = s.newConn(k.Reverse(), false)
	} else {
		s.s2c.reset(k.Reverse(), false)
	}

	s.c2s.reverse = s.s2c
	s.s2c.reverse = s.c2s

	if s.connMap == nil {
		s.connMap = make(map[key]*conn, 2)
	}

	s.connMap[k] = s.c2s
	s.connMap[k.Reverse()] = s.s2c

	s.firstSeen = ts
	s.lastSeen = ts
	s.closed = false

	if s.data == nil {
		s.data = make(chan pbody, 10)
	}

	s.MinRTT = math.MaxInt64
	s.MaxRTT = math.MinInt64
	s.TotalRTT = 0
	s.RttCount = 0
	s.SYNRTT = -1

	if s.RttCache == nil {
		s.RttCache = cache.NewRTTCache(time.Duration(*timeoutRtt) * time.Millisecond)
	}

	s.RttCache.Reset()
	s.stat = stat

	s.StreamType = dpi.UNKNOWN
	s.dpitotal = 0
	if s.dpier == nil {
		s.dpier = dpi.DefaultDPIEnginer
	}

	s.Req = nil
	s.Resp = nil

	go s.dump(60000)
}

func (s *stream) getConn(k key) *conn {
	if conn, ok := s.connMap[k]; ok {
		return conn
	}
	return nil
}

func (s *stream) handle(key key, tcp *layers.TCP, ts time.Time) {
	s.mu.Lock()

	if s.closed {
		s.mu.Unlock()
		return
	}

	s.mu.Unlock()

	ttcp := *tcp

	s.data <- pbody{
		key: key,
		tcp: ttcp,
		ts:  ts,
	}
}

func (s *stream) dump(interval int) {

	ticker := time.NewTicker(time.Duration(interval) * time.Millisecond)
	defer ticker.Stop()
	var conn *conn

	for !s.closed {
		select {
		case data, ok := <-s.data:
			if ok && !s.closed {
				conn = s.getConn(data.key)
				if conn == nil {
					continue
				}

				if s.lastSeen.Before(data.ts) {
					s.lastSeen = data.ts
				}

				conn.handle(data.tcp, data.ts)
			}
		case <-ticker.C:
			if s.lastSeen.Before(time.Now().Add(-timeout)) {
				s.close(true)
			}
		}
	}

	for len(s.data) > 0 {
		<-s.data
	}

	return
}

func (s *stream) close(timeout bool) {
	s.mu.Lock()
	if !s.closed {
		s.closed = true
		s.resetMap()
		s.RttCache.RemoveAll()
		s.finish(timeout)
		s.pool.remove(s)
	}
	s.mu.Unlock()
}

func (s *stream) resetMap() {
	delete(s.connMap, s.key)
	delete(s.connMap, s.key.Reverse())
}

func (s *stream) dpicb(streamType int, entry interface{}) {

	switch streamType {
	case dpi.UNKNOWN:
	case dpi.HTTP_REQUEST:
		s.Req = entry.(*httpassembly.HTTPRequest)
	case dpi.HTTP_RESPONSE:
		s.Resp = entry.(*httpassembly.HTTPResponse)
		if s.Req != nil && s.Resp != nil {
			httpStream := httpassembly.NewHttpStream(s.Req, s.Resp)
			log.Alertf("[%v] %s", s.key, httpStream)

			s.StreamType = dpi.HTTP
			s.dpitotal = 0
		}
	default:

	}
}

func (s *stream) finish(timeout bool) {
	var timeoutFinish string

	if timeout {
		timeoutFinish = "TIMEOUT FINISH"
	} else {
		timeoutFinish = "FINISH"
	}

	switch s.StreamType {
	default:
		log.Noticef("[%v] %s %s %s, Duration[%v]",
			s.key, timeoutFinish, s.BPStat(true), s.RTTStat(), s.lastSeen.Sub(s.firstSeen))
	}

}
