package httpassembly

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/textproto"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

type HTTPRequest struct {
	*http.Request
	ts time.Time
}

func (req *HTTPRequest) GetTimestamp() time.Time {
	return req.ts
}

func Request(data []byte, timestamp time.Time) (httpRequest *HTTPRequest, err error) {
	httpRequest = &HTTPRequest{new(http.Request), timestamp}
	b := bufio.NewReader((bytes.NewReader(data)))
	tp := newTextprotoReader(b)

	// First line: GET /index.html HTTP/1.0
	var s string
	if s, err = tp.ReadLine(); err != nil {
		return nil, err
	}

	defer func() {
		putTextprotoReader(tp)
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
	}()

	var ok bool
	httpRequest.Method, httpRequest.RequestURI, httpRequest.Proto, ok = parseRequestLine(s)
	if !ok {
		return nil, errors.New(fmt.Sprintf("%s %q", "malformed HTTP request", s))
	}

	rawurl := httpRequest.RequestURI
	if httpRequest.ProtoMajor, httpRequest.ProtoMinor, ok = ParseHTTPVersion(httpRequest.Proto); !ok {
		return nil, errors.New(fmt.Sprintf("%s %q", "malformed HTTP request", httpRequest.Proto))
	}

	if httpRequest.URL, err = url.ParseRequestURI(rawurl); err != nil {
		//log.Error("ParseRequestURI:", err)
		return nil, err
	}

	// Subsequent lines: Key: value.
	mimeHeader, _ := tp.ReadMIMEHeader()

	httpRequest.Header = http.Header(mimeHeader)

	// RFC2616: Must treat
	//	GET /index.html HTTP/1.1
	//	Host: www.google.com
	// and
	//	GET http://www.google.com/index.html HTTP/1.1
	//	Host: doesntmatter
	// the same.  In the second case, any Host line is ignored.
	httpRequest.Host = httpRequest.URL.Host
	if httpRequest.Host == "" {
		httpRequest.Host = httpRequest.Header.Get("Host")
	}

	fixPragmaCacheControl(httpRequest.Header)

	return httpRequest, nil
}

var textprotoReaderPool sync.Pool

func newTextprotoReader(br *bufio.Reader) *textproto.Reader {
	if v := textprotoReaderPool.Get(); v != nil {
		tr := v.(*textproto.Reader)
		tr.R = br
		return tr
	}
	return textproto.NewReader(br)
}

func putTextprotoReader(r *textproto.Reader) {
	r.R = nil
	textprotoReaderPool.Put(r)
}

// parseRequestLine parses "GET /foo HTTP/1.1" into its three parts.
func parseRequestLine(line string) (method, requestURI, proto string, ok bool) {
	s1 := strings.Index(line, " ")
	s2 := strings.Index(line[s1+1:], " ")
	if s1 < 0 || s2 < 0 {
		return
	}
	s2 += s1 + 1
	return line[:s1], line[s1+1 : s2], line[s2+1:], true
}

// ParseHTTPVersion parses a HTTP version string.
// "HTTP/1.0" returns (1, 0, true).
func ParseHTTPVersion(vers string) (major, minor int, ok bool) {
	const Big = 1000000 // arbitrary upper bound
	switch vers {
	case "HTTP/1.1":
		return 1, 1, true
	case "HTTP/1.0":
		return 1, 0, true
	}
	if !strings.HasPrefix(vers, "HTTP/") {
		return 0, 0, false
	}
	dot := strings.Index(vers, ".")
	if dot < 0 {
		return 0, 0, false
	}
	major, err := strconv.Atoi(vers[5:dot])
	if err != nil || major < 0 || major > Big {
		return 0, 0, false
	}
	minor, err = strconv.Atoi(vers[dot+1:])
	if err != nil || minor < 0 || minor > Big {
		return 0, 0, false
	}
	return major, minor, true
}

// RFC2616: Should treat
//	Pragma: no-cache
// like
//	Cache-Control: no-cache
func fixPragmaCacheControl(header http.Header) {
	if hp, ok := header["Pragma"]; ok && len(hp) > 0 && hp[0] == "no-cache" {
		if _, presentcc := header["Cache-Control"]; !presentcc {
			header["Cache-Control"] = []string{"no-cache"}
		}
	}
}
