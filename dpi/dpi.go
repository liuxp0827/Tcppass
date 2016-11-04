package dpi

import (
	"time"
)

func init() {
	DefaultDPIEnginer = NewDPIEnginer()
	DefaultDPIEnginer.Register("http")
}

type dpiType func() DPI

var DefaultDPIEnginer *DPIEnginer

type DPI interface {
	Decoder([]byte, time.Time, func(streamType int, entry interface{})) error
}

type DPIEnginer struct {
	adapters map[string]dpiType
}

func NewDPIEnginer() *DPIEnginer {
	return &DPIEnginer{
		adapters: make(map[string]dpiType),
	}
}

// Register makes a dpi provide available by the provided name.
// If Register is called twice with the same name or if driver is nil,
// it panics.
func (this *DPIEnginer) register(name string, dpiT dpiType) {
	if dpiT == nil {
		panic("dpi: Register provide is nil")
	}
	if _, dup := this.adapters[name]; dup {
		panic("dpi: Register called twice for provider " + name)
	}
	this.adapters[name] = dpiT
}

func (this *DPIEnginer) Register(names ...string) {
	for _, name := range names {
		switch name {
		case "http":
			this.register("http", NewHttpDPI)
		}
	}
}

func (this *DPIEnginer) DPIDecoder(data []byte, ts time.Time, cb func(streamType int, entry interface{})) {
	for _, dup := range this.adapters {
		decoder := dup()
		err := decoder.Decoder(data, ts, cb)
		if err == nil {
			return
		}
	}
}
