// Package api provide a client for the withny API.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/Darkness4/withny-dl/notify/notifier"
	"github.com/Darkness4/withny-dl/utils"
	"github.com/golang-jwt/jwt/v5"
	"github.com/rs/zerolog/log"
)

const (
	apiURL              = "https://www.withny.fun/api"
	loginURL            = apiURL + "/auth/login"
	refreshURL          = apiURL + "/auth/token"
	userURL             = apiURL + "/user"
	streamsURL          = apiURL + "/streams"
	streamsWithRoomsURL = apiURL + "/streams/with-rooms"
	streamPlaybackURL   = streamsURL + "/%s/playback-url"
)

// HTTPError represents an HTTP error.
type HTTPError struct {
	Status int
	Body   string
	Method string
	URL    string
}

// Error returns the error message.
func (e HTTPError) Error() string {
	return fmt.Sprintf("HTTP error %s %s, code=%d, body=%s", e.Method, e.URL, e.Status, e.Body)
}

// GetPlaybackURLError is an error given by the GetStreamPlaybackURL API.
type GetPlaybackURLError struct {
	Err      error
	StreamID string
}

// Error returns the error message.
func (e GetPlaybackURLError) Error() string {
	return e.Err.Error()
}

// ErrStreamNotFound is when no stream is found when looking for the playback URL.
var ErrStreamNotFound = errors.New("stream not found")

// UnauthorizedError is when the request is unauthorized.
type UnauthorizedError struct {
	Body string
}

// Error returns the error message.
func (e UnauthorizedError) Error() string {
	return fmt.Sprintf("unauthorized: %s", e.Body)
}

// Claims is the JWT claims for the withny API.
type Claims struct {
	jwt.RegisteredClaims
	UserUUID  string `json:"userUuid"`
	TokenUUID string `json:"tokenUuid"`
	Scope     string `json:"scope"`
}

// Credentials is the credentials for the withny API.
type Credentials struct {
	Claims
	LoginResponse
}

// SavedCredentials is the saved credentials given by the user for the withny API.
type SavedCredentials struct {
	Username     string `yaml:"username"     json:"username"`
	Password     string `yaml:"password"     json:"password"`
	Token        string `yaml:"token"        json:"token"`
	RefreshToken string `yaml:"refreshToken" json:"refreshToken"`
}

// Hash returns the hash of the credentials.
func (r SavedCredentials) Hash() string {
	return utils.Hash(r)
}

// CredentialsReader is an interface for reading saved credentials.
type CredentialsReader interface {
	Read() (SavedCredentials, error)
}

// CachedCredentials is the information stored in the cached file.
type CachedCredentials struct {
	Credentials

	// Hash is used to invalidate when the original secret changes.
	Hash string
}

// CredentialsCache is an interface for caching credentials.
type CredentialsCache interface {
	// Set writes the credentials to a file.
	Set(creds Credentials) error
	// Set writes the credentials to a file, but store the hash of the credentials.
	Init(creds Credentials, hash string) error
	// Get reads the credentials from a file.
	Get() (CachedCredentials, error)
	// Invalidate removes the credentials file.
	Invalidate() error
}

// ClientOptions is the options for the withny API client.
type ClientOptions struct {
	clearCredentialCacheOnFailureAfter int
}

// ClientOption is a function that configures the withny API client.
type ClientOption func(*ClientOptions)

// Client is a withny API client.
type Client struct {
	*http.Client
	credentialsReader                  CredentialsReader
	credentialsCache                   CredentialsCache
	clearCredentialCacheOnFailureAfter int
}

// WithClearCredentialCacheOnFailureAfter sets the number of times to retry
// before clearing the credentials cache.
func WithClearCredentialCacheOnFailureAfter(i int) ClientOption {
	return func(opts *ClientOptions) {
		opts.clearCredentialCacheOnFailureAfter = i
	}
}

