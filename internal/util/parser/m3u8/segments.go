package m3u8

import (
	"github.com/grafov/m3u8"
)

func (p *M3U8Parser) extractSegments(playlist *m3u8.MediaPlaylist) ([]string, string, float64) {
	segments := make([]string, 0, len(playlist.Segments))

	var totalDuration float64
	var initSegment string

	if playlist.Map != nil && playlist.Map.URI != "" {
		initSegment = p.resolveURL(playlist.Map.URI)
	}

	var byteRange bool
	for _, segment := range playlist.Segments {
		if segment == nil || segment.URI == "" {
			continue
		}

		totalDuration += segment.Duration
		if byteRange {
			continue
		}

		segmentURL := p.resolveURL(segment.URI)
		segments = append(segments, segmentURL)

		if segment.Limit > 0 {
			if initSegment == segmentURL {
				initSegment = ""
			}
			byteRange = true
		}
	}
	return segments, initSegment, totalDuration
}
