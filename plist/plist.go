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

package plist

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
)

type Type int

const (
	VariantPlist Type = iota
	MasterPlist
)

type Entry struct {
	// BandwidthBps is the bandwidth of a stream rendition, only valid for master playlists.
	BandwidthBps uint64
	// Codecs is the list of codecs in use, only valid for master playlists.
	Codecs string
	// MediaSequence is the unique sequence number of the segment, only valid for variant playlists.
	MediaSequence uint64
	// DurationSec is the duration of the segment in seconds, only valid for variant playlists.
	DurationSec float64
	// ExtraInfo is additional information about the segment that is not needed for decoding/parsing
	ExtraInfo string
	// URL is the absolute URL of the referenced segment or playlist.
	URL string
}

type Plist struct {
	// Type defines if this is a master playlist containing other playlists or a variant playlist containing segments.
	Type Type
	// Entries is an array of entries in this playlist.
	Entries []Entry
	// CurrentMediaSequence is the current value of the sequence number for the next segment. To be incremented after each segment.
	CurrentMediaSequence uint64
	// TargetDurationSec is the target duration of each segment.
	TargetDurationSec uint64
}

// parseStreamInfTag will parse an #EXT-X-STREAM-INF tag.
func parseStreamInfTag(e *Entry, tag string) error {
	attrListStr, attrListStrValid := strings.CutPrefix(tag, "EXT-X-STREAM-INF:")
	if !attrListStrValid {
		return fmt.Errorf("malformed EXT-X-STREAM-INF tag")
	}
	attrList := strings.Split(attrListStr, ",")

	bandwidthPresent := false
	for _, attr := range attrList {
		attrSplit := strings.Split(attr, "=")
		if len(attrSplit) < 2 {
			return fmt.Errorf("malformed attribute in EXT-X-STREAM-INF tag")
		}

		attrName := attrSplit[0]
		attrValue := strings.Join(attrSplit[1:], "=")
		switch attrName {
		case "BANDWIDTH":
			bandwidthPresent = true
			bandwidth, err := strconv.ParseUint(attrValue, 10, 64)
			if err != nil {
				return fmt.Errorf("unable to parse bandwidth attribute in EXT-X-STREAM-INF tag")
			}
			e.BandwidthBps = bandwidth
		case "CODECS":
			e.Codecs = strings.Trim(attrValue, "\" ")
		}
	}

	if !bandwidthPresent {
		return fmt.Errorf("missing bandwidth attribute in EXT-X-STREAM-INF tag")
	}

	return nil
}

// parseInfTag will parse an EXTINF tag
func parseInfTag(e *Entry, tag string) error {
	attrListStr, attrListStrValid := strings.CutPrefix(tag, "EXTINF:")
	if !attrListStrValid {
		return fmt.Errorf("malformed EXTINF tag")
	}
	attrList := strings.Split(attrListStr, ",")

	if len(attrList) == 0 {
		return fmt.Errorf("EXTINF tag missing segment duration")
	}

	segmentDurationSec, err := strconv.ParseFloat(attrList[0], 64)
	if err != nil {
		return fmt.Errorf("unable to parse segment duration from EXTINF tag")
	}
	e.DurationSec = segmentDurationSec

	if len(attrList) > 1 {
		e.ExtraInfo = strings.Join(attrList[1:], ",")
	}

	return nil
}

// parseMediaSequenceTag will parse an EXT-X-MEDIA-SEQUENCE tag
func parseMediaSequenceTag(pl *Plist, tag string) error {
	tagValue, tagValid := strings.CutPrefix(tag, "EXT-X-MEDIA-SEQUENCE:")
	if !tagValid {
		return fmt.Errorf("malformed EXT-X-MEDIA-SEQUENCE tag")
	}

	mediaSequence, err := strconv.ParseUint(tagValue, 10, 64)
	if err != nil {
		return fmt.Errorf("unable to parse media sequence from EXT-X-MEDIA-SEQUENCE tag")
	}

	pl.CurrentMediaSequence = mediaSequence

	return nil
}

