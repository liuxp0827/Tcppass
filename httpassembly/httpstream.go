package httpassembly

import (
	"fmt"
	"net/url"
	"time"
)

type httpStream struct {
	URL          *url.URL
	Host, Method string
	StatusCode   int
	RTT          time.Duration
	//TsReq        time.Time
	//TsResp       time.Time
}

func NewHttpStream(req *HTTPRequest, resp *HTTPResponse) *httpStream {
	httpStream := httpStream{
		URL:        req.URL,
		Host:       req.Host,
		Method:     req.Method,
		StatusCode: resp.StatusCode,
		RTT:        resp.GetTimestamp().Sub(req.GetTimestamp()),
		//TsReq:      req.GetTimestamp(),
		//TsResp:     resp.GetTimestamp(),
	}
	return &httpStream
}

func (hStream *httpStream) String() string {
	return fmt.Sprintf("HTTP %s, %s, %d, RTT %v",
		hStream.Method, hStream.Host+hStream.URL.String(), hStream.StatusCode, hStream.RTT)
}
