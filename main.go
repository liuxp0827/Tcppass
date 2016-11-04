package main

import (
	"flag"
	"github.com/google/gopacket/pcap"
	"net/http"
	_ "net/http/pprof"
	"runtime"
	"github.com/liuxp0827/Tcppass/common/cache"
	. "github.com/liuxp0827/Tcppass/common/config"
	"github.com/liuxp0827/Tcppass/common/log"
	. "github.com/liuxp0827/Tcppass/dump"
	"github.com/liuxp0827/Tcppass/stat"
	"github.com/liuxp0827/Tcppass/tcpassembly"
	"time"
)

var conf = flag.String("config", "pass.json", "pass config file")
var fname = flag.String("r", "", "Filename to read from, overrides -i")

const timeout time.Duration = time.Minute * 2

func main() {
	flag.Parse()

	if pcapfile := *fname; pcapfile != "" {
		streamPool := tcpassembly.NewStreamPool()
		InitOfflineCapture(pcapfile, streamPool)
	} else {

		err := InitConfig(*conf)
		if err != nil {
			log.Fatal(err)
		}

		log.SetLevel(TConfig.Loglevel)

		stat.Stat(10)

		if logFile := *Log; logFile != "" {
			log.Infof("Set Capture Log: %s", logFile)
			Dumper.SetFile(logFile, false)
		}

		if TConfig.CacheLog != "" {
			cache.SetCacheLog(TConfig.CacheLog)
		}

		streamPool := tcpassembly.NewStreamPool()

		for i := 0; i < len(TConfig.Interfaces); i++ {
			go InitCapture(TConfig.Interfaces[i], streamPool)
		}
	}

	httpPprof()
}

func InitOfflineCapture(pcapFile string, streamPool *tcpassembly.StreamPool) {
	runtime.LockOSThread()

	var handle *pcap.Handle
	var err error
	if handle, err = pcap.OpenOffline(pcapFile); err != nil {
		log.Fatal("PCAP OpenOffline error:", err)
	}

	defer handle.Close()

	// Set up tcpassembly
	assembler := tcpassembly.NewAssembler("offline", streamPool)

	log.Info("reading in packets")
	handle1(handle, assembler)
}

func InitCapture(iface *NetworkIface, streamPool *tcpassembly.StreamPool) {
	runtime.LockOSThread()

	var handle *pcap.Handle
	log.Infof("starting capture on interface %s", iface.Name)
	inactive, err := pcap.NewInactiveHandle(iface.Name)
	if err != nil {
		log.Fatal("could not create: %v", err)
	}

	defer inactive.CleanUp()

	if err = inactive.SetBufferSize(iface.BufferSize * 1024 * 1024); err != nil {
		log.Fatal("could not set buffersize: %v", err)
	} else if err = inactive.SetSnapLen(iface.Snaplen); err != nil {
		log.Fatal("could not set snap length: %v", err)
	} else if err = inactive.SetPromisc(iface.Promisc); err != nil {
		log.Fatal("could not set promisc mode: %v", err)
	} else if err = inactive.SetTimeout(pcap.BlockForever); err != nil {
		log.Fatal("could not set timeout: %v", err)
	}

	if handle, err = inactive.Activate(); err != nil {
		log.Fatal("PCAP Activate error:", err)
	}

	defer handle.Close()

	if iface.BPFFilter != "" {
		if err = handle.SetBPFFilter(iface.BPFFilter); err != nil {
			log.Fatalf("BPF %s filter error: %v", iface.BPFFilter, err)
		}
	}

	// Set up tcpassembly
	assembler := tcpassembly.NewAssembler(iface.Name, streamPool)

	log.Info("reading in packets")
	handle1(handle, assembler)

}

func httpPprof() {
	err := http.ListenAndServe(":9005", nil)
	if err != nil {
		log.Fatal("ListenAndServe:", err)
	}
}
