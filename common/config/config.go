package config

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"github/liuxp0827/tcppass/common/json"
	"github/liuxp0827/tcppass/common/log"
	"strings"
)

func init() {
	TConfig = &Config{}
}

type Config struct {
	Interfaces []*NetworkIface `json:"ifaces"`
	Loglevel   int             `json:"loglevel"`
	CacheLog   string          `json:"cacheLog"`
	Timeout    int             `json:"timeout"`
}

type NetworkIface struct {
	Name       string `json:"name"`
	Snaplen    int    `json:"snaplen"`
	BPFFilter  string `json:"filter"`
	BufferSize int    `json:"bufferSize"`
	Promisc    bool   `json:"promisc"`
}

var TConfig *Config

func InitConfig(filename string) error {
	return TConfig.Initialize(filename)
}

func (this *Config) Initialize(filename string) error {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("%s:%s", err, filename)
	}

	if err = json.Unmarshal(data, &this); err != nil {
		if serr, ok := err.(*json.SyntaxError); ok {
			line, col := getOffsetPosition(bytes.NewBuffer(data), serr.Offset)
			highlight := getHighLightString(bytes.NewBuffer(data), line, col)
			fmt.Printf("\n%v", err)
			fmt.Printf(":\n:Error at line %d, column %d (file offset %d):\n%s",
				line, col, serr.Offset, highlight)
		}
		return err
	}

	if this.Interfaces == nil || len(this.Interfaces) == 0 {
		return fmt.Errorf("Interfaces to listen can not be nil")
	}

	if this.Loglevel < 0 || this.Loglevel > 7 {
		this.Loglevel = 6
	}

	if this.Timeout <= 0 {
		this.Timeout = 120
	}

	for _, iface := range this.Interfaces {
		if iface.Snaplen <= 0 {
			iface.Snaplen = 2048
		}

		if iface.BufferSize <= 0 {
			iface.BufferSize = 5120
		}
	}

	log.Info("load config success...")
	return nil
}

func getOffsetPosition(f io.Reader, pos int64) (line, col int) {
	line = 1
	br := bufio.NewReader(f)
	thisLine := new(bytes.Buffer)
	for n := int64(0); n < pos; n++ {
		b, err := br.ReadByte()
		if err != nil {
			break
		}
		if b == '\n' {
			thisLine.Reset()
			line++
			col = 1
		} else {
			col++
			thisLine.WriteByte(b)
		}
	}

	return
}
func getHighLightString(f io.Reader, line int, col int) (highlight string) {
	br := bufio.NewReader(f)
	var thisLine []byte
	var err error
	for i := 1; i <= line; i++ {
		thisLine, _, err = br.ReadLine()
		if err != nil {
			fmt.Println(err)
			return
		}
		if i >= line-2 {
			highlight += fmt.Sprintf("%5d: %s\n", i, string(thisLine))
		}
	}
	highlight += fmt.Sprintf("%s^\n", strings.Repeat(" ", col+5))
	return
}

func exist(path string) bool {
	_, err := os.Stat(path)
	return err == nil || os.IsExist(err)
}
