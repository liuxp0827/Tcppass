package dpi

import (
	"github/liuxp0827/tcppass/httpassembly"
	"time"
)

func NewHttpDPI() DPI {
	return &httpDPI{}
}

type httpDPI struct {
}

func (hd *httpDPI) Decoder(data []byte, ts time.Time, cb func(streamType int, entry interface{})) error {
	if len(data) > 4 {
		switch string(data[:4]) {
		case "HEAD": // Get
			return hd.checkReq(data, ts, cb)
		case "GET ": // Get
			return hd.checkReq(data, ts, cb)
		case "PUT ": // Put
			return hd.checkReq(data, ts, cb)
		case "POST": // Post
			return hd.checkReq(data, ts, cb)
		case "DELE": // Delete
			return hd.checkReq(data, ts, cb)
		case "HTTP":
			return hd.checkRes(data, ts, cb)
		}
	}
	return nil
}

func (hd *httpDPI) checkReq(payload []byte, ts time.Time, cb func(streamType int, entry interface{})) error {
	req, err := httpassembly.Request(payload, ts)
	if err == nil && cb != nil {
		cb(HTTP_REQUEST, req)
		return nil
	}
	return err
}

func (hd *httpDPI) checkRes(payload []byte, ts time.Time, cb func(streamType int, entry interface{})) error {
	resp, err := httpassembly.Response(payload, ts)
	if err == nil && cb != nil {
		cb(HTTP_RESPONSE, resp)
		return nil
	}
	return err
}

type HTTPBody struct {
	Type int // 0: req, 1: resp
	Req  *httpassembly.HTTPRequest
	Resp *httpassembly.HTTPResponse
}
