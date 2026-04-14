package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"

	"github.com/rs/zerolog/log"
)

var (
	// ErrNoStreamUUIDFound is returned when the stream UUID is not found in the body.
	ErrNoStreamUUIDFound = errors.New("no suuid found in body")
)

// Scraper is used to scrape the withny website.
type Scraper struct {
	*Client
}

var streamUUIDRegex = regexp.MustCompile(`(?m)\\"streamUUID\\":\\"([^\\]*)\\`)

// FetchStreamUUID finds the stream UUID.
//
// The Stream UUID is server-side rendered on the website.
func (s *Scraper) FetchStreamUUID(
	ctx context.Context,
	channelID string,
	passCode string,
) (suuid string, err error) {
	channelURL := fmt.Sprintf("https://www.withny.fun/channels/%s", channelID)
	if passCode != "" {
		// unsafe join, but it's small so that's fine
		channelURL = fmt.Sprintf("%s?passCode=%s", channelURL, passCode)
	}
	req, err := s.NewAuthRequestWithContext(ctx, "GET", channelURL, nil)
	if err != nil {
		panic(err)
	}

	resp, err := s.Do(req)
	if err != nil {
		log.Err(err).Msg("failed to fetch channel page")
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", HTTPError{
			Status: resp.StatusCode,
			Method: "GET",
			URL:    channelURL,
		}
	}

	suuid, err = findStreamUUID(resp.Body)
	if err != nil {
		log.Err(err).Msg("failed to find suuid endpoint")
		return "", err
	}
	return suuid, nil
}

// findStreamUUID finds the GraphQL endpoint and stream UUID.
func findStreamUUID(r io.Reader) (suuid string, err error) {
	buf, err := io.ReadAll(r)
	if err != nil {
		log.Err(err).Msg("failed to read body")
		return "", err
	}
	// Check if a stream uuid was found
	matches := streamUUIDRegex.FindStringSubmatch(string(buf))
	if len(matches) < 2 {
		return "", ErrNoStreamUUIDFound
	}

	return matches[1], nil
}
