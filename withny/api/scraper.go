package api

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"strconv"
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

// FindGraphQLAndStreamUUID finds the GraphQL endpoint.
//
// The GraphQL endpoint is hard-coded on the website and uses AWS AppSync.
// Technically, we could just hard-code it too, but to avoid any "unexpected" changes,
// we'll just scrape it.
func (s *Scraper) FindGraphQLAndStreamUUID(
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
		return "", "", err
	}
	defer resp.Body.Close()

	endpoint, suuid, err = FindGraphQLEndpointAndStreamUUID(resp.Body)
	if err != nil {
		return "", "", err
	}
	endpoint, err = strconv.Unquote(endpoint)
	if err != nil {
		return "", "", err
	}
	// Hack from the website itself.
	return endpoint, suuid, nil
}

var graphqlURLRegex = regexp.MustCompile(`(?m)"https:\\u002F\\u002F[^"]*\\u002Fgraphql"`)
var streamUUIDRegex = regexp.MustCompile(`(?m)uuid="([^"]*)"`)

// FindGraphQLEndpointAndStreamUUID finds the GraphQL endpoint and stream UUID.
func FindGraphQLEndpointAndStreamUUID(r io.Reader) (endpoint, suuid string, err error) {
	buf, err := io.ReadAll(r)
	if err != nil {
		return "", "", err
	}
	gql := graphqlURLRegex.FindString(string(buf))

	// Check if a gql was found
	if gql == "" {
		return "", "", fmt.Errorf("no match found")
	}

	// Check if a stream uuid was found
	matches := streamUUIDRegex.FindStringSubmatch(string(buf))
	if len(matches) < 2 {
		return "", "", fmt.Errorf("no match found")
	}

	return gql, matches[1], nil
}
