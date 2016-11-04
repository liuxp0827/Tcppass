package cache

import (
	"container/list"
	"fmt"
	"github.com/google/gopacket"
	"github/liuxp0827/tcppass/common/log"
	"github/liuxp0827/tcppass/tcp"
	"sync"
	"time"
)

var logger *log.ZeusLogger

var blog bool

func init() {
	logger = log.NewLogger(100000)
}

func SetCacheLog(cacheLogFile string) {
	if cacheLogFile != "" {
		logger.SetLogFile(cacheLogFile, log.LevelDebug, true, true, 15)
		blog = true
	}
}

type RTTCache struct {
	closed bool
	sync.Mutex
	Duration   time.Duration
	cache      map[interface{}]*list.Element
	linkedList *list.List
}

type RTTCacheKey struct {
	Net       gopacket.Flow
	Transport gopacket.Flow
	Seq       tcp.Sequence
}

func (k RTTCacheKey) String() string {
	return fmt.Sprintf("%s:%s->%s:%s [%d]", k.Net.Src(), k.Transport.Src(), k.Net.Dst(), k.Transport.Dst(), k.Seq)
}

func (k RTTCacheKey) sameKey(rckey RTTCacheKey) bool {
	return k.Net.String() == rckey.Net.String() && k.Transport.String() == rckey.Transport.String()
}

type RTTCacheValue struct {
	key  RTTCacheKey
	Seen time.Time
}

func (rcache *RTTCacheValue) String() string {
	return fmt.Sprintf("%s", rcache.key)
}

func NewRTTCache(Duration time.Duration) *RTTCache {

	return &RTTCache{
		Duration:   Duration,
		linkedList: list.New(),
		cache:      make(map[interface{}]*list.Element),
	}
}

func (c *RTTCache) Length() int {
	return c.linkedList.Len()
}

func (c *RTTCache) output(in bool, format string, v ...interface{}) {
	if blog {
		if in {
			logger.Infof(format, v...)
		} else {
			logger.Debugf(format, v...)
		}
	}
}

func (c *RTTCache) Push(key RTTCacheKey, ts time.Time) (exist bool, err error) {
	c.Lock()
	defer c.Unlock()
	if c.closed {
		return false, fmt.Errorf("RTTCache is closed")
	}

	//c.output(true, "Push key: %v", key)
	var e *list.Element
	if e, ok := c.cache[key]; ok {
		c.linkedList.MoveToFront(e)
		e.Value.(*RTTCacheValue).Seen = ts
		exist = true
		return
	}
	e = c.linkedList.PushBack(&RTTCacheValue{key, ts})
	c.cache[key] = e
	return
}

// looks up a key's value from the cache, move item to the front of LRU queue
func (c *RTTCache) Pull(key RTTCacheKey) (*RTTCacheValue, bool, error) {
	c.Lock()
	defer c.Unlock()
	if c.closed {
		return nil, false, fmt.Errorf("RTTCache is closed")
	}

	//c.output(false, "Pull key: %v", key)
	var e *list.Element
	var value *RTTCacheValue
	var ok bool

	if e, ok = c.cache[key]; ok {
		value = e.Value.(*RTTCacheValue)
		for e != nil {
			tmp := e.Prev()
			c.removeElement(e)
			e = tmp
		}
		//c.output(false, "%s Pull value: %v", c.Name, value)
		return value, true, nil
	}

	for e = c.linkedList.Front(); e != nil; {
		if tmpV := e.Value.(*RTTCacheValue); tmpV != nil && tmpV.key.sameKey(key) {
			if tmpV.key.Seq <= key.Seq+1 {
				tmpE := e.Next()
				c.removeElement(e)
				e = tmpE
				value = tmpV
			} else {
				break
			}
		} else {
			e = e.Next()
		}
	}

	if value == nil || (value.key.Seq != key.Seq+1 && value.key.Seq != key.Seq-1) {
		return nil, false, nil
	}

	//c.output(false, "%s Pull value: %v", c.Name, value)
	return value, true, nil

}

func (c *RTTCache) Purge() (err error) {
	c.Lock()
	defer c.Unlock()

	for e := c.linkedList.Front(); e != nil; {
		key := e.Value.(*RTTCacheValue).key
		timestamp := e.Value.(*RTTCacheValue).Seen
		if c.Duration != 0 && time.Now().Sub(timestamp) > c.Duration {
			if ele, hit := c.cache[key]; hit {
				e = c.removeElement(ele)
			}
			continue
		}
		return
	}
	return
}

// Remove removes the provided key from the cache.
func (c *RTTCache) Remove(key RTTCacheKey) (err error) {
	c.Lock()
	defer c.Unlock()
	if e, hit := c.cache[key]; hit {
		c.removeElement(e)
	}
	return
}

func (c *RTTCache) RemoveAll() error {
	c.Lock()
	defer c.Unlock()
	for e := c.linkedList.Front(); e != nil; {
		e = c.removeElement(e)
	}
	c.closed = true
	return nil
}

func (c *RTTCache) Reset() {
	c.Lock()
	defer c.Unlock()
	c.closed = false
}

func (c *RTTCache) RemoveOldest() (err error) {
	c.Lock()
	defer c.Unlock()

	c.removeOldest()
	return
}
func (c *RTTCache) removeOldest() {
	if c.cache == nil {
		return
	}
	e := c.linkedList.Back()
	if e != nil {
		c.removeElement(e)
	}
}

func (c *RTTCache) removeElement(e *list.Element) *list.Element {
	eNext := e.Next()
	c.linkedList.Remove(e)
	kv := e.Value.(*RTTCacheValue)
	delete(c.cache, kv.key)
	return eNext
}

// Len returns the number of items in the cache.
func (c *RTTCache) Len() int {
	c.Lock()
	defer c.Unlock()
	if c.cache == nil {
		return 0
	}
	return c.linkedList.Len()
}
