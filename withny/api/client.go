// Package api provide a client for the withny API.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/Darkness4/withny-dl/notify/notifier"
	"github.com/golang-jwt/jwt/v5"
	"github.com/rs/zerolog/log"
)

const (
	apiURL            = "https://www.withny.fun/api"
	loginURL          = apiURL + "/auth/login"
	refreshURL        = apiURL + "/auth/token"
	userURL           = apiURL + "/user"
	streamsURL        = apiURL + "/streams"
	streamPlaybackURL = streamsURL + "/%s/playback-url"
)

type UnauthorizedError struct {
	Err      error
	StreamID string
}

func (e UnauthorizedError) Error() string {
	return fmt.Sprintf("unauthorized: %s", e.Err)
}

// Claims is the JWT claims for the withny API.
type Claims struct {
	jwt.RegisteredClaims
	UUID  string `json:"uuid"`
	Scope string `json:"scope"`
}

// Credentials is the credentials for the withny API.
type Credentials struct {
	Claims
	LoginResponse
}

// SavedCredentials is the saved credentials given by the user for the withny API.
type SavedCredentials struct {
	Username     string `yaml:"username"`
	Password     string `yaml:"password"`
	Token        string `json:"token"`
	RefreshToken string `json:"refreshToken"`
}

// CredentialsReader is an interface for reading saved credentials.
type CredentialsReader interface {
	Read() (SavedCredentials, error)
}

// Client is a withny API client.
type Client struct {
	*http.Client
	credentialsReader CredentialsReader
	credentials       Credentials
}

// SetCredentials sets the credentials for the client.
func (c *Client) SetCredentials(creds Credentials) {
	c.credentials = creds
}

// NewClient creates a new withny API client.
func NewClient(client *http.Client, reader CredentialsReader) *Client {
	if reader == nil {
		log.Warn().Msg("no user and password provided")
	}
	return &Client{
		Client:            client,
		credentialsReader: reader,
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
		return nil, err
	}
	if c.credentials.TokenType != "" {
		req.Header.Set("Authorization", c.credentials.TokenType+" "+c.credentials.Token)
	}
	return req, nil
}

// Login will login to withny and store the credentials in the client.
func (c *Client) Login(ctx context.Context) (err error) {
	var creds Credentials

	switch {
	case c.credentials.Token != "":
		creds, err = c.LoginWithRefreshToken(ctx, c.credentials.RefreshToken)
		if err != nil {
			log.Err(err).Msg("failed to refresh token, will use saved credentials")
			creds, err = c.loginWithSaved(ctx)
		}
	default:
		creds, err = c.loginWithSaved(ctx)
	}
	if err != nil {
		return err
	}

	c.credentials = creds
	return nil
}

func (c *Client) loginWithSaved(ctx context.Context) (Credentials, error) {
	if c.credentialsReader == nil {
		return Credentials{}, fmt.Errorf("no credentials provided")
	}
	saved, err := c.credentialsReader.Read()
	if err != nil {
		return Credentials{}, err
	}
	if saved.Username != "" {
		return c.LoginWithUserPassword(ctx, saved.Username, saved.Password)
	} else if saved.Token != "" {
		// Hijack the client token to override authorization header
		c.credentials.Token = saved.Token
		c.credentials.TokenType = "Bearer"
		c.credentials.RefreshToken = saved.RefreshToken
		return c.LoginWithRefreshToken(ctx, saved.RefreshToken)
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
		return GetUserResponse{}, err
	}

	res, err := c.Do(req)
	if err != nil {
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
		return GetUserResponse{}, err
	}

	var parsed GetUserResponse
	err = json.NewDecoder(res.Body).Decode(&parsed)
	return parsed, err
}

// GetStreams will fetch the streams for the given channelID.
func (c *Client) GetStreams(ctx context.Context, channelID string) (GetStreamsResponse, error) {
	u, err := url.Parse(streamsURL)
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
		return GetStreamsResponse{}, err
	}

	res, err := c.Do(req)
	if err != nil {
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
		return GetStreamsResponse{}, err
	}

	var parsed GetStreamsResponse
	err = json.NewDecoder(res.Body).Decode(&parsed)
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

	res, err := c.Do(req)
	if err != nil {
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
		return Credentials{}, err
	}

	var lr Credentials
	if err := json.NewDecoder(res.Body).Decode(&lr.LoginResponse); err != nil {
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

	res, err := c.Do(req)
	if err != nil {
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
		return Credentials{}, err
	}

	var lr Credentials
	if err := json.NewDecoder(res.Body).Decode(&lr.LoginResponse); err != nil {
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
		return "", err
	}

	res, err := c.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		if res.StatusCode == http.StatusUnauthorized {
			return "", UnauthorizedError{
				Err:      fmt.Errorf("unauthorized"),
				StreamID: streamID,
			}
		}
		body, _ := io.ReadAll(res.Body)
		err := fmt.Errorf("unexpected status code: %d", res.StatusCode)
		log.Err(err).
			Str("response", string(body)).
			Int("status", res.StatusCode).
			Msg("unexpected status code")
		return "", err
	}

	var parsed string
	err = json.NewDecoder(res.Body).Decode(&parsed)
	return parsed, err
}

// GetPlaylists will fetch the playlists from the given playbackURL.
func (c *Client) GetPlaylists(ctx context.Context, playbackURL string) ([]Playlist, error) {
	// No need for auth request. Token is included in the playback URL.
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		playbackURL,
		nil,
	)
	if err != nil {
		return nil, err
	}
	req.Header.Set(
		"Accept",
		"application/x-mpegURL, application/vnd.apple.mpegurl, application/json, text/plain",
	)
	req.Header.Set("Referer", "https://www.withny.fun/")
	req.Header.Set("Origin", "https://www.withny.fun")

	res, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		err := fmt.Errorf("unexpected status code: %d", res.StatusCode)
		log.Err(err).
			Str("response", string(body)).
			Int("status", res.StatusCode).
			Msg("unexpected status code")
		return nil, err
	}

	return ParseM3U8(res.Body), nil
}

// LoginLoop will login to withny and refresh the token when needed.
func (c *Client) LoginLoop(ctx context.Context) error {
	if err := c.Login(ctx); err != nil {
		return err
	}

	date, err := c.credentials.GetExpirationTime()
	if err != nil {
		panic(err)
	}

	// Refresh token 5 minutes before it expires
	refreshTime := date.Add(-5 * time.Minute)

	ticker := time.NewTicker(time.Until(refreshTime))
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := c.Login(ctx); err != nil {
				if err := notifier.NotifyLoginFailed(ctx, err); err != nil {
					log.Err(err).Msg("notify failed")
				}
				log.Err(err).
					Msg("failed to login to withny, we will try again in 5 minutes")
				ticker.Reset(5 * time.Minute)
				continue
			}
			date, err := c.credentials.GetExpirationTime()
			if err != nil {
				panic(err)
			}
			refreshTime = date.Add(-5 * time.Minute)
			ticker.Reset(time.Until(refreshTime))
		}
	}
}