// NewClient creates a new withny API client.
func NewClient(
	client *http.Client,
	reader CredentialsReader,
	cache CredentialsCache,
	opt ...ClientOption,
) *Client {
	if reader == nil {
		log.Warn().Msg("no user and password provided")
	}
	if cache == nil {
		log.Panic().Msg("no credentials cache provided")
	}
	if reader == nil {
		log.Panic().Msg("no credentials reader provided")
	}
	opts := &ClientOptions{}
	for _, o := range opt {
		o(opts)
	}
	if opts.clearCredentialCacheOnFailureAfter == 0 {
		opts.clearCredentialCacheOnFailureAfter = 300
	}
	return &Client{
		Client:                             client,
		credentialsReader:                  reader,
		credentialsCache:                   cache,
		clearCredentialCacheOnFailureAfter: opts.clearCredentialCacheOnFailureAfter,
	}
}

// NewAuthRequestWithContext creates a new authenticated request with the given context.
func (c *Client) NewAuthRequestWithContext(
	ctx context.Context,
	method, url string,
	body io.Reader,
) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		log.Err(err).Msg("failed to create request")
		return nil, err
	}
	creds, err := c.credentialsCache.Get()
	if err != nil {
		log.Err(err).Msg("failed to get cached credentials")
	}
	req.Header.Set("Authorization", "Bearer "+creds.Token)
	req.Header.Set(
		"User-Agent",
		"Mozilla/5.0 (X11; Linux x86_64; rv:144.0) Gecko/20100101 Firefox/144.0",
	)
	return req, nil
}

// Login will login to withny and store the credentials in the client.
func (c *Client) Login(ctx context.Context) (err error) {
	var creds Credentials

	// Check if cache is valid
	cachedCreds, err := c.credentialsCache.Get()
	if err != nil {
		log.Err(err).Msg("failed to get cached credentials")
	}
	original, err := c.credentialsReader.Read()
	if err != nil {
		log.Err(err).Msg("failed to read credentials")
		return err
	}
	if cachedCreds.Hash != original.Hash() {
		log.Info().Msg("credentials changed, clearing cache")
		err := c.credentialsCache.Invalidate()
		if err != nil {
			log.Err(err).Msg("failed to invalidate cache")
		}
		cachedCreds = CachedCredentials{}
	}

	switch {
	case cachedCreds.Token != "":
		tries := 0
		for {
			creds, err = c.LoginWithRefreshToken(ctx, cachedCreds.RefreshToken)
			if err != nil {
				if tries < c.clearCredentialCacheOnFailureAfter {
					log.Err(err).
						Int("tries", tries).
						Msg("failed to refresh token from cache, retrying in 60 seconds")
					tries++
					time.Sleep(60 * time.Second)
					continue
				}
				log.Err(err).
					Msg("failed to refresh token from cache, will use provided credentials")
				if err := notifier.NotifyLoginFailed(ctx, err); err != nil {
					log.Err(err).Msg("notify failed")
				}
				if err := c.credentialsCache.Invalidate(); err != nil {
					log.Err(err).Msg("failed to invalidate cache")
				}
				creds, err = c.loginWithReader(ctx)
			}
			break
		}
	default:
		creds, err = c.loginWithReader(ctx)
	}
	if err != nil {
		log.Err(err).Msg("failed to login")
		return err
	}
	log.Info().Msg("login successful and token refreshed")

	if err := c.credentialsCache.Set(creds); err != nil {
		log.Err(err).Msg("failed to cache credentials")
	}
	return nil
}

func (c *Client) loginWithReader(ctx context.Context) (Credentials, error) {
	creds, err := c.credentialsReader.Read()
	if err != nil {
		log.Err(err).Msg("failed to read credentials")
		return Credentials{}, err
	}
	if creds.Username != "" {
		resp, err := c.LoginWithUserPassword(ctx, creds.Username, creds.Password)
		if err != nil {
			return Credentials{}, err
		}
		if err := c.credentialsCache.Init(resp, creds.Hash()); err != nil {
			log.Err(err).Msg("failed to cache credentials")
		}
		return resp, err
	} else if creds.Token != "" {
		// Hijack the client token to override authorization header
		newCredentials := Credentials{
			LoginResponse: LoginResponse{
				Token:        creds.Token,
				RefreshToken: creds.RefreshToken,
				TokenType:    "Bearer",
			},
		}
		if err := c.credentialsCache.Init(newCredentials, creds.Hash()); err != nil {
			log.Err(err).Msg("failed to cache credentials")
		}
		return c.LoginWithRefreshToken(ctx, creds.RefreshToken)
	}
	return Credentials{}, fmt.Errorf("no credentials provided")
}

