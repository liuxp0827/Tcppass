package cache

import (
	"github.com/google/gopacket"
	"github/liuxp0827/tcppass/common/log"
	"github/liuxp0827/tcppass/tcp"
	"testing"
	"time"
)

func TestRTTCache(t *testing.T) {
	cache := NewRTTCache(300000 * time.Millisecond)

	net := gopacket.InvalidFlow
	transport := gopacket.InvalidFlow
	cache.Push(RTTCacheKey{net, transport, tcp.Sequence(3)}, time.Now())

	value, ok, err := cache.Pull(RTTCacheKey{net, transport, tcp.Sequence(2)})
	if err != nil {
		t.Error(err)
	}
	if !ok {
		log.Info("can not find", RTTCacheKey{net, transport, tcp.Sequence(2)})
	}
	log.Info("value:", value)
}
