//go:build contract

package api_test

import (
	"context"
	"net/http"
	"net/http/cookiejar"
	"testing"
	"time"

	"github.com/Darkness4/withny-dl/utils/secret"
	"github.com/Darkness4/withny-dl/withny/api"
	"github.com/joho/godotenv"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"
)

func init() {
	log.Logger = log.Logger.With().Caller().Logger()
	_ = godotenv.Load(".env")
	_ = godotenv.Load(".env.test")
}

func findAnyLiveStream(t *testing.T, client *api.Client) (username string) {
	streams, err := client.GetStreams(context.Background(), "")
	require.NoError(t, err)

	if len(streams) == 0 {
		t.Skip("No live streams found")
	}

	// Find a live stream that is not restricted
	for _, stream := range streams {
		if stream.Price.String() == "0" {
			return stream.Cast.AgencySecret.ChannelName
		}
	}

	return streams[0].Cast.AgencySecret.ChannelName
}

func TestClient(t *testing.T) {
	// Arrange
	jar, err := cookiejar.New(&cookiejar.Options{})
	require.NoError(t, err)
	hclient := &http.Client{Jar: jar, Timeout: time.Minute}
	credReader := &secret.CredentialsFromEnv{}
	saved, _ := credReader.Read()
	client := api.NewClient(hclient, credReader, secret.NewTmpCache())

	t.Run("Login with refresh token", func(t *testing.T) {
		// Act
		newCredentials := api.Credentials{
			LoginResponse: api.LoginResponse{
				Token:        saved.Token,
				RefreshToken: saved.RefreshToken,
				TokenType:    "Bearer",
			},
		}
		client.SetCredentials(newCredentials)

		require.NoError(t, err)
		time.Sleep(2 * time.Second)
		res, err := client.LoginWithRefreshToken(
			context.Background(),
			saved.RefreshToken,
		)

		// Assert
		require.NoError(t, err)
		require.NotEmpty(t, res.Token)
		require.NotEmpty(t, res.RefreshToken)
		require.NotEmpty(t, res.TokenType)
		require.Equal(t, "Bearer", res.TokenType)
		require.NotEmpty(t, res.UserUUID)
		require.NoError(t, err)
		time, err := res.GetExpirationTime()
		require.NoError(t, err)
		require.NotZero(t, time.Time)
	})

	t.Run("Token-based authentication", func(t *testing.T) {
		// Act
		static := secret.Static{
			SavedCredentials: saved,
		}
		client := api.NewClient(hclient, &static, secret.NewTmpCache())
		err = client.Login(context.Background())

		// Assert
		require.NoError(t, err)
	})

	t.Run("Get user", func(t *testing.T) {
		// Act
		const fixture = "admin"
		const expectedUUID = "b4fa8557-7423-4fde-aec0-54775cea6f74"
		err := client.Login(context.Background())
		require.NoError(t, err)

		user, err := client.GetUser(context.Background(), fixture)

		// Assert
		require.NoError(t, err)
		require.NotEmpty(t, user.ID)
		require.NotEmpty(t, user.UUID)
		require.Equal(t, expectedUUID, user.UUID)
		require.NotEmpty(t, user.Username)
		require.Equal(t, fixture, user.Username)
		require.NotEmpty(t, user.Name)
	})

	t.Run("Get streams", func(t *testing.T) {
		// Act
		username := findAnyLiveStream(t, client)
		err := client.Login(context.Background())
		require.NoError(t, err)

		streams, err := client.GetStreams(context.Background(), username)

		// Assert
		require.NoError(t, err)
		require.NotEmpty(t, streams)
		require.Greater(t, len(streams), 0)
	})

	t.Run("Get stream playback URL", func(t *testing.T) {
		err := client.Login(context.Background())
		require.NoError(t, err)

		// Act
		streams, err := client.GetStreams(context.Background(), findAnyLiveStream(t, client))
		require.NoError(t, err)
		require.Greater(t, len(streams), 0)

		playbackURL, err := client.GetStreamPlaybackURL(context.Background(), streams[0].UUID)

		// Assert
		require.NoError(t, err)
		require.NotEmpty(t, playbackURL)
	})

	t.Run("Get playlists", func(t *testing.T) {
		err := client.Login(context.Background())
		require.NoError(t, err)

		// Act
		streams, err := client.GetStreams(context.Background(), findAnyLiveStream(t, client))
		require.NoError(t, err)
		require.Greater(t, len(streams), 0)

		playbackURL, err := client.GetStreamPlaybackURL(context.Background(), streams[0].UUID)
		require.NoError(t, err)

		playlists, err := client.GetPlaylists(context.Background(), playbackURL, 0)

		// Assert
		require.NoError(t, err)
		require.NotEmpty(t, playlists)
		require.Greater(t, len(playlists), 0)
	})
}