// GetUser will fetch the user for the given channelID.
func (c *Client) GetUser(ctx context.Context, channelID string) (GetUserResponse, error) {
	u, err := url.Parse(userURL)
	if err != nil {
		panic(err)
	}
	q := u.Query()
	q.Set("username", channelID)
	u.RawQuery = q.Encode()

	req, err := c.NewAuthRequestWithContext(
		ctx,
		http.MethodGet,
		u.String(),
		nil,
	)
	if err != nil {
		log.Err(err).Msg("failed to create request")
		return GetUserResponse{}, err
	}

	log := log.With().
		Str("method", "GET").
		Stringer("url", u).
		Str("channelID", channelID).
		Logger()

	res, err := c.Do(req)
	if err != nil {
		log.Err(err).Msg("failed to get user")
		return GetUserResponse{}, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		err := fmt.Errorf("unexpected status code: %d", res.StatusCode)
		body, _ := io.ReadAll(res.Body)
		log.Err(err).
			Str("response", string(body)).
			Int("status", res.StatusCode).
			Msg("unexpected status code")
		if res.StatusCode >= http.StatusInternalServerError {
			return GetUserResponse{}, HTTPError{
				Status: res.StatusCode,
				Body:   string(body),
				Method: req.Method,
				URL:    req.URL.String(),
			}
		}
		return GetUserResponse{}, err
	}

	var parsed GetUserResponse
	err = utils.JSONDecodeAndPrintOnError(res.Body, &parsed)
	return parsed, err
}

// GetStreams will fetch the streams for the given channelID.
func (c *Client) GetStreams(
	ctx context.Context,
	channelID string,
	passCode string,
) (GetStreamsResponse, error) {
	u, err := url.Parse(streamsWithRoomsURL)
	if err != nil {
		panic(err)
	}
	q := u.Query()
	if channelID != "" {
		q.Set("username", channelID)
	}
	if passCode != "" {
		q.Set("passCode", passCode)
	}
	u.RawQuery = q.Encode()

	req, err := c.NewAuthRequestWithContext(
		ctx,
		http.MethodGet,
		u.String(),
		nil,
	)
	if err != nil {
		log.Err(err).Msg("failed to create request")
		return GetStreamsResponse{}, err
	}

	log := log.With().
		Str("method", "GET").
		Stringer("url", u).
		Str("channelID", channelID).
		Logger()

	res, err := c.Do(req)
	if err != nil {
		log.Err(err).Msg("failed to get streams")
		return GetStreamsResponse{}, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		err := fmt.Errorf("unexpected status code: %d", res.StatusCode)
		log.Err(err).
			Str("response", string(body)).
			Int("status", res.StatusCode).
			Msg("unexpected status code")
		if res.StatusCode >= http.StatusInternalServerError {
			return GetStreamsResponse{}, HTTPError{
				Status: res.StatusCode,
				Body:   string(body),
				Method: req.Method,
				URL:    req.URL.String(),
			}
		}
		return GetStreamsResponse{}, err
	}

	var parsed GetStreamsResponse
	err = utils.JSONDecodeAndPrintOnError(res.Body, &parsed)
	return parsed, err
}

// LoginWithRefreshToken will login with the given refreshToken.
func (c *Client) LoginWithRefreshToken(
	ctx context.Context,
	refreshToken string,
) (Credentials, error) {
	log.Info().Msg("refreshing token")
	buf := new(bytes.Buffer)
	if err := json.NewEncoder(buf).Encode(map[string]string{
		"refreshToken": refreshToken,
	}); err != nil {
		panic(err)
	}

	req, err := c.NewAuthRequestWithContext(
		ctx,
		http.MethodPost,
		refreshURL,
		buf,
	)
	if err != nil {
		panic(err)
	}
	req.Header.Set("Content-Type", "application/json")

	log := log.With().
		Str("method", "POST").
		Str("url", refreshURL).
		Logger()

	res, err := c.Do(req)
	if err != nil {
		log.Err(err).Msg("failed to refresh token")
		return Credentials{}, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		if res.StatusCode == http.StatusUnauthorized {
			log.Err(err).
				Str("response", string(body)).
				Int("status", res.StatusCode).
				Str("refreshToken", refreshToken).
				Msg("unexpected status code (refresh token is already used?)")
		}
		err := fmt.Errorf("unexpected status code: %d", res.StatusCode)
		log.Err(err).
			Str("response", string(body)).
			Int("status", res.StatusCode).
			Str("refreshToken", refreshToken).
			Msg("unexpected status code")
		if res.StatusCode >= http.StatusInternalServerError {
			return Credentials{}, HTTPError{
				Status: res.StatusCode,
				Body:   string(body),
				Method: req.Method,
				URL:    req.URL.String(),
			}
		}
		return Credentials{}, err
	}

	var lr Credentials
	if err := utils.JSONDecodeAndPrintOnError(res.Body, &lr.LoginResponse); err != nil {
		return lr, err
	}
	_, _, err = jwt.NewParser().ParseUnverified(lr.Token, &lr.Claims)
	return lr, err
}

