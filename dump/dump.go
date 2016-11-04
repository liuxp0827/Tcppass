package dump

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/liuxp0827/Tcppass/common/log"
	"time"
)

func init() {
	Dumper = NewDump()
}

var Log = flag.String("capture", "", "Log file which save detail packets captured by Pass")
var VDump = flag.Bool("v", false, "If true, dump verbose info on each packet")
var VVdump = flag.Bool("V", false, "If true, dump very verbose info on each packet")

type Dump struct {
	log  *log.ZeusLogger
	init bool
}

var Dumper *Dump

func NewDump() *Dump {
	logger := log.NewLogger(100000)
	logger.EnableFuncCallDepth(false)
	return &Dump{
		log: logger,
	}
}

func (d *Dump) SetFile(file string, color bool) {
	d.init = true
	d.log.SetLogFile(file, log.LevelDebug, true, color, 15)
}

func (d *Dump) Dump(v ...interface{}) {
	d.log.Info(v...)
}

func (d *Dump) DumpTcp(netFlow gopacket.Flow, tcp *layers.TCP, ts time.Time) {
	if !d.init {
		return
	}

	flag := "["
	if tcp.SYN {
		flag += " SYN"
	}
	if tcp.FIN {
		flag += " FIN"
	}
	if tcp.RST {
		flag += " RST"
	}
	if tcp.PSH {
		flag += " PSH"
	}
	if tcp.URG {
		flag += " URG"
	}
	if tcp.ACK {
		flag += " ACK"
	}
	flag += " ]"

	from := fmt.Sprintf("%s:%s", netFlow.Src(), tcp.TransportFlow().Src())
	to := fmt.Sprintf("%s:%s", netFlow.Dst(), tcp.TransportFlow().Dst())
	direct := fmt.Sprintf("%-21s -> %-21s", from, to)
	time := fmt.Sprintf("Time[ %-26s ]", ts.Format("2006-01-02 15:04:05.999999"))
	d.log.Infof("TCP  %-46s %s Len[ %-4d ] Seq[ %-10d ] Ack[ %-10d ] Win[ %-5d ] Flag%s",
		direct, time, len(tcp.Payload), tcp.Seq, tcp.Ack, tcp.Window, flag)
}

func (d *Dump) DumpUDP(netFlow gopacket.Flow, udp *layers.UDP, ts time.Time) {
	if !d.init {
		return
	}

	from := fmt.Sprintf("%s:%s", netFlow.Src(), udp.TransportFlow().Src())
	to := fmt.Sprintf("%s:%s", netFlow.Dst(), udp.TransportFlow().Dst())
	direct := fmt.Sprintf("%-21s -> %-21s", from, to)
	time := fmt.Sprintf("Time[ %-26s ]", ts.Format("2006-01-02 15:04:05.999999"))
	d.log.Infof("UDP  %-46s %s Len[ %-4d ]",
		direct, time, len(udp.Payload))
}