// parseTargetDurationTag will parse an EXT-X-TARGETDURATION tag
func parseTargetDurationTag(pl *Plist, tag string) error {
	tagValue, tagValid := strings.CutPrefix(tag, "EXT-X-TARGETDURATION:")
	if !tagValid {
		return fmt.Errorf("malformed EXT-X-TARGETDURATION tag")
	}

	targetDurationSec, err := strconv.ParseUint(tagValue, 10, 64)
	if err != nil {
		return fmt.Errorf("unable to parse target duration from EXT-X-TARGETDURATION tag")
	}

	pl.TargetDurationSec = targetDurationSec

	return nil
}

// Parse will parse a HLS M3U8 playlist from a string.
func Parse(pl *Plist, plUrlStr string, str string) error {
	plUrl, err := url.Parse(plUrlStr)
	if err != nil {
		return fmt.Errorf("failed to parse playlist url: %v", err)
	}
	// Remove the name of the playlist of the path to get the base URL.
	plUrl.Path = path.Dir(plUrl.Path)
	baseUrl := plUrl.String()

	isExtM3U := false
	currentEntry := Entry{}
	for lineIdx, line := range strings.Split(str, "\n") {
		if strings.HasPrefix(line, "#") {
			// Ignore all comment lines
			if !strings.HasPrefix(line, "#EXT") {
				continue
			}

			// We can ignore the presence flag as we have checked that before.
			extTag, _ := strings.CutPrefix(line, "#")

			// An extended M3U playlist must always have an #EXTM3U tag!
			if strings.HasPrefix(extTag, "EXTM3U") {
				isExtM3U = true
			} else if strings.HasPrefix(extTag, "EXT-X-STREAM-INF") {
				// A EXT-X-STREAM-INF tag indicates this is a master playlist and that a new variant entry will follow.
				pl.Type = MasterPlist
				currentEntry = Entry{}

				if err = parseStreamInfTag(&currentEntry, extTag); err != nil {
					return fmt.Errorf("line %d: %v", lineIdx, err)
				}
			} else if strings.HasPrefix(extTag, "EXTINF") {
				// A EXTINF tag indicates this is a variant playlist and that a segment entry will follow.
				pl.Type = VariantPlist
				currentEntry = Entry{}

				if err = parseInfTag(&currentEntry, extTag); err != nil {
					return fmt.Errorf("line %d: %v", lineIdx, err)
				}
			} else if strings.HasPrefix(extTag, "EXT-X-MEDIA-SEQUENCE") {
				// We store the media sequence indicated in the playlist to get the unique segment number for each
				// segment. This can then be used to ensure every segment is only fetched once.
				if err = parseMediaSequenceTag(pl, extTag); err != nil {
					return fmt.Errorf("line %d: %v", lineIdx, err)
				}
			} else if strings.HasPrefix(extTag, "EXT-X-TARGETDURATION") {
				if err = parseTargetDurationTag(pl, extTag); err != nil {
					return fmt.Errorf("line %d: %v", lineIdx, err)
				}
			}

			continue
		}

		// Skip empty lines
		if len(line) == 0 {
			continue
		}

		// Lines that do not start with http(s) are links relative to the playlist URL.
		if strings.HasPrefix(line, "http") {
			currentEntry.URL = line
		} else {
			currentEntry.URL, err = url.JoinPath(baseUrl, line)
			if err != nil {
				return fmt.Errorf("line %d: unable to join url: %v", lineIdx, err)
			}
		}

		// Store the media sequence of the segment.
		currentEntry.MediaSequence = pl.CurrentMediaSequence
		pl.CurrentMediaSequence += 1

		// Then, append it to the list of entries.
		pl.Entries = append(pl.Entries, currentEntry)
		currentEntry = Entry{}
	}

	if !isExtM3U {
		return fmt.Errorf("playlist is not in extended m3u format")
	}

	return nil
}

// FetchAndParse will fetch a playlist using HTTP and try to parse it.
func FetchAndParse(pl *Plist, url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("fetching playlist failed: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("fetching playlist failed: could not read response body: %v", err)
	}

	return Parse(pl, url, string(respBody))
}
