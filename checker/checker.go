/*
	Copyright 2025 Jan Blaesi

	Permission is hereby granted, free of charge, to any person obtaining a copy of this software
	and associated documentation files (the “Software”), to deal in the Software without
	restriction, including without limitation the rights to use, copy, modify, merge, publish,
	distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the
	Software is furnished to do so, subject to the following conditions:

	The above copyright notice and this permission notice shall be included in all copies or
	substantial portions of the Software.

	THE SOFTWARE IS PROVIDED “AS IS”, WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
	IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
	FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL
	THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR
	OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE,
	ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR
	OTHER DEALINGS IN THE SOFTWARE.
*/

package checker

import (
	"hlscheck/plist"
	"io"
	"log/slog"
	"net/http"
	"time"
)

type Checker struct {
	// URL is the URL of the playlist to fetch.
	URL string
	// CurrentMediaSequence is the current media sequence value used to indicate which segments still need to be checked.
	CurrentMediaSequence uint64
	// ClientErrorCount is the number of HTTP client (4xx) errors that occured while checking.
	ClientErrorCount uint64
	// ServerErrorCount is the number of HTTP server (5xx) errors that occured while checking.
	ServerErrorCount uint64
	// ProtocolErrorCount is the number of HTTP protocol errors that occured while checking.
	ProtocolErrorCount uint64
	// EmptySegmentErrorCount is the number of empty segment errors that occured while checking.
	EmptySegmentErrorCount uint64
}

type CheckSegmentResult uint

const (
	CheckOK CheckSegmentResult = iota
	CheckClientError
	CheckServerError
	CheckProtocolError
	CheckEmptySegmentError
)

// New creates a new instance of the HLS checker for a variant.
func New(url string) Checker {
	slog.Info("Starting HLS checker", "url", url)

	c := Checker{
		URL: url,
	}
	go c.Loop()

	return c
}

// Loop will periodically check the playlist for new entries and check the segments.
func (c *Checker) Loop() {
	runTimer := time.NewTicker(time.Second)
	defer runTimer.Stop()

	for {
		<-runTimer.C

		pl := plist.Plist{}
		if err := plist.FetchAndParse(&pl, c.URL); err != nil {
			slog.Error("Fetching variant playlist failed", "url", c.URL)
			continue
		}

		for _, seg := range pl.Entries {
			// Skip all segments that have already been checked.
			if seg.MediaSequence <= c.CurrentMediaSequence {
				continue
			}

			c.CurrentMediaSequence = seg.MediaSequence
		}
	}
}

// RetryCheckSegment will try to fetch a segment three times before failing.
func (c *Checker) RetryCheckSegment(seg plist.Entry) {
	result := CheckOK

	numRetries := 3
	for numRetries > 0 {
		result = c.CheckSegment(seg)
		if result == CheckOK {
			break
		}

		numRetries--
		time.Sleep(250 * time.Millisecond)
	}

	switch result {
	case CheckClientError:
		c.ClientErrorCount++
		slog.Error("Client (4xx) error while fetching segment", "url", seg.URL)
	case CheckServerError:
		c.ServerErrorCount++
		slog.Error("Server (5xx) error while fetching segment", "url", seg.URL)
	case CheckProtocolError:
		c.ProtocolErrorCount++
		slog.Error("HTTP Protocol error while fetching segment", "url", seg.URL)
	case CheckEmptySegmentError:
		c.EmptySegmentErrorCount++
		slog.Error("Received empty segment", "url", seg.URL)
	default:
		break
	}
}

// CheckSegment will try to fetch a segment by its URL
func (c *Checker) CheckSegment(seg plist.Entry) CheckSegmentResult {
	resp, err := http.Get(seg.URL)
	if err != nil {
		return CheckProtocolError
	}
	defer resp.Body.Close()

	if resp.StatusCode > 500 {
		return CheckServerError
	} else if resp.StatusCode > 400 {
		return CheckClientError
	}

	bodyData, err := io.ReadAll(resp.Body)
	if err != nil {
		return CheckProtocolError
	}
	if len(bodyData) == 0 {
		return CheckEmptySegmentError
	}

	return CheckOK
}
