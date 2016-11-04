package tcpassembly

import (
	"fmt"
	"github/liuxp0827/tcppass/common/cache"
	"github/liuxp0827/tcppass/dpi"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github/liuxp0827/tcppass/common/log"
	. "github/liuxp0827/tcppass/tcp"
	"time"
)

type key [2]gopacket.Flow

func (k key) String() string {
	return fmt.Sprintf("%s:%s->%s:%s", k[0].Src(), k[1].Src(), k[0].Dst(), k[1].Dst())
}
func (k key) Equal(key key) bool {
	return k[0].String() == key[0].String() && k[1].String() == key[1].String()
}

func (k key) Reverse() key {
	return key{k[0].Reverse(), k[1].Reverse()}
}

type conn struct {
	reverse   *conn // 与当前flow反向的flow流
	s         *stream
	key       key
	cli2srv   bool
	closed    bool // 已经关闭
	waitClose bool // 收到FIN，RST 等待关闭

	streamType int
	dpiTotal   int

	Bytes      int64 // total bytes seen on this stream.
	Packets    int64 // total packets seen on this stream.
	OldBytes   int64 // old total bytes seen on this stream.
	OldPackets int64 // old total packets seen on this stream.
}

func (s *stream) newConn(k key, cli2srv bool) *conn {
	return &conn{
		key:        k,
		s:          s,
		cli2srv:    cli2srv,
		streamType: dpi.UNKNOWN,
	}
}

func (c *conn) reset(k key, cli2srv bool) {
	c.key = k
	c.closed = false
	c.waitClose = false
	c.streamType = dpi.UNKNOWN
	c.dpiTotal = 0

	c.cli2srv = cli2srv
	c.Bytes = 0
	c.Packets = 0
	c.OldBytes = 0
	c.OldPackets = 0
}

func (c *conn) close() {
	c.closed = true
	c.reverse.closed = true

	c.s.close(false)
}

func (c *conn) handle(tcp layers.TCP, ts time.Time) {
	end := tcp.FIN || tcp.RST
	if c.closed {
		log.Warnf("conn %s is closed, seq:%d, ack:%d, len:%d, ts:%s",
			c.key, tcp.Seq, tcp.Ack, len(tcp.Payload), ts.Format("2006-01-02 15:04:05.999999"))
		return
	}

	finish := false

	if end {
		// 如果有两个方向的流，且同时都已经收到FIN或RST，则关闭
		c.waitClose = true
		if c.reverse.waitClose {
			finish = true
		}
	}

	seq, ack, bytes := Sequence(tcp.Seq), Sequence(tcp.Ack), tcp.Payload

	if c.closed {
		return
	}

	c.stat(&Reassembly{
		Seq:   seq,
		Ack:   ack,
		Bytes: bytes,
		Seen:  ts,
		End:   end,
	})

	if finish {
		c.close()
	}
}

func (c *conn) stat(ret *Reassembly) {
	var value *cache.RTTCacheValue
	var ok bool
	var err error

	bytes := int64(len(ret.Bytes))
	c.Bytes += bytes
	c.Packets += 1

	var rcKey cache.RTTCacheKey
	if c.cli2srv { // client 2 server

		if len(ret.Bytes) > 0 {
			rcKey = cache.RTTCacheKey{c.key[0], c.key[1], ret.Seq + Sequence(len(ret.Bytes))}
		} else {
			rcKey = cache.RTTCacheKey{c.key[0], c.key[1], ret.Seq + 1}
		}
		_, err = c.s.RttCache.Push(rcKey, ret.Seen)

		if err != nil {
			log.Errorf("stream %s RttCache %v Push %s failed, %v", c.s.key, &(c.s.RttCache), rcKey, err)
			return
		}

		c.s.stat.AddTXBytes(bytes)
		c.s.stat.AddTXPackets(1)

	} else { // server 2 client
		rcKey = cache.RTTCacheKey{c.key[0].Reverse(), c.key[1].Reverse(), ret.Ack}
		value, ok, err = c.s.RttCache.Pull(rcKey)
		if err != nil {
			log.Errorf("stream %s RttCache %v Pull %s failed, %v", c.s.key, &(c.s.RttCache), rcKey, err)
			return
		}

		c.s.stat.AddRXBytes(bytes)
		c.s.stat.AddRXPackets(1)

		if ok /*&& !ret.End*/ {
			c.s.RttCount++
			rtt := ret.Seen.Sub(value.Seen)
			rttt := rtt.Nanoseconds() / (1000)
			if rttt < 0 {
				return
			}

			if c.s.RttCount == 1 {
				c.s.SYNRTT = rttt
			}

			if rttt < c.s.MinRTT {
				c.s.MinRTT = rttt
			}
			if rttt > c.s.MaxRTT {
				c.s.MaxRTT = rttt
			}
			c.s.TotalRTT += rttt
		}
	}

	// 对前5个包进行dpi流量识别，如果在某个包识别出流量类型，则后续不再进行识别
	if len(ret.Bytes) > 0 &&
		((c.s.StreamType == dpi.UNKNOWN && c.s.dpitotal <= 5) || c.s.StreamType == dpi.HTTP_REQUEST || c.s.StreamType == dpi.HTTP) {
		c.s.dpitotal++
		c.s.dpier.DPIDecoder(ret.Bytes, ret.Seen, c.s.dpicb)
	}
}