func (d *Dump) DumpPPLUDP(netFlow gopacket.Flow, udp *layers.UDP, ts time.Time) {

	from := fmt.Sprintf("%s:%s", netFlow.Src(), udp.TransportFlow().Src())
	to := fmt.Sprintf("%s:%s", netFlow.Dst(), udp.TransportFlow().Dst())
	//direct := fmt.Sprintf("%-21s -> %-21s", from, to)
	//time := fmt.Sprintf("Time[ %-26s ]", ts.Format("2006-01-02 15:04:05.999999"))
	////log.Infof("UDP  %-46s %s Len[ %-4d ]",
	////	direct, time, len(udp.Payload))

	if udp.Payload[4] == 0x5b {
		pRequest := PieceRequest{}
		pRequest.Length = len(udp.Payload)
		for i := 0; i < 4; i++ {
			pRequest.CheckSum[i] = udp.Payload[i]
		}

		buf := bytes.NewBuffer(udp.Payload[5:9])
		binary.Read(buf, binary.LittleEndian, &pRequest.SeqId)
		buf = bytes.NewBuffer(udp.Payload[9:13])
		binary.Read(buf, binary.LittleEndian, &pRequest.Version)
		pRequest.Hash = udp.Payload[13:29]

		var count uint16
		buf = bytes.NewBuffer(udp.Payload[29:31])
		binary.Read(buf, binary.LittleEndian, &count)

		if length := len(udp.Payload); length != PPL_PEER_PACKET_HEADER_LEN+PPL_HASH_LEN+2+int(count)*4+2 {
			log.Fatalf("invalid piece requset data, length of piece requset expect %d, but %d",
				PPL_PEER_PACKET_HEADER_LEN+PPL_HASH_LEN+2+int(count)*4+2, length)
			return
		}

		pRequest.Requests = make([]*Request, count, count)

		index := 31
		for i := 0; i < int(count); i++ {
			buf = bytes.NewBuffer(udp.Payload[index : index+2])
			req := new(Request)

			binary.Read(buf, binary.LittleEndian, &req.PieceIndex)
			index += 2
			buf = bytes.NewBuffer(udp.Payload[index : index+2])
			binary.Read(buf, binary.LittleEndian, &req.BlockIndex)
			index += 2
			pRequest.Requests[i] = req
		}

		for _, req := range pRequest.Requests {

			if v, ok := PPLreqmap[fmt.Sprintf("%s#%X#%d:%d", from, pRequest.Hash, req.PieceIndex, req.BlockIndex)]; !ok {
				PPLreqmap[fmt.Sprintf("%s#%X#%d:%d", from, pRequest.Hash, req.PieceIndex, req.BlockIndex)] = 1
			} else {
				v++
				PPLreqmap[fmt.Sprintf("%s#%X#%d:%d", from, pRequest.Hash, req.PieceIndex, req.BlockIndex)] = v
			}
		}
	} else if udp.Payload[4] == 0x56 {
		pRes := PieceResponse{}
		pRes.Length = PPL_PEER_PIECE_RESP_LEN
		buf := bytes.NewBuffer(udp.Payload[5:9])
		binary.Read(buf, binary.LittleEndian, &pRes.SeqId)
		buf = bytes.NewBuffer(udp.Payload[9:13])
		binary.Read(buf, binary.LittleEndian, &pRes.Version)
		pRes.Hash = udp.Payload[13:29]
		pRes.PeerId = udp.Payload[29:45]

		buf = bytes.NewBuffer(udp.Payload[45:47])
		binary.Read(buf, binary.LittleEndian, &pRes.PieceIndex)
		buf = bytes.NewBuffer(udp.Payload[47:49])
		binary.Read(buf, binary.LittleEndian, &pRes.BlockIndex)
		buf = bytes.NewBuffer(udp.Payload[49:51])
		binary.Read(buf, binary.LittleEndian, &pRes.Size)

		if pRes.Size > PPL_PEER_PIECE_BLOCK_LEN {
			log.Fatalf("invalid piece response data, length of piece Data must <= %d, but %d", PPL_PEER_PIECE_BLOCK_LEN, pRes.Size)
			return
		}

		if v, ok := PPLrespmap[fmt.Sprintf("%s#%X#%d:%d", to, pRes.Hash, pRes.PieceIndex, pRes.BlockIndex)]; !ok {
			PPLrespmap[fmt.Sprintf("%s#%X#%d:%d", to, pRes.Hash, pRes.PieceIndex, pRes.BlockIndex)] = 1
		} else {
			v++
			PPLrespmap[fmt.Sprintf("%s#%X#%d:%d", to, pRes.Hash, pRes.PieceIndex, pRes.BlockIndex)] = v
		}
	}

}

const PPL_PEER_PACKET_HEADER_LEN = 4 + 1 + 4 + 4
const PPL_HASH_LEN = 16
const PPL_PEER_PIECE_RESP_LEN = 1075
const PPL_PEER_PIECE_BLOCK_LEN = 0x0400

type Request struct {
	PieceIndex uint16
	BlockIndex uint16
}

type PieceRequest struct {
	CheckSum [4]byte
	Length   int
	SeqId    uint32 // sequence id
	Version  uint32
	Hash     []byte // file hash
	Requests []*Request
}

type PieceResponse struct {
	Length     int
	SeqId      uint32 // sequence id
	Version    uint32
	Hash       []byte // file hash
	PeerId     []byte
	PieceIndex uint16
	BlockIndex uint16
	Size       uint16
}

var PPLreqmap map[string]int = make(map[string]int)
var PPLrespmap map[string]int = make(map[string]int)

func (d *Dump) DumpMap() {
	var output string
	retran := 0
	reqcount := len(PPLreqmap)
	respcount := len(PPLrespmap)
	for k, v := range PPLreqmap {
		if v > 2 {
			retran++
		}
		output = fmt.Sprintf("req %s %d", k, v)
		if resp, ok := PPLrespmap[k]; ok {
			output += fmt.Sprintf(" || resp %s %d", k, resp)
			delete(PPLrespmap, k)
		}
		d.log.Infof("%s", output)
	}

	d.log.Infof("##################################################")
	for k, v := range PPLrespmap {
		d.log.Infof("resp %s %d", k, v)
	}
	d.log.Infof("reqcount :%d, respcount: %d, %d", reqcount, respcount, retran)
}
