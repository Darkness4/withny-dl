package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Darkness4/withny-dl/utils/secret"
	"github.com/Darkness4/withny-dl/withny/api"
	"github.com/stretchr/testify/assert"

	_ "embed"
)

func TestGetPlaylistsRetry(t *testing.T) {
	t.Run("retry until failure", func(t *testing.T) {
		// Arrange
		server := httptest.NewServer(
			http.HandlerFunc(func(res http.ResponseWriter, _ *http.Request) {
				res.WriteHeader(http.StatusInternalServerError)
			}),
		)
		impl := api.NewClient(
			server.Client(),
			secret.CredentialsFromEnv{},
			secret.NewTmpCache(),
		)

		// Act
		playlists, err := impl.GetPlaylists(context.Background(), server.URL, 2)

		// Assert
		assert.ErrorIs(t, err, api.HTTPError{
			Body:   "",
			Status: http.StatusInternalServerError,
			Method: "GET",
			URL:    server.URL,
		})
		assert.Equal(t, 0, len(playlists))
	})

	t.Run("retry until found", func(t *testing.T) {
		// Arrange
		counter := 0
		server := httptest.NewServer(
			http.HandlerFunc(func(res http.ResponseWriter, _ *http.Request) {
				if counter < 3 {
					res.WriteHeader(http.StatusInternalServerError)
				} else {
					_, _ = res.Write([]byte(fixture))
				}
				counter++
			}),
		)
		impl := api.NewClient(
			server.Client(),
			secret.CredentialsFromEnv{},
			secret.NewTmpCache(),
		)

		// Act
		playlists, err := impl.GetPlaylists(context.Background(), server.URL, 4)

		// Assert
		assert.NoError(t, err)
		assert.Equal(t, expectedStreams, playlists)
	})
}
