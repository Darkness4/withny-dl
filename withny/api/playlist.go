package api

import (
	"bufio"
	"io"
	"strconv"
	"strings"
)

// Playlist represents a stream in an M3U8 playlist.
type Playlist struct {
	Bandwidth  int64
	Resolution string
	Codecs     string
	Video      string
	FrameRate  float64
	URL        string
}

// ParseM3U8 parses an M3U8 playlist and returns a list of streams.
func ParseM3U8(r io.Reader) (streams []Playlist) {
	scanner := bufio.NewScanner(r)
	var currentStream Playlist

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "#EXT-X-STREAM-INF") {
			currentStream = Playlist{}

			// Parse stream attributes
			attributes := splitByCommaAvoidQuote(line[18:])
			for _, attribute := range attributes {
				keyValue := strings.SplitN(attribute, "=", 2)
				key := keyValue[0]
				value := strings.Trim(keyValue[1], "\"")

				switch key {
				case "BANDWIDTH":
					v, _ := strconv.ParseInt(value, 10, 64)
					currentStream.Bandwidth = v
				case "RESOLUTION":
					currentStream.Resolution = value
				case "CODECS":
					currentStream.Codecs = value
				case "FRAME-RATE":
					v, _ := strconv.ParseFloat(value, 64)
					currentStream.FrameRate = v
				case "VIDEO":
					currentStream.Video = value
				}
			}
		} else if strings.HasPrefix(line, "https://") {
			currentStream.URL = line
			streams = append(streams, currentStream)
		}
	}
	return streams
}

func splitByCommaAvoidQuote(s string) []string {
	commasCount := strings.Count(s, ",")
	result := make([]string, 0, commasCount+1)
	var current strings.Builder
	inQuotes := false
	escapeNext := false

	for _, r := range s {
		switch r {
		case ',':
			if inQuotes {
				// Inside quotes, so we just add the comma to the current field
				current.WriteRune(r)
			} else {
				// Outside quotes, we have a complete field
				result = append(result, strings.TrimSpace(current.String()))
				current.Reset()
			}
		case '"':
			if escapeNext {
				current.WriteRune(r)
				escapeNext = false
			} else {
				inQuotes = !inQuotes
			}
		case '\\':
			// Handle escape character
			if inQuotes {
				escapeNext = true
			} else {
				current.WriteRune(r)
			}
		default:
			current.WriteRune(r)
		}
	}

	// Add the last field to the result
	if current.Len() > 0 {
		result = append(result, strings.TrimSpace(current.String()))
	}

	return result
}

// PlaylistConstraint is used to filter playlists based on their attributes.
type PlaylistConstraint struct {
	MinBandwidth int64   `yaml:"minBandwidth"`
	MaxBandwidth int64   `yaml:"maxBandwidth"`
	MinHeight    int64   `yaml:"minHeight"`
	MaxHeight    int64   `yaml:"maxHeight"`
	MinWidth     int64   `yaml:"minWidth"`
	MaxWidth     int64   `yaml:"maxWidth"`
	MinFrameRate float64 `yaml:"minFrameRate"`
	MaxFrameRate float64 `yaml:"maxFrameRate"`
	AudioOnly    bool    `yaml:"audioOnly"`
}

// GetBestPlaylist returns the best playlist based on the constraints.
func GetBestPlaylist(
	streams []Playlist,
	constraints ...PlaylistConstraint,
) (best Playlist, found bool) {
streamLoop:
	for _, stream := range streams {
		for _, constraint := range constraints {
			width, height := parseResolution(stream.Resolution)
			switch {
			case constraint.MinBandwidth > 0 && stream.Bandwidth < constraint.MinBandwidth,
				constraint.MaxBandwidth > 0 && stream.Bandwidth > constraint.MaxBandwidth,
				constraint.MinHeight > 0 && int64(height) < constraint.MinHeight,
				constraint.MaxHeight > 0 && int64(height) > constraint.MaxHeight,
				constraint.MinWidth > 0 && int64(width) < constraint.MinWidth,
				constraint.MaxWidth > 0 && int64(width) > constraint.MaxWidth,
				constraint.MinFrameRate > 0 && stream.FrameRate < constraint.MinFrameRate,
				constraint.MaxFrameRate > 0 && stream.FrameRate > constraint.MaxFrameRate,
				constraint.AudioOnly && stream.Video != "audio_only":
				continue streamLoop
			}
		}

		if !found || compareStreams(stream, best) > 0 {
			best = stream
			found = true
		}
	}
	return best, found
}

func parseResolution(resolution string) (width, height int) {
	w, h, _ := strings.Cut(resolution, "x")
	width, _ = strconv.Atoi(w)
	height, _ = strconv.Atoi(h)
	return width, height
}

func compareStreams(s1, s2 Playlist) int64 {
	// Compare Resolution
	_, h1 := parseResolution(s1.Resolution)
	_, h2 := parseResolution(s2.Resolution)

	if h1 != h2 {
		return int64(h1 - h2) // Higher resolution has priority
	}

	// Compare FrameRate
	if s1.FrameRate != s2.FrameRate {
		if s1.FrameRate > s2.FrameRate {
			return 1
		}
		return -1
	}

	// Compare Bandwidth
	return s1.Bandwidth - s2.Bandwidth
}
