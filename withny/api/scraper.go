package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"

	"github.com/rs/zerolog/log"
)

var (
	ErrNoGQLFound        = errors.New("no gql url found in body")
	ErrNoStreamUUIDFound = errors.New("no suuid found in body")
)

// Scraper is used to scrape the withny website.
type Scraper struct {
	*Client
}

// FetchCommentsGraphQLAndStreamUUID finds the GraphQL endpoint.
//
// The GraphQL endpoint is server-side rendered on the website and uses AWS AppSync.
// Technically, we could just hard-code it too, but to avoid any "unexpected" changes,
// we'll just scrape it.
func (s *Scraper) FetchCommentsGraphQLAndStreamUUID(
	ctx context.Context,
	channelID string,
	passCode string,
) (endpoint, suuid string, err error) {
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
		return "", "", err
	}
	defer resp.Body.Close()

	endpoint, suuid, err = findGraphQLEndpointAndStreamUUID(resp.Body)
	if err != nil {
		log.Err(err).Msg("failed to find graphql endpoint")
		return "", "", err
	}
	return endpoint, suuid, nil
}

var graphqlURLRegex = regexp.MustCompile(`(?m)"https:\\u002F\\u002F[^"]*\\u002Fgraphql"`)
var streamUUIDRegex = regexp.MustCompile(`(?m)uuid="([^"]*)"`)

// findGraphQLEndpointAndStreamUUID finds the GraphQL endpoint and stream UUID.
func findGraphQLEndpointAndStreamUUID(r io.Reader) (endpoint, suuid string, err error) {
	buf, err := io.ReadAll(r)
	if err != nil {
		log.Err(err).Msg("failed to read body")
		return "", "", err
	}
	gql := graphqlURLRegex.FindString(string(buf))

	// Check if a gql was found
	if gql == "" {
		return "", "", ErrNoGQLFound
	}
	decoded, err := strconv.Unquote(gql)
	if err != nil {
		log.Err(err).Str("endpoint", gql).Msg("failed to unquote graphql endpoint")
	} else {
		gql = decoded
	}

	// Check if a stream uuid was found
	matches := streamUUIDRegex.FindStringSubmatch(string(buf))
	if len(matches) < 2 {
		return "", "", ErrNoStreamUUIDFound
	}

	return gql, matches[1], nil
}

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

	suuid, err = fetchStreamUUID(resp.Body)
	if err != nil {
		log.Err(err).Msg("failed to find suuid endpoint")
		return "", err
	}
	return suuid, nil
}

// fetchStreamUUID finds the GraphQL endpoint and stream UUID.
func fetchStreamUUID(r io.Reader) (suuid string, err error) {
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