// LoginWithUserPassword will login with the given email and password.
func (c *Client) LoginWithUserPassword(
	ctx context.Context,
	username, password string,
) (Credentials, error) {
	log.Warn().
		Msg("login with user password is deprecated, and will not work since withny has a captcha, please login with refresh token instead")

	log.Info().Str("username", username).Msg("logging in")
	buf := new(bytes.Buffer)
	if err := json.NewEncoder(buf).Encode(map[string]string{
		"email":    username, // email can also be the username
		"password": password,
	}); err != nil {
		panic(err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		loginURL,
		buf,
	)
	if err != nil {
		panic(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(
		"User-Agent",
		"Mozilla/5.0 (X11; Linux x86_64; rv:144.0) Gecko/20100101 Firefox/144.0",
	)

	log := log.With().
		Str("method", "POST").
		Str("url", loginURL).
		Logger()

	res, err := c.Do(req)
	if err != nil {
		log.Err(err).Msg("failed to login")
		return Credentials{}, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		err := fmt.Errorf("unexpected status code: %d", res.StatusCode)
		log.Err(err).
			Str("response", string(body)).
			Int("status", res.StatusCode).
			Msg("unexpected status code")
		if res.StatusCode >= http.StatusInternalServerError {
			return Credentials{}, HTTPError{
				Status: res.StatusCode,
				Body:   string(body),
				Method: req.Method,
				URL:    req.URL.String(),
			}
		}
		return Credentials{}, err
	}

	var lr Credentials
	if err := utils.JSONDecodeAndPrintOnError(res.Body, &lr.LoginResponse); err != nil {
		return lr, err
	}
	_, _, err = jwt.NewParser().ParseUnverified(lr.Token, &lr.Claims)
	return lr, err
}

// GetStreamPlaybackURL will fetch the playback URL for the given streamID.
func (c *Client) GetStreamPlaybackURL(ctx context.Context, streamID string) (string, error) {
	u, err := url.Parse(fmt.Sprintf(streamPlaybackURL, streamID))
	if err != nil {
		panic(err)
	}
	req, err := c.NewAuthRequestWithContext(
		ctx,
		http.MethodGet,
		u.String(),
		nil,
	)
	if err != nil {
		log.Err(err).Msg("failed to create request")
		return "", err
	}
	req.Header.Set("Accept", "application/json")

	log := log.With().
		Str("method", "GET").
		Stringer("url", u).
		Str("streamID", streamID).
		Logger()

	res, err := c.Do(req)
	if err != nil {
		log.Err(err).Msg("failed to get playback URL")
		return "", err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		switch res.StatusCode {
		case http.StatusUnauthorized:
			return "", GetPlaybackURLError{
				Err:      UnauthorizedError{Body: string(body)},
				StreamID: streamID,
			}
		case http.StatusInternalServerError:
			var errMsg ErrorResponse
			_ = json.Unmarshal(body, &errMsg)
			if errMsg.Message == "Stream not found" {
				// Is a json message.
				return "", GetPlaybackURLError{
					Err:      ErrStreamNotFound,
					StreamID: streamID,
				}
			}
		default:
			err := fmt.Errorf("unexpected status code: %d", res.StatusCode)
			log.Err(err).
				Str("response", string(body)).
				Int("status", res.StatusCode).
				Msg("unexpected status code")
			if res.StatusCode >= http.StatusInternalServerError {
				return "", HTTPError{
					Status: res.StatusCode,
					Body:   string(body),
					Method: req.Method,
					URL:    req.URL.String(),
				}
			}
			return "", err
		}
	}

	var parsed string
	if err = utils.JSONDecodeAndPrintOnError(res.Body, &parsed); err != nil {
		return "", GetPlaybackURLError{
			Err:      err,
			StreamID: streamID,
		}
	}
	return parsed, nil
}

// GetPlaylists will fetch the playlists from the given playbackURL.
func (c *Client) GetPlaylists(
	ctx context.Context,
	playbackURL string,
	playlistRetries int,
) ([]Playlist, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		playbackURL,
		nil,
	)
	if err != nil {
		log.Err(err).Msg("failed to create request")
		return nil, err
	}
	req.Header.Set(
		"Accept",
		"application/x-mpegURL, application/vnd.apple.mpegurl, application/json, text/plain",
	)
	req.Header.Set("Referer", "https://www.withny.fun/")
	req.Header.Set("Origin", "https://www.withny.fun")
	req.Header.Set(
		"User-Agent",
		"Mozilla/5.0 (X11; Linux x86_64; rv:144.0) Gecko/20100101 Firefox/144.0",
	)

	log := log.With().
		Str("method", "GET").
		Str("url", playbackURL).
		Logger()

	var respBody io.ReadCloser
	var lastHTTPError HTTPError
	var count int
	for count = 0; count <= playlistRetries; count++ {
		res, err := c.Do(req)
		if err != nil {
			log.Err(err).Msg("failed to get playlists")
			return nil, err
		}

		if res.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(res.Body)
			res.Body.Close()

			if res.StatusCode >= 500 && res.StatusCode < 600 {
				lastHTTPError = HTTPError{
					Status: res.StatusCode,
					Body:   string(body),
					Method: req.Method,
					URL:    req.URL.String(),
				}
				log.Error().
					Str("url", lastHTTPError.URL).
					Int("response.status", lastHTTPError.Status).
					Str("response.body", lastHTTPError.Body).
					Str("method", lastHTTPError.Method).
					Int("count", count).
					Int("playlistRetries", playlistRetries).
					Msg("http error, retrying")
				continue
			}

			log.Error().
				Str("url", req.URL.String()).
				Int("response.status", res.StatusCode).
				Str("response.body", string(body)).
				Str("method", req.Method).
				Msg("http error")
			return nil, HTTPError{
				Status: res.StatusCode,
				Body:   string(body),
				Method: req.Method,
				URL:    req.URL.String(),
			}
		}

		respBody = res.Body
		break
	}
	if count > playlistRetries {
		log.Error().
			Str("url", lastHTTPError.URL).
			Int("response.status", lastHTTPError.Status).
			Str("response.body", lastHTTPError.Body).
			Str("method", req.Method).
			Int("playlistRetries", playlistRetries).
			Msg("giving up after too many http error")
		return nil, lastHTTPError
	}
	defer respBody.Close()

	return ParseM3U8(respBody), nil
}

// LoginLoop will login to withny and refresh the token when needed.
func (c *Client) LoginLoop(ctx context.Context) error {
	creds, err := c.credentialsCache.Get()
	if err != nil {
		log.Err(err).Msg("failed to get cached credentials")
	}
	date, err := creds.GetExpirationTime()
	if err != nil {
		panic(err)
	}
	var refreshTime time.Time
	if date == nil {
		// Refresh in 5 minutes
		refreshTime = time.Now().Add(5 * time.Minute)
	} else {
		// Refresh token 5 minutes before it expires
		refreshTime = date.Add(-5 * time.Minute)
	}

	ticker := time.NewTicker(time.Until(refreshTime))
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Err(ctx.Err()).Msg("context canceled, stopping login loop")
			return ctx.Err()
		case <-ticker.C:
			if err := c.Login(ctx); err != nil {
				log.Err(err).Msg("failed to login to withny, stopping login loop")
				return err
			}
			creds, err := c.credentialsCache.Get()
			if err != nil {
				log.Err(err).Msg("failed to get cached credentials")
			}
			date, err := creds.GetExpirationTime()
			if err != nil {
				panic(err)
			}
			if date == nil {
				refreshTime = time.Now().Add(5 * time.Minute)
			} else {
				refreshTime = date.Add(-5 * time.Minute)
			}
			ticker.Reset(time.Until(refreshTime))
		}
	}
}
