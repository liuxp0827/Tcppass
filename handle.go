package main

import (
	. "github/liuxp0827/tcppass/dump"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github/liuxp0827/tcppass/common/log"
	"github/liuxp0827/tcppass/tcpassembly"
	"time"
	"os"
)

func handle1(handle *pcap.Handle, assembler *tcpassembly.Assembler) {

	var packetSource *gopacket.PacketSource
	packetSource = gopacket.NewPacketSource(handle, handle.LinkType())
	packets := packetSource.Packets()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	//var PacketsReceived, PacketsDropped, PacketsIfDropped int64

	Dumper.SetFile("ppl.log", false)

	for {
		select {
		case packet := <-packets:
			if packet == nil {
				log.Errorf("packet is nil")
				Dumper.DumpMap()
				os.Exit(0)
			}

			if packet.NetworkLayer() == nil || packet.TransportLayer() == nil {
				continue
			}

			layerType := packet.TransportLayer().LayerType()
			switch layerType {
			case layers.LayerTypeTCP:
				tcp := packet.TransportLayer().(*layers.TCP)
				//if *VDump {
				//	Dumper.Dump(packet)
				//} else if *VVdump {
				//	Dumper.Dump(packet.Dump())
				//} else {
				//	Dumper.DumpTcp(packet.NetworkLayer().NetworkFlow(), tcp, packet.Metadata().Timestamp)
				//}
				assembler.Assemble(packet.NetworkLayer().NetworkFlow(), tcp, packet.Metadata().Timestamp)

			case layers.LayerTypeUDP:
				udp := packet.TransportLayer().(*layers.UDP)
				Dumper.DumpPPLUDP(packet.NetworkLayer().NetworkFlow(), udp, packet.Metadata().Timestamp)
			default:
			}

		case <-ticker.C:
			//stats, _ := handle.Stats()
			//log.Infof("[Pcap] %s increase Received %d, Dropped %d, IfDropped %d",
			//	assembler.Iface, int64(stats.PacketsReceived)-PacketsReceived,
			//	int64(stats.PacketsDropped)-PacketsDropped, int64(stats.PacketsIfDropped)-PacketsIfDropped)
			//
			//PacketsReceived = int64(stats.PacketsReceived)
			//PacketsDropped = int64(stats.PacketsDropped)
			//PacketsIfDropped = int64(stats.PacketsIfDropped)
		}
	}
}

func handle2(handle *pcap.Handle, assembler *tcpassembly.Assembler) {

	// We use a DecodingLayerParser here instead of a simpler PacketSource.
	// This approach should be measurably faster, but is also more rigid.
	// PacketSource will handle any known type of packet safely and easily,
	// but DecodingLayerParser will only handle those packet types we
	// specifically pass in.  This trade-off can be quite useful, though, in
	// high-throughput situations.
	var eth layers.Ethernet
	var dot1q layers.Dot1Q
	var ip4 layers.IPv4
	var ip6 layers.IPv6
	var ip6extensions layers.IPv6ExtensionSkipper
	var tcp layers.TCP
	var udp layers.UDP
	var payload gopacket.Payload
	parser := gopacket.NewDecodingLayerParser(layers.LayerTypeEthernet,
		&eth, &dot1q, &ip4, &ip6, &ip6extensions, &tcp, &udp, &payload)
	decoded := make([]gopacket.LayerType, 0, 4)
	nextFlush := time.Now().Add(timeout / 2)
loop:
	for {
		// Check to see if we should flush the streams we have
		// that haven't seen any new data in a while.  Note we set a
		// timeout on our PCAP handle, so this should happen even if we
		// never see packet data.
		if time.Now().After(nextFlush) {
			stats, _ := handle.Stats()
			log.Infof("Flushing all streams that haven't seen packets in the last 2 minutes, pcap stats: %+v", stats)
			//duration := assembler.FlushOlderThan(time.Now().Add(-timeout))
			//log.Infof("FlushOlderThan Duration: %v", duration)
			nextFlush = time.Now().Add(timeout / 2)
		}

		// To speed things up, we're also using the ZeroCopy method for
		// reading packet data.  This method is faster than the normal
		// ReadPacketData, but the returned bytes in 'data' are
		// invalidated by any subsequent ZeroCopyReadPacketData call.
		// Note that tcptcpassembly is entirely compatible with this packet
		// reading method.  This is another trade-off which might be
		// appropriate for high-throughput sniffing:  it avoids a packet
		// copy, but its cost is much more careful handling of the
		// resulting byte slice.
		data, ci, err := handle.ReadPacketData()
		//		data, ci, err := handle.ZeroCopyReadPacketData()

		if err != nil {
			log.Errorf("error getting packet: %v", err)
			continue
		}
		err = parser.DecodeLayers(data, &decoded)
		if err != nil {
			//log.Errorf("error decoding packet: %v", err)
			continue
		}

		// Find either the IPv4 or IPv6 address to use as our network
		// layer.
		foundNetLayer := false
		var netFlow gopacket.Flow

		for _, typ := range decoded {
			switch typ {
			case layers.LayerTypeIPv4:
				netFlow = ip4.NetworkFlow()
				foundNetLayer = true
			case layers.LayerTypeIPv6:
				netFlow = ip6.NetworkFlow()
				foundNetLayer = true
			case layers.LayerTypeTCP:
				if foundNetLayer {
					//Dumper.DumpTcp(netFlow, &tcp, ci.Timestamp)
					assembler.Assemble(netFlow, &tcp, ci.Timestamp)
				}
				continue loop
				//case layers.LayerTypeUDP:
				//	if foundNetLayer {
				//		Dumper.DumpUdp(netFlow, &udp, ci.Timestamp)
				//	}
				//	continue loop
			}
		}
		log.Warn("could not find TCP/UDP layer")
	}
}
