package httpassembly

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type HTTPResponse struct {
	*http.Response
	ts time.Time
}

func (resp *HTTPResponse) GetTimestamp() time.Time {
	return resp.ts
}

func Response(data []byte, timestamp time.Time) (httpResponse *HTTPResponse, err error) {
	httpResponse = &HTTPResponse{new(http.Response), timestamp}
	b := bufio.NewReader((bytes.NewReader(data)))
	tp := newTextprotoReader(b)

	// Parse the first line of the response.
	line, err := tp.ReadLine()
	if err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return nil, err
	}

	f := strings.SplitN(line, " ", 3)
	if len(f) < 2 {
		return nil, errors.New(fmt.Sprintf("%s %q", "malformed HTTP response", line))
	}

	reasonPhrase := ""
	if len(f) > 2 {
		reasonPhrase = f[2]
	}

	if len(f[1]) != 3 {
		return nil, errors.New(fmt.Sprintf("%s %q", "malformed HTTP status code", f[1]))
	}

	httpResponse.StatusCode, err = strconv.Atoi(f[1])
	if err != nil || httpResponse.StatusCode < 0 {
		return nil, errors.New(fmt.Sprintf("%s %q", "malformed HTTP status code", f[1]))
	}

	httpResponse.Status = f[1] + " " + reasonPhrase
	httpResponse.Proto = f[0]
	var ok bool
	if httpResponse.ProtoMajor, httpResponse.ProtoMinor, ok = ParseHTTPVersion(httpResponse.Proto); !ok {
		return nil, errors.New(fmt.Sprintf("%s %q", "malformed HTTP version", httpResponse.Proto))
	}

	mimeHeader, _ := tp.ReadMIMEHeader()
	httpResponse.Header = http.Header(mimeHeader)
	fixPragmaCacheControl(httpResponse.Header)

	return httpResponse, nil
}
