package api

import (
	"context"
	"fmt"
	"io"
	"regexp"

	"github.com/rs/zerolog/log"
)

// Scraper is used to scrape the withny website.
type Scraper struct {
	*Client
}

// NewScraper creates a new Scraper.
//
// A scraper is needed since Withny is using SSR.
func NewScraper(client *Client) *Scraper {
	return &Scraper{client}
}

// FetchGraphQLAndStreamUUID finds the GraphQL endpoint.
//
// The GraphQL endpoint is hard-coded on the website and uses AWS AppSync.
// Technically, we could just hard-code it too, but to avoid any "unexpected" changes,
// we'll just scrape it.
func (s *Scraper) FetchGraphQLAndStreamUUID(
	ctx context.Context,
	channelID string,
) (endpoint, suuid string, err error) {
	req, err := s.NewAuthRequestWithContext(
		ctx,
		"GET",
		fmt.Sprintf("https://www.withny.fun/channels/%s", channelID),
		nil,
	)
	if err != nil {
		panic(err)
	}

	resp, err := s.Do(req)
	if err != nil {
		log.Err(err).Msg("failed to fetch channel page")
		return "", "", err
	}
	defer resp.Body.Close()

	mainAppEndpoint, suuid, err := findMainAppAndStreamUUID(resp.Body)
	if err != nil {
		log.Err(err).Msg("failed to find main app endpoint, or suuid")
		return "", "", err
	}

	endpoint, err = s.FetchGraphQLEndpoint(ctx, mainAppEndpoint)
	if err != nil {
		log.Err(err).Msg("failed to find graphql endpoint")
		return "", "", err
	}
	// Hack from the website itself.
	return endpoint, suuid, nil
}

var mainAppURLRegex = regexp.MustCompile(`(?m)"(\/[^"]*main-app[^"]*\.js)"`)
var graphqlURLRegex = regexp.MustCompile(`(?m)NEXT_PUBLIC_GRAPHQL_ENDPOINT: "([^"]*)"`)
var streamUUIDRegex = regexp.MustCompile(`(?m)ivsChannelUuid\\":\\"([^"]*)\\"`)

// findMainAppAndStreamUUID finds the GraphQL endpoint and stream UUID.
func findMainAppAndStreamUUID(r io.Reader) (path, suuid string, err error) {
	buf, err := io.ReadAll(r)
	if err != nil {
		log.Err(err).Msg("failed to read body")
		return "", "", err
	}
	mainAppMatches := mainAppURLRegex.FindStringSubmatch(string(buf))

	// Check if a path was found
	if len(mainAppMatches) < 2 {
		return "", "", fmt.Errorf("no match found")
	}

	// Check if a stream uuid was found
	suuidMatches := streamUUIDRegex.FindStringSubmatch(string(buf))
	if len(suuidMatches) < 2 {
		return "", "", fmt.Errorf("no match found")
	}

	return mainAppMatches[1], suuidMatches[1], nil
}

// FetchGraphQLEndpoint finds the GraphQL endpoint.
func (s *Scraper) FetchGraphQLEndpoint(
	ctx context.Context,
	mainAppPath string,
) (endpoint string, err error) {
	req, err := s.NewAuthRequestWithContext(
		ctx,
		"GET",
		fmt.Sprintf("https://www.withny.fun%s", mainAppPath),
		nil,
	)
	if err != nil {
		panic(err)
	}

	resp, err := s.Do(req)
	if err != nil {
		log.Err(err).Msg("failed to fetch main app page")
		return "", err
	}
	defer resp.Body.Close()

	return findGraphQLEndpoint(resp.Body)
}

// findGraphQLEndpoint finds the GraphQL endpoint.
func findGraphQLEndpoint(r io.Reader) (endpoint string, err error) {
	buf, err := io.ReadAll(r)
	if err != nil {
		log.Err(err).Msg("failed to read body")
		return "", err
	}
	matches := graphqlURLRegex.FindStringSubmatch(string(buf))
	if len(matches) < 2 {
		return "", fmt.Errorf("no match found")
	}
	return matches[1], nil
}
